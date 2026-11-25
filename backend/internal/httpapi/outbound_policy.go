package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
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
		if testTencentMapSearchAllowed(req) || testTencentStaticMapAllowed(req) {
			return t.next.RoundTrip(req)
		}
		return nil, fmt.Errorf("outbound policy: request not allowlisted in test mode")
	}
	return t.next.RoundTrip(req)
}

// Static map previews are also needed by the local Admin workspace. Keep this
// allowlist narrow: only Tencent's image endpoint, fixed image dimensions,
// bounded zoom/scale, and validated center/marker coordinates may leave test
// mode. No redirects or arbitrary query parameters are allowed.
func testTencentStaticMapAllowed(req *http.Request) bool {
	key := strings.TrimSpace(os.Getenv("TENCENT_MAP_KEY"))
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("TEST_REAL_MAP_ENABLED")), "true") || key == "" || req.Method != http.MethodGet {
		return false
	}
	if !strings.EqualFold(req.URL.Scheme, "https") || !strings.EqualFold(req.URL.Hostname(), "apis.map.qq.com") || req.URL.Port() != "" || req.URL.User != nil || req.URL.Path != "/ws/staticmap/v2/" {
		return false
	}
	if req.Host != "" && !strings.EqualFold(req.Host, "apis.map.qq.com") {
		return false
	}
	query := req.URL.Query()
	if query.Get("key") != key || !validMapCoordinatePair(query.Get("center")) || query.Get("size") != "620*260" || (query.Get("scale") != "1" && query.Get("scale") != "2") {
		return false
	}
	zoom, err := strconv.Atoi(query.Get("zoom"))
	if err != nil || zoom < 1 || zoom > 20 {
		return false
	}
	for name := range query {
		if name != "key" && name != "center" && name != "zoom" && name != "size" && name != "scale" && name != "markers" {
			return false
		}
	}
	markers := query["markers"]
	if len(markers) == 0 || len(markers) > 8 {
		return false
	}
	for _, marker := range markers {
		parts := strings.Split(marker, "|")
		if len(parts) != 4 || (parts[0] != "size:large" && parts[0] != "size:mid") || (parts[1] != "color:0x40543f" && parts[1] != "color:0xd76440") || len(parts[2]) != len("label:A") || !strings.HasPrefix(parts[2], "label:") || !validMapCoordinatePair(parts[3]) {
			return false
		}
		label := strings.TrimPrefix(parts[2], "label:")
		if label != "P" && (len(label) != 1 || label[0] < 'A' || label[0] > 'H') {
			return false
		}
	}
	return true
}

func validMapCoordinatePair(value string) bool {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return false
	}
	latitude, latitudeErr := strconv.ParseFloat(parts[0], 64)
	longitude, longitudeErr := strconv.ParseFloat(parts[1], 64)
	return latitudeErr == nil && longitudeErr == nil && latitude != 0 && longitude != 0 && latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
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
