package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOutboundTestModeRejectsBeforeTransport(t *testing.T) {
	calls := 0
	mock := roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("unexpected") })
	client := &http.Client{Transport: newOutboundPolicyTransport(mock, true, "")}
	for _, target := range []string{"https://api.xiaopizufang.cn/api", "https://api.weixin.qq.com/", "https://apis.map.qq.com/", "https://example.cos.ap-shanghai.myqcloud.com/", "http://127.0.0.1:18083/"} {
		t.Run(target, func(t *testing.T) {
			if _, err := client.Get(target); err == nil {
				t.Fatal("expected test-mode rejection")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("underlying transport called %d times", calls)
	}
}

func TestMockAndProductionTransport(t *testing.T) {
	calls := 0
	mock := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	for _, client := range []*http.Client{{Transport: mock}, {Transport: newOutboundPolicyTransport(mock, false, "")}} {
		resp, err := client.Get("https://example.invalid/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if calls != 2 {
		t.Fatalf("mock calls = %d", calls)
	}
}

func TestCOSDisabledInTestMode(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	server := testServer(&fakeStore{})
	if _, err := server.cosClient(); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("cosClient: %v", err)
	}
	if _, err := server.putCOSObject(context.Background(), "test", "text/plain", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("upload: %v", err)
	}
	if err := server.deleteCOSObject(context.Background(), "https://example.invalid/test"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("delete: %v", err)
	}
}
