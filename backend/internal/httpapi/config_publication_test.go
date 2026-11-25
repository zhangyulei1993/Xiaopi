package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"xiaopi/backend/internal/domain"
)

type publicationTestStore struct {
	*fakeStore
	mu         sync.Mutex
	config     domain.PublicConfig
	history    []domain.ConfigPublication
	doorplates []doorplate
	fail       bool
}

func (f *publicationTestStore) GetAppConfig(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if key == publicConfigKey {
		return json.Marshal(f.config)
	}
	if key == "doorplate_catalog" {
		return json.Marshal(f.doorplates)
	}
	for _, e := range f.history {
		if key == "miniapp_publish_"+e.ID {
			return json.Marshal(e)
		}
	}
	return nil, sql.ErrNoRows
}
func (f *publicationTestStore) UpsertAppConfig(context.Context, string, []byte) error {
	return errors.New("must use transaction")
}
func (f *publicationTestStore) ListConfigPublications(context.Context, string) ([]domain.ConfigPublication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.ConfigPublication{}, f.history...), nil
}
func (f *publicationTestStore) PublishPublicConfig(_ context.Context, expected string, c domain.PublicConfig, operator, source string) (domain.ConfigPublication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var empty domain.ConfigPublication
	if f.fail {
		return empty, errors.New("injected storage failure")
	}
	if expected != domain.ConfigRevision(f.config) {
		return empty, domain.ErrConfigConflict
	}
	e := domain.ConfigPublication{ID: fmt.Sprintf("%020d", len(f.history)+1), Operator: operator, CreatedAt: time.Now(), Revision: domain.ConfigRevision(c), Before: f.config, After: c, RestoredFrom: source}
	f.config = c
	f.history = append([]domain.ConfigPublication{e}, f.history...)
	return e, nil
}
func newPublicationTest(t *testing.T) (*Server, *publicationTestStore, string) {
	t.Helper()
	store := &publicationTestStore{fakeStore: &fakeStore{}, config: domain.DefaultPublicConfig()}
	for index := range store.config.FeaturedChannels {
		if store.config.FeaturedChannels[index].Latitude == 0 || store.config.FeaturedChannels[index].Longitude == 0 {
			store.config.FeaturedChannels[index].Latitude = 30.456040
			store.config.FeaturedChannels[index].Longitude = 114.425337
		}
	}
	store.config.Normalize()
	s := NewServerWithConfig(store, Config{AdminUsername: "admin", AdminPassword: "test-password", TokenTTL: time.Hour, TokenSecret: "test-publication-secret"})
	token, err := s.tokens.issue("admin", roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return s, store, token
}

func TestFeaturedChannelPublicationPreservesLegacyMapAnchors(t *testing.T) {
	store := &publicationTestStore{
		fakeStore:  &fakeStore{},
		doorplates: []doorplate{{Community: "凤凰花园三期", Building: "1栋", Unit: "1单元", Room: "101"}},
	}
	config := domain.DefaultPublicConfig()
	config.FeaturedChannels[0].Community = "凤凰花园三期"
	config.FeaturedChannels[0].Keyword = "凤凰花园三期"
	config.FeaturedChannels[1].Latitude = 30.460402
	config.FeaturedChannels[1].Longitude = 114.422247
	config.FeaturedChannels[2].Latitude = 30.436999
	config.FeaturedChannels[2].Longitude = 114.445507
	if err := canonicalizeFeaturedChannelCommunities(context.Background(), store, &config); err != nil {
		t.Fatalf("mixed community and legacy map channels should publish: %v", err)
	}
	if config.FeaturedChannels[0].Community != "凤凰花园三期" || config.FeaturedChannels[0].Keyword != "凤凰花园三期" {
		t.Fatal("selected doorplate community was not canonicalized")
	}
	if config.FeaturedChannels[1].Community != "" || config.FeaturedChannels[1].Keyword != "光谷" || config.FeaturedChannels[1].Latitude != 30.460402 {
		t.Fatal("legacy channel map anchor or keyword was changed")
	}

	config.FeaturedChannels[2].Latitude = 0
	config.FeaturedChannels[2].Longitude = 0
	if err := canonicalizeFeaturedChannelCommunities(context.Background(), store, &config); err == nil {
		t.Fatal("unbound channel without a map anchor should not publish")
	}
}

func TestFeaturedChannelPublicationRequiresLocationForNewUnboundChannel(t *testing.T) {
	store := &publicationTestStore{fakeStore: &fakeStore{}}
	config := domain.DefaultPublicConfig()
	config.FeaturedChannels[0].Latitude = 30.460402
	config.FeaturedChannels[0].Longitude = 114.422247
	config.FeaturedChannels[1].Latitude = 30.436999
	config.FeaturedChannels[1].Longitude = 114.445507
	config.FeaturedChannels[2].Latitude = 0
	config.FeaturedChannels[2].Longitude = 0
	if err := canonicalizeFeaturedChannelCommunities(context.Background(), store, &config); err == nil {
		t.Fatal("new unbound channel without a map point should not publish")
	}
}
func TestConfigPublishHistoryConflictAndRecovery(t *testing.T) {
	s, store, token := newPublicationTest(t)
	h := s.Handler()
	for _, path := range []string{"/api/v1/admin/config/state", "/api/v1/admin/config/history"} {
		if r := performRequest(t, h, "GET", path, nil, ""); r.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, r.Code)
		}
	}
	state := responseData[struct {
		Config   domain.PublicConfig `json:"config"`
		Revision string              `json:"revision"`
	}](t, performRequest(t, h, "GET", "/api/v1/admin/config/state", nil, token))
	next := state.Config
	next.HeroTitle = "验收品牌"
	body := map[string]any{"config": next, "expectedRevision": state.Revision}
	r := performRequest(t, h, "POST", "/api/v1/admin/config/publish", body, token)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	first := responseData[domain.ConfigPublication](t, r)
	if first.Operator != "admin" || first.Before.HeroTitle != state.Config.HeroTitle || first.After.HeroTitle != "验收品牌" {
		t.Fatal("incorrect audit snapshot")
	}
	if r = performRequest(t, h, "POST", "/api/v1/admin/config/publish", body, token); r.Code != 409 {
		t.Fatalf("stale publish accepted: %d", r.Code)
	}
	if len(store.history) != 1 || store.config.HeroTitle != "验收品牌" {
		t.Fatal("conflict mutated storage")
	}
	r = performRequest(t, h, "POST", "/api/v1/admin/config/publish", map[string]any{"config": state.Config, "expectedRevision": first.Revision, "restoredFrom": first.ID}, token)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	history := responseData[[]domain.ConfigPublication](t, performRequest(t, h, "GET", "/api/v1/admin/config/history", nil, token))
	if len(history) != 2 || history[0].RestoredFrom != first.ID {
		t.Fatal("restore provenance missing")
	}
}
func TestConfigPublishFailureAndLegacyBypass(t *testing.T) {
	s, store, token := newPublicationTest(t)
	h := s.Handler()
	revision := domain.ConfigRevision(store.config)
	next := store.config
	next.HeroTitle = "不能生效"
	store.fail = true
	r := performRequest(t, h, "POST", "/api/v1/admin/config/publish", map[string]any{"config": next, "expectedRevision": revision}, token)
	if r.Code != 500 || len(store.history) != 0 || domain.ConfigRevision(store.config) != revision {
		t.Fatal("failed publish changed state")
	}
	if r = performRequest(t, h, "PUT", "/api/v1/admin/config/public", next, token); r.Code != 428 {
		t.Fatal("legacy write bypasses version guard")
	}
	if r = performRequest(t, h, "POST", "/api/v1/admin/config/publish", map[string]any{"config": next}, token); r.Code != 428 {
		t.Fatal("missing revision accepted")
	}
}
