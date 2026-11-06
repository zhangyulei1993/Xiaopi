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

// PagePresentation is intentionally small: it gives operations control over
// page-facing copy and an optional background without making layout editable.
type PagePresentation struct {
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	BackgroundImage string `json:"backgroundImage"`
}

type ShareCardConfig struct {
	TitlePrefix  string   `json:"titlePrefix"`
	TitleFields  []string `json:"titleFields"`
	PosterFields []string `json:"posterFields"`
}

type ListingTitleConfig struct {
	EnabledFields []string `json:"enabledFields"`
}

type PublicConfig struct {
	HeroTitle               string                      `json:"heroTitle"`
	HeroSubtitle            string                      `json:"heroSubtitle"`
	HeroDescription         string                      `json:"heroDescription"`
	Notice                  string                      `json:"notice"`
	ContactPhone            string                      `json:"contactPhone"`
	RelatedPropertyRadiusKm float64                     `json:"relatedPropertyRadiusKm"`
	Locations               []MiniappLocation           `json:"locations"`
	FeaturedChannels        []FeaturedChannel           `json:"featuredChannels"`
	PagePresentations       map[string]PagePresentation `json:"pagePresentations"`
	ShareCard               ShareCardConfig             `json:"shareCard"`
	ListingTitle            ListingTitleConfig          `json:"listingTitle"`
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
			{Icon: "commute", Title: "金融港", Subtitle: "通勤优选", Keyword: "金融港", Latitude: 30.456040, Longitude: 114.425337, RadiusKm: 3},
			{Icon: "campus", Title: "光谷", Subtitle: "品质房源", Keyword: "光谷", RadiusKm: 3},
			{Icon: "life", Title: "街道口", Subtitle: "生活便利", Keyword: "街道口", RadiusKm: 3},
		},
		PagePresentations: map[string]PagePresentation{
			"home":              {Title: "今天想住哪里？", Subtitle: "搜索小区、地铁或商圈"},
			"properties":        {Title: "找一处合适的家", Subtitle: "按地区和预算筛选"},
			"propertyDetail":    {Title: "房源详情", Subtitle: "真实房源，放心查看"},
			"requests":          {Title: "小皮帮你找房", Subtitle: "告诉我们你的找房需求"},
			"publish":           {Title: "发布房源", Subtitle: "提交真实房源线索"},
			"me":                {Title: "我的", Subtitle: "管理关注与找房需求"},
			"footprints":        {Title: "浏览足迹", Subtitle: "最近看过的房源"},
			"adminWorkbench":    {Title: "小皮工作台", Subtitle: "房源运营与数据处理"},
			"adminPropertyEdit": {Title: "编辑房源", Subtitle: "完善房源信息后发布"},
			"authLogin":         {Title: "微信登录", Subtitle: "登录后管理你的找房服务"},
		},
		ShareCard: ShareCardConfig{
			TitleFields:  []string{"community", "layout"},
			PosterFields: []string{"rentMonthly", "depositPayment", "community", "areaSqm", "floorInfo", "availableDate"},
		},
		ListingTitle: ListingTitleConfig{
			// 门牌属于精确住址信息，默认不向普通用户列表公开；管理员可在后台
			// 明确确认风险后开启独立的“门牌标签”。
			EnabledFields: []string{"community", "floor", "layout", "rentType", "listingSource", "fee", "area"},
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
			config.FeaturedChannels[index].RadiusKm = 3
		}
	}
	if config.PagePresentations == nil {
		config.PagePresentations = map[string]PagePresentation{}
	}
	for key, fallback := range DefaultPublicConfig().PagePresentations {
		if _, exists := config.PagePresentations[key]; !exists {
			config.PagePresentations[key] = fallback
		}
	}
	for key, presentation := range config.PagePresentations {
		presentation.Title = strings.TrimSpace(presentation.Title)
		presentation.Subtitle = strings.TrimSpace(presentation.Subtitle)
		presentation.BackgroundImage = strings.TrimSpace(presentation.BackgroundImage)
		config.PagePresentations[key] = presentation
	}
	config.ShareCard.TitlePrefix = strings.TrimSpace(config.ShareCard.TitlePrefix)
	if len(config.ShareCard.TitleFields) == 0 {
		config.ShareCard.TitleFields = append([]string(nil), DefaultPublicConfig().ShareCard.TitleFields...)
	}
	if len(config.ShareCard.PosterFields) == 0 {
		config.ShareCard.PosterFields = append([]string(nil), DefaultPublicConfig().ShareCard.PosterFields...)
	}
	if len(config.ListingTitle.EnabledFields) == 0 {
		config.ListingTitle.EnabledFields = append([]string(nil), DefaultPublicConfig().ListingTitle.EnabledFields...)
	}
}

var shareCardFieldLabels = map[string]string{
	"community": "小区", "layout": "户型/房间", "roomNo": "门牌", "rentMonthly": "月租",
	"listingSource": "直租/转租", "rentType": "整租/合租", "depositPayment": "押付方式",
	"areaSqm": "面积", "floorInfo": "楼层", "availableDate": "入住时间",
}

var listingTitleFieldLabels = map[string]string{
	"community": "小区", "unit": "门牌", "floor": "楼层", "layout": "户型", "rentType": "整租/合租", "listingSource": "直租/转租", "fee": "中介费", "area": "面积",
}

func validateShareFields(fields []string, minimum, maximum int, name string) error {
	if len(fields) < minimum || len(fields) > maximum {
		return fmt.Errorf("%s字段数量必须在 %d 到 %d 个之间", name, minimum, maximum)
	}
	seen := map[string]struct{}{}
	for _, field := range fields {
		if _, exists := shareCardFieldLabels[field]; !exists {
			return fmt.Errorf("%s包含不支持的字段", name)
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf("%s字段不能重复", name)
		}
		seen[field] = struct{}{}
	}
	return nil
}

func validateListingTitleFields(fields []string) error {
	if len(fields) == 0 || len(fields) > len(listingTitleFieldLabels) {
		return fmt.Errorf("列表标签数量必须在 1 到 %d 个之间", len(listingTitleFieldLabels))
	}
	seen := map[string]struct{}{}
	for _, field := range fields {
		if _, exists := listingTitleFieldLabels[field]; !exists {
			return errors.New("列表标签包含不支持的字段")
		}
		if _, exists := seen[field]; exists {
			return errors.New("列表标签不能重复")
		}
		seen[field] = struct{}{}
	}
	return nil
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
	if utf8.RuneCountInString(config.ShareCard.TitlePrefix) > 12 {
		return errors.New("分享标题前缀不能超过 12 个字符")
	}
	if err := validateShareFields(config.ShareCard.TitleFields, 1, 4, "分享标题"); err != nil {
		return err
	}
	if err := validateShareFields(config.ShareCard.PosterFields, 3, 8, "分享海报"); err != nil {
		return err
	}
	if err := validateListingTitleFields(config.ListingTitle.EnabledFields); err != nil {
		return err
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
	for _, key := range []string{"home", "properties", "propertyDetail", "requests", "publish", "me", "adminWorkbench", "adminPropertyEdit", "authLogin"} {
		presentation := config.PagePresentations[key]
		if utf8.RuneCountInString(presentation.Title) > 30 || utf8.RuneCountInString(presentation.Subtitle) > 60 || utf8.RuneCountInString(presentation.BackgroundImage) > 300 {
			return fmt.Errorf("%s 页面展示配置内容过长", key)
		}
	}
	return nil
}
