package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xiaopi/backend/internal/domain"
)

const materialCategoriesConfigKey = "property_material_categories"
const materialCategoriesSchemaVersion = 2

var legacyMaterialCategories = map[string]bool{"精选房源": true, "社区房源": true, "待整理": true}

type materialCategoryConfig struct {
	Version     int      `json:"version"`
	Communities []string `json:"communities"`
}

func (s *Server) listMaterialLibrary(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		items := make([]domain.Property, 0)
		for _, item := range s.properties {
			if item.IsMaterial {
				items = append(items, item)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
		return
	}
	items, err := s.store.ListProperties(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "素材库加载失败")
		return
	}
	materials := make([]domain.Property, 0)
	for _, item := range items {
		if item.IsMaterial {
			materials = append(materials, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": materials})
}

func (s *Server) createMaterial(w http.ResponseWriter, r *http.Request) {
	var input struct {
		domain.Property
		CreateKey string `json:"createKey"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "素材信息无效")
		return
	}
	p := input.Property
	if len(input.CreateKey) > 100 {
		writeError(w, 400, "素材任务编号无效")
		return
	}
	p.Title = strings.TrimSpace(p.Title)
	p.MaterialCategory = strings.TrimSpace(p.MaterialCategory)
	if p.Title == "" {
		p.Title = p.Community
	}
	if p.Title == "" || p.MaterialCategory == "" || p.RentMonthly < 0 {
		writeError(w, http.StatusBadRequest, "请选择小区分类并填写素材名称，租金不能小于 0")
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "data store unavailable")
		return
	}
	categories, err := s.readMaterialCategories(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "小区分类读取失败")
		return
	}
	canonical, _ := resolveCommunityName(p.MaterialCategory, categories)
	if !containsString(categories, canonical) {
		writeError(w, http.StatusBadRequest, "请先创建或选择有效的小区分类")
		return
	}
	if strings.TrimSpace(p.Title) == p.MaterialCategory {
		p.Title = canonical
	}
	p.MaterialCategory = canonical
	p.Community = canonical
	if strings.TrimSpace(p.Location) == "" {
		p.Location = p.Community
	}
	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "素材创建失败")
		return
	}

	now := time.Now()
	p.ID = id
	p.IsMaterial = true
	p.Status = domain.PropertyDraft
	p.OwnerID = claimsFromContext(r.Context()).Subject
	p.CreatedAt = now
	p.UpdatedAt = now
	p, err = s.createPropertyWithTask(r.Context(), p, input.CreateKey)
	if err != nil {
		if errors.Is(err, domain.ErrPropertyEditConflict) {
			writeError(w, http.StatusConflict, "小区分类已更新，请刷新后重新选择")
			return
		}
		writeError(w, http.StatusInternalServerError, "素材保存失败")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "material", p.ID, "保存房源素材", fmt.Sprintf("%s · %s", p.MaterialCategory, p.Title))
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success", "data": p})
}

func (s *Server) deleteMaterial(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("propertyID"))
	if id == "" || s.store == nil {
		writeError(w, http.StatusBadRequest, "素材编号无效")
		return
	}
	items, err := s.store.ListProperties(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "素材读取失败")
		return
	}
	var target *domain.Property
	for index := range items {
		if items[index].ID == id && items[index].IsMaterial {
			target = &items[index]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "素材不存在")
		return
	}
	if err := s.store.DeleteProperty(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "素材删除失败")
		return
	}
	// Keep committed files: converted listings may still reference the same
	// URL, including while conversion or another edit is in flight.
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "material", id, "删除房源素材", fmt.Sprintf("%s · %s", target.MaterialCategory, target.Title))
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
}

func (s *Server) convertMaterial(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("propertyID"))
	if id == "" || s.store == nil {
		writeError(w, http.StatusBadRequest, "素材编号无效")
		return
	}
	items, err := s.store.ListProperties(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "素材读取失败")
		return
	}
	for _, item := range items {
		if item.ID != id || !item.IsMaterial {
			continue
		}
		if item.ConvertedPropertyID != "" {
			for _, converted := range items {
				if converted.ID == item.ConvertedPropertyID && !converted.IsMaterial {
					writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": converted})
					return
				}
			}
			// The linked draft may have been deleted. Keep the retained source usable
			// and allow it to create a fresh draft below.
		}
		converted := item
		propertyID, idErr := newID()
		if idErr != nil {
			writeError(w, http.StatusInternalServerError, "房源草稿创建失败")
			return
		}
		converted.ID = propertyID
		converted.IsMaterial = false
		converted.MaterialCategory = ""
		converted.ConvertedPropertyID = ""
		converted.Status = domain.PropertyDraft
		converted.CreatedAt = time.Now()
		converted.UpdatedAt = converted.CreatedAt
		if converted.Community == "" {
			converted.Community = converted.Title
		}
		if converted.Title == "" {
			converted.Title = converted.Community
		}
		converted.Media = make([]domain.PropertyMedia, 0, len(item.Media))
		for index, media := range item.Media {
			mediaID, err := newID()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "素材媒体复制失败")
				return
			}
			media.ID, media.PropertyID, media.Sort = mediaID, propertyID, index
			converted.Media = append(converted.Media, media)
		}
		store, ok := s.store.(interface {
			CommitMaterialConversion(context.Context, domain.Property, domain.Property) error
		})
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "素材转换暂不可用")
			return
		}
		if err := store.CommitMaterialConversion(r.Context(), item, converted); err != nil {
			if errors.Is(err, domain.ErrMaterialConversionChanged) {
				writeError(w, http.StatusConflict, "素材已更新，请重试转换")
				return
			}
			writeError(w, http.StatusInternalServerError, "素材转换失败")
			return
		}
		s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "material", id, "素材转为房源", fmt.Sprintf("%s · %s", item.Community, item.Title))
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": converted})
		return
	}
	writeError(w, http.StatusNotFound, "素材不存在")
}

func (s *Server) materialCategories(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(appConfigStore)
	if !ok {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": []string{}})
			return
		}
		writeError(w, http.StatusServiceUnavailable, "小区分类存储不可用")
		return
	}
	if r.Method == http.MethodGet {
		categories, err := s.readMaterialCategories(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "素材分类读取失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": categories})
		return
	}
	var input struct {
		Categories []string `json:"categories"`
		Add        string   `json:"add"`
		Remove     string   `json:"remove"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "分类数据无效")
		return
	}
	if _, err := s.readMaterialCategories(r); err != nil {
		writeError(w, http.StatusInternalServerError, "旧小区分类迁移失败")
		return
	}
	before, err := store.GetAppConfig(r.Context(), materialCategoriesConfigKey)
	if err != nil {
		writeError(w, 500, "分类读取失败")
		return
	}
	var baseline materialCategoryConfig
	if json.Unmarshal(before, &baseline) != nil {
		writeError(w, 500, "分类格式异常")
		return
	}
	removed := []string{}
	if input.Add != "" || input.Remove != "" {
		input.Categories = []string{}
		for _, name := range baseline.Communities {
			if input.Remove != "" && normalizeCommunityName(name) == normalizeCommunityName(input.Remove) {
				removed = append(removed, name)
			} else {
				input.Categories = append(input.Categories, name)
			}
		}
		if input.Add != "" {
			input.Categories = append(input.Categories, input.Add)
		}
	} else {
		for _, name := range baseline.Communities {
			found := false
			for _, candidate := range input.Categories {
				if normalizeCommunityName(name) == normalizeCommunityName(candidate) {
					found = true
					break
				}
			}
			if !found {
				removed = append(removed, name)
			}
		}
	}
	seen := map[string]bool{}
	categories := make([]string, 0, len(input.Categories))
	for _, category := range input.Categories {
		category = strings.TrimSpace(category)
		identity := normalizeCommunityName(category)
		if identity == "" || len([]rune(category)) > 60 || seen[identity] {
			continue
		}
		seen[identity] = true
		categories = append(categories, category)
	}
	allowed := make(map[string]bool, len(categories))
	for _, category := range categories {
		allowed[normalizeCommunityName(category)] = true
	}
	if s.store != nil {
		properties, listErr := s.store.ListProperties(r.Context())
		if listErr != nil {
			writeError(w, http.StatusInternalServerError, "素材分类更新失败")
			return
		}
		for _, property := range properties {
			if property.IsMaterial && property.MaterialCategory != "" && !allowed[normalizeCommunityName(property.MaterialCategory)] {
				writeError(w, http.StatusConflict, "该小区分类仍有素材，请先删除素材或转为正式房源，再移除分类")
				return
			}
		}
	}
	payload, _ := json.Marshal(materialCategoryConfig{Version: materialCategoriesSchemaVersion, Communities: categories})
	atomic, ok := s.store.(interface {
		ChangeMaterialCategoriesAtomic(context.Context, []byte, []byte, []string) error
	})
	if !ok {
		writeError(w, 503, "分类保存暂不可用")
		return
	}
	if err := atomic.ChangeMaterialCategoriesAtomic(r.Context(), before, payload, removed); err != nil {
		writeError(w, 409, "分类已更新或仍有素材，请刷新后重试")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": categories})
}

func (s *Server) renameMaterialCategory(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(appConfigStore)
	if !ok || s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "小区分类存储不可用")
		return
	}
	var input struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "分类名称无效")
		return
	}
	from, to := strings.TrimSpace(input.From), strings.TrimSpace(input.To)
	if from == "" || to == "" || len([]rune(to)) > 60 {
		writeError(w, http.StatusBadRequest, "请输入 1 到 60 个字符的小区名称")
		return
	}
	if from == to {
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success"})
		return
	}
	categories, err := s.readMaterialCategories(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "素材分类读取失败")
		return
	}
	fromIndex := -1
	for index, category := range categories {
		if category == from {
			fromIndex = index
			continue
		}
		if normalizeCommunityName(category) == normalizeCommunityName(to) {
			writeError(w, http.StatusConflict, "已存在名称相同的小区分类")
			return
		}
	}
	if fromIndex < 0 {
		writeError(w, http.StatusNotFound, "原小区分类不存在")
		return
	}
	atomic, ok := s.store.(interface {
		RenameMaterialCategoryAtomic(context.Context, string, string, []byte, []byte, []byte, []byte) error
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "素材分类修改暂不可用")
		return
	}
	categoryRaw, err := store.GetAppConfig(r.Context(), materialCategoriesConfigKey)
	var currentConfig materialCategoryConfig
	if err != nil || json.Unmarshal(categoryRaw, &currentConfig) != nil || !equalStrings(currentConfig.Communities, categories) {
		writeError(w, http.StatusConflict, "分类已更新，请刷新后重试")
		return
	}
	locationRaw, err := store.GetAppConfig(r.Context(), communityLocationKey)
	locations := map[string]communityLocation{}
	if (err != nil && !errors.Is(err, sql.ErrNoRows)) || (err == nil && json.Unmarshal(locationRaw, &locations) != nil) {
		writeError(w, http.StatusInternalServerError, "小区位置读取失败")
		return
	}
	if locations == nil {
		locations = map[string]communityLocation{}
	}
	var nextLocations []byte
	for name, position := range consolidateCommunityLocations(locations) {
		if normalizeCommunityName(name) != normalizeCommunityName(from) || normalizeCommunityName(name) == normalizeCommunityName(to) {
			continue
		}
		for target, existing := range locations {
			if normalizeCommunityName(target) == normalizeCommunityName(to) && existing != position {
				writeError(w, http.StatusConflict, "目标名称已关联其他小区位置")
				return
			}
		}
		locations[to] = position
		nextLocations, _ = json.Marshal(locations)
		break
	}
	categories[fromIndex] = to
	payload, _ := json.Marshal(materialCategoryConfig{Version: materialCategoriesSchemaVersion, Communities: categories})
	if err := atomic.RenameMaterialCategoryAtomic(r.Context(), from, to, categoryRaw, locationRaw, payload, nextLocations); err != nil {
		if errors.Is(err, domain.ErrPropertyEditConflict) {
			writeError(w, http.StatusConflict, "分类或小区位置已更新，请刷新后重试")
			return
		}
		writeError(w, http.StatusInternalServerError, "素材分类修改失败")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "material_category", from, "修改素材小区分类", from+" → "+to)
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"from": from, "to": to}})
}

