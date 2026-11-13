package httpapi

import (
	"net/http"
	"os"
	"strings"

	"xiaopi/backend/internal/domain"
)

// testWechatLogin only accepts server-configured synthetic codes and derives
// the role from the test database. The code path is unreachable outside APP_ENV=test.
func (s *Server) testWechatLogin(w http.ResponseWriter, r *http.Request, code string) {
	accounts := make(map[string]string)
	for _, entry := range strings.Split(os.Getenv("TEST_WECHAT_CODE_MAP"), ",") {
		alias, userID, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if ok && alias != "" && strings.HasPrefix(userID, "qa-") {
			accounts[alias] = userID
		}
	}
	userID, allowed := accounts[code]
	if !allowed {
		writeError(w, http.StatusUnauthorized, "unknown test account")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "test user store unavailable")
		return
	}
	user, err := store.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "test account is not seeded")
		return
	}
	if user.Status != domain.UserActive {
		writeError(w, http.StatusForbidden, "test account is disabled")
		return
	}
	role := s.wechatUserRole(r.Context(), user.ID)
	token, err := s.tokens.issue(user.ID, role)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "test token signing is not configured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": token, "role": role, "phone": user.Phone, "isDevMock": true, "testOnly": true}})
}
