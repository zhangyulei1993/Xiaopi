package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var publicConfigPhonePattern = regexp.MustCompile(`^\+?\d{6,20}$`)

type MiniappLocation struct {
	Label   string `json:"label"`
	Keyword string `json:"keyword"`
}

type FeaturedChannel struct {
	Icon      string  `json:"icon"`
	Title     string  `json:"title"`
	Subtitle  string  `json:"subtitle"`
	Keyword   string  `json:"keyword"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radiusKm"`
}

type PublicConfig struct {
	HeroTitle               string            `json:"heroTitle"`
	HeroSubtitle            string            `json:"heroSubtitle"`
	HeroDescription         string            `json:"heroDescription"`
	Notice                  string            `json:"notice"`
	ContactPhone            string            `json:"contactPhone"`
	RelatedPropertyRadiusKm float64           `json:"relatedPropertyRadiusKm"`
	Locations               []MiniappLocation `json:"locations"`
	FeaturedChannels        []FeaturedChannel `json:"featuredChannels"`
}

func DefaultPublicConfig() PublicConfig {
	return PublicConfig{
		HeroTitle:               "小皮租房",
		HeroSubtitle:            "在武汉，住得更省心",
		HeroDescription:         "真实房源 · 可靠服务",
		Notice:                  "签约前请核对合同条款与费用明细，保障自身权益。",
		RelatedPropertyRadiusKm: 3,
		Locations: []MiniappLocation{
			{Label: "光谷", Keyword: "光谷"},
			{Label: "金融港", Keyword: "金融港"},
			{Label: "街道口", Keyword: "街道口"},
		},
		FeaturedChannels: []FeaturedChannel{
			{Icon: "commute", Title: "金融港", Subtitle: "通勤优选", Keyword: "金融港", Latitude: 30.456040, Longitude: 114.425337, RadiusKm: 10},
			{Icon: "campus", Title: "光谷", Subtitle: "品质房源", Keyword: "光谷", RadiusKm: 5},
			{Icon: "life", Title: "街道口", Subtitle: "生活便利", Keyword: "街道口", RadiusKm: 5},
		},
	}
}

func (config *PublicConfig) Normalize() {
	config.HeroTitle = strings.TrimSpace(config.HeroTitle)
	config.HeroSubtitle = strings.TrimSpace(config.HeroSubtitle)
	config.HeroDescription = strings.TrimSpace(config.HeroDescription)
	config.Notice = strings.TrimSpace(config.Notice)
	config.ContactPhone = strings.TrimSpace(config.ContactPhone)
	if config.RelatedPropertyRadiusKm <= 0 {
		config.RelatedPropertyRadiusKm = 3
	}
	for index := range config.Locations {
		config.Locations[index].Label = strings.TrimSpace(config.Locations[index].Label)
		config.Locations[index].Keyword = strings.TrimSpace(config.Locations[index].Keyword)
	}
	for index := range config.FeaturedChannels {
		config.FeaturedChannels[index].Icon = strings.TrimSpace(config.FeaturedChannels[index].Icon)
		config.FeaturedChannels[index].Title = strings.TrimSpace(config.FeaturedChannels[index].Title)
		config.FeaturedChannels[index].Subtitle = strings.TrimSpace(config.FeaturedChannels[index].Subtitle)
		config.FeaturedChannels[index].Keyword = strings.TrimSpace(config.FeaturedChannels[index].Keyword)
		if config.FeaturedChannels[index].RadiusKm <= 0 {
			config.FeaturedChannels[index].RadiusKm = 5
		}
	}
}

func (config PublicConfig) Validate() error {
	if config.HeroTitle == "" {
		return errors.New("品牌名称不能为空")
	}
	if utf8.RuneCountInString(config.HeroTitle) > 20 {
		return errors.New("品牌名称不能超过 20 个字符")
	}
	if config.HeroSubtitle == "" {
		return errors.New("首页主标题不能为空")
	}
	if utf8.RuneCountInString(config.HeroSubtitle) > 30 {
		return errors.New("首页主标题不能超过 30 个字符")
	}
	if config.HeroDescription == "" {
		return errors.New("首页副标题不能为空")
	}
	if utf8.RuneCountInString(config.HeroDescription) > 40 {
		return errors.New("首页副标题不能超过 40 个字符")
	}
	if utf8.RuneCountInString(config.Notice) > 120 {
		return errors.New("首页公告不能超过 120 个字符")
	}
	if config.ContactPhone != "" && !publicConfigPhonePattern.MatchString(config.ContactPhone) {
		return errors.New("联系电话格式不正确")
	}
	if config.RelatedPropertyRadiusKm < 0.5 || config.RelatedPropertyRadiusKm > 20 {
		return errors.New("附近房源关联距离必须在 0.5 到 20 公里之间")
	}
	if len(config.Locations) == 0 || len(config.Locations) > 20 {
		return errors.New("地理位置数量必须在 1 到 20 个之间")
	}
	locationKeywords := make(map[string]struct{}, len(config.Locations))
	for index, location := range config.Locations {
		if location.Label == "" || location.Keyword == "" {
			return fmt.Errorf("第 %d 个地理位置缺少名称或搜索关键词", index+1)
		}
		if utf8.RuneCountInString(location.Label) > 12 || utf8.RuneCountInString(location.Keyword) > 20 {
			return fmt.Errorf("第 %d 个地理位置内容过长", index+1)
		}
		if _, exists := locationKeywords[location.Keyword]; exists {
			return errors.New("地理位置搜索关键词不能重复")
		}
		locationKeywords[location.Keyword] = struct{}{}
	}
	if len(config.FeaturedChannels) == 0 || len(config.FeaturedChannels) > 12 {
		return errors.New("精选频道数量必须在 1 到 12 个之间")
	}
	channelKeywords := make(map[string]struct{}, len(config.FeaturedChannels))
	for index, channel := range config.FeaturedChannels {
		if channel.Title == "" || channel.Subtitle == "" || channel.Keyword == "" {
			return fmt.Errorf("第 %d 个精选频道信息不完整", index+1)
		}
		if utf8.RuneCountInString(channel.Icon) > 16 || utf8.RuneCountInString(channel.Title) > 12 || utf8.RuneCountInString(channel.Subtitle) > 20 || utf8.RuneCountInString(channel.Keyword) > 20 {
			return fmt.Errorf("第 %d 个精选频道内容过长", index+1)
		}
		if channel.RadiusKm <= 0 || channel.RadiusKm > 100 {
			return fmt.Errorf("第 %d 个精选频道关联半径必须在 0 到 100 公里之间", index+1)
		}
		if (channel.Latitude == 0) != (channel.Longitude == 0) || channel.Latitude < -90 || channel.Latitude > 90 || channel.Longitude < -180 || channel.Longitude > 180 {
			return fmt.Errorf("第 %d 个精选频道中心坐标不正确", index+1)
		}
		if _, exists := channelKeywords[channel.Keyword]; exists {
			return errors.New("精选频道搜索关键词不能重复")
		}
		channelKeywords[channel.Keyword] = struct{}{}
	}
	return nil
}
