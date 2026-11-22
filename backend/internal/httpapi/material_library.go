package httpapi

import (
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
	var p domain.Property
	if err := decodeJSON(w, r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "素材信息无效")
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
	if !containsString(categories, p.MaterialCategory) {
		writeError(w, http.StatusBadRequest, "请先创建或选择有效的小区分类")
		return
	}
	p.Community = p.MaterialCategory
	p.Location = p.Community
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
	if err := s.store.CreateProperty(r.Context(), p); err != nil {
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
	for _, media := range target.Media {
		s.removeStoredUpload(media.URL)
	}
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
		item.IsMaterial = false
		item.MaterialCategory = ""
		if item.Community == "" {
			item.Community = item.Title
		}
		if item.Title == "" {
			item.Title = item.Community
		}
		if err := s.store.UpdateProperty(r.Context(), item); err != nil {
			writeError(w, http.StatusInternalServerError, "素材转换失败")
			return
		}
		s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "material", id, "素材转为房源", fmt.Sprintf("%s · %s", item.Community, item.Title))
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": item})
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
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "分类数据无效")
		return
	}
	if _, err := s.readMaterialCategories(r); err != nil {
		writeError(w, http.StatusInternalServerError, "旧小区分类迁移失败")
		return
	}
	seen := map[string]bool{}
	categories := make([]string, 0, len(input.Categories))
	for _, category := range input.Categories {
		category = strings.TrimSpace(category)
		if category == "" || len([]rune(category)) > 60 || seen[category] {
			continue
		}
		seen[category] = true
		categories = append(categories, category)
	}
	allowed := make(map[string]bool, len(categories))
	for _, category := range categories {
		allowed[category] = true
	}
	if s.store != nil {
		properties, listErr := s.store.ListProperties(r.Context())
		if listErr != nil {
			writeError(w, http.StatusInternalServerError, "素材分类更新失败")
			return
		}
		for _, property := range properties {
			if property.IsMaterial && property.MaterialCategory != "" && !allowed[property.MaterialCategory] {
				writeError(w, http.StatusConflict, "该小区分类仍有素材，请先删除素材或转为正式房源，再移除分类")
				return
			}
		}
	}
	payload, _ := json.Marshal(materialCategoryConfig{Version: materialCategoriesSchemaVersion, Communities: categories})
	if err := store.UpsertAppConfig(r.Context(), materialCategoriesConfigKey, payload); err != nil {
		writeError(w, http.StatusInternalServerError, "素材分类保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": categories})
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
	seen := map[string]bool{}
	clean := make([]string, 0, len(categories))
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category != "" && (versioned || !legacyMaterialCategories[category]) && !seen[category] {
			seen[category] = true
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
			if dirty {
				if updateErr := s.store.UpdateProperty(r.Context(), property); updateErr != nil {
					return nil, updateErr
				}
			}
			category = strings.TrimSpace(property.MaterialCategory)
			if category == "" {
				category = community
				if category != "" {
					property.MaterialCategory = category
					if strings.TrimSpace(property.Location) == "" {
						property.Location = category
					}
					if updateErr := s.store.UpdateProperty(r.Context(), property); updateErr != nil {
						return nil, updateErr
					}
				}
			}
			if category != "" && !seen[category] {
				seen[category] = true
				clean = append(clean, category)
			}
		}
	}
	if !versioned || !equalStrings(categories, clean) {
		payload, _ := json.Marshal(materialCategoryConfig{Version: materialCategoriesSchemaVersion, Communities: clean})
		if err := store.UpsertAppConfig(r.Context(), materialCategoriesConfigKey, payload); err != nil {
			return nil, err
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
