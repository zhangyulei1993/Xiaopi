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
	ID      string `json:"id,omitempty"`
	Label   string `json:"label"`
	Keyword string `json:"keyword"`
}

type FeaturedChannel struct {
	ID        string `json:"id,omitempty"`
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
	ChannelTitle string `json:"channelTitle,omitempty"`
	AllChannelsText string `json:"allChannelsText,omitempty"`
	RecommendationTitle string `json:"recommendationTitle,omitempty"`
	ViewAllText string `json:"viewAllText,omitempty"`
	EmptyText string `json:"emptyText,omitempty"`
	NearbyLabel string `json:"nearbyLabel,omitempty"`
}

type PropertyStatusLabels struct {
	Draft string `json:"draft"`
	PendingReview string `json:"pendingReview"`
	Published string `json:"published"`
	Available string `json:"available"`
	Rented string `json:"rented"`
	Offline string `json:"offline"`
}

type ShareCardConfig struct {
	TitlePrefix  string   `json:"titlePrefix"`
	TitleFields  []string `json:"titleFields"`
	PosterFields []string `json:"posterFields"`
}

type ListingTitleConfig struct {
	// CustomerAllowedFields is the operator-managed set of fields the customer
	// perspective is allowed to choose from. EnabledFields is its final visible
	// subset, so operators can prepare choices without exposing every field.
	CustomerAllowedFields []string `json:"customerAllowedFields"`
	// EnabledFields applies to customer-facing cards. A listing can only
	// use the configured result; publishers do not choose tags.
	EnabledFields []string `json:"enabledFields"`
	// CustomerDetailFields independently controls the customer detail page.
	// It is constrained by CustomerAllowedFields just like list labels.
	CustomerDetailFields []string `json:"customerDetailFields"`
	// CustomerShareFields independently controls the public labels used by
	// friend sharing. It is constrained by the customer-facing field pool, so
	// private values cannot leak into sharing.
	CustomerShareFields []string `json:"customerShareFields"`
	// CustomerPosterFields independently controls the public labels used by
	// Moments posters.
	CustomerPosterFields []string `json:"customerPosterFields"`
	// AdminEnabledFields is the optional doorplate supplement for operator
	// cards and details. All other labels are shared with the customer view.
	AdminEnabledFields []string `json:"adminEnabledFields"`
	// ListTagLayout and DetailTagLayout control presentation only. They never
	// alter which fields are public, so a layout change cannot expose data.
	ListTagLayout   string `json:"listTagLayout"`
	DetailTagLayout string `json:"detailTagLayout"`
	// Tag ordering is presentation-only. ListTagTopFields is the first row in
	// grouped cards; the remaining ordered labels flow into the second row.
	ListTagOrder     []string `json:"listTagOrder"`
	ListTagTopFields []string `json:"listTagTopFields"`
	DetailTagOrder   []string `json:"detailTagOrder"`
	// DetailTagTopFields fixes the first tag row on the detail page so a
	// layout does not drift when the device width changes.
	DetailTagTopFields []string `json:"detailTagTopFields"`
}

// PropertyDetailConfig controls the content blocks of a property detail page.
// It deliberately keeps common blocks lightweight while allowing the nearby
// recommendation and facility catalog to carry their own operational rules.
type PropertyDetailModule struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
}

type PropertyDetailNearbyConfig struct {
	RadiusKm float64 `json:"radiusKm"`
	Relation string  `json:"relation"` // distance | channel
	Sort     string  `json:"sort"`     // distance | newest
	Limit    int     `json:"limit"`
}

type PropertyDetailFacility struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Group   string `json:"group"`
	Enabled bool   `json:"enabled"`
}

type PropertyDetailConfig struct {
	Modules    []PropertyDetailModule     `json:"modules"`
	Nearby     PropertyDetailNearbyConfig `json:"nearby"`
	Facilities []PropertyDetailFacility   `json:"facilities"`
}

