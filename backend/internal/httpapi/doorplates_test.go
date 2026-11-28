package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xiaopi/backend/internal/domain"
)

type doorplateConfigStore struct {
	*fakeStore
	configs map[string][]byte
}

func (s *doorplateConfigStore) GetAppConfig(_ context.Context, key string) ([]byte, error) {
	value, ok := s.configs[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return append([]byte(nil), value...), nil
}

func (s *doorplateConfigStore) UpsertAppConfig(_ context.Context, key string, value []byte) error {
	if s.configs == nil {
		s.configs = map[string][]byte{}
	}
	s.configs[key] = append([]byte(nil), value...)
	return nil
}

func TestCommunityNameNormalizationAndLegacyDoorplateConsolidation(t *testing.T) {
	if normalizeCommunityName(" 藏龙·新天地（东区） ") != normalizeCommunityName("藏龙 新天地-东区") {
		t.Fatal("punctuation and spacing variants should normalize identically")
	}
	rows := consolidateDoorplateRows([]doorplate{
		{Community: "藏龙·新天地", Building: "3栋", Unit: "1单元", Room: "3301"},
		{Community: "藏龙新天地", Building: "3栋", Unit: "1单元", Room: "3301"},
		{Community: "藏龙新天地", Building: "3栋", Unit: "1单元", Room: "3302"},
	})
	if len(rows) != 2 {
		t.Fatalf("consolidated rows = %d, want duplicate room removed but distinct room retained", len(rows))
	}
	if rows[0].Community != rows[1].Community {
		t.Fatalf("community aliases were not unified: %#v", rows)
	}
}

func TestDoorplateImportAndLookupUseCanonicalCommunity(t *testing.T) {
	store := &doorplateConfigStore{fakeStore: &fakeStore{}, configs: map[string][]byte{}}
	seed, _ := json.Marshal([]doorplate{{Community: "藏龙·新天地", Building: "3栋", Unit: "1单元", Room: "3301"}})
	store.configs["doorplate_catalog"] = seed
	server := &Server{store: store}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/doorplates", strings.NewReader(`[{"community":"藏龙新天地","building":"3栋","unit":"1单元","room":"3301"},{"community":"藏龙 新天地","building":"3栋","unit":"1单元","room":"3302"}]`))
	response := httptest.NewRecorder()
	server.doorplates(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
	lookup := httptest.NewRequest(http.MethodGet, "/api/v1/admin/doorplates?community=%E8%97%8F%E9%BE%99%E6%96%B0%E5%A4%A9%E5%9C%B0", nil)
	lookupResponse := httptest.NewRecorder()
	server.doorplates(lookupResponse, lookup)
	var envelope struct {
		Data []doorplate `json:"data"`
	}
	if err := json.Unmarshal(lookupResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("variant lookup returned %d rows, want 2: %s", len(envelope.Data), lookupResponse.Body.String())
	}
	for _, row := range envelope.Data {
		if row.Community != "藏龙·新天地" {
			t.Fatalf("import did not preserve canonical display name: %#v", row)
		}
	}
}

func TestCollectionPlanAndLocationUseCanonicalCommunity(t *testing.T) {
	store := &doorplateConfigStore{fakeStore: &fakeStore{}, configs: map[string][]byte{}}
	seed, _ := json.Marshal([]doorplate{{Community: "藏龙·新天地", Building: "3栋", Unit: "1单元", Room: "3301"}})
	store.configs["doorplate_catalog"] = seed
	server := &Server{store: store}
	if err := server.ensureDoorplateCollectionPlan(context.Background(), "藏龙新天地"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.configs["doorplate_collection_plans"]; ok {
		t.Fatal("existing community with doorplate data should not create a duplicate collection plan")
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/community-locations?community=%E8%97%8F%E9%BE%99%E6%96%B0%E5%A4%A9%E5%9C%B0", strings.NewReader(`{"address":"武汉市高新六路4号","latitude":30.45,"longitude":114.42}`))
	response := httptest.NewRecorder()
	server.communityLocations(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("location save status=%d body=%s", response.Code, response.Body.String())
	}
	var stored map[string]communityLocation
	if err := json.Unmarshal(store.configs[communityLocationKey], &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored["藏龙·新天地"].Address == "" {
		t.Fatalf("location not stored under canonical community: %#v", stored)
	}
	newCommunityRequest := httptest.NewRequest(http.MethodPut, "/api/v1/admin/community-locations?community=%E5%87%A1%E8%B0%B7%E5%B0%8F%E5%8C%BA", strings.NewReader(`{"address":"武汉市洪山区凡谷路1号","latitude":30.46,"longitude":114.42}`))
	newCommunityResponse := httptest.NewRecorder()
	server.communityLocations(newCommunityResponse, newCommunityRequest)
	if newCommunityResponse.Code != http.StatusOK {
		t.Fatalf("new community location save status=%d body=%s", newCommunityResponse.Code, newCommunityResponse.Body.String())
	}
	if err := json.Unmarshal(store.configs[communityLocationKey], &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored["凡谷小区"].Address != "武汉市洪山区凡谷路1号" {
		t.Fatalf("new community location not stored: %#v", stored)
	}
}

var _ appConfigStore = (*doorplateConfigStore)(nil)
var _ Store = (*doorplateConfigStore)(nil)

func TestCommunityPhaseAliasesKeepDistinctPhasesAndConfirmedPositions(t *testing.T) {
	rows := consolidateDoorplateRows([]doorplate{
		{Community: "凤凰花园1期", Building: "1栋", Room: "101"},
		{Community: "凤凰花园一期", Building: "1栋", Room: "101"},
		{Community: "凤凰花园１期", Building: "1栋", Room: "102"},
		{Community: "凤凰花园二期", Building: "1栋", Room: "101"},
	})
	if len(rows) != 3 || rows[0].Community != rows[1].Community || rows[0].Community == rows[2].Community {
		t.Fatalf("unexpected consolidation: %#v", rows)
	}
	confirmed := communityLocation{Address: "武汉市凤凰花园", Latitude: 30.4, Longitude: 114.4}
	locations := consolidateCommunityLocations(map[string]communityLocation{"凤凰花园1期": {Address: "待补充"}, "凤凰花园一期": confirmed})
	if len(locations) != 1 || locations["凤凰花园1期"] != confirmed {
		t.Fatalf("confirmed position was lost: %#v", locations)
	}
	plans := consolidateDoorplatePlans(map[string]doorplatePlan{"凤凰花园1期": {Status: "pending"}, "凤凰花园一期": {Status: "done", Note: "已采集"}})
	if len(plans) != 1 || plans["凤凰花园1期"].Status != "done" || plans["凤凰花园1期"].Note != "已采集" {
		t.Fatalf("collection progress was lost: %#v", plans)
	}
}

func TestConfirmedMapPositionOverridesListingCoordinatesAcrossAliases(t *testing.T) {
	items := []domain.Property{
		{Community: "凤凰花园一期", Latitude: 30.1, Longitude: 114.1},
		{Community: "凤凰花园二期", Latitude: 30.2, Longitude: 114.2},
	}
	applyConfirmedMapPositions(items, map[string]communityLocation{
		"凤凰花园1期": {Address: "凤凰花园一期", Latitude: 30.5, Longitude: 114.5},
		"凤凰花园2期": {Address: "待补充", Latitude: 30.9, Longitude: 114.9},
	})
	if items[0].Latitude != 30.5 || items[0].Longitude != 114.5 || items[1].Latitude != 30.2 || items[1].Longitude != 114.2 {
		t.Fatalf("unexpected map coordinates: %#v", items)
	}
}

func TestDoorplateImportReceiptCountsActualWrites(t *testing.T) {
	store := &doorplateConfigStore{fakeStore: &fakeStore{}, configs: map[string][]byte{}}
	server := &Server{store: store}
	for index, want := range []int{1, 0} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/doorplates?receipt=1", strings.NewReader(`[{"community":"凤凰花园一期","building":"1栋","room":"101"},{"community":"凤凰花园1期","building":"1栋","room":"101"}]`))
		response := httptest.NewRecorder()
		server.doorplates(response, request)
		got := responseData[struct{ Added, Duplicates int }](t, response)
		if response.Code != http.StatusOK || got.Added != want || got.Duplicates != 2-want {
			t.Fatalf("receipt %d: %d %s", index, response.Code, response.Body.String())
		}
	}
}
