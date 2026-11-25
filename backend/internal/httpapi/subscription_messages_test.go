package httpapi

import (
	"context"
	"net/http"
	"testing"

	"xiaopi/backend/internal/domain"
)

type subscriptionTestStore struct {
	*fakeStore
	grants []subscriptionGrant
}

type subscriptionGrant struct {
	userID, templateID, area string
}

func (s *subscriptionTestStore) AddMessageSubscriptionGrant(_ context.Context, userID, templateID, area string) error {
	s.grants = append(s.grants, subscriptionGrant{userID: userID, templateID: templateID, area: area})
	return nil
}

func (*subscriptionTestStore) ListAdminMessageRecipients(context.Context, string) ([]domain.MessageRecipient, error) {
	return nil, nil
}
func (*subscriptionTestStore) ListAreaMessageRecipients(context.Context, string, domain.Property) ([]domain.MessageRecipient, error) {
	return nil, nil
}
func (*subscriptionTestStore) ConsumeMessageSubscriptionGrant(context.Context, uint64) error {
	return nil
}
func (*subscriptionTestStore) ListUnnotifiedRentalRequests(context.Context, string) ([]domain.RentalRequest, error) {
	return nil, nil
}
func (*subscriptionTestStore) MarkRentalRequestAdminNotified(context.Context, string) error {
	return nil
}

func TestMessageSubscriptionAppointmentGrantRequiresAdmin(t *testing.T) {
	t.Setenv("WECHAT_AREA_LISTING_TEMPLATE_ID", "area-template")
	t.Setenv("WECHAT_APPOINTMENT_TEMPLATE_ID", "appointment-template")
	store := &subscriptionTestStore{fakeStore: &fakeStore{}}
	server := testServer(store)
	adminToken, err := server.tokens.issue("operator", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := server.tokens.issue("customer", roleUser)
	if err != nil {
		t.Fatal(err)
	}

	denied := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/message-subscriptions", map[string]string{
		"templateId": "appointment-template",
	}, userToken)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("regular user appointment grant status = %d, want %d; body=%s", denied.Code, http.StatusForbidden, denied.Body.String())
	}
	if len(store.grants) != 0 {
		t.Fatalf("denied appointment grant was stored: %+v", store.grants)
	}

	area := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/message-subscriptions", map[string]string{
		"templateId": "area-template",
		"area":       "金融港",
	}, userToken)
	if area.Code != http.StatusCreated {
		t.Fatalf("regular user area grant status = %d, want %d; body=%s", area.Code, http.StatusCreated, area.Body.String())
	}

	admin := performRequest(t, server.Handler(), http.MethodPost, "/api/v1/message-subscriptions", map[string]string{
		"templateId": "appointment-template",
	}, adminToken)
	if admin.Code != http.StatusCreated {
		t.Fatalf("admin appointment grant status = %d, want %d; body=%s", admin.Code, http.StatusCreated, admin.Body.String())
	}
	if len(store.grants) != 2 || store.grants[0].templateID != "area-template" || store.grants[1].templateID != "appointment-template" {
		t.Fatalf("unexpected stored grants: %+v", store.grants)
	}
}
