package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"xiaopi/backend/internal/domain"
)

func (s *Server) listPropertyLeads(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": []domain.PropertyLead{}})
		return
	}
	items, err := s.store.ListPropertyLeads(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) createPropertyLead(w http.ResponseWriter, r *http.Request) {
	if claims, ok := s.optionalClaims(r); ok && s.effectiveRole(r.Context(), claims) == roleAdmin {
		writeError(w, http.StatusForbidden, "管理员无需提交房源线索")
		return
	}
	var lead domain.PropertyLead
	if err := decodeJSON(w, r, &lead); err != nil || strings.TrimSpace(lead.Community) == "" || lead.ExpectedRentMonthly < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "community is required"})
		return
	}
	leadID, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create lead id")
		return
	}
	lead.ID = leadID
	lead.Status = domain.LeadNew
	lead.CreatedAt = time.Now()
	lead.UpdatedAt = lead.CreatedAt
	if s.store != nil {
		if err := s.store.CreatePropertyLead(r.Context(), lead); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	} else {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": lead})
}

func (s *Server) updatePropertyStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PropertyID string `json:"propertyId"`
		Status     string `json:"status"`
		OperatorID string `json:"operatorId"`
		Note       string `json:"note"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.PropertyID == "" || !domain.IsPropertyStatus(input.Status) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "propertyId and status are required"})
		return
	}
	input.OperatorID = claimsFromContext(r.Context()).Subject
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	previousStatus := "未知"
	propertyName := input.PropertyID
	if properties, err := s.store.ListProperties(r.Context()); err == nil {
		for _, property := range properties {
			if property.ID == input.PropertyID {
				previousStatus = string(property.Status)
				propertyName = property.Community
				if propertyName == "" {
					propertyName = property.Title
				}
				break
			}
		}
	}
	if err := s.store.UpdatePropertyStatus(r.Context(), input.PropertyID, input.Status, input.OperatorID, input.Note); err != nil {
		if errors.Is(err, domain.ErrInvalidPropertyStatusTransition) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	s.logOperation(r.Context(), input.OperatorID, "property", input.PropertyID, "变更房源状态", fmt.Sprintf("%s：%s → %s", propertyName, previousStatus, input.Status))
	if input.Status == string(domain.PropertyPublished) || input.Status == string(domain.PropertyAvailable) {
		if items, err := s.store.ListProperties(r.Context()); err == nil {
			for _, item := range items {
				if item.ID == input.PropertyID {
					go s.notifyAreaSubscribers(context.Background(), item)
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func (s *Server) replacePropertyMedia(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PropertyID string   `json:"propertyId"`
		MediaIDs   []string `json:"mediaIds"`
	}
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.PropertyID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "propertyId is required"})
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	var previous []domain.PropertyMedia
	if properties, err := s.store.ListProperties(r.Context()); err == nil {
		for _, property := range properties {
			if property.ID == input.PropertyID {
				previous = property.Media
				break
			}
		}
	}
	if err := s.store.ReplacePropertyMedia(r.Context(), input.PropertyID, input.MediaIDs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	kept := make(map[string]struct{}, len(input.MediaIDs))
	for _, id := range input.MediaIDs {
		kept[id] = struct{}{}
	}
	for _, media := range previous {
		if _, ok := kept[media.ID]; !ok {
			s.removeStoredUpload(media.URL)
		}
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "property", input.PropertyID, "更新房源素材", fmt.Sprintf("保留 %d 个素材", len(input.MediaIDs)))
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func mediaStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("FILE_STORAGE_LOCAL_PATH")); dir != "" {
		return dir
	}
	return "uploads"
}

func (s *Server) removeStoredUpload(url string) {
	s.deleteCOSObject(context.Background(), url)
	const prefix = "/uploads/"
	if !strings.HasPrefix(url, prefix) {
		return
	}
	name := filepath.Base(strings.TrimPrefix(url, prefix))
	if name == "." || name == string(filepath.Separator) {
		return
	}
	_ = os.Remove(filepath.Join(mediaStorageDir(), name))
}

func (s *Server) convertPropertyLead(w http.ResponseWriter, r *http.Request) {
	var input struct {
		LeadID      string `json:"leadId"`
		Title       string `json:"title"`
		Community   string `json:"community"`
		Location    string `json:"location"`
		Layout      string `json:"layout"`
		RentMonthly int64  `json:"rentMonthly"`
		FeeText     string `json:"feeText"`
		Description string `json:"description"`
		OwnerID     string `json:"ownerId"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.LeadID == "" || strings.TrimSpace(input.Community) == "" || input.RentMonthly < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "leadId and community are required"})
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	propertyID, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create property id")
		return
	}
	now := time.Now()
	p := domain.Property{ID: propertyID, Title: input.Title, Community: input.Community, Location: input.Location, Layout: input.Layout, RentMonthly: input.RentMonthly, FeeText: input.FeeText, Description: input.Description, Status: domain.PropertyDraft, OwnerID: input.OwnerID, CreatedAt: now, UpdatedAt: now}
	if err := s.store.ConvertLeadToProperty(r.Context(), input.LeadID, p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": p})
}