type PublicConfig struct {
	HeroTitle               string                      `json:"heroTitle"`
	ContactPhone            string                      `json:"contactPhone"`
	RelatedPropertyRadiusKm float64                     `json:"relatedPropertyRadiusKm"`
	Locations               []MiniappLocation           `json:"locations"`
	FeaturedChannels        []FeaturedChannel           `json:"featuredChannels"`
	PagePresentations       map[string]PagePresentation `json:"pagePresentations"`
	ShareCard               ShareCardConfig             `json:"shareCard"`
	ListingTitle            ListingTitleConfig          `json:"listingTitle"`
	PropertyDetail          PropertyDetailConfig        `json:"propertyDetail"`
	StatusLabels            PropertyStatusLabels        `json:"statusLabels"`
}

func DefaultPublicConfig() PublicConfig {
	return PublicConfig{
		HeroTitle:               "小皮租房",
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
			"home":              {Title: "今天想住哪里？", Subtitle: "搜索小区、地铁或商圈", ChannelTitle: "精选频道", AllChannelsText: "全部频道", RecommendationTitle: "推荐房源", ViewAllText: "查看全部", EmptyText: "暂时没有匹配的房源", NearbyLabel: "附近"},
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
			CustomerAllowedFields: []string{"community", "floor", "layout", "rentType", "listingSource", "fee", "area"},
			EnabledFields:         []string{"community", "layout", "rentType", "fee"},
			CustomerDetailFields:  []string{"community", "layout", "rentType", "fee"},
			CustomerShareFields:   []string{"community", "layout", "rentType", "fee"},
			CustomerPosterFields:  []string{"community", "layout", "rentType", "fee"},
			AdminEnabledFields:    []string{"unit"},
			ListTagLayout:         "grouped",
			DetailTagLayout:       "wrap",
			ListTagOrder:          []string{"rentType", "listingSource", "layout", "fee", "floor", "area"},
			ListTagTopFields:      []string{"rentType", "listingSource"},
			DetailTagOrder:        []string{"layout", "rentType", "listingSource", "fee", "floor", "area"},
			DetailTagTopFields:    []string{"listingSource", "rentType", "fee"},
		},
		PropertyDetail: PropertyDetailConfig{
			Modules: []PropertyDetailModule{
				{Key: "overview", Title: "房屋概况", Enabled: true},
				{Key: "location", Title: "位置与周边", Enabled: true},
				{Key: "facilities", Title: "房屋设施与环境", Enabled: true},
				{Key: "description", Title: "房源描述", Enabled: true},
				{Key: "nearby", Title: "附近房源", Enabled: true},
			},
			Nearby: PropertyDetailNearbyConfig{RadiusKm: 3, Relation: "distance", Sort: "distance", Limit: 4},
			Facilities: []PropertyDetailFacility{
				{Key: "refrigerator", Label: "冰箱", Group: "家具家电", Enabled: true},
				{Key: "washer", Label: "洗衣机", Group: "家具家电", Enabled: true},
				{Key: "air_conditioner", Label: "空调", Group: "家具家电", Enabled: true},
				{Key: "water_heater", Label: "热水器", Group: "家具家电", Enabled: true},
				{Key: "television", Label: "电视", Group: "家具家电", Enabled: true},
				{Key: "broadband", Label: "宽带", Group: "家具家电", Enabled: true},
				{Key: "gas", Label: "燃气", Group: "厨房", Enabled: true},
				{Key: "hood", Label: "油烟机", Group: "厨房", Enabled: true},
				{Key: "sofa", Label: "沙发", Group: "家具家电", Enabled: true},
				{Key: "wardrobe", Label: "衣柜", Group: "家具家电", Enabled: true},
				{Key: "desk", Label: "书桌", Group: "家具家电", Enabled: true},
				{Key: "bed", Label: "床", Group: "家具家电", Enabled: true},
			},
		},
		StatusLabels: PropertyStatusLabels{Draft: "草稿", PendingReview: "待审核", Published: "已发布", Available: "可租", Rented: "已租", Offline: "已下架"},
	}
}

