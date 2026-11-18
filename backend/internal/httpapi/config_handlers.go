package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"xiaopi/backend/internal/domain"
)

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
	w.Header().Set("Cache-Control", "no-store")
	config, err := s.loadPublicConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load public config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": config})
}

// Compatibility endpoint: old clients must also supply a revision; no write
// path may bypass the publish audit and concurrency check.
func (s *Server) updatePublicConfig(w http.ResponseWriter, r *http.Request) {
	var config domain.PublicConfig
	if decodeJSON(w, r, &config) != nil {
		writeError(w, 400, "invalid public config")
		return
	}
	expected := strings.Trim(r.Header.Get("If-Match"), "\"")
	if expected == "" {
		writeError(w, 428, "请使用新版发布确认清单，或提供 If-Match 配置版本")
		return
	}
	config.Normalize()
	if err := config.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	store, ok := s.store.(configPublicationStore)
	if !ok {
		writeError(w, 503, "发布存储不可用")
		return
	}
	entry, err := store.PublishPublicConfig(r.Context(), expected, config, claimsFromContext(r.Context()).Subject, "")
	if errors.Is(err, domain.ErrConfigConflict) {
		writeError(w, 409, "配置已更新，请重新读取")
		return
	}
	if err != nil {
		writeError(w, 500, "未确认发布成功，请刷新核对")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": entry.After})
}
