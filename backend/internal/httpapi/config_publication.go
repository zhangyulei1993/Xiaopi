package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"xiaopi/backend/internal/domain"
)

type configPublicationStore interface {
	PublishPublicConfig(context.Context, string, domain.PublicConfig, string, string) (domain.ConfigPublication, error)
	ListConfigPublications(context.Context, string) ([]domain.ConfigPublication, error)
}

func (s *Server) configState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	config, err := s.loadPublicConfig(r.Context())
	if err != nil {
		writeError(w, 500, "读取配置失败")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"config": config, "revision": domain.ConfigRevision(config)}})
}
func (s *Server) configHistory(w http.ResponseWriter, r *http.Request) {
	store, ok := s.store.(configPublicationStore)
	if !ok {
		writeError(w, 503, "发布历史存储不可用")
		return
	}
	entries, err := store.ListConfigPublications(r.Context(), r.URL.Query().Get("before"))
	if err != nil {
		writeError(w, 500, "读取发布历史失败")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": entries})
}
func (s *Server) publishConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config           domain.PublicConfig `json:"config"`
		ExpectedRevision string              `json:"expectedRevision"`
		RestoredFrom     string              `json:"restoredFrom"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, 400, "配置格式错误")
		return
	}
	if input.ExpectedRevision == "" {
		writeError(w, 428, "请先读取最新配置再发布")
		return
	}
	if err := canonicalizeFeaturedChannelCommunities(r.Context(), s.store, &input.Config); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	input.Config.Normalize()
	if err := input.Config.Validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	store, ok := s.store.(configPublicationStore)
	if !ok {
		writeError(w, 503, "发布历史存储不可用，未保存配置")
		return
	}
	if domain.ConfigRevision(input.Config) == input.ExpectedRevision {
		writeError(w, 400, "没有待发布修改")
		return
	}
	if input.RestoredFrom != "" {
		historyStore, ok := s.store.(appConfigStore)
		if !ok || !regexp.MustCompile(`^[0-9]{20}$`).MatchString(input.RestoredFrom) {
			writeError(w, 400, "历史恢复来源无效")
			return
		}
		raw, err := historyStore.GetAppConfig(r.Context(), "miniapp_publish_"+input.RestoredFrom)
		var source domain.ConfigPublication
		if err != nil || json.Unmarshal(raw, &source) != nil || source.Status == "failed" {
			writeError(w, 400, "历史恢复来源不存在")
			return
		}
	}
	entry, err := store.PublishPublicConfig(r.Context(), input.ExpectedRevision, input.Config, claimsFromContext(r.Context()).Subject, input.RestoredFrom)
	if errors.Is(err, domain.ErrConfigConflict) {
		s.recordConfigFailure(r, input.Config, input.RestoredFrom, "版本冲突，未生效")
		writeError(w, 409, "其他管理员已发布配置，请核对冲突字段后重试")
		return
	}
	if err != nil {
		s.recordConfigFailure(r, input.Config, input.RestoredFrom, "存储错误，未确认生效")
		writeError(w, 500, "未确认发布成功，请刷新配置和历史核对后再试")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": entry})
}

func canonicalizeFeaturedChannelCommunities(ctx context.Context, rawStore any, config *domain.PublicConfig) error {
	usesCatalogSelection := false
	for _, channel := range config.FeaturedChannels {
		if strings.TrimSpace(channel.Community) != "" {
			usesCatalogSelection = true
			break
		}
	}
	// Existing channels may be anchored to a verified Tencent map point instead
	// of a doorplate community. Preserve those map-based channels so a new Admin
	// client can publish unrelated changes without inventing a community match.
	if !usesCatalogSelection {
		for _, channel := range config.FeaturedChannels {
			if strings.TrimSpace(channel.Keyword) == "" || !validFeaturedChannelLocation(channel) {
				return errors.New("精选频道必须选择门牌库小区或保留有效地图定位")
			}
		}
		return nil
	}
	store, ok := rawStore.(appConfigStore)
	if !ok {
		return errors.New("读取门牌库失败，无法验证精选频道小区")
	}
	raw, err := store.GetAppConfig(ctx, "doorplate_catalog")
	if err != nil {
		return errors.New("门牌库暂不可用，无法发布精选频道")
	}
	var rows []doorplate
	if err := json.Unmarshal(raw, &rows); err != nil {
		return errors.New("门牌库数据格式异常，无法发布精选频道")
	}
	known := doorplateCommunities(consolidateDoorplateRows(rows))
	if len(known) == 0 {
		return errors.New("门牌库还没有小区数据，无法配置精选频道")
	}
	for index := range config.FeaturedChannels {
		channel := &config.FeaturedChannels[index]
		requested := strings.TrimSpace(channel.Community)
		if requested == "" {
			if strings.TrimSpace(channel.Keyword) != "" && validFeaturedChannelLocation(*channel) {
				continue
			}
			return errors.New("精选频道必须选择门牌库小区或保留有效地图定位")
		}
		canonical, _ := resolveCommunityName(requested, known)
		found := false
		for _, candidate := range known {
			if normalizeCommunityName(candidate) == normalizeCommunityName(requested) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("精选频道关联小区必须从门牌库选择：" + requested)
		}
		channel.Community = canonical
		channel.Keyword = canonical
	}
	return nil
}

func validFeaturedChannelLocation(channel domain.FeaturedChannel) bool {
	return channel.Latitude != 0 && channel.Longitude != 0 && channel.Latitude >= -90 && channel.Latitude <= 90 && channel.Longitude >= -180 && channel.Longitude <= 180
}

func (s *Server) recordConfigFailure(r *http.Request, after domain.PublicConfig, source, message string) {
	recorder, ok := s.store.(interface {
		RecordConfigFailure(context.Context, domain.ConfigPublication) error
	})
	if !ok {
		return
	}
	before, err := s.loadPublicConfig(r.Context())
	if err != nil {
		return
	}
	_ = recorder.RecordConfigFailure(r.Context(), domain.ConfigPublication{Operator: claimsFromContext(r.Context()).Subject, Before: before, After: after, RestoredFrom: source, Error: message})
}
