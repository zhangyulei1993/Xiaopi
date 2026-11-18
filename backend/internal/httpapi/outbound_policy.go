package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// outboundPolicyTransport prevents real HTTP transport from running in test mode.
// Explicitly injected test doubles may be used by tests without network access.
type outboundPolicyTransport struct {
	next     http.RoundTripper
	testMode bool
}

func newOutboundPolicyTransport(next http.RoundTripper, testMode bool, _ string) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &outboundPolicyTransport{next: next, testMode: testMode}
}

func (t *outboundPolicyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("outbound policy: missing request URL")
	}
	if t.testMode {
		if testTencentMapSearchAllowed(req) {
			return t.next.RoundTrip(req)
		}
		return nil, fmt.Errorf("outbound policy: request not allowlisted in test mode")
	}
	return t.next.RoundTrip(req)
}

func testTencentMapSearchAllowed(req *http.Request) bool {
	key := strings.TrimSpace(os.Getenv("TENCENT_MAP_KEY"))
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("TEST_REAL_MAP_ENABLED")), "true") || key == "" || req.Method != http.MethodGet {
		return false
	}
	if !strings.EqualFold(req.URL.Scheme, "https") || !strings.EqualFold(req.URL.Hostname(), "apis.map.qq.com") || req.URL.Port() != "" || req.URL.User != nil || req.URL.Path != "/ws/place/v1/search" {
		return false
	}
	if req.Host != "" && !strings.EqualFold(req.Host, "apis.map.qq.com") {
		return false
	}
	query := req.URL.Query()
	if query.Get("key") != key || query.Get("keyword") == "" || len([]rune(query.Get("keyword"))) > 32 {
		return false
	}
	for name := range query {
		switch name {
		case "key", "keyword", "boundary", "page_size", "page_index", "orderby":
		default:
			return false
		}
	}
	return true
}

func testOutboundTarget() string { return "" }

func newOutboundHTTPClient(timeout time.Duration) *http.Client {
	testMode := os.Getenv("APP_ENV") == "test"
	return &http.Client{Timeout: timeout, Transport: newOutboundPolicyTransport(http.DefaultTransport, testMode, "")}
}
