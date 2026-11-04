package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const maxAvatarBytes = 5 << 20

func (s *Server) getUserProfile(w http.ResponseWriter, r *http.Request) {
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "user store unavailable")
		return
	}
	user, err := store.GetUser(r.Context(), claimsFromContext(r.Context()).Subject)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "微信用户不存在，请重新登录")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user profile")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": user})
}

func (s *Server) updateUserProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	input.Nickname = strings.TrimSpace(input.Nickname)
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	if input.Nickname == "" && input.AvatarURL == "" {
		writeError(w, http.StatusBadRequest, "请选择头像或填写昵称")
		return
	}
	if utf8.RuneCountInString(input.Nickname) > 32 {
		writeError(w, http.StatusBadRequest, "昵称不能超过32个字符")
		return
	}
	if input.AvatarURL != "" && !strings.HasPrefix(input.AvatarURL, "/uploads/avatars/") {
		writeError(w, http.StatusBadRequest, "头像地址无效")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "user store unavailable")
		return
	}
	userID := claimsFromContext(r.Context()).Subject
	if err := store.UpdateUserProfile(r.Context(), userID, input.Nickname, input.AvatarURL); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "微信用户不存在，请重新登录")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user profile")
		return
	}
	user, err := store.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user profile")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": user})
}

func (s *Server) uploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+(1<<20))
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		writeError(w, http.StatusBadRequest, "头像不能超过5MB")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择头像")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxAvatarBytes {
		writeError(w, http.StatusBadRequest, "头像不能超过5MB")
		return
	}
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[http.DetectContentType(content)]
	if extension == "" {
		writeError(w, http.StatusBadRequest, "仅支持 JPG、PNG 或 WebP 头像")
		return
	}
	userID := safeFilePart(claimsFromContext(r.Context()).Subject)
	directory := filepath.Join("uploads", "avatars")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare avatar storage")
		return
	}
	name := fmt.Sprintf("%s-%d%s", userID, time.Now().UnixNano(), extension)
	if err := os.WriteFile(filepath.Join(directory, name), content, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save avatar")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": map[string]string{"url": "/uploads/avatars/" + name}})
}

func safeFilePart(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			builder.WriteRune(character)
		}
	}
	if builder.Len() == 0 {
		return "user"
	}
	return builder.String()
}
