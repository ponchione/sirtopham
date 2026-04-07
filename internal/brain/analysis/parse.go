package analysis

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	wikilinkRegexp  = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	inlineTagRegexp = regexp.MustCompile(`(?:^|\s)#([[:alnum:]_/-]+)\b`)
)

func ParseDocument(docPath, content string) (Document, error) {
	frontmatter, body := splitFrontmatter(content)
	var fm map[string]any
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return Document{}, fmt.Errorf("parse frontmatter for %s: %w", docPath, err)
		}
	}
	if fm == nil {
		fm = map[string]any{}
	}

	updatedAt, hasUpdatedAt := extractUpdatedAt(fm)
	return Document{
		Path:         docPath,
		Content:      content,
		Frontmatter:  fm,
		Tags:         extractTags(body, fm),
		Wikilinks:    extractWikilinks(content),
		UpdatedAt:    updatedAt,
		HasUpdatedAt: hasUpdatedAt,
		Title:        extractTitle(docPath, body),
	}, nil
}

func splitFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content
	}
	fm := strings.TrimSpace(rest[:idx])
	body := strings.TrimLeft(rest[idx+4:], "\n")
	return fm, body
}

func extractWikilinks(content string) []string {
	matches := wikilinkRegexp.FindAllStringSubmatch(content, -1)
	seen := map[string]struct{}{}
	links := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		link := strings.TrimSpace(match[1])
		if idx := strings.Index(link, "|"); idx >= 0 {
			link = link[:idx]
		}
		link = normalizeLinkTarget(link)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		links = append(links, link)
	}
	return links
}

func extractTags(body string, fm map[string]any) []string {
	seen := map[string]struct{}{}
	var tags []string
	add := func(tag string) {
		tag = normalizeTag(tag)
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	if raw, ok := fm["tags"]; ok {
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				if s, ok := item.(string); ok {
					add(s)
				}
			}
		case []string:
			for _, item := range typed {
				add(item)
			}
		case string:
			for _, part := range strings.Split(typed, ",") {
				add(part)
			}
		}
	}

	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, match := range inlineTagRegexp.FindAllStringSubmatch(line, -1) {
			if len(match) >= 2 {
				add(match[1])
			}
		}
	}

	slices.Sort(tags)
	return tags
}

func extractUpdatedAt(fm map[string]any) (time.Time, bool) {
	raw, ok := fm["updated_at"]
	if !ok {
		return time.Time{}, false
	}
	switch typed := raw.(type) {
	case time.Time:
		return typed.UTC(), true
	case string:
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(typed))
		if err != nil {
			return time.Time{}, false
		}
		return parsed.UTC(), true
	default:
		return time.Time{}, false
	}
}

func normalizeLinkTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	target = path.Clean(target)
	if target == "." {
		return ""
	}
	return strings.TrimSuffix(target, ".md")
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return strings.ToLower(tag)
}

func extractTitle(docPath, body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
	}
	base := path.Base(docPath)
	return strings.TrimSuffix(base, path.Ext(base))
}
