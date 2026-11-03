package domain

import "testing"

func TestCanTransitionPropertyStatus(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{string(PropertyDraft), string(PropertyPending), true},
		{string(PropertyPending), string(PropertyPublished), true},
		{string(PropertyPending), string(PropertyDraft), true},
		{string(PropertyPublished), string(PropertyOffline), true},
		{string(PropertyOffline), string(PropertyPending), true},
		{string(PropertyDraft), string(PropertyPublished), true},
		{string(PropertyPublished), string(PropertyDraft), false},
	}
	for _, test := range tests {
		if got := CanTransitionPropertyStatus(test.from, test.to); got != test.want {
			t.Errorf("CanTransitionPropertyStatus(%q, %q) = %v; want %v", test.from, test.to, got, test.want)
		}
	}
}
