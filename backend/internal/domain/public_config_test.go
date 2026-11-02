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
