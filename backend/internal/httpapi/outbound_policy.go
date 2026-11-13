package httpapi

import (
	"fmt"
	"net/http"
	"os"
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
		return nil, fmt.Errorf("outbound policy: HTTP requests disabled in test mode: %s", req.URL.Redacted())
	}
	return t.next.RoundTrip(req)
}

func testOutboundTarget() string { return "" }

func newOutboundHTTPClient(timeout time.Duration) *http.Client {
	testMode := os.Getenv("APP_ENV") == "test"
	return &http.Client{Timeout: timeout, Transport: newOutboundPolicyTransport(http.DefaultTransport, testMode, "")}
}
