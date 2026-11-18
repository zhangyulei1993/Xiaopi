package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"xiaopi/backend/internal/domain"
)

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid request"})
		return
	}
	if s.webAdminDB() == nil && !s.config.authConfigured() {
		writeError(w, http.StatusServiceUnavailable, "admin authentication is not configured")
		return
	}
	key := s.loginAttemptKey(r, input.Username)
	if wait, blocked := s.loginBlocked(key); blocked {
		blockedLoginResponse(w, wait)
		return
	}
	if db := s.webAdminDB(); db != nil {
		s.loginWebAdmin(w, r, db, input.Username, input.Password, key)
		return
	}
	if !secureEqual(input.Username, s.config.AdminUsername) || !secureEqual(input.Password, s.config.AdminPassword) {
		s.recordLoginFailure(key)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "账号或密码错误"})
		return
	}
	s.clearLoginFailures(key)
	token, err := s.tokens.issue(s.config.AdminUsername, roleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"accessToken": token, "username": s.config.AdminUsername}})
}

func (s *Server) wechatLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.Code) == "" {
		if os.Getenv("APP_ENV") == "test" {
			writeError(w, http.StatusBadRequest, "test mode requires a synthetic test credential")
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "code is required"})
		return
	}
	if os.Getenv("APP_ENV") == "test" {
		code := strings.TrimSpace(input.Code)
		if !strings.HasPrefix(code, "qa-") || len(code) > 64 {
			writeError(w, http.StatusUnauthorized, "test mode does not allow real WeChat authentication")
			return
		}
		s.testWechatLogin(w, r, code)
		return
	}
	appID, secret := os.Getenv("WECHAT_MINI_APP_ID"), os.Getenv("WECHAT_MINI_APP_SECRET")
	isDevelopmentMock := os.Getenv("APP_ENV") != "production" &&
		(os.Getenv("WECHAT_MOCK_ENABLED") == "true" || appID == "" || secret == "")
	if isDevelopmentMock {
		if len(s.tokens.secret) == 0 {
			writeError(w, http.StatusServiceUnavailable, "user authentication is not configured")
			return
		}
		userID := "dev-user"
		userRole := s.wechatUserRole(r.Context(), userID)
		token, _ := s.tokens.issue(userID, userRole)
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": token, "role": userRole, "isDevMock": true}})
		return
	}
	if appID == "" || secret == "" || len(s.tokens.secret) == 0 {
		writeError(w, http.StatusServiceUnavailable, "wechat authentication is not configured")
		return
	}
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?" + url.Values{"appid": {appID}, "secret": {secret}, "js_code": {input.Code}, "grant_type": {"authorization_code"}}.Encode()
	resp, err := s.httpClient.Get(endpoint)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"code": 502, "message": "wechat service unavailable"})
		return
	}
	defer resp.Body.Close()
	var result struct {
		OpenID     string `json:"openid"`
		SessionKey string `json:"session_key"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.OpenID == "" {
		writeJSON(w, http.StatusBadGateway, map[string]any{"code": 502, "message": result.ErrMsg})
		return
	}
	userID := wechatUserID(result.OpenID)
	if s.store != nil {
		if err := s.store.UpsertUser(r.Context(), domain.User{ID: userID, OpenID: result.OpenID, Status: domain.UserActive}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	}
	userRole := s.wechatUserRole(r.Context(), userID)
	token, err := s.tokens.issue(userID, userRole)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": token, "role": userRole, "isDevMock": false}})
}

func wechatUserID(openID string) string {
	digest := sha256.Sum256([]byte(openID))
	return fmt.Sprintf("wx-%x", digest[:16])
}

func (s *Server) wechatUserRole(ctx context.Context, userID string) string {
	store, ok := s.userAdminStore()
	if !ok {
		return roleUser
	}
	isAdmin, err := store.IsAdminUser(ctx, userID)
	if err == nil && isAdmin {
		return roleAdmin
	}
	return roleUser
}

func (s *Server) boundUserPhone(ctx context.Context, userID string) (string, bool) {
	store, ok := s.userAdminStore()
	if !ok {
		return "", false
	}
	user, err := store.GetUser(ctx, userID)
	if err != nil {
		return "", false
	}
	phone := normalizePhone(user.Phone)
	return phone, phone != ""
}

func (s *Server) refreshAuth(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	role := roleUser
	if strings.HasPrefix(claims.Subject, webAdminPrefix) || (claims.Subject == s.config.AdminUsername && claims.Role == roleAdmin) {
		role = roleAdmin
	} else {
		store, ok := s.userAdminStore()
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "user store unavailable")
			return
		}
		if _, err := store.GetUser(r.Context(), claims.Subject); errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "登录账号已失效，请重新登录")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify user")
			return
		}
		role = s.wechatUserRole(r.Context(), claims.Subject)
	}
	token, err := s.tokens.issue(claims.Subject, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to refresh access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"accessToken": token, "role": role}})
}
