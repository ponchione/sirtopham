package tool

import (
	"strings"
	"unicode"
)

func stringSliceHasAllFolded(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = normalizeBrainTag(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	for _, want := range required {
		want = normalizeBrainTag(want)
		if want == "" {
			continue
		}
		if _, ok := seen[want]; !ok {
			return false
		}
	}
	return true
}

func normalizeBrainSearchTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := normalizeBrainTag(tag)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeBrainTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return normalizeBrainSearchText(tag)
}

func normalizeBrainSearchText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastWasSep := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastWasSep = false
			continue
		}
		if !lastWasSep {
			b.WriteByte(' ')
			lastWasSep = true
		}
	}
	return strings.TrimSpace(b.String())
}

func brainDocumentHasAllTags(content string, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	frontmatterTags := parseBrainFrontmatterTags(content)
	metadataTags := parseBrainMetadataTags(content)
	inlineTags := extractBrainInlineTags(content)
	for _, tag := range tags {
		if _, ok := frontmatterTags[tag]; ok {
			continue
		}
		if _, ok := metadataTags[tag]; ok {
			continue
		}
		if _, ok := inlineTags[tag]; ok {
			continue
		}
		return false
	}
	return true
}

func parseBrainFrontmatterTags(content string) map[string]struct{} {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}

	tags := map[string]struct{}{}
	inTagsList := false
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if inTagsList {
			if strings.HasPrefix(line, "-") {
				if tag := normalizeBrainTag(strings.TrimSpace(strings.TrimPrefix(line, "-"))); tag != "" {
					tags[tag] = struct{}{}
				}
				continue
			}
			inTagsList = false
		}
		if !strings.HasPrefix(strings.ToLower(line), "tags:") {
			continue
		}
		rest := strings.TrimSpace(line[len("tags:"):])
		if rest == "" {
			inTagsList = true
			continue
		}
		for _, part := range strings.Split(strings.Trim(rest, "[]"), ",") {
			if tag := normalizeBrainTag(part); tag != "" {
				tags[tag] = struct{}{}
			}
		}
	}
	return tags
}

func parseBrainMetadataTags(content string) map[string]struct{} {
	tags := map[string]struct{}{}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		lower := strings.ToLower(line)
		for _, prefix := range []string{"family:", "tag:", "tags:"} {
			if !strings.HasPrefix(lower, prefix) {
				continue
			}
			rest := strings.TrimSpace(line[len(prefix):])
			for _, part := range strings.Split(strings.Trim(rest, "[]"), ",") {
				if tag := normalizeBrainTag(part); tag != "" {
					tags[tag] = struct{}{}
				}
			}
		}
	}
	return tags
}

func extractBrainInlineTags(content string) map[string]struct{} {
	tags := map[string]struct{}{}
	var current strings.Builder
	capturing := false
	flush := func() {
		if !capturing {
			return
		}
		if tag := normalizeBrainTag(current.String()); tag != "" {
			tags[tag] = struct{}{}
		}
		current.Reset()
		capturing = false
	}
	for _, r := range content {
		switch {
		case r == '#':
			flush()
			capturing = true
		case capturing && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_'):
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return tags
}
