package httpapi

import (
	"crypto/subtle"
	"net/http"
	"os"
	"regexp"
	"strings"

	"xiaopi/backend/internal/domain"
)

var testAccessPhonePattern = regexp.MustCompile(`^\+?\d{6,20}$`)
var testAccessCodePattern = regexp.MustCompile(`^\d{6}$`)

// testAccessLogin is registered only by the isolated test server. Each entry in
// TEST_ACCESS_CODE_MAP uses phone:code=userID and is checked against the seeded
// user's phone before a test token can be issued.
func (s *Server) testAccessLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone      string `json:"phone"`
		AccessCode string `json:"accessCode"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "请输入测试手机号和访问码")
		return
	}
	phone := normalizePhone(input.Phone)
	code := strings.TrimSpace(input.AccessCode)
	if !testAccessPhonePattern.MatchString(phone) || !testAccessCodePattern.MatchString(code) {
		writeError(w, http.StatusBadRequest, "请输入有效手机号和 6 位数字访问码")
		return
	}
	key := s.loginAttemptKey(r, "test-access:"+phone)
	if wait, blocked := s.loginBlocked(key); blocked {
		blockedLoginResponse(w, wait)
		return
	}
	userID := ""
	for _, entry := range strings.Split(os.Getenv("TEST_ACCESS_CODE_MAP"), ",") {
		credential, candidateID, ok := strings.Cut(strings.TrimSpace(entry), "=")
		candidatePhone, candidateCode, credentialOK := strings.Cut(credential, ":")
		if !ok || !credentialOK || normalizePhone(candidatePhone) != phone || !strings.HasPrefix(candidateID, "qa-") {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidateCode), []byte(code)) == 1 {
			userID = candidateID
			break
		}
	}
	if userID == "" {
		s.recordLoginFailure(key)
		writeError(w, http.StatusUnauthorized, "测试手机号或访问码错误")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "测试账号暂不可用")
		return
	}
	user, err := store.GetUser(r.Context(), userID)
	if err != nil || normalizePhone(user.Phone) != phone {
		s.recordLoginFailure(key)
		writeError(w, http.StatusUnauthorized, "测试账号未初始化或手机号不匹配")
		return
	}
	if user.Status != domain.UserActive {
		writeError(w, http.StatusForbidden, "测试账号已停用")
		return
	}
	s.clearLoginFailures(key)
	role := s.wechatUserRole(r.Context(), user.ID)
	token, err := s.tokens.issue(user.ID, role)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "测试登录暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]any{"accessToken": token, "role": role, "phone": user.Phone, "testOnly": true}})
}

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
