package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xiaopi/backend/internal/domain"
)

type temporaryAuthStore interface {
	ListTemporaryAccess(context.Context) ([]domain.TemporaryAccess, error)
	UpsertTemporaryAccess(context.Context, string, string, string, string, time.Time) (domain.TemporaryAccess, error)
	SetTemporaryAccessStatus(context.Context, string, string) error
	LoginTemporaryUser(context.Context, string, string, time.Time) (domain.User, error)
	FindBoundWechatUserByPhone(context.Context, string) (domain.User, error)
}

func (s *Server) temporaryAuthStore() (temporaryAuthStore, bool) {
	store, ok := s.store.(temporaryAuthStore)
	return store, ok
}

func (s *Server) listTemporaryAccess(w http.ResponseWriter, r *http.Request) {
	store, ok := s.temporaryAuthStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "temporary authentication unavailable")
		return
	}
	items, err := store.ListTemporaryAccess(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list temporary access")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) createTemporaryAccess(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone          string `json:"phone"`
		ExpiresInHours int    `json:"expiresInHours"`
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
	if input.ExpiresInHours == 0 {
		input.ExpiresInHours = 168
	}
	if input.ExpiresInHours < 1 || input.ExpiresInHours > 720 {
		writeError(w, http.StatusBadRequest, "有效期必须在 1 至 720 小时之间")
		return
	}
	code, err := temporaryAccessCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate access code")
		return
	}
	store, ok := s.temporaryAuthStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "temporary authentication unavailable")
		return
	}
	if _, err := store.FindBoundWechatUserByPhone(r.Context(), phone); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "该手机号尚未绑定微信用户")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify wechat user")
		return
	}
	expiresAt := time.Now().Add(time.Duration(input.ExpiresInHours) * time.Hour)
	item, err := store.UpsertTemporaryAccess(r.Context(), phone, s.temporaryCodeHash(phone, code), code, claimsFromContext(r.Context()).Subject, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save temporary access")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": map[string]any{"access": item, "accessCode": code}})
}

func (s *Server) updateTemporaryAccessStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone  string `json:"phone"`
		Status string `json:"status"`
	}
	if decodeJSON(w, r, &input) != nil || (input.Status != "active" && input.Status != "revoked") {
		writeError(w, http.StatusBadRequest, "phone and a valid status are required")
		return
	}
	store, ok := s.temporaryAuthStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "temporary authentication unavailable")
		return
	}
	if err := store.SetTemporaryAccessStatus(r.Context(), normalizePhone(input.Phone), input.Status); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "临时访问码不存在")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update temporary access")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"phone": normalizePhone(input.Phone), "status": input.Status}})
}

func (s *Server) temporaryLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone      string `json:"phone"`
		AccessCode string `json:"accessCode"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	phone := normalizePhone(input.Phone)
	code := strings.TrimSpace(input.AccessCode)
	if phone == "" || len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		writeError(w, http.StatusBadRequest, "请输入手机号和 6 位数字访问码")
		return
	}
	store, ok := s.temporaryAuthStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "temporary authentication unavailable")
		return
	}
	authenticatedUser, err := store.LoginTemporaryUser(r.Context(), phone, s.temporaryCodeHash(phone, code), time.Now())
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "手机号或访问码错误，或访问码已过期")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "temporary login failed")
		return
	}
	role := s.wechatUserRole(r.Context(), authenticatedUser.ID)
	token, err := s.tokens.issue(authenticatedUser.ID, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": token, "role": role, "phone": phone, "temporary": true}})
}

func temporaryAccessCode() (string, error) {
	const alphabet = "0123456789"
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index := range random {
		random[index] = alphabet[int(random[index])%len(alphabet)]
	}
	return string(random), nil
}

func temporaryUserID(phone string) string {
	digest := sha256.Sum256([]byte("temporary:" + phone))
	return fmt.Sprintf("tmp-%x", digest[:16])
}

func (s *Server) temporaryCodeHash(phone, code string) string {
	mac := hmac.New(sha256.New, s.tokens.secret)
	_, _ = mac.Write([]byte(phone + "\x00" + strings.ToUpper(code)))
	return hex.EncodeToString(mac.Sum(nil))
}
