package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"
	"xiaopi/backend/internal/domain"
)

func (s *Server) updatePropertyLeadFollowUp(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		FollowUpNote string `json:"followUpNote"`
	}
	if decodeJSON(w, r, &input) != nil || input.ID == "" || !validFollowUpStatus(input.Status) {
		writeError(w, http.StatusBadRequest, "invalid follow-up")
		return
	}
	if err := s.store.UpdatePropertyLeadFollowUp(r.Context(), input.ID, input.Status, strings.TrimSpace(input.FollowUpNote)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update lead")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "lead", input.ID, "更新客户线索", input.Status)
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func (s *Server) updateRentalRequestFollowUp(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		FollowUpNote string `json:"followUpNote"`
	}
	if decodeJSON(w, r, &input) != nil || input.ID == "" || !validFollowUpStatus(input.Status) {
		writeError(w, http.StatusBadRequest, "invalid follow-up")
		return
	}
	if err := s.store.UpdateRentalRequestFollowUp(r.Context(), input.ID, input.Status, strings.TrimSpace(input.FollowUpNote)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update request")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "request", input.ID, "更新客户需求", input.Status)
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func (s *Server) listOperationLogs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListOperationLogs(r.Context(), 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list operation logs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) listRentalRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRentalRequests(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rental requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) listContactEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListContactEvents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list contact events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) createContactEvent(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PropertyID string `json:"propertyId"`
		Type       string `json:"type"`
		Source     string `json:"source"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid contact event")
		return
	}
	input.PropertyID = strings.TrimSpace(input.PropertyID)
	input.Type = strings.TrimSpace(input.Type)
	input.Source = strings.TrimSpace(input.Source)
	if input.PropertyID == "" || (input.Type != "consult_entry_click" && input.Type != "phone_entry_click") {
		writeError(w, http.StatusBadRequest, "invalid contact event")
		return
	}
	if !s.isAvailableProperty(r.Context(), input.PropertyID) {
		writeError(w, http.StatusNotFound, "可租房源不存在或已下架")
		return
	}
	if input.Source == "" {
		input.Source = "detail"
	}
	eventID, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create contact event")
		return
	}
	created, err := s.store.CreateContactEvent(r.Context(), domain.ContactEvent{
		ID:         eventID,
		UserID:     claimsFromContext(r.Context()).Subject,
		PropertyID: input.PropertyID,
		Type:       input.Type,
		Source:     input.Source,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record contact event")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": map[string]any{"recorded": created}})
}

func (s *Server) createRentalRequest(w http.ResponseWriter, r *http.Request) {
	var item domain.RentalRequest
	if decodeJSON(w, r, &item) != nil {
		writeError(w, http.StatusBadRequest, "invalid rental request")
		return
	}
	item.RequestType = strings.TrimSpace(item.RequestType)
	item.ContactName = strings.TrimSpace(item.ContactName)
	item.Phone = normalizePhone(item.Phone)
	item.PreferredTime = strings.TrimSpace(item.PreferredTime)
	item.Location = strings.TrimSpace(item.Location)
	item.Layout = strings.TrimSpace(item.Layout)
	item.Note = strings.TrimSpace(item.Note)
	userID := claimsFromContext(r.Context()).Subject
	if item.BudgetMin < 0 || item.BudgetMax < item.BudgetMin || (item.RequestType != "general" && item.RequestType != "appointment" && item.RequestType != "area_subscription") {
		writeError(w, http.StatusBadRequest, "请填写有效的找房需求")
		return
	}
	if item.RequestType == "appointment" && item.PropertyID == "" {
		writeError(w, http.StatusBadRequest, "预约房源不能为空")
		return
	}
	if item.RequestType == "area_subscription" && item.Location == "" {
		writeError(w, http.StatusBadRequest, "订阅片区不能为空")
		return
	}
	if item.RequestType == "appointment" {
		property, ok := s.findProperty(r.Context(), item.PropertyID)
		if !ok || property.Status != domain.PropertyAvailable {
			writeError(w, http.StatusNotFound, "可租房源不存在或已下架")
			return
		}
		item.PropertyTitle = strings.TrimSpace(property.Community)
		if item.PropertyTitle == "" {
			item.PropertyTitle = strings.TrimSpace(property.Title)
		}
	}
	if item.RequestType == "appointment" || item.RequestType == "area_subscription" || item.RequestType == "general" {
		phone, ok := s.boundUserPhone(r.Context(), userID)
		if !ok {
			writeError(w, http.StatusBadRequest, "提交找房需求需要先绑定手机号")
			return
		}
		// Never accept an arbitrary number from the client for a high-intent
		// request: the verified WeChat-bound number is the contact of record.
		item.Phone = phone
	}
	if item.RequestType == "appointment" && item.ContactName == "" {
		writeError(w, http.StatusBadRequest, "请填写怎么称呼您")
		return
	}
	if item.RequestType == "appointment" && item.PreferredTime == "" {
		writeError(w, http.StatusBadRequest, "请选择方便看房的时间")
		return
	}
	item.UserID = userID
	item.Status = "new"
	item.CreatedAt = time.Now()
	// Protect against double taps and network retries. The store lookup also
	// covers a retried request after the process has restarted.
	if item.RequestType == "appointment" {
		s.appointmentMu.Lock()
		defer s.appointmentMu.Unlock()
		current, err := s.store.FindRecentAppointment(r.Context(), userID, item.PropertyID, item.CreatedAt.Add(-30*time.Minute))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check appointment")
			return
		}
		if current != nil {
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "appointment already submitted", "data": current})
			return
		}
	}
	item.ID, _ = newID()
	if err := s.store.CreateRentalRequest(r.Context(), item); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rental request")
		return
	}
	go s.notifyAdminsOfRequest(context.Background(), item)
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": item})
}

// listMyRentalRequests deliberately scopes records at the store boundary. A
// customer can review progress without ever receiving another customer's
// phone number, note, or follow-up history.
func (s *Server) listMyRentalRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRentalRequestsByUser(r.Context(), claimsFromContext(r.Context()).Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rental requests")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) listFavoriteContacts(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListFavoriteContacts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list favorite contacts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) listFavorites(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListFavoritePropertyIDs(r.Context(), claimsFromContext(r.Context()).Subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list favorites")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) favoriteProperty(w http.ResponseWriter, r *http.Request) { s.setFavorite(w, r, true) }

func (s *Server) unfavoriteProperty(w http.ResponseWriter, r *http.Request) {
	s.setFavorite(w, r, false)
}

func (s *Server) setFavorite(w http.ResponseWriter, r *http.Request, favorite bool) {
	propertyID := strings.TrimSpace(r.PathValue("propertyID"))
	if propertyID == "" {
		writeError(w, http.StatusBadRequest, "propertyId is required")
		return
	}
	if favorite && !s.isAvailableProperty(r.Context(), propertyID) {
		writeError(w, http.StatusNotFound, "可租房源不存在或已下架")
		return
	}
	if err := s.store.SetPropertyFavorite(r.Context(), claimsFromContext(r.Context()).Subject, propertyID, favorite); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"propertyId": propertyID, "favorite": favorite}})
}
