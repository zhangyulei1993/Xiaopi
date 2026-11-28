package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xiaopi/backend/internal/domain"
)

type competingCommunityStore struct {
	*doorplateConfigStore
	conflictKey string
	comparisons int
}

func (s *competingCommunityStore) CompareAndSwapAppConfig(ctx context.Context, key string, before, after []byte) error {
	s.comparisons++
	if key == s.conflictKey {
		// Another API instance commits a different community after our read.
		s.configs[key] = []byte(`{"小区乙":{"address":"乙地址","latitude":30,"longitude":114}}`)
		s.conflictKey = ""
	}
	if !bytes.Equal(s.configs[key], before) {
		return domain.ErrConfigConflict
	}
	return s.UpsertAppConfig(ctx, key, after)
}

func TestCommunitySyncDoesNotOverwriteAnotherAPIInstance(t *testing.T) {
	storage := &competingCommunityStore{doorplateConfigStore: &doorplateConfigStore{fakeStore: &fakeStore{}, configs: map[string][]byte{}}, conflictKey: communityLocationKey}
	server := &Server{store: storage}
	err := server.ensureDoorplateCollectionPlan(context.Background(), "小区甲", communityLocation{Address: "甲地址", Latitude: 31, Longitude: 115})
	if !errors.Is(err, domain.ErrConfigConflict) {
		t.Fatalf("stale write accepted: %v", err)
	}
	if err := server.ensureDoorplateCollectionPlan(context.Background(), "小区甲", communityLocation{Address: "甲地址", Latitude: 31, Longitude: 115}); err != nil {
		t.Fatal(err)
	}
	locations, err := loadCommunityLocations(context.Background(), storage)
	if err != nil || len(locations) != 2 {
		t.Fatalf("lost location on retry: %v %v", locations, err)
	}
}

func TestExplicitLocationWriteUsesCompareAndSwap(t *testing.T) {
	storage := &competingCommunityStore{doorplateConfigStore: &doorplateConfigStore{fakeStore: &fakeStore{}, configs: map[string][]byte{}}, conflictKey: communityLocationKey}
	server := &Server{store: storage}
	response := httptest.NewRecorder()
	server.communityLocations(response, httptest.NewRequest(http.MethodPut, "/location?community=小区甲", strings.NewReader(`{"address":"甲地址","latitude":31,"longitude":115}`)))
	if response.Code != 409 || storage.comparisons != 1 {
		t.Fatalf("expected conflict instead of lost write: %d %s", response.Code, response.Body.String())
	}
}
