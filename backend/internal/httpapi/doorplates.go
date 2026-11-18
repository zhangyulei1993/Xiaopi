package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

type doorplate struct {
	Community string `json:"community"`
	Building  string `json:"building"`
	Unit      string `json:"unit"`
	Room      string `json:"room"`
}

func (s *Server) doorplates(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(appConfigStore)
	if !ok {
		writeError(w, 503, "门牌库不可用")
		return
	}
	s.doorplateMu.Lock()
	defer s.doorplateMu.Unlock()
	const key = "doorplate_catalog"
	rows := []doorplate{}
	raw, err := store.GetAppConfig(r.Context(), key)
	if (err != nil && !errors.Is(err, sql.ErrNoRows)) || (err == nil && json.Unmarshal(raw, &rows) != nil) {
		writeError(w, 500, "读取门牌库失败")
		return
	}
	rows = consolidateDoorplateRows(rows)
	if r.Method == http.MethodPost {
		var input []doorplate
		if err := decodeJSONLimit(w, r, &input, 8<<20); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "门牌数据超过 8 MB，请分批导入")
			} else {
				writeError(w, 400, "门牌数据格式不正确")
			}
			return
		}
		if len(input) == 0 || len(input) > 30000 {
			writeError(w, 400, "请选择有效的门牌数据")
			return
		}
		known := doorplateCommunities(rows)
		locations, locationErr := loadCommunityLocations(r.Context(), store)
		if locationErr != nil {
			writeError(w, 500, "读取小区位置失败")
			return
		}
		for name := range locations {
			known = append(known, name)
		}
		plans, planErr := loadDoorplatePlans(r.Context(), store)
		if planErr != nil {
			writeError(w, 500, "读取采集计划失败")
			return
		}
		for name := range plans {
			known = append(known, name)
		}
		for i, row := range input {
			row = doorplate{strings.TrimSpace(row.Community), strings.TrimSpace(row.Building), strings.TrimSpace(row.Unit), strings.TrimSpace(row.Room)}
			if normalizeCommunityName(row.Community) == "" || row.Building == "" || row.Room == "" {
				writeError(w, 400, "请填写有效的小区名称、楼栋和房号")
				return
			}
			input[i] = row
		}
		// Index existing names once, then normalize the entire batch before writing.
		canonical := make(map[string]string, len(known))
		for _, name := range known {
			identity := normalizeCommunityName(name)
			if current := canonical[identity]; identity != "" && (current == "" || name < current) {
				canonical[identity] = name
			}
		}
		for i, row := range input {
			identity := normalizeCommunityName(row.Community)
			if canonical[identity] == "" {
				canonical[identity] = row.Community
			}
			input[i].Community = canonical[identity]
		}
		rows = consolidateDoorplateRows(append(rows, input...))
		raw, _ = json.Marshal(rows)
		if err := store.UpsertAppConfig(r.Context(), key, raw); err != nil {
			writeError(w, 500, "保存门牌库失败")
			return
		}
	}
	community := strings.TrimSpace(r.URL.Query().Get("community"))
	if community != "" {
		matches := []doorplate{}
		for _, row := range rows {
			if normalizeCommunityName(row.Community) == normalizeCommunityName(community) {
				matches = append(matches, row)
			}
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": matches})
		return
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Community]++
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": counts})
}

func doorplateCommunities(rows []doorplate) []string {
	names := make([]string, 0)
	for _, row := range rows {
		names = append(names, row.Community)
	}
	result := uniqueCommunityNames(names)
	sort.Strings(result)
	return result
}

func consolidateDoorplateRows(rows []doorplate) []doorplate {
	canonical := map[string]string{}
	for _, row := range rows {
		key := normalizeCommunityName(row.Community)
		if key == "" {
			continue
		}
		if current := canonical[key]; current == "" || strings.TrimSpace(row.Community) < current {
			canonical[key] = strings.TrimSpace(row.Community)
		}
	}
	seen := make(map[doorplate]bool, len(rows))
	result := make([]doorplate, 0, len(rows))
	for _, row := range rows {
		if name := canonical[normalizeCommunityName(row.Community)]; name != "" {
			row.Community = name
		}
		row.Building, row.Unit, row.Room = strings.TrimSpace(row.Building), strings.TrimSpace(row.Unit), strings.TrimSpace(row.Room)
		if !seen[row] {
			result = append(result, row)
			seen[row] = true
		}
	}
	return result
}

