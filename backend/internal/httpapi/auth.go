package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	roleAdmin = "admin"
	roleUser  = "user"
)

var errInvalidToken = errors.New("invalid or expired access token")

type Config struct {
	AdminUsername string
	AdminPassword string
	TokenSecret   string
	TokenTTL      time.Duration
}

func ConfigFromEnv() Config {
	ttl := 24 * time.Hour
	if parsed, err := time.ParseDuration(os.Getenv("AUTH_TOKEN_TTL")); err == nil && parsed > 0 {
		ttl = parsed
	}
	secret := os.Getenv("AUTH_TOKEN_SECRET")
	if secret == "" {
		secret = os.Getenv("JWT_ADMIN_SECRET")
	}
	return Config{
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TokenSecret:   secret,
		TokenTTL:      ttl,
	}
}

func (c Config) authConfigured() bool {
	return c.AdminUsername != "" && c.AdminPassword != "" && c.TokenSecret != ""
}

type tokenClaims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Expires int64  `json:"exp"`
}

type tokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func newTokenManager(config Config) tokenManager {
	return tokenManager{secret: []byte(config.TokenSecret), ttl: config.TokenTTL, now: time.Now}
}

func (m tokenManager) issue(subject, role string) (string, error) {
	if len(m.secret) == 0 || subject == "" || (role != roleAdmin && role != roleUser) {
		return "", errInvalidToken
	}
	payload, err := json.Marshal(tokenClaims{Subject: subject, Role: role, Expires: m.now().Add(m.ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m tokenManager) verify(token string) (tokenClaims, error) {
	var claims tokenClaims
	parts := strings.Split(token, ".")
	if len(m.secret) == 0 || len(parts) != 2 {
		return claims, errInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errInvalidToken
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(parts[0]))
	if subtle.ConstantTimeCompare(signature, mac.Sum(nil)) != 1 {
		return claims, errInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(payload, &claims) != nil || claims.Subject == "" || claims.Expires <= m.now().Unix() {
		return tokenClaims{}, errInvalidToken
	}
	if claims.Role != roleAdmin && claims.Role != roleUser {
		return tokenClaims{}, errInvalidToken
	}
	return claims, nil
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func (s *Server) optionalClaims(r *http.Request) (tokenClaims, bool) {
	claims, err := s.tokens.verify(bearerToken(r))
	return claims, err == nil
}

func (s *Server) requireRoles(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.optionalClaims(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if _, ok := allowed[claims.Role]; !ok {
			writeError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
	}
}

func secureEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
