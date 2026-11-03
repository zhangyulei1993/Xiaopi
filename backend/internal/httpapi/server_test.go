package httpapi

import (
	"bytes"
	"context"
	"database/sql"
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
	users           []domain.User
	admins          []domain.AdminUser
	temporaryAccess []domain.TemporaryAccess
	temporaryHashes map[string]string
	rentalRequests  []domain.RentalRequest
	favorites       map[string][]string
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
func (*fakeStore) UpdatePropertyLeadFollowUp(context.Context, string, string, string) error {
	return nil
}
func (f *fakeStore) UpdatePropertyStatus(context.Context, string, string, string, string) error {
	return f.statusErr
}
func (*fakeStore) ConvertLeadToProperty(context.Context, string, domain.Property) error { return nil }
func (*fakeStore) AddPropertyMedia(context.Context, domain.PropertyMedia) error         { return nil }
func (*fakeStore) UpsertUser(context.Context, domain.User) error                        { return nil }
func (f *fakeStore) ListRentalRequests(context.Context) ([]domain.RentalRequest, error) {
	return f.rentalRequests, nil
}
func (f *fakeStore) CreateRentalRequest(_ context.Context, item domain.RentalRequest) error {
	f.rentalRequests = append(f.rentalRequests, item)
	return nil
}
func (*fakeStore) UpdateRentalRequestFollowUp(context.Context, string, string, string) error {
	return nil
}
func (*fakeStore) CreateOperationLog(context.Context, domain.OperationLog) error { return nil }
func (*fakeStore) ListOperationLogs(context.Context, int) ([]domain.OperationLog, error) {
	return nil, nil
}
func (f *fakeStore) ListFavoritePropertyIDs(_ context.Context, userID string) ([]string, error) {
	return f.favorites[userID], nil
}
func (f *fakeStore) SetPropertyFavorite(_ context.Context, userID, propertyID string, favorite bool) error {
	if f.favorites == nil {
		f.favorites = map[string][]string{}
	}
	if favorite {
		for _, id := range f.favorites[userID] {
			if id == propertyID {
				return nil
			}
		}
		f.favorites[userID] = append(f.favorites[userID], propertyID)
		return nil
	}
	items := f.favorites[userID][:0]
	for _, id := range f.favorites[userID] {
		if id != propertyID {
			items = append(items, id)
		}
	}
	f.favorites[userID] = items
	return nil
}
func (f *fakeStore) ListUsers(context.Context) ([]domain.User, error) { return f.users, nil }
func (f *fakeStore) UpdateUserPhone(_ context.Context, userID, phone string) error {
	for index := range f.users {
		if f.users[index].ID == userID {
			f.users[index].Phone = phone
			return nil
		}
	}
	return sql.ErrNoRows
}
func (f *fakeStore) ListAdminUsers(context.Context) ([]domain.AdminUser, error) { return f.admins, nil }
func (f *fakeStore) AssignAdminByPhone(_ context.Context, phone, createdBy string) (domain.AdminUser, error) {
	for _, user := range f.users {
		if user.Phone == phone {
			admin := domain.AdminUser{ID: uint64(len(f.admins) + 1), UserID: user.ID, Nickname: user.Nickname, Phone: phone, Role: roleAdmin, Status: "active", CreatedBy: createdBy}
			f.admins = append(f.admins, admin)
			return admin, nil
		}
	}
	return domain.AdminUser{}, sql.ErrNoRows
}
func (f *fakeStore) SetAdminStatus(_ context.Context, userID, status string) error {
	for index := range f.admins {
		if f.admins[index].UserID == userID {
			f.admins[index].Status = status
			return nil
		}
	}
	return sql.ErrNoRows
}
func (f *fakeStore) DeleteAdminUser(_ context.Context, userID string) error {
	for index := range f.admins {
		if f.admins[index].UserID == userID {
			f.admins = append(f.admins[:index], f.admins[index+1:]...)
			return nil
		}
	}
	return sql.ErrNoRows
}
func (f *fakeStore) IsAdminUser(_ context.Context, userID string) (bool, error) {
	for _, admin := range f.admins {
		if admin.UserID == userID && admin.Status == "active" {
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeStore) ListTemporaryAccess(context.Context) ([]domain.TemporaryAccess, error) {
	return f.temporaryAccess, nil
}
func (f *fakeStore) UpsertTemporaryAccess(_ context.Context, phone, codeHash, accessCode, createdBy string, expiresAt time.Time) (domain.TemporaryAccess, error) {
	if f.temporaryHashes == nil {
		f.temporaryHashes = map[string]string{}
	}
	f.temporaryHashes[phone] = codeHash
	item := domain.TemporaryAccess{ID: 1, Phone: phone, AccessCode: accessCode, Status: "active", ExpiresAt: expiresAt, CreatedBy: createdBy, CreatedAt: time.Now()}
	f.temporaryAccess = []domain.TemporaryAccess{item}
	return item, nil
}
func (f *fakeStore) SetTemporaryAccessStatus(_ context.Context, phone, status string) error {
	for index := range f.temporaryAccess {
		if f.temporaryAccess[index].Phone == phone {
			f.temporaryAccess[index].Status = status
			return nil
		}
	}
	return sql.ErrNoRows
}
func (f *fakeStore) LoginTemporaryUser(_ context.Context, user domain.User, phone, codeHash string, now time.Time) (domain.User, error) {
	if f.temporaryHashes[phone] != codeHash || len(f.temporaryAccess) == 0 || f.temporaryAccess[0].Status != "active" || !f.temporaryAccess[0].ExpiresAt.After(now) {
		return domain.User{}, sql.ErrNoRows
	}
	for _, existing := range f.users {
		if existing.Phone == phone {
			return existing, nil
		}
	}
	f.users = append(f.users, user)
	return user, nil
}

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

func TestAdminCanListUsersAndAssignAdminByPhone(t *testing.T) {
	dataStore := &fakeStore{users: []domain.User{{ID: "wx-user-1", Nickname: "租客甲", Phone: "13800138000", Status: domain.UserActive}}}
	server := testServer(dataStore)
	token, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	usersResponse := performRequest(t, server.Handler(), http.MethodGet, "/api/v1/admin/users", nil, token)
	if usersResponse.Code != http.StatusOK || len(responseData[[]domain.User](t, usersResponse)) != 1 {
		t.Fatalf("expected user list, got %d: %s", usersResponse.Code, usersResponse.Body.String())
	}
	assignResponse := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/admin/admin-users", map[string]string{"phone": "13800138000"}, token)
	if assignResponse.Code != http.StatusCreated {
		t.Fatalf("expected admin assignment, got %d: %s", assignResponse.Code, assignResponse.Body.String())
	}
	admin := responseData[domain.AdminUser](t, assignResponse)
	if admin.UserID != "wx-user-1" || admin.CreatedBy != "operator" {
		t.Fatalf("unexpected admin assignment: %#v", admin)
	}
	deleted := performRequest(t, server.Handler(), http.MethodDelete, "/api/v1/admin/admin-users/wx-user-1", nil, token)
	if deleted.Code != http.StatusOK || len(dataStore.admins) != 0 {
		t.Fatalf("expected deleted admin mapping, got %d: %s admins=%#v", deleted.Code, deleted.Body.String(), dataStore.admins)
	}
}

func TestAssignAdminRequiresExistingWechatUserPhone(t *testing.T) {
	server := testServer(&fakeStore{})
	token, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/admin/admin-users", map[string]string{"phone": "13800138000"}, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.Code, response.Body.String())
	}
}

func TestTemporaryAccessCreatesDistinctAuthenticatedUser(t *testing.T) {
	dataStore := &fakeStore{}
	server := testServer(dataStore)
	adminToken, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	created := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/admin/temporary-access", map[string]any{"phone": "13800138000", "expiresInHours": 24}, adminToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected access code, got %d: %s", created.Code, created.Body.String())
	}
	data := responseData[struct {
		AccessCode string `json:"accessCode"`
	}](t, created)
	if len(data.AccessCode) != 8 {
		t.Fatalf("expected 8 character access code, got %q", data.AccessCode)
	}
	listed := performRequest(t, server.Handler(), http.MethodGet, "/api/v1/admin/temporary-access", nil, adminToken)
	items := responseData[[]domain.TemporaryAccess](t, listed)
	if len(items) != 1 || items[0].AccessCode != data.AccessCode {
		t.Fatalf("expected admin list to expose the current test access code, got %#v", items)
	}
	login := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/temporary/login", map[string]string{"phone": "13800138000", "accessCode": data.AccessCode}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected temporary login, got %d: %s", login.Code, login.Body.String())
	}
	auth := responseData[struct {
		AccessToken string `json:"accessToken"`
		Role        string `json:"role"`
	}](t, login)
	if auth.AccessToken == "" || auth.Role != roleUser || len(dataStore.users) != 1 || dataStore.users[0].Phone != "13800138000" {
		t.Fatalf("unexpected temporary auth result: %#v users=%#v", auth, dataStore.users)
	}
}