func loadDoorplatePlans(ctx context.Context, store appConfigStore) (map[string]doorplatePlan, error) {
	plans := map[string]doorplatePlan{}
	raw, err := store.GetAppConfig(ctx, "doorplate_collection_plans")
	if errors.Is(err, sql.ErrNoRows) {
		return plans, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &plans)
	if plans == nil {
		plans = map[string]doorplatePlan{}
	}
	return plans, err
}

type doorplatePlan struct {
	Community string `json:"community"`
	Status    string `json:"status"`
	Due       string `json:"due"`
	Note      string `json:"note"`
}

// ensureDoorplateCollectionPlan makes a newly used community visible to the
// collection workflow when no doorplate catalog has been imported for it yet.
// Existing plans (including active and done plans) are intentionally preserved.
func (s *Server) ensureDoorplateCollectionPlan(ctx context.Context, community string, confirmedLocation ...communityLocation) error {
	community = strings.TrimSpace(community)
	store, ok := s.store.(appConfigStore)
	if !ok || normalizeCommunityName(community) == "" {
		return nil
	}

	s.doorplateMu.Lock()
	defer s.doorplateMu.Unlock()

	var rows []doorplate
	raw, err := store.GetAppConfig(ctx, "doorplate_catalog")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && json.Unmarshal(raw, &rows) != nil {
		return errors.New("读取门牌库失败")
	}
	rows = consolidateDoorplateRows(rows)
	known := doorplateCommunities(rows)
	catalogNames := known
	locations, err := loadCommunityLocations(ctx, store)
	if err != nil {
		return errors.New("读取小区位置失败")
	}
	for name := range locations {
		known = append(known, name)
	}
	const key = "doorplate_collection_plans"
	plans, err := loadDoorplatePlans(ctx, store)
	if err != nil {
		return errors.New("读取采集计划失败")
	}
	for name := range plans {
		known = append(known, name)
	}
	canonical, err := resolveCommunityName(community, known)
	if err != nil {
		return err
	}
	community = canonical
	if len(confirmedLocation) > 0 {
		location := confirmedLocation[0]
		location.Address = strings.TrimSpace(location.Address)
		hasPosition := location.Address != "" &&
			!strings.Contains(location.Address, "QA 地图占位地址") &&
			!strings.Contains(location.Address, "测试环境") &&
			location.Latitude != 0 && location.Longitude != 0 &&
			location.Latitude >= -90 && location.Latitude <= 90 &&
			location.Longitude >= -180 && location.Longitude <= 180
		if hasPosition {
			for name := range locations {
				if normalizeCommunityName(name) == normalizeCommunityName(community) {
					hasPosition = false
					break
				}
			}
			if hasPosition {
				locations[community] = location
				payload, err := json.Marshal(locations)
				if err != nil {
					return err
				}
				if err := store.UpsertAppConfig(ctx, communityLocationKey, payload); err != nil {
					return err
				}
			}
		}
	}
	for _, name := range catalogNames {
		if normalizeCommunityName(name) == normalizeCommunityName(community) {
			return nil
		}
	}
	for name := range plans {
		if normalizeCommunityName(name) == normalizeCommunityName(community) {
			return nil
		}
	}
	plans[community] = doorplatePlan{
		Community: community,
		Status:    "pending",
		Note:      "房源录入时自动发现，待采集楼栋、单元和房号",
	}
	payload, err := json.Marshal(plans)
	if err != nil {
		return err
	}
	return store.UpsertAppConfig(ctx, key, payload)
}

