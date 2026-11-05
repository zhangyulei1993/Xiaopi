package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"xiaopi/backend/internal/domain"
)

type Store interface {
	ListProperties(context.Context) ([]domain.Property, error)
	CreateProperty(context.Context, domain.Property) error
	UpdateProperty(context.Context, domain.Property) error
	DeleteProperty(context.Context, string) error
	ListBuildingRules(context.Context, string) ([]domain.BuildingRule, error)
	UpsertBuildingRule(context.Context, domain.BuildingRule) error
	ListPropertyLeads(context.Context) ([]domain.PropertyLead, error)
	CreatePropertyLead(context.Context, domain.PropertyLead) error
	UpdatePropertyLeadFollowUp(context.Context, string, string, string) error
	UpdatePropertyStatus(context.Context, string, string, string, string) error
	ConvertLeadToProperty(context.Context, string, domain.Property) error
	AddPropertyMedia(context.Context, domain.PropertyMedia) error
	ReplacePropertyMedia(context.Context, string, []string) error
	UpsertUser(context.Context, domain.User) error
	ListRentalRequests(context.Context) ([]domain.RentalRequest, error)
	CreateRentalRequest(context.Context, domain.RentalRequest) error
	UpdateRentalRequestFollowUp(context.Context, string, string, string) error
	CreateOperationLog(context.Context, domain.OperationLog) error
	ListOperationLogs(context.Context, int) ([]domain.OperationLog, error)
	ListFavoritePropertyIDs(context.Context, string) ([]string, error)
	SetPropertyFavorite(context.Context, string, string, bool) error
	ListFavoriteContacts(context.Context) ([]domain.FavoriteContact, error)
}

type appConfigStore interface {
	GetAppConfig(context.Context, string) ([]byte, error)
	UpsertAppConfig(context.Context, string, []byte) error
}

const publicConfigKey = "miniapp_public"

type Server struct {
	properties    []domain.Property
	store         Store
	config        Config
	tokens        tokenManager
	httpClient    *http.Client
	staticMapMu   sync.Mutex
	staticMaps    map[string]staticMapCacheEntry
	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt
}

type staticMapCacheEntry struct {
	Body        []byte
	ContentType string
	ExpiresAt   time.Time
}

func NewServer(dataStore Store) *Server { return NewServerWithConfig(dataStore, ConfigFromEnv()) }