func (config *PublicConfig) Normalize() {
	config.HeroTitle = strings.TrimSpace(config.HeroTitle)
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
		presentation.ChannelTitle = strings.TrimSpace(presentation.ChannelTitle)
		presentation.AllChannelsText = strings.TrimSpace(presentation.AllChannelsText)
		presentation.RecommendationTitle = strings.TrimSpace(presentation.RecommendationTitle)
		presentation.ViewAllText = strings.TrimSpace(presentation.ViewAllText)
		presentation.EmptyText = strings.TrimSpace(presentation.EmptyText)
		presentation.NearbyLabel = strings.TrimSpace(presentation.NearbyLabel)
		config.PagePresentations[key] = presentation
	}
	statusDefaults := DefaultPublicConfig().StatusLabels
	if config.StatusLabels.Draft == "" { config.StatusLabels.Draft = statusDefaults.Draft }
	if config.StatusLabels.PendingReview == "" { config.StatusLabels.PendingReview = statusDefaults.PendingReview }
	if config.StatusLabels.Published == "" { config.StatusLabels.Published = statusDefaults.Published }
	if config.StatusLabels.Available == "" { config.StatusLabels.Available = statusDefaults.Available }
	if config.StatusLabels.Rented == "" { config.StatusLabels.Rented = statusDefaults.Rented }
	if config.StatusLabels.Offline == "" { config.StatusLabels.Offline = statusDefaults.Offline }
	config.ShareCard.TitlePrefix = strings.TrimSpace(config.ShareCard.TitlePrefix)
	if len(config.ShareCard.TitleFields) == 0 {
		config.ShareCard.TitleFields = append([]string(nil), DefaultPublicConfig().ShareCard.TitleFields...)
	}
	if len(config.ShareCard.PosterFields) == 0 {
		config.ShareCard.PosterFields = append([]string(nil), DefaultPublicConfig().ShareCard.PosterFields...)
	}
	// An omitted legacy field is nil. An explicit [] means operations has
	// intentionally hidden all optional public tags, and must stay empty.
	if config.ListingTitle.EnabledFields == nil {
		config.ListingTitle.EnabledFields = append([]string(nil), DefaultPublicConfig().ListingTitle.EnabledFields...)
	}
	if config.ListingTitle.CustomerDetailFields == nil {
		// Legacy configurations had one public tag list; preserve that result
		// as the initial detail setting until operations changes it explicitly.
		config.ListingTitle.CustomerDetailFields = append([]string(nil), config.ListingTitle.EnabledFields...)
	}
	if config.ListingTitle.CustomerShareFields == nil {
		// Older configurations shared the list labels with share surfaces.
		config.ListingTitle.CustomerShareFields = append([]string(nil), config.ListingTitle.EnabledFields...)
	}
	if config.ListingTitle.CustomerPosterFields == nil {
		// A legacy share setting becomes the starting point for Moments.
		config.ListingTitle.CustomerPosterFields = append([]string(nil), config.ListingTitle.CustomerShareFields...)
	}
	if config.ListingTitle.CustomerAllowedFields == nil {
		config.ListingTitle.CustomerAllowedFields = append([]string(nil), DefaultPublicConfig().ListingTitle.CustomerAllowedFields...)
	}
	if config.ListingTitle.ListTagLayout == "" {
		config.ListingTitle.ListTagLayout = DefaultPublicConfig().ListingTitle.ListTagLayout
	}
	if config.ListingTitle.DetailTagLayout == "" {
		config.ListingTitle.DetailTagLayout = DefaultPublicConfig().ListingTitle.DetailTagLayout
	}
	if config.ListingTitle.ListTagOrder == nil {
		config.ListingTitle.ListTagOrder = append([]string(nil), DefaultPublicConfig().ListingTitle.ListTagOrder...)
	}
	if config.ListingTitle.ListTagTopFields == nil {
		config.ListingTitle.ListTagTopFields = append([]string(nil), DefaultPublicConfig().ListingTitle.ListTagTopFields...)
	}
	if config.ListingTitle.DetailTagOrder == nil {
		config.ListingTitle.DetailTagOrder = append([]string(nil), DefaultPublicConfig().ListingTitle.DetailTagOrder...)
	}
	if config.ListingTitle.DetailTagTopFields == nil {
		config.ListingTitle.DetailTagTopFields = append([]string(nil), DefaultPublicConfig().ListingTitle.DetailTagTopFields...)
	}
	// Historical configurations held a full administrator tag set. The public
	// set is now shared globally, so preserve only the optional doorplate bit.
	adminDoorplate := false
	for _, field := range config.ListingTitle.AdminEnabledFields {
		if field == "unit" {
			adminDoorplate = true
			break
		}
	}
	if adminDoorplate {
		config.ListingTitle.AdminEnabledFields = []string{"unit"}
	} else {
		config.ListingTitle.AdminEnabledFields = []string{}
	}
	defaults := DefaultPublicConfig().PropertyDetail
	if len(config.PropertyDetail.Modules) == 0 {
		config.PropertyDetail.Modules = append([]PropertyDetailModule(nil), defaults.Modules...)
	}
	for index := range config.PropertyDetail.Modules {
		config.PropertyDetail.Modules[index].Key = strings.TrimSpace(config.PropertyDetail.Modules[index].Key)
		config.PropertyDetail.Modules[index].Title = strings.TrimSpace(config.PropertyDetail.Modules[index].Title)
	}
	if config.PropertyDetail.Nearby.RadiusKm <= 0 {
		config.PropertyDetail.Nearby.RadiusKm = defaults.Nearby.RadiusKm
	}
	if config.PropertyDetail.Nearby.Relation == "" {
		config.PropertyDetail.Nearby.Relation = defaults.Nearby.Relation
	}
	if config.PropertyDetail.Nearby.Sort == "" {
		config.PropertyDetail.Nearby.Sort = defaults.Nearby.Sort
	}
	if config.PropertyDetail.Nearby.Limit <= 0 {
		config.PropertyDetail.Nearby.Limit = defaults.Nearby.Limit
	}
	// The facility library is shared by the property entry page, detail page and
	// the admin canvas. Older configs contained environment-only entries (such
	// as elevator); those have their own detail fields and are not amenities.
	configuredFacilities := make(map[string]PropertyDetailFacility, len(config.PropertyDetail.Facilities))
	for _, facility := range config.PropertyDetail.Facilities {
		facility.Key = strings.TrimSpace(facility.Key)
		facility.Label = strings.TrimSpace(facility.Label)
		facility.Group = strings.TrimSpace(facility.Group)
		if facility.Key == "" || facility.Key == "elevator" || facility.Key == "private_bathroom" || facility.Key == "balcony" {
			continue
		}
		configuredFacilities[facility.Key] = facility
	}
	// A submitted catalogue is authoritative: an operator may remove a default
	// facility and re-add it later from the preset list. Only old empty configs
	// receive the complete default catalogue for backward compatibility.
	if len(config.PropertyDetail.Facilities) == 0 {
		config.PropertyDetail.Facilities = append([]PropertyDetailFacility(nil), defaults.Facilities...)
		return
	}
	normalizedFacilities := make([]PropertyDetailFacility, 0, len(config.PropertyDetail.Facilities))
	seenFacilities := make(map[string]struct{}, len(config.PropertyDetail.Facilities))
	for _, saved := range config.PropertyDetail.Facilities {
		cleaned, exists := configuredFacilities[saved.Key]
		if !exists {
			continue
		}
		if _, exists := seenFacilities[saved.Key]; exists {
			continue
		}
		seenFacilities[saved.Key] = struct{}{}
		normalizedFacilities = append(normalizedFacilities, cleaned)
	}
	config.PropertyDetail.Facilities = normalizedFacilities
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