func (s *Server) doorplatePlans(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(appConfigStore)
	if !ok {
		writeError(w, 503, "采集计划不可用")
		return
	}
	s.doorplateMu.Lock()
	defer s.doorplateMu.Unlock()

	const key = "doorplate_collection_plans"
	plans := map[string]doorplatePlan{}
	raw, err := store.GetAppConfig(r.Context(), key)
	if (err != nil && !errors.Is(err, sql.ErrNoRows)) || (err == nil && json.Unmarshal(raw, &plans) != nil) {
		writeError(w, 500, "读取采集计划失败")
		return
	}
	if r.Method == http.MethodPut {
		var plan doorplatePlan
		if decodeJSON(w, r, &plan) != nil {
			writeError(w, 400, "采集计划格式不正确")
			return
		}
		plan.Community = strings.TrimSpace(plan.Community)
		plan.Note = strings.TrimSpace(plan.Note)
		if plan.Community == "" || len([]rune(plan.Community)) > 100 || len([]rune(plan.Note)) > 500 ||
			(plan.Status != "pending" && plan.Status != "active" && plan.Status != "done") {
			writeError(w, 400, "请填写有效的小区名称、状态和备注")
			return
		}
		if plan.Due != "" {
			if _, err := time.Parse(time.DateOnly, plan.Due); err != nil {
				writeError(w, 400, "计划日期不正确")
				return
			}
		}
		known := make([]string, 0, len(plans))
		for name := range plans {
			known = append(known, name)
		}
		var rows []doorplate
		if catalog, catalogErr := store.GetAppConfig(r.Context(), "doorplate_catalog"); catalogErr == nil {
			_ = json.Unmarshal(catalog, &rows)
		}
		known = append(known, doorplateCommunities(rows)...)
		locations, locationErr := loadCommunityLocations(r.Context(), store)
		if locationErr != nil {
			writeError(w, 500, "读取小区位置失败")
			return
		}
		for name := range locations {
			known = append(known, name)
		}
		canonical, err := resolveCommunityName(plan.Community, known)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		plan.Community = canonical
		for name := range plans {
			if normalizeCommunityName(name) == normalizeCommunityName(canonical) {
				delete(plans, name)
			}
		}
		plans[canonical] = plan
		payload, err := json.Marshal(plans)
		if err != nil || store.UpsertAppConfig(r.Context(), key, payload) != nil {
			writeError(w, 500, "保存采集计划失败")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": plans})
}

type communityLocation struct {
	Address   string  `json:"address"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (s *Server) communityLocations(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(appConfigStore)
	if !ok {
		writeError(w, 503, "小区位置服务不可用")
		return
	}
	if r.Method == http.MethodPut {
		name := strings.TrimSpace(r.URL.Query().Get("community"))
		var location communityLocation
		if decodeJSON(w, r, &location) != nil || name == "" || len([]rune(name)) > 100 {
			writeError(w, 400, "小区位置格式不正确")
			return
		}
		location.Address = strings.TrimSpace(location.Address)
		if location.Address == "" || len([]rune(location.Address)) > 60 ||
			location.Latitude == 0 || location.Longitude == 0 ||
			location.Latitude < -90 || location.Latitude > 90 ||
			location.Longitude < -180 || location.Longitude > 180 {
			writeError(w, 400, "请填写地址和有效的经纬度")
			return
		}
		s.doorplateMu.Lock()
		defer s.doorplateMu.Unlock()
		locations, err := loadCommunityLocations(r.Context(), store)
		if err == nil {
			known := make([]string, 0, len(locations))
			for existing := range locations {
				known = append(known, existing)
			}
			var rows []doorplate
			if raw, loadErr := store.GetAppConfig(r.Context(), "doorplate_catalog"); loadErr == nil {
				_ = json.Unmarshal(raw, &rows)
			}
			known = append(known, doorplateCommunities(consolidateDoorplateRows(rows))...)
			plans, planErr := loadDoorplatePlans(r.Context(), store)
			if planErr != nil {
				err = planErr
			} else {
				for existing := range plans {
					known = append(known, existing)
				}
				var canonical string
				canonical, err = resolveCommunityName(name, known)
				if err == nil {
					for existing := range locations {
						if normalizeCommunityName(existing) == normalizeCommunityName(canonical) {
							delete(locations, existing)
						}
					}
					locations[canonical] = location
				}
			}
			if err == nil {
				payload, _ := json.Marshal(locations)
				err = store.UpsertAppConfig(r.Context(), communityLocationKey, payload)
			}
		}
		if err != nil {
			writeError(w, 500, "保存小区位置失败")
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": consolidateCommunityLocations(locations)})
		return
	}
	locations, err := loadCommunityLocations(r.Context(), store)
	if err != nil {
		writeError(w, 500, "读取小区位置失败")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": consolidateCommunityLocations(locations)})
}

func consolidateCommunityLocations(locations map[string]communityLocation) map[string]communityLocation {
	result := make(map[string]communityLocation, len(locations))
	keys := make([]string, 0, len(locations))
	for name := range locations {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		canonical, err := resolveCommunityName(name, keys)
		if err != nil {
			continue
		}
		// Prefer data stored under the canonical display name. Otherwise the
		// sorted key order makes legacy alias consolidation repeatable.
		if _, exists := result[canonical]; !exists || name == canonical {
			result[canonical] = locations[name]
		}
	}
	return result
}

const communityLocationKey = "doorplate_community_locations"

func loadCommunityLocations(ctx context.Context, store appConfigStore) (map[string]communityLocation, error) {
	locations := map[string]communityLocation{}
	raw, err := store.GetAppConfig(ctx, communityLocationKey)
	if errors.Is(err, sql.ErrNoRows) {
		return locations, nil
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &locations)
	if locations == nil {
		locations = map[string]communityLocation{}
	}
	return locations, err
}
