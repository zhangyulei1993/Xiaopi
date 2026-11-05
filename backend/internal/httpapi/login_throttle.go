package httpapi

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// loginThrottle is deliberately in-memory: it protects the public edge without
// adding a database dependency to authentication. A process restart clears it,
// while normal repeated guessing is stopped quickly.
type loginAttempt struct {
	failures   int
	windowFrom time.Time
	blockedTo  time.Time
}

func (s *Server) loginAttemptKey(r *http.Request, subject string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host + "|" + strings.TrimSpace(subject)
}

func (s *Server) loginBlocked(key string) (time.Duration, bool) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	attempt, ok := s.loginAttempts[key]
	if !ok || time.Now().After(attempt.blockedTo) {
		return 0, false
	}
	return time.Until(attempt.blockedTo).Round(time.Second), true
}

func (s *Server) recordLoginFailure(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := time.Now()
	attempt := s.loginAttempts[key]
	if attempt.windowFrom.IsZero() || now.Sub(attempt.windowFrom) > 15*time.Minute {
		attempt = loginAttempt{windowFrom: now}
	}
	attempt.failures++
	if attempt.failures >= 5 {
		attempt.blockedTo = now.Add(15 * time.Minute)
		attempt.failures = 0
		attempt.windowFrom = now
	}
	s.loginAttempts[key] = attempt
}

func (s *Server) clearLoginFailures(key string) {
	s.loginMu.Lock()
	delete(s.loginAttempts, key)
	s.loginMu.Unlock()
}

func blockedLoginResponse(w http.ResponseWriter, wait time.Duration) {
	if wait < time.Second {
		wait = time.Second
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
}