// validateOptionalListingTitleFields is used by presentation-only controls.
// A row may intentionally be empty while an operator is arranging labels.
func validateOptionalListingTitleFields(fields []string) error {
	if len(fields) > len(listingTitleFieldLabels) {
		return fmt.Errorf("列表标签数量不能超过 %d 个", len(listingTitleFieldLabels))
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
	if err := validateListingTitleFields(config.ListingTitle.CustomerAllowedFields); err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(config.ListingTitle.CustomerAllowedFields))
	for _, field := range config.ListingTitle.CustomerAllowedFields {
		allowed[field] = struct{}{}
	}
	for _, field := range config.ListingTitle.EnabledFields {
		if _, exists := allowed[field]; !exists {
			return errors.New("客户展示标签必须先在客户可选标签中开启")
		}
	}
	if err := validateListingTitleFields(config.ListingTitle.CustomerDetailFields); err != nil {
		return err
	}
	for _, field := range config.ListingTitle.CustomerDetailFields {
		if _, exists := allowed[field]; !exists {
			return errors.New("客户详情标签必须先在客户可选标签中开启")
		}
	}
	if err := validateListingTitleFields(config.ListingTitle.CustomerShareFields); err != nil {
		return err
	}
	for _, field := range config.ListingTitle.CustomerShareFields {
		if _, exists := allowed[field]; !exists {
			return errors.New("客户分享标签必须先在客户可选标签中开启")
		}
	}
	if err := validateListingTitleFields(config.ListingTitle.CustomerPosterFields); err != nil {
		return err
	}
	for _, field := range config.ListingTitle.CustomerPosterFields {
		if _, exists := allowed[field]; !exists {
			return errors.New("客户朋友圈标签必须先在客户可选标签中开启")
		}
	}
	for _, field := range config.ListingTitle.AdminEnabledFields {
		if field != "unit" {
			return errors.New("管理员附加标签只支持门牌")
		}
	}
	if config.ListingTitle.ListTagLayout != "grouped" && config.ListingTitle.ListTagLayout != "wrap" {
		return errors.New("列表标签布局不支持")
	}
	if config.ListingTitle.DetailTagLayout != "wrap" && config.ListingTitle.DetailTagLayout != "grouped" {
		return errors.New("详情标签布局不支持")
	}
	if err := validateOptionalListingTitleFields(config.ListingTitle.ListTagOrder); err != nil {
		return errors.New("列表标签排序不支持")
	}
	if err := validateOptionalListingTitleFields(config.ListingTitle.ListTagTopFields); err != nil {
		return errors.New("列表第一行标签不支持")
	}
	if err := validateOptionalListingTitleFields(config.ListingTitle.DetailTagOrder); err != nil {
		return errors.New("详情标签排序不支持")
	}
	if err := validateOptionalListingTitleFields(config.ListingTitle.DetailTagTopFields); err != nil {
		return errors.New("详情第一行标签不支持")
	}
	validDetailModules := map[string]struct{}{"overview": {}, "location": {}, "facilities": {}, "description": {}, "nearby": {}}
	if len(config.PropertyDetail.Modules) != len(validDetailModules) {
		return errors.New("房源详情模块配置不完整")
	}
	seenModules := map[string]struct{}{}
	for _, module := range config.PropertyDetail.Modules {
		if _, exists := validDetailModules[module.Key]; !exists || module.Title == "" || utf8.RuneCountInString(module.Title) > 12 {
			return errors.New("房源详情模块配置不正确")
		}
		if _, exists := seenModules[module.Key]; exists {
			return errors.New("房源详情模块不能重复")
		}
		seenModules[module.Key] = struct{}{}
	}
	nearby := config.PropertyDetail.Nearby
	if nearby.RadiusKm < 0.5 || nearby.RadiusKm > 20 || (nearby.Relation != "distance" && nearby.Relation != "channel") || (nearby.Sort != "distance" && nearby.Sort != "newest") || nearby.Limit < 1 || nearby.Limit > 8 {
		return errors.New("附近房源推荐规则不正确")
	}
	if len(config.PropertyDetail.Facilities) == 0 || len(config.PropertyDetail.Facilities) > 24 {
		return errors.New("设施类型数量必须在 1 到 24 项之间")
	}
	seenFacilities := map[string]struct{}{}
	for _, facility := range config.PropertyDetail.Facilities {
		if facility.Key == "" || facility.Label == "" || facility.Group == "" || utf8.RuneCountInString(facility.Label) > 12 || utf8.RuneCountInString(facility.Group) > 12 {
			return errors.New("设施类型配置不完整")
		}
		if _, exists := seenFacilities[facility.Key]; exists {
			return errors.New("设施类型不能重复")
		}
		seenFacilities[facility.Key] = struct{}{}
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
