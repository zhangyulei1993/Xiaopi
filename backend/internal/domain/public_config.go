package domain

import "strings"

type MiniappLocation struct {
	Label   string `json:"label"`
	Keyword string `json:"keyword"`
}

type FeaturedChannel struct {
	Icon     string `json:"icon"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Keyword  string `json:"keyword"`
}

type PublicConfig struct {
	HeroTitle        string            `json:"heroTitle"`
	HeroSubtitle     string            `json:"heroSubtitle"`
	HeroDescription  string            `json:"heroDescription"`
	Notice           string            `json:"notice"`
	ContactPhone     string            `json:"contactPhone"`
	Locations        []MiniappLocation `json:"locations"`
	FeaturedChannels []FeaturedChannel `json:"featuredChannels"`
}

func DefaultPublicConfig() PublicConfig {
	return PublicConfig{
		HeroTitle:       "小皮租房",
		HeroSubtitle:    "在武汉，住得更省心",
		HeroDescription: "真实房源 · 可靠服务",
		Notice:          "签约前请核对合同条款与费用明细，保障自身权益。",
		Locations: []MiniappLocation{
			{Label: "光谷", Keyword: "光谷"},
			{Label: "金融港", Keyword: "金融港"},
			{Label: "街道口", Keyword: "街道口"},
		},
		FeaturedChannels: []FeaturedChannel{
			{Icon: "🏢", Title: "金融港", Subtitle: "通勤优选", Keyword: "金融港"},
			{Icon: "🌳", Title: "光谷", Subtitle: "品质房源", Keyword: "光谷"},
			{Icon: "🌆", Title: "街道口", Subtitle: "生活便利", Keyword: "街道口"},
		},
	}
}

func (config *PublicConfig) Normalize() {
	config.HeroTitle = strings.TrimSpace(config.HeroTitle)
	config.HeroSubtitle = strings.TrimSpace(config.HeroSubtitle)
	config.HeroDescription = strings.TrimSpace(config.HeroDescription)
	config.Notice = strings.TrimSpace(config.Notice)
	config.ContactPhone = strings.TrimSpace(config.ContactPhone)
	for index := range config.Locations {
		config.Locations[index].Label = strings.TrimSpace(config.Locations[index].Label)
		config.Locations[index].Keyword = strings.TrimSpace(config.Locations[index].Keyword)
	}
	for index := range config.FeaturedChannels {
		config.FeaturedChannels[index].Icon = strings.TrimSpace(config.FeaturedChannels[index].Icon)
		config.FeaturedChannels[index].Title = strings.TrimSpace(config.FeaturedChannels[index].Title)
		config.FeaturedChannels[index].Subtitle = strings.TrimSpace(config.FeaturedChannels[index].Subtitle)
		config.FeaturedChannels[index].Keyword = strings.TrimSpace(config.FeaturedChannels[index].Keyword)
	}
}
