package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"xiaopi/backend/internal/domain"
	"xiaopi/backend/internal/store"
)

type Server struct {
	properties []domain.Property
	store      *store.MySQL
}

func NewServer(mysqlStore *store.MySQL) *Server { return &Server{store: mysqlStore} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/v1/admin/login", s.adminLogin)
	mux.HandleFunc("POST /api/v1/auth/wechat/login", s.wechatLogin)
	mux.HandleFunc("GET /api/v1/properties", s.listProperties)
	mux.HandleFunc("POST /api/v1/properties", s.createProperty)
	mux.HandleFunc("PATCH /api/v1/properties/update", s.updateProperty)
	mux.HandleFunc("PATCH /api/v1/properties/status", s.updatePropertyStatus)
	mux.HandleFunc("POST /api/v1/property-leads/convert", s.convertPropertyLead)
	mux.HandleFunc("POST /api/v1/media/upload", s.uploadMedia)
	mux.HandleFunc("GET /api/v1/property-leads", s.listPropertyLeads)
	mux.HandleFunc("POST /api/v1/property-leads", s.createPropertyLead)
	return logging(cors(mux))
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid request"})
		return
	}
	username, password := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD")
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "xiaopi_admin_2026"
	}
	if input.Username != username || input.Password != password {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "账号或密码错误"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"accessToken": signToken(username), "username": username}})
}

func signToken(subject string) string {
	secret := os.Getenv("JWT_ADMIN_SECRET")
	if secret == "" {
		secret = "xiaopi-admin-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(subject))
	return base64.RawURLEncoding.EncodeToString([]byte(subject + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))))
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "code is required"})
		return
	}
	appID, secret := os.Getenv("WECHAT_MINI_APP_ID"), os.Getenv("WECHAT_MINI_APP_SECRET")
	if os.Getenv("WECHAT_MOCK_ENABLED") == "true" || appID == "" || secret == "" {
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": "xiaopi-dev-token", "isDevMock": true}})
		return
	}
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?" + url.Values{"appid": {appID}, "secret": {secret}, "js_code": {input.Code}, "grant_type": {"authorization_code"}}.Encode()
	resp, err := http.Get(endpoint)
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
	userID := fmt.Sprintf("wx-%x", result.OpenID)
	if s.store != nil {
		if err := s.store.UpsertUser(r.Context(), domain.User{ID: userID, OpenID: result.OpenID, Status: domain.UserActive}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": "xiaopi-user-" + userID, "isDevMock": false}})
}

func (s *Server) listProperties(w http.ResponseWriter, _ *http.Request) {
	if s.store != nil {
		items, err := s.store.ListProperties(context.Background())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": s.properties})
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var p domain.Property
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid property"})
		return
	}
	p.ID = time.Now().Format("20060102150405.000000000")
	p.Status = domain.PropertyDraft
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
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "id is required"})
		return
	}
	if err := s.store.UpdateProperty(r.Context(), p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": p})
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
	var lead domain.PropertyLead
	if err := json.NewDecoder(r.Body).Decode(&lead); err != nil || lead.Community == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "community is required"})
		return
	}
	lead.ID = time.Now().Format("20060102150405.000000000")
	lead.Status = domain.LeadNew
	lead.CreatedAt = time.Now()
	lead.UpdatedAt = lead.CreatedAt
	if s.store != nil {
		if err := s.store.CreatePropertyLead(r.Context(), lead); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.PropertyID == "" || input.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "propertyId and status are required"})
		return
	}
	if s.store != nil {
		if err := s.store.UpdatePropertyStatus(r.Context(), input.PropertyID, input.Status, input.OperatorID, input.Note); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
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
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.LeadID == "" || input.Community == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "leadId and community are required"})
		return
	}
	now := time.Now()
	p := domain.Property{ID: now.Format("20060102150405.000000000"), Title: input.Title, Community: input.Community, Location: input.Location, Layout: input.Layout, RentMonthly: input.RentMonthly, FeeText: input.FeeText, Description: input.Description, Status: domain.PropertyDraft, OwnerID: input.OwnerID, CreatedAt: now, UpdatedAt: now}
	if err := s.store.ConvertLeadToProperty(r.Context(), input.LeadID, p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": p})
}

func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
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
	dir := os.Getenv("FILE_STORAGE_LOCAL_PATH")
	if dir == "" {
		dir = "uploads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	name := fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(header.Filename))
	path := filepath.Join(dir, name)
	out, err := os.Create(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	mediaPath := "/uploads/" + name
	if propertyID := r.FormValue("propertyId"); propertyID != "" && s.store != nil {
		_ = s.store.AddPropertyMedia(r.Context(), domain.PropertyMedia{ID: fmt.Sprintf("%d", time.Now().UnixNano()), PropertyID: propertyID, Type: r.FormValue("mediaType"), URL: mediaPath, Sort: 0})
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": map[string]string{"name": name, "path": mediaPath}})
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
