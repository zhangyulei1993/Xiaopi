package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"xiaopi/backend/internal/domain"
)

type userAdminStore interface {
	ListUsers(context.Context) ([]domain.User, error)
	BindUserPhone(context.Context, string, string) error
	ListAdminUsers(context.Context) ([]domain.AdminUser, error)
	AssignAdminByPhone(context.Context, string, string) (domain.AdminUser, error)
	SetAdminStatus(context.Context, string, string) error
	DeleteAdminUser(context.Context, string) error
	IsAdminUser(context.Context, string) (bool, error)
}

func (s *Server) userAdminStore() (userAdminStore, bool) {
	store, ok := s.store.(userAdminStore)
	return store, ok
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "user store unavailable")
		return
	}
	users, err := store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": users})
}

func (s *Server) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "admin user store unavailable")
		return
	}
	admins, err := store.ListAdminUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list admin users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": admins})
}

func (s *Server) assignAdminUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone string `json:"phone"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	phone := normalizePhone(input.Phone)
	if len(phone) < 6 || len(phone) > 20 {
		writeError(w, http.StatusBadRequest, "请输入有效手机号")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "admin user store unavailable")
		return
	}
	admin, err := store.AssignAdminByPhone(r.Context(), phone, claimsFromContext(r.Context()).Subject)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "未找到绑定该手机号的微信用户")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign admin user")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": admin})
}

func (s *Server) updateAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID string `json:"userId"`
		Status string `json:"status"`
	}
	if decodeJSON(w, r, &input) != nil || strings.TrimSpace(input.UserID) == "" || (input.Status != "active" && input.Status != "disabled") {
		writeError(w, http.StatusBadRequest, "userId and a valid status are required")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "admin user store unavailable")
		return
	}
	if err := store.SetAdminStatus(r.Context(), input.UserID, input.Status); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "管理员不存在")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update admin status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"userId": input.UserID, "status": input.Status}})
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("userID"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "admin user store unavailable")
		return
	}
	if err := store.DeleteAdminUser(r.Context(), userID); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "管理员不存在")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete admin user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"userId": userID}})
}

func normalizePhone(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsDigit(char) || (char == '+' && builder.Len() == 0) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
