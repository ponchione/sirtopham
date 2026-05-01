package chaininput

import "strings"

func ParseSpecs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return NormalizeSpecs(strings.Split(value, ","))
}

func NormalizeSpecs(specs []string) []string {
	seen := make(map[string]struct{}, len(specs))
	normalized := make([]string, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if _, ok := seen[spec]; ok {
			continue
		}
		seen[spec] = struct{}{}
		normalized = append(normalized, spec)
	}
	return normalized
}
