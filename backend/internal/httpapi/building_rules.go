package httpapi

import (
	"net/http"
	"strings"

	"xiaopi/backend/internal/domain"
)

func (s *Server) listBuildingRules(w http.ResponseWriter, r *http.Request) {
	community := strings.TrimSpace(r.URL.Query().Get("community"))
	if community == "" {
		writeError(w, http.StatusBadRequest, "community is required")
		return
	}
	items, err := s.store.ListBuildingRules(r.Context(), community)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取楼栋规则失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func (s *Server) upsertBuildingRule(w http.ResponseWriter, r *http.Request) {
	var input domain.BuildingRule
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "楼栋规则格式不正确")
		return
	}
	input.Community = strings.TrimSpace(input.Community)
	input.BuildingNo = strings.TrimSpace(input.BuildingNo)
	if input.Community == "" || input.BuildingNo == "" || input.UnitCount < 0 || input.UnitCount > 20 || input.FloorCount < 0 || input.FloorCount > 99 || input.HouseholdsPerFloor < 0 || input.HouseholdsPerFloor > 20 {
		writeError(w, http.StatusBadRequest, "请填写有效的小区和楼栋规则")
		return
	}
	configured := input.UnitCount > 0 || input.FloorCount > 0 || input.HouseholdsPerFloor > 0
	if configured && (input.UnitCount < 1 || input.FloorCount < 1 || input.HouseholdsPerFloor < 1) {
		writeError(w, http.StatusBadRequest, "单元数、层数和每层户数需完整填写")
		return
	}
	if input.FloorStart == 0 {
		input.FloorStart = 1
	}
	if err := s.store.UpsertBuildingRule(r.Context(), input); err != nil {
		writeError(w, http.StatusInternalServerError, "保存楼栋规则失败")
		return
	}
	s.logOperation(r.Context(), claimsFromContext(r.Context()).Subject, "building_rule", input.Community+"/"+input.BuildingNo, "更新楼栋规则", "")
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": input})
}
