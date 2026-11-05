package domain

import "testing"

func TestCanTransitionPropertyStatus(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{string(PropertyDraft), string(PropertyPending), false},
		{string(PropertyPending), string(PropertyPublished), true},
		{string(PropertyPending), string(PropertyAvailable), true},
		{string(PropertyPending), string(PropertyDraft), false},
		{string(PropertyPublished), string(PropertyOffline), true},
		{string(PropertyOffline), string(PropertyPending), false},
		{string(PropertyDraft), string(PropertyPublished), true},
		{string(PropertyDraft), string(PropertyAvailable), true},
		{string(PropertyPublished), string(PropertyDraft), false},
		{string(PropertyAvailable), string(PropertyRented), true},
		{string(PropertyAvailable), string(PropertyPublished), false},
		{string(PropertyRented), string(PropertyAvailable), true},
	}
	for _, test := range tests {
		if got := CanTransitionPropertyStatus(test.from, test.to); got != test.want {
			t.Errorf("CanTransitionPropertyStatus(%q, %q) = %v; want %v", test.from, test.to, got, test.want)
		}
	}
}