func NewServerWithConfig(dataStore Store, config Config) *Server {
	return &Server{
		store:         dataStore,
		config:        config,
		tokens:        newTokenManager(config),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		staticMaps:    map[string]staticMapCacheEntry{},
		loginAttempts: map[string]loginAttempt{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/uploads/", staticMediaCache(http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads")))))
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/v1/admin/login", s.adminLogin)
	mux.HandleFunc("POST /api/v1/auth/wechat/login", s.wechatLogin)
	mux.HandleFunc("POST /api/v1/auth/wechat/phone", s.requireRoles(s.bindWechatPhone, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/auth/profile", s.requireRoles(s.getUserProfile, roleUser, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/auth/profile", s.requireRoles(s.updateUserProfile, roleUser, roleAdmin))
	mux.HandleFunc("POST /api/v1/auth/avatar", s.requireRoles(s.uploadUserAvatar, roleUser, roleAdmin))
	mux.HandleFunc("POST /api/v1/auth/temporary/login", s.temporaryLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.requireRoles(s.refreshAuth, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/subscription-templates", s.subscriptionTemplates)
	mux.HandleFunc("POST /api/v1/message-subscriptions", s.requireRoles(s.registerMessageSubscription, roleUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/users", s.requireRoles(s.listUsers, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/admin-users", s.requireRoles(s.listAdminUsers, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/admin-users", s.requireRoles(s.assignAdminUser, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/admin-users/status", s.requireRoles(s.updateAdminUserStatus, roleAdmin))
	mux.HandleFunc("DELETE /api/v1/admin/admin-users/{userID}", s.requireRoles(s.deleteAdminUser, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/temporary-access", s.requireRoles(s.listTemporaryAccess, roleAdmin))
	mux.HandleFunc("POST /api/v1/admin/temporary-access", s.requireRoles(s.createTemporaryAccess, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/temporary-access/status", s.requireRoles(s.updateTemporaryAccessStatus, roleAdmin))
	mux.HandleFunc("GET /api/v1/config/public", s.publicConfig)
	mux.HandleFunc("PUT /api/v1/admin/config/public", s.requireRoles(s.updatePublicConfig, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/location/search", s.requireRoles(s.searchLocations, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/building-rules", s.requireRoles(s.listBuildingRules, roleAdmin))
	mux.HandleFunc("PUT /api/v1/admin/building-rules", s.requireRoles(s.upsertBuildingRule, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/location/static", s.requireRoles(s.staticLocationMap, roleAdmin))
	mux.HandleFunc("GET /api/v1/properties", s.listProperties)
	mux.HandleFunc("POST /api/v1/properties", s.requireRoles(s.createProperty, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/update", s.requireRoles(s.updateProperty, roleAdmin))
	mux.HandleFunc("DELETE /api/v1/properties/{propertyID}", s.requireRoles(s.deleteProperty, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/status", s.requireRoles(s.updatePropertyStatus, roleAdmin))
	mux.HandleFunc("POST /api/v1/property-leads/convert", s.requireRoles(s.convertPropertyLead, roleAdmin))
	mux.HandleFunc("POST /api/v1/media/upload", s.requireRoles(s.uploadMedia, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/properties/media", s.requireRoles(s.replacePropertyMedia, roleAdmin))
	mux.HandleFunc("GET /api/v1/property-leads", s.requireRoles(s.listPropertyLeads, roleAdmin))
	mux.HandleFunc("POST /api/v1/property-leads", s.createPropertyLead)
	mux.HandleFunc("PATCH /api/v1/property-leads/follow-up", s.requireRoles(s.updatePropertyLeadFollowUp, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/rental-requests", s.requireRoles(s.listRentalRequests, roleAdmin))
	mux.HandleFunc("PATCH /api/v1/admin/rental-requests/follow-up", s.requireRoles(s.updateRentalRequestFollowUp, roleAdmin))
	mux.HandleFunc("GET /api/v1/admin/operation-logs", s.requireRoles(s.listOperationLogs, roleAdmin))
	mux.HandleFunc("POST /api/v1/rental-requests", s.requireRoles(s.createRentalRequest, roleUser))
	mux.HandleFunc("GET /api/v1/favorites", s.requireRoles(s.listFavorites, roleUser))
	mux.HandleFunc("PUT /api/v1/favorites/{propertyID}", s.requireRoles(s.favoriteProperty, roleUser))
	mux.HandleFunc("DELETE /api/v1/favorites/{propertyID}", s.requireRoles(s.unfavoriteProperty, roleUser))
	mux.HandleFunc("GET /api/v1/admin/favorite-contacts", s.requireRoles(s.listFavoriteContacts, roleAdmin))
	mux.HandleFunc("POST /api/v1/footprints/{propertyID}", s.requireRoles(s.recordFootprint, roleUser))
	mux.HandleFunc("GET /api/v1/footprints", s.requireRoles(s.listFootprints, roleUser))
	mux.HandleFunc("DELETE /api/v1/footprints", s.requireRoles(s.clearFootprints, roleUser))
	return logging(cors(mux))
}

// Uploaded filenames are content-addressed by a timestamp. Long browser/CDN caching
// is therefore safe and avoids re-downloading the same listing media on every visit.
func staticMediaCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logOperation(ctx context.Context, operator, targetType, targetID, action, detail string) {
	if s.store == nil {
		return
	}
	id, err := newID()
	if err != nil {
		return
	}
	_ = s.store.CreateOperationLog(ctx, domain.OperationLog{ID: id, OperatorID: operator, TargetType: targetType, TargetID: targetID, Action: action, Detail: detail, CreatedAt: time.Now()})
}

func validFollowUpStatus(status string) bool {
	switch status {
	case "new", "contacted", "matched", "completed", "closed", "invalid", "inspection_pending":
		return true
	}
	return false
}

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

func (s *Server) createRentalRequest(w http.ResponseWriter, r *http.Request) {
	var item domain.RentalRequest
	if decodeJSON(w, r, &item) != nil {
		writeError(w, http.StatusBadRequest, "invalid rental request")
		return
	}
	item.Phone = normalizePhone(item.Phone)
	item.Location = strings.TrimSpace(item.Location)
	item.Layout = strings.TrimSpace(item.Layout)
	item.Note = strings.TrimSpace(item.Note)
	userID := claimsFromContext(r.Context()).Subject
	if item.Phone == "" {
		if store, ok := s.userAdminStore(); ok {
			if user, err := store.GetUser(r.Context(), userID); err == nil {
				item.Phone = normalizePhone(user.Phone)
			}
		}
	}
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
	if (item.RequestType == "appointment" || item.RequestType == "area_subscription") && item.Phone == "" {
		writeError(w, http.StatusBadRequest, "预约服务需要先绑定手机号")
		return
	}
	item.ID, _ = newID()
	item.UserID = userID
	item.Status = "new"
	item.CreatedAt = time.Now()
	if err := s.store.CreateRentalRequest(r.Context(), item); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rental request")
		return
	}
	go s.notifyAdminsOfRequest(context.Background(), item)
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": item})
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
	if err := s.store.SetPropertyFavorite(r.Context(), claimsFromContext(r.Context()).Subject, propertyID, favorite); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"propertyId": propertyID, "favorite": favorite}})
}

func (s *Server) loadPublicConfig(ctx context.Context) (domain.PublicConfig, error) {
	config := domain.DefaultPublicConfig()
	config.ContactPhone = os.Getenv("CONTACT_PHONE")
	configStore, ok := s.store.(appConfigStore)
	if !ok {
		return config, nil
	}
	value, err := configStore.GetAppConfig(ctx, publicConfigKey)
	if errors.Is(err, sql.ErrNoRows) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(value, &config); err != nil {
		return domain.PublicConfig{}, err
	}
	// Keep newly added optional configuration fields usable for configurations
	// that were saved before those fields existed.
	config.Normalize()
	return config, nil
}

func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	config, err := s.loadPublicConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load public config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": config})
}

func (s *Server) updatePublicConfig(w http.ResponseWriter, r *http.Request) {
	var config domain.PublicConfig
	if decodeJSON(w, r, &config) != nil {
		writeError(w, http.StatusBadRequest, "invalid public config")
		return
	}
	config.Normalize()
	if err := config.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	configStore, ok := s.store.(appConfigStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "configuration store unavailable")
		return
	}
	value, err := json.Marshal(config)
	if err != nil || configStore.UpsertAppConfig(r.Context(), publicConfigKey, value) != nil {
		writeError(w, http.StatusInternalServerError, "failed to save public config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": config})
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid request"})
		return
	}
	if !s.config.authConfigured() {
		writeError(w, http.StatusServiceUnavailable, "admin authentication is not configured")
		return
	}
	key := s.loginAttemptKey(r, input.Username)
	if wait, blocked := s.loginBlocked(key); blocked {
		blockedLoginResponse(w, wait)
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

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"status": "ok"}})
}

func (s *Server) wechatLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &input); err != nil || strings.TrimSpace(input.Code) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "code is required"})
		return
	}
	appID, secret := os.Getenv("WECHAT_MINI_APP_ID"), os.Getenv("WECHAT_MINI_APP_SECRET")
	isDevelopmentMock := os.Getenv("APP_ENV") != "production" && (appID == "" || secret == "")
	if os.Getenv("WECHAT_MOCK_ENABLED") == "true" || isDevelopmentMock {
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

func (s *Server) refreshAuth(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	role := roleUser
	if claims.Subject == s.config.AdminUsername && claims.Role == roleAdmin {
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

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	showAll := false
	if claims, ok := s.optionalClaims(r); ok && claims.Role == roleAdmin {
		showAll = true
	}
	if s.store != nil {
		items, err := s.store.ListProperties(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": visibleProperties(items, showAll)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": visibleProperties(s.properties, showAll)})
}

func visibleProperties(items []domain.Property, showAll bool) []domain.Property {
	result := make([]domain.Property, 0, len(items))
	for _, item := range items {
		// "已发布" is an internal workflow state. Only listings explicitly
		// marked as 可租 may leave the management boundary and be returned to
		// ordinary visitors.
		if showAll || item.Status == domain.PropertyAvailable {
			result = append(result, item)
		}
	}
	return result
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var p domain.Property
	if err := decodeJSON(w, r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid property"})
		return
	}
	p.Title, p.Community = strings.TrimSpace(p.Title), strings.TrimSpace(p.Community)
	if p.Title == "" || p.Community == "" || p.RentMonthly < 0 {
		writeError(w, http.StatusBadRequest, "title, community and a non-negative rentMonthly are required")
		return
	}
	propertyID, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create property id")
		return
	}
	p.ID = propertyID
	p.Status = domain.PropertyDraft
	claims := claimsFromContext(r.Context())
	if claims.Role == roleUser {
		p.OwnerID = claims.Subject
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	if s.store != nil {
		if err := s.store.CreateProperty(r.Context(), p); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	} else {
		s.properties = append(s.properties, p)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": p})
}

func (s *Server) updateProperty(w http.ResponseWriter, r *http.Request) {
	var p domain.Property
	if err := decodeJSON(w, r, &p); err != nil || p.ID == "" || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Community) == "" || p.RentMonthly < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "id, title, community and a non-negative rentMonthly are required"})
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	if err := s.store.UpdateProperty(r.Context(), p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": p})
}

// deleteProperty removes a listing after the administrator has confirmed it in
// the client. The store keeps customer appointment records but detaches them
// from the deleted listing, so operational follow-up is not lost.
func (s *Server) deleteProperty(w http.ResponseWriter, r *http.Request) {
	propertyID := strings.TrimSpace(r.PathValue("propertyID"))
	if propertyID == "" || s.store == nil {
		writeError(w, http.StatusBadRequest, "propertyId is required")
		return
	}
	items, err := s.store.ListProperties(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load property")
		return
	}
	var property *domain.Property
	for index := range items {
		if items[index].ID == propertyID {
			property = &items[index]
			break
		}
	}
	if property == nil {
		writeError(w, http.StatusNotFound, "房源不存在")
		return
	}
	if err := s.store.DeleteProperty(r.Context(), propertyID); err != nil {
		writeError(w, http.StatusInternalServerError, "删除房源失败")
		return
	}
	for _, media := range property.Media {
		s.removeStoredUpload(media.URL)
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "property", propertyID, "删除房源", fmt.Sprintf("%s（删除前状态：%s）", property.Title, property.Status))
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

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
	if claims, ok := s.optionalClaims(r); ok && (claims.Role == roleAdmin || s.wechatUserRole(r.Context(), claims.Subject) == roleAdmin) {
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
	// A public rental listing without a cover image is not usable: it renders as
	// a placeholder in the discovery feed and undermines the verification claim.
	if input.Status == string(domain.PropertyPublished) || input.Status == string(domain.PropertyAvailable) {
		items, err := s.store.ListProperties(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify property media")
			return
		}
		for _, item := range items {
			if item.ID == input.PropertyID {
				hasImage := false
				for _, media := range item.Media {
					if media.Type == "image" {
						hasImage = true
						break
					}
				}
				if !hasImage {
					writeError(w, http.StatusBadRequest, "发布房源前请至少上传一张实拍图片")
					return
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
	r.Body = http.MaxBytesReader(w, r.Body, (20<<20)+(1<<20))
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid multipart form"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "file is required"})
		return
	}
	defer file.Close()
	if header.Size > 20<<20 {
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