func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
	const maxMediaUploadBytes = 40 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(maxMediaUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid multipart form"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "file is required"})
		return
	}
	defer file.Close()
	if header.Size > maxMediaUploadBytes {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "file too large"})
		return
	}
	buffer := make([]byte, 512)
	read, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "failed to inspect file")
		return
	}
	contentType := http.DetectContentType(buffer[:read])
	allowedTypes := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "video/mp4": true}
	if !allowedTypes[contentType] {
		writeError(w, http.StatusUnsupportedMediaType, "only JPEG, PNG, WebP and MP4 files are supported")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process file")
		return
	}
	name := fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(header.Filename))
	mediaPath := "/uploads/" + name
	localPath := ""
	if s.config.cosConfigured() {
		objectKey := "media/" + time.Now().Format("2006/01/02/") + name
		url, uploadErr := s.putCOSObject(r.Context(), objectKey, contentType, file)
		if uploadErr != nil {
			writeError(w, http.StatusBadGateway, "上传 COS 失败，请稍后重试")
			return
		}
		mediaPath = url
	} else {
		dir := mediaStorageDir()
		if err := os.MkdirAll(dir, 0755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		localPath = filepath.Join(dir, name)
		out, createErr := os.Create(localPath)
		if createErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": createErr.Error()})
			return
		}
		defer out.Close()
		if _, copyErr := io.Copy(out, file); copyErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": copyErr.Error()})
			return
		}
	}
	mediaID := ""
	mediaType := ""
	sortOrder := 0
	propertyID := r.FormValue("propertyId")
	if propertyID != "" && s.store != nil {
		var idErr error
		mediaID, idErr = newID()
		if idErr != nil {
			if localPath != "" {
				_ = os.Remove(localPath)
			} else {
				s.removeStoredUpload(mediaPath)
			}
			writeError(w, http.StatusInternalServerError, "failed to create media id")
			return
		}
		mediaType = "image"
		if strings.HasPrefix(contentType, "video/") {
			mediaType = "video"
		}
		sortOrder, _ = strconv.Atoi(r.FormValue("sort"))
		if sortOrder < 0 {
			sortOrder = 0
		}
		if err := s.store.AddPropertyMedia(r.Context(), domain.PropertyMedia{ID: mediaID, PropertyID: propertyID, Type: mediaType, URL: mediaPath, Sort: sortOrder}); err != nil {
			if localPath != "" {
				_ = os.Remove(localPath)
			} else {
				s.removeStoredUpload(mediaPath)
			}
			writeError(w, http.StatusInternalServerError, "failed to save media")
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": map[string]any{"id": mediaID, "propertyId": propertyID, "type": mediaType, "url": mediaPath, "sort": sortOrder, "name": name, "path": mediaPath}})
}
