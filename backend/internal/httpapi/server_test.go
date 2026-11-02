package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xiaopi/backend/internal/domain"
)

type fakeStore struct {
	properties      []domain.Property
	createdProperty domain.Property
	leads           []domain.PropertyLead
	statusErr       error
}

func (f *fakeStore) ListProperties(context.Context) ([]domain.Property, error) {
	return f.properties, nil
}
func (f *fakeStore) CreateProperty(_ context.Context, property domain.Property) error {
	f.createdProperty = property
	return nil
}
func (*fakeStore) UpdateProperty(context.Context, domain.Property) error { return nil }
func (f *fakeStore) ListPropertyLeads(context.Context) ([]domain.PropertyLead, error) {
	return f.leads, nil
}
func (*fakeStore) CreatePropertyLead(context.Context, domain.PropertyLead) error { return nil }
func (f *fakeStore) UpdatePropertyStatus(context.Context, string, string, string, string) error {
	return f.statusErr
}
func (*fakeStore) ConvertLeadToProperty(context.Context, string, domain.Property) error { return nil }
func (*fakeStore) AddPropertyMedia(context.Context, domain.PropertyMedia) error         { return nil }
func (*fakeStore) UpsertUser(context.Context, domain.User) error                        { return nil }

func testServer(dataStore Store) *Server {
	return NewServerWithConfig(dataStore, Config{
		AdminUsername: "operator",
		AdminPassword: "correct horse battery staple",
		TokenSecret:   "test-secret-with-enough-entropy",
		TokenTTL:      time.Hour,
	})
}

func performRequest(t *testing.T, handler http.Handler, method, target string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, target, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func responseData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return envelope.Data
}

func TestAdminLoginAndProtectedEndpoint(t *testing.T) {
	server := testServer(&fakeStore{})
	handler := server.Handler()

	unauthorized := performRequest(t, handler, http.MethodGet, "/api/v1/property-leads", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", unauthorized.Code, unauthorized.Body.String())
	}

	login := performRequest(t, handler, http.MethodPost, "/api/v1/admin/login", map[string]string{
		"username": "operator", "password": "correct horse battery staple",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected login success, got %d: %s", login.Code, login.Body.String())
	}
	auth := responseData[struct {
		AccessToken string `json:"accessToken"`
	}](t, login)
	if auth.AccessToken == "" {
		t.Fatal("expected access token")
	}

	authorized := performRequest(t, handler, http.MethodGet, "/api/v1/property-leads", nil, auth.AccessToken)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", authorized.Code, authorized.Body.String())
	}
}

func TestPropertyVisibilityDependsOnAdminRole(t *testing.T) {
	dataStore := &fakeStore{properties: []domain.Property{
		{ID: "draft", Status: domain.PropertyDraft},
		{ID: "published", Status: domain.PropertyPublished},
	}}
	server := testServer(dataStore)
	handler := server.Handler()

	publicResponse := performRequest(t, handler, http.MethodGet, "/api/v1/properties", nil, "")
	publicItems := responseData[[]domain.Property](t, publicResponse)
	if len(publicItems) != 1 || publicItems[0].ID != "published" {
		t.Fatalf("public response leaked unpublished properties: %#v", publicItems)
	}

	adminToken, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	adminResponse := performRequest(t, handler, http.MethodGet, "/api/v1/properties", nil, adminToken)
	adminItems := responseData[[]domain.Property](t, adminResponse)
	if len(adminItems) != 2 {
		t.Fatalf("admin expected all properties, got %#v", adminItems)
	}
}

func TestCreatePropertyRequiresUserAndUsesTokenOwner(t *testing.T) {
	dataStore := &fakeStore{}
	server := testServer(dataStore)
	handler := server.Handler()
	payload := map[string]any{"title": "整租一居", "community": "测试小区", "rentMonthly": 3500, "ownerId": "forged-owner"}

	unauthorized := performRequest(t, handler, http.MethodPost, "/api/v1/properties", payload, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorized.Code)
	}

	userToken, err := server.tokens.issue("wx-user-1", roleUser)
	if err != nil {
		t.Fatal(err)
	}
	created := performRequest(t, handler, http.MethodPost, "/api/v1/properties", payload, userToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Code, created.Body.String())
	}
	if dataStore.createdProperty.OwnerID != "wx-user-1" || dataStore.createdProperty.Status != domain.PropertyDraft {
		t.Fatalf("server did not enforce owner/status: %#v", dataStore.createdProperty)
	}
}

func TestInvalidStatusTransitionReturnsConflict(t *testing.T) {
	dataStore := &fakeStore{statusErr: domain.ErrInvalidPropertyStatusTransition}
	server := testServer(dataStore)
	adminToken, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(t, server.Handler(), http.MethodPatch, "/api/v1/properties/status", map[string]string{
		"propertyId": "property-1", "status": string(domain.PropertyPublished),
	}, adminToken)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", response.Code, response.Body.String())
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	manager := newTokenManager(Config{TokenSecret: "secret", TokenTTL: time.Minute})
	now := time.Unix(1_700_000_000, 0)
	manager.now = func() time.Time { return now }
	token, err := manager.issue("user-1", roleUser)
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := manager.verify(token); !errors.Is(err, errInvalidToken) {
		t.Fatalf("expected expired token error, got %v", err)
	}
}
