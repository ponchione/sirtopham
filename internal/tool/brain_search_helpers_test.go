package tool

import (
	"fmt"
	"reflect"
	"testing"

	appcontext "github.com/ponchione/sodoryard/internal/context"
)

func TestNormalizeBrainSearchTags(t *testing.T) {
	got := normalizeBrainSearchTags([]string{" #Debug-History ", "debug history", "#runtime_cache", "runtime cache", "", "#"})
	want := []string{"debug history", "runtime cache"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeBrainSearchTags() = %#v, want %#v", got, want)
	}
}

func TestNormalizeBrainSearchText(t *testing.T) {
	got := normalizeBrainSearchText("  Vite rebuild-loop_fix!!! 2026  ")
	want := "vite rebuild loop fix 2026"
	if got != want {
		t.Fatalf("normalizeBrainSearchText() = %q, want %q", got, want)
	}
}

func TestParseBrainFrontmatterTags(t *testing.T) {
	t.Run("inline list", func(t *testing.T) {
		content := "---\ntags: [debug-history, #runtime-cache]\nstatus: active\n---\n# Note"
		got := parseBrainFrontmatterTags(content)
		want := map[string]struct{}{"debug history": {}, "runtime cache": {}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseBrainFrontmatterTags() = %#v, want %#v", got, want)
		}
	})

	t.Run("multiline list", func(t *testing.T) {
		content := "---\ntags:\n  - debug-history\n  - #layout-rationale\n---\n# Note"
		got := parseBrainFrontmatterTags(content)
		want := map[string]struct{}{"debug history": {}, "layout rationale": {}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseBrainFrontmatterTags() = %#v, want %#v", got, want)
		}
	})
}

func TestParseBrainMetadataTags(t *testing.T) {
	content := "Family: debug-history\nTag: #runtime-cache\nTags: [layout-rationale, #ops-review]\n# Note"
	got := parseBrainMetadataTags(content)
	want := map[string]struct{}{
		"debug history":    {},
		"runtime cache":    {},
		"layout rationale": {},
		"ops review":       {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBrainMetadataTags() = %#v, want %#v", got, want)
	}
}

func TestExtractBrainInlineTags(t *testing.T) {
	content := "Mixed prose with #debug-history, #runtime_cache, and a trailing #ops-review."
	got := extractBrainInlineTags(content)
	want := map[string]struct{}{"debug history": {}, "runtime cache": {}, "ops review": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractBrainInlineTags() = %#v, want %#v", got, want)
	}
}

func TestBrainDocumentHasAllTags(t *testing.T) {
	content := "---\ntags: [debug-history]\n---\nFamily: runtime-cache\nThe journal links #ops-review follow-ups."
	if !brainDocumentHasAllTags(content, []string{"debug history", "runtime cache", "ops review"}) {
		t.Fatal("brainDocumentHasAllTags() = false, want true when tags are split across frontmatter, metadata, and inline tags")
	}
	if brainDocumentHasAllTags(content, []string{"debug history", "missing tag"}) {
		t.Fatal("brainDocumentHasAllTags() = true, want false when any required tag is absent")
	}
}

func TestFormatRuntimeBrainSearchHits(t *testing.T) {
	hits := []appcontext.BrainSearchResult{
		{
			DocumentPath: "notes/runtime-cache.md",
			Snippet:      "Runtime cache note.",
			MatchMode:    "",
			MatchSources: []string{"semantic", "graph"},
		},
		{
			DocumentPath: "notes/ops-checklist.md",
			Title:        "Ops Checklist",
			Snippet:      "Explicit graph expansion hit.",
			MatchMode:    "graph",
		},
	}

	got := formatRuntimeBrainSearchHits(hits)
	want := []formattedSearchHit{
		{Path: "notes/runtime-cache.md", Title: "Runtime Cache [semantic+graph]", Snippet: "Runtime cache note."},
		{Path: "notes/ops-checklist.md", Title: "Ops Checklist [graph]", Snippet: "Explicit graph expansion hit."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("formatRuntimeBrainSearchHits() = %#v, want %#v", got, want)
	}
}

func TestDescribeBrainSearchQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		tags  []string
		want  string
	}{
		{name: "query only", query: "runtime cache", want: "runtime cache"},
		{name: "tags only", tags: []string{"debug history", "ops review"}, want: "tags: debug history, ops review"},
		{name: "query and tags", query: "runtime cache", tags: []string{"debug history"}, want: "runtime cache (tags: debug history)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describeBrainSearchQuery(tt.query, tt.tags); got != tt.want {
				t.Fatalf("describeBrainSearchQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTitleFromPath(t *testing.T) {
	got := titleFromPath("notes/runtime_cache-rationale.md")
	want := "Runtime Cache Rationale"
	if got != want {
		t.Fatalf("titleFromPath() = %q, want %q", got, want)
	}
}

func TestTitleCase(t *testing.T) {
	got := titleCase("runtime cache rationale")
	want := "Runtime Cache Rationale"
	if got != want {
		t.Fatalf("titleCase() = %q, want %q", got, want)
	}
}

func TestPluralizeBrainSearchResults(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{count: 1, want: "result"},
		{count: 2, want: "results"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("count=%d", tt.count), func(t *testing.T) {
			if got := pluralizeBrainSearchResults(tt.count); got != tt.want {
				t.Fatalf("pluralizeBrainSearchResults(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestStringSliceHasAllFoldedNormalizesBrainTags(t *testing.T) {
	if !stringSliceHasAllFolded([]string{"debug-history", "runtime_cache"}, []string{"debug history", "runtime cache"}) {
		t.Fatal("stringSliceHasAllFolded() = false, want true for equivalent normalized brain tags")
	}
}