func (s *Server) readMaterialCategories(r *http.Request) ([]string, error) {
	store, ok := s.store.(appConfigStore)
	if !ok {
		return nil, nil
	}
	raw, err := store.GetAppConfig(r.Context(), materialCategoriesConfigKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var categories []string
	versioned := false
	if len(raw) > 0 {
		var config materialCategoryConfig
		if json.Unmarshal(raw, &config) == nil && config.Version == materialCategoriesSchemaVersion {
			categories = config.Communities
			versioned = true
		} else {
			_ = json.Unmarshal(raw, &categories)
		}
	}
	updates := []domain.Property{}
	canonical := map[string]string{}
	clean := make([]string, 0, len(categories))
	for _, category := range categories {
		category = strings.TrimSpace(category)
		identity := normalizeCommunityName(category)
		if identity != "" && (versioned || !legacyMaterialCategories[category]) && canonical[identity] == "" {
			canonical[identity] = category
			clean = append(clean, category)
		}
	}
	if s.store != nil {
		properties, listErr := s.store.ListProperties(r.Context())
		if listErr != nil {
			return nil, listErr
		}
		for _, property := range properties {
			if !property.IsMaterial {
				continue
			}
			community := strings.TrimSpace(property.Community)
			category := strings.TrimSpace(property.MaterialCategory)
			dirty := false
			if legacyMaterialCategories[category] && community == "" {
				property.MaterialCategory = ""
				category = ""
				dirty = true
			}
			if community == "" && category != "" && (versioned || !legacyMaterialCategories[category]) {
				community = category
				property.Community = community
				dirty = true
			}
			if community != "" && category != community {
				property.MaterialCategory = community
				dirty = true
			}
			if community != "" && strings.TrimSpace(property.Location) == "" {
				property.Location = community
				dirty = true
			}
			if identity := normalizeCommunityName(community); identity != "" {
				name := canonical[identity]
				if name == "" {
					name = community
					canonical[identity] = name
					clean = append(clean, name)
				}
				if property.Community != name || property.MaterialCategory != name {
					if property.Title == property.Community {
						property.Title = name
					}
					if property.Location == property.Community {
						property.Location = name
					}
					property.Community, property.MaterialCategory = name, name
					dirty = true
				}
			}
			if dirty {
				updates = append(updates, property)
			}
		}
	}
	if len(updates) > 0 || !versioned || !equalStrings(categories, clean) {
		payload, _ := json.Marshal(materialCategoryConfig{Version: materialCategoriesSchemaVersion, Communities: clean})
		if atomic, ok := s.store.(interface {
			NormalizeMaterialCategoriesAtomic(context.Context, []byte, []byte, []domain.Property) error
		}); ok {
			if err := atomic.NormalizeMaterialCategoriesAtomic(r.Context(), raw, payload, updates); err != nil {
				return nil, err
			}
		} else {
			// Stores used by legacy migration tests have no transaction support.
			for _, property := range updates {
				if err := s.store.UpdateProperty(r.Context(), property); err != nil {
					return nil, err
				}
			}
			if err := store.UpsertAppConfig(r.Context(), materialCategoriesConfigKey, payload); err != nil {
				return nil, err
			}
		}
	}
	return clean, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
