package domain

import (
	"strings"
	"testing"
)

func TestPublicConfigValidateDefault(t *testing.T) {
	config := DefaultPublicConfig()
	config.Normalize()
	if err := config.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestPublicConfigValidateRequiredCollections(t *testing.T) {
	config := DefaultPublicConfig()
	config.Locations = nil
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "地理位置数量") {
		t.Fatalf("expected location count error, got %v", err)
	}

	config = DefaultPublicConfig()
	config.FeaturedChannels = nil
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "精选频道数量") {
		t.Fatalf("expected channel count error, got %v", err)
	}
}

func TestPublicConfigValidateDuplicateKeywords(t *testing.T) {
	config := DefaultPublicConfig()
	config.Locations[1].Keyword = config.Locations[0].Keyword
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "地理位置搜索关键词不能重复") {
		t.Fatalf("expected duplicate location error, got %v", err)
	}

	config = DefaultPublicConfig()
	config.FeaturedChannels[1].Keyword = config.FeaturedChannels[0].Keyword
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "精选频道搜索关键词不能重复") {
		t.Fatalf("expected duplicate channel error, got %v", err)
	}
}

func TestPublicConfigNormalizeBeforeValidation(t *testing.T) {
	config := DefaultPublicConfig()
	config.HeroTitle = "  小皮租房  "
	config.ContactPhone = "  02788886666  "
	config.Locations[0].Keyword = "  光谷  "
	config.Normalize()

	if config.HeroTitle != "小皮租房" || config.ContactPhone != "02788886666" || config.Locations[0].Keyword != "光谷" {
		t.Fatalf("config was not normalized: %+v", config)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("normalized config should be valid: %v", err)
	}
}

func TestPublicConfigNormalizeLegacyShareCard(t *testing.T) {
	config := DefaultPublicConfig()
	config.ShareCard = ShareCardConfig{}
	config.ListingTitle = ListingTitleConfig{}
	config.Normalize()
	if len(config.ShareCard.TitleFields) == 0 || len(config.ShareCard.PosterFields) == 0 {
		t.Fatalf("legacy config should receive share defaults: %+v", config.ShareCard)
	}
	if len(config.ListingTitle.EnabledFields) == 0 {
		t.Fatalf("legacy config should receive listing title defaults: %+v", config.ListingTitle)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("normalized share config should be valid: %v", err)
	}
}

func TestPublicConfigNormalizeKeepsEditedFacilityCatalogue(t *testing.T) {
	config := DefaultPublicConfig()
	// Operations may remove a preset facility. A non-empty submitted catalogue
	// is authoritative and must not be silently repopulated with defaults.
	config.PropertyDetail.Facilities = []PropertyDetailFacility{
		{Key: "refrigerator", Label: "冰箱", Group: "家具家电", Enabled: true},
		{Key: "desk", Label: "书桌", Group: "家具家电", Enabled: false},
	}

	config.Normalize()

	if len(config.PropertyDetail.Facilities) != 2 {
		t.Fatalf("expected edited catalogue to keep 2 facilities, got %+v", config.PropertyDetail.Facilities)
	}
	if config.PropertyDetail.Facilities[0].Key != "refrigerator" || config.PropertyDetail.Facilities[1].Key != "desk" {
		t.Fatalf("expected facility order and keys to be preserved, got %+v", config.PropertyDetail.Facilities)
	}
	if config.PropertyDetail.Facilities[1].Enabled {
		t.Fatalf("expected explicit facility enabled state to be preserved")
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("edited facility catalogue should remain valid: %v", err)
	}
}