func TestUserFavoriteAndRentalRequestFlow(t *testing.T) {
	dataStore := &fakeStore{}
	server := testServer(dataStore)
	userToken, err := server.tokens.issue("user-1", roleUser)
	if err != nil {
		t.Fatal(err)
	}

	favorite := performRequest(t, server.Handler(), http.MethodPut, "/api/v1/favorites/property-1", nil, userToken)
	if favorite.Code != http.StatusOK {
		t.Fatalf("favorite failed: %d %s", favorite.Code, favorite.Body.String())
	}
	listed := performRequest(t, server.Handler(), http.MethodGet, "/api/v1/favorites", nil, userToken)
	ids := responseData[[]string](t, listed)
	if len(ids) != 1 || ids[0] != "property-1" {
		t.Fatalf("unexpected favorites: %#v", ids)
	}

	created := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/rental-requests", map[string]any{"requestType": "appointment", "propertyId": "property-1", "phone": "13800138000", "budgetMin": 1500, "budgetMax": 2500}, userToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("rental request failed: %d %s", created.Code, created.Body.String())
	}
	if len(dataStore.rentalRequests) != 1 || dataStore.rentalRequests[0].UserID != "user-1" {
		t.Fatalf("unexpected request: %#v", dataStore.rentalRequests)
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

func TestCreatePropertyRequiresAdmin(t *testing.T) {
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
	denied := performRequest(t, handler, http.MethodPost, "/api/v1/properties", payload, userToken)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("expected user to be denied, got %d: %s", denied.Code, denied.Body.String())
	}
	adminToken, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	created := performRequest(t, handler, http.MethodPost, "/api/v1/properties", payload, adminToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Code, created.Body.String())
	}
	if dataStore.createdProperty.OwnerID != "forged-owner" || dataStore.createdProperty.Status != domain.PropertyDraft {
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

func TestAdminCannotSubmitPropertyLead(t *testing.T) {
	dataStore := &fakeStore{admins: []domain.AdminUser{{UserID: "wx-admin-1", Status: "active"}}}
	server := testServer(dataStore)
	staleUserToken, err := server.tokens.issue("wx-admin-1", roleUser)
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/property-leads", map[string]any{"community": "测试小区", "expectedRentMonthly": 3000}, staleUserToken)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected assigned admin to be forbidden even with stale user token, got %d: %s", response.Code, response.Body.String())
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

func TestWechatUserIDFitsDatabaseColumnAndIsStable(t *testing.T) {
	first := wechatUserID("oExampleOpenIdWithTypicalLength")
	second := wechatUserID("oExampleOpenIdWithTypicalLength")
	if first != second {
		t.Fatalf("wechat user id must be stable: %q != %q", first, second)
	}
	if len(first) > 36 {
		t.Fatalf("wechat user id exceeds users.id column: %d (%q)", len(first), first)
	}
}

func TestRefreshAuthReflectsCurrentAdminAssignment(t *testing.T) {
	dataStore := &fakeStore{admins: []domain.AdminUser{{UserID: "wx-user-1", Status: "active"}}}
	server := testServer(dataStore)
	userToken, err := server.tokens.issue("wx-user-1", roleUser)
	if err != nil {
		t.Fatal(err)
	}
	upgraded := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/refresh", nil, userToken)
	if upgraded.Code != http.StatusOK {
		t.Fatalf("expected refresh success, got %d: %s", upgraded.Code, upgraded.Body.String())
	}
	upgradedAuth := responseData[struct {
		Role string `json:"role"`
	}](t, upgraded)
	if upgradedAuth.Role != roleAdmin {
		t.Fatalf("expected admin role, got %q", upgradedAuth.Role)
	}
	dataStore.admins[0].Status = "disabled"
	adminToken, err := server.tokens.issue("wx-user-1", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	downgraded := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/refresh", nil, adminToken)
	downgradedAuth := responseData[struct {
		Role string `json:"role"`
	}](t, downgraded)
	if downgradedAuth.Role != roleUser {
		t.Fatalf("expected user role after disable, got %q", downgradedAuth.Role)
	}
}
