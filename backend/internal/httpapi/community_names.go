package httpapi

import (
	"strings"
	"unicode"
)

// normalizeCommunityName keeps only letters and numbers. This makes punctuation,
// spacing, and common typographic variants irrelevant to community identity.
func normalizeCommunityName(value string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

// resolveCommunityName maps a user/import spelling to the first known display
// name with the same normalized identity. Empty known names are ignored.
func resolveCommunityName(name string, known []string) (string, error) {
	name = strings.TrimSpace(name)
	key := normalizeCommunityName(name)
	if key == "" {
		return "", nil
	}
	matched := ""
	for _, candidate := range known {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || normalizeCommunityName(candidate) != key {
			continue
		}
		if matched == "" {
			matched = candidate
			continue
		}
		// Existing spellings with the same identity are already legacy aliases;
		// prefer a stable lexicographic representative during consolidation.
		if candidate < matched {
			matched = candidate
		}
	}
	if matched != "" {
		return matched, nil
	}
	return name, nil
}

func uniqueCommunityNames(names ...[]string) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, list := range names {
		for _, name := range list {
			name = strings.TrimSpace(name)
			key := normalizeCommunityName(name)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, name)
		}
	}
	return result
}
