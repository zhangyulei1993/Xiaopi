package httpapi

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// normalizeCommunityName keeps only letters and numbers. This makes punctuation,
// spacing, and common typographic variants irrelevant to community identity.
func normalizeCommunityName(value string) string {
	var normalized strings.Builder
	runes := []rune(strings.ToLower(strings.TrimSpace(value)))
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if isChineseCommunityNumber(r) {
			end := index + 1
			for end < len(runes) && isChineseCommunityNumber(runes[end]) {
				end++
			}
			normalized.WriteString(chineseCommunityNumber(runes[index:end]))
			index = end - 1
			continue
		}
		if r >= '０' && r <= '９' {
			normalized.WriteRune('0' + (r - '０'))
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

func isChineseCommunityNumber(r rune) bool {
	return strings.ContainsRune("零〇一二两三四五六七八九十百千万亿", r)
}

func chineseCommunityNumber(value []rune) string {
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	hasUnit := false
	for _, r := range value {
		if strings.ContainsRune("十百千万亿", r) {
			hasUnit = true
			break
		}
	}
	if !hasUnit {
		var out strings.Builder
		for _, r := range value {
			out.WriteByte(byte('0' + digits[r]))
		}
		return out.String()
	}
	total, section, number := 0, 0, 0
	for _, r := range value {
		if digit, ok := digits[r]; ok {
			number = digit
			continue
		}
		switch r {
		case '十':
			if number == 0 {
				number = 1
			}
			section += number * 10
			number = 0
		case '百':
			if number == 0 {
				number = 1
			}
			section += number * 100
			number = 0
		case '千':
			if number == 0 {
				number = 1
			}
			section += number * 1000
			number = 0
		case '万':
			total += (section + number) * 10000
			section, number = 0, 0
		case '亿':
			total += (section + number) * 100000000
			section, number = 0, 0
		}
	}
	return strconv.Itoa(total + section + number)
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

// Building-specific map points are not community records.
var buildingPointSuffix = regexp.MustCompile(`(?i)(?:[-—·\s]+)?[0-9０-９]+\s*(?:栋|幢|号楼|号栋|座|单元)(?:[0-9０-９]+(?:单元|室))?$`)

func isBuildingPointName(name string) bool {
	return buildingPointSuffix.MatchString(strings.TrimSpace(name))
}
