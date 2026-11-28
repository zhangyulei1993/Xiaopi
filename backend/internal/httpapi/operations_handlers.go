package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
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
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "该房源线索已不存在，请刷新列表")
			return
		}
		if errors.Is(err, domain.ErrLeadFollowUpConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "房源线索跟进保存失败")
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
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "该租客需求已不存在，请刷新列表")
			return
		}
		writeError(w, http.StatusInternalServerError, "租客需求跟进保存失败")
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

func (s *Server) queryOperationLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	category := strings.TrimSpace(query.Get("category"))
	if category == "" {
		category = "all"
	}
	if category != "all" && category != "property" && category != "customer" && category != "message" {
		writeError(w, http.StatusBadRequest, "invalid operation log category")
		return
	}
	page := 1
	pageSize := 20
	if value := query.Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 1_000_000 {
			writeError(w, http.StatusBadRequest, "invalid page")
			return
		}
		page = parsed
	}
	if value := query.Get("pageSize"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid page size")
			return
		}
		pageSize = parsed
	}
	startDate := strings.TrimSpace(query.Get("startDate"))
	endDate := strings.TrimSpace(query.Get("endDate"))
	for _, value := range []string{startDate, endDate} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			writeError(w, http.StatusBadRequest, "invalid date range")
			return
		}
	}
	if startDate != "" && endDate != "" && startDate > endDate {
		writeError(w, http.StatusBadRequest, "start date must not exceed end date")
		return
	}
	items, total, err := s.store.ListOperationLogsPage(r.Context(), category, startDate, endDate, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query operation logs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{
		"items": items, "total": total, "page": page, "pageSize": pageSize, "pageCount": (total + pageSize - 1) / pageSize,
	}})
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

func (s *Server) queryContactEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, pageSize := 1, 20
	for _, field := range []struct {
		name  string
		value *int
		max   int
	}{{"page", &page, 1_000_000}, {"pageSize", &pageSize, 100}} {
		if raw := query.Get(field.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > field.max {
				writeError(w, http.StatusBadRequest, "分页参数不正确")
				return
			}
			*field.value = value
		}
	}
	store, ok := s.store.(interface {
		ListContactEventsPage(context.Context, int, int) ([]domain.ContactEvent, int, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "联系记录分页暂不可用")
		return
	}
	items, total, err := store.ListContactEventsPage(r.Context(), pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "联系记录读取失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{
		"items": items, "total": total, "page": page, "pageSize": pageSize,
	}})
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
	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create request id")
		return
	}
	item.ID = id
	store, atomicAppointment := s.store.(interface {
		CreateAppointmentOnce(context.Context, domain.RentalRequest) (*domain.RentalRequest, error)
	})
	if item.RequestType == "appointment" && atomicAppointment {
		existing, err := store.CreateAppointmentOnce(r.Context(), item)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "可租房源不存在或已下架")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create appointment")
			return
		}
		if existing != nil {
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "appointment already submitted", "data": existing})
			return
		}
	} else {
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
		if err := s.store.CreateRentalRequest(r.Context(), item); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create rental request")
			return
		}
	}
	if !s.hasSubscriptionOutbox() {
		go s.notifyAdminsOfRequest(context.Background(), item, 0)
	}
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
	type customerRequest struct {
		ID                string    `json:"id"`
		RequestType       string    `json:"requestType"`
		PropertyID        string    `json:"propertyId,omitempty"`
		PropertyTitle     string    `json:"propertyTitle,omitempty"`
		PropertyAvailable bool      `json:"propertyAvailable"`
		ContactName       string    `json:"contactName"`
		Phone             string    `json:"phone"`
		PreferredTime     string    `json:"preferredTime"`
		Location          string    `json:"location"`
		Layout            string    `json:"layout"`
		BudgetMin         int64     `json:"budgetMin"`
		BudgetMax         int64     `json:"budgetMax"`
		MoveInDate        string    `json:"moveInDate"`
		Note              string    `json:"note"`
		Status            string    `json:"status"`
		CreatedAt         time.Time `json:"createdAt"`
	}
	result := make([]customerRequest, 0, len(items))
	for _, item := range items {
		result = append(result, customerRequest{
			ID: item.ID, RequestType: item.RequestType, PropertyID: item.PropertyID, PropertyTitle: item.PropertyTitle,
			PropertyAvailable: item.PropertyAvailable, ContactName: item.ContactName, Phone: item.Phone,
			PreferredTime: item.PreferredTime, Location: item.Location, Layout: item.Layout,
			BudgetMin: item.BudgetMin, BudgetMax: item.BudgetMax, MoveInDate: item.MoveInDate, Note: item.Note,
			Status: item.Status, CreatedAt: item.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": result})
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
