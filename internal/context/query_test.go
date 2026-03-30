package context

import (
	"strings"
	"testing"
)

func TestHeuristicQueryExtractorStripsFillerAndBuildsCleanedQuery(t *testing.T) {
	extractor := HeuristicQueryExtractor{}

	queries := extractor.ExtractQueries("Hey, can you please fix the auth middleware validation?", &ContextNeeds{})

	if len(queries) == 0 {
		t.Fatal("expected at least one query")
	}
	if got := queries[0]; got != "fix the auth middleware validation" {
		t.Fatalf("queries[0] = %q, want %q", got, "fix the auth middleware validation")
	}
}

func TestHeuristicQueryExtractorSplitsLongMessageIntoTwoQueries(t *testing.T) {
	extractor := HeuristicQueryExtractor{}

	queries := extractor.ExtractQueries("Please inspect token caching. Then review retry logic! Finally summarize.", &ContextNeeds{})

	if len(queries) < 2 {
		t.Fatalf("len(queries) = %d, want at least 2", len(queries))
	}
	if !strings.Contains(queries[0], "inspect token caching") {
		t.Fatalf("queries[0] = %q, want it to contain %q", queries[0], "inspect token caching")
	}
	if !strings.Contains(queries[1], "review retry logic") {
		t.Fatalf("queries[1] = %q, want it to contain %q", queries[1], "review retry logic")
	}
}

func TestHeuristicQueryExtractorAddsSupplementaryTechnicalQuery(t *testing.T) {
	extractor := HeuristicQueryExtractor{}

	queries := extractor.ExtractQueries("The bug is in validateToken and validate_token inside auth.Service returning 401 from middleware.", &ContextNeeds{})

	if len(queries) < 2 {
		t.Fatalf("len(queries) = %d, want at least 2", len(queries))
	}
	if !queryContains(queries, "validateToken") {
		t.Fatalf("queries = %v, want a supplementary technical query with validateToken", queries)
	}
	if !queryContains(queries, "validate_token") {
		t.Fatalf("queries = %v, want a supplementary technical query with validate_token", queries)
	}
	if !queryContains(queries, "auth.Service") {
		t.Fatalf("queries = %v, want a supplementary technical query with auth.Service", queries)
	}
}

func TestHeuristicQueryExtractorAddsMomentumEnhancedQuery(t *testing.T) {
	extractor := HeuristicQueryExtractor{}
	needs := &ContextNeeds{MomentumModule: "internal/auth"}

	queries := extractor.ExtractQueries("fix the tests", needs)

	if !sliceContainsExact(queries, "internal/auth fix the tests") {
		t.Fatalf("queries = %v, want %q", queries, "internal/auth fix the tests")
	}
}

func TestHeuristicQueryExtractorExcludesExplicitEntities(t *testing.T) {
	extractor := HeuristicQueryExtractor{}
	needs := &ContextNeeds{
		ExplicitFiles:   []string{"internal/auth/middleware.go"},
		ExplicitSymbols: []string{"ValidateToken"},
	}

	queries := extractor.ExtractQueries("Please fix ValidateToken in internal/auth/middleware.go because middleware returns 401.", needs)

	for _, query := range queries {
		if strings.Contains(query, "internal/auth/middleware.go") {
			t.Fatalf("query %q unexpectedly contains explicit file", query)
		}
		if strings.Contains(query, "ValidateToken") {
			t.Fatalf("query %q unexpectedly contains explicit symbol", query)
		}
	}
}

func TestHeuristicQueryExtractorCapsAtThreeQueries(t *testing.T) {
	extractor := HeuristicQueryExtractor{}
	needs := &ContextNeeds{MomentumModule: "internal/auth"}

	queries := extractor.ExtractQueries("Please inspect adapter wiring. Then debug router middleware for validateToken returning 401.", needs)

	if len(queries) != 3 {
		t.Fatalf("len(queries) = %d, want 3", len(queries))
	}
}

func queryContains(queries []string, substring string) bool {
	for _, query := range queries {
		if strings.Contains(query, substring) {
			return true
		}
	}
	return false
}

func sliceContainsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
