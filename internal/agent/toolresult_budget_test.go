package agent

import (
	stdctx "context"
	"strings"
	"testing"

	"github.com/ponchione/sirtopham/internal/provider"
)

func TestBuildPersistedToolResultMessageIncludesStructuredReferenceAndPreview(t *testing.T) {
	ref := "/tmp/persisted/search_text-tc-1.txt"
	content := strings.Repeat("SEARCH-RESULT-LINE\n", 20)

	got := buildPersistedToolResultMessage(ref, "tc-1", "search_text", content, 220)

	for _, want := range []string{
		"[persisted_tool_result]",
		"path=/tmp/persisted/search_text-tc-1.txt",
		"tool=search_text",
		"tool_use_id=tc-1",
		"preview=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message missing %q: %q", want, got)
		}
	}
	if !strings.Contains(got, "SEARCH-RESULT-LINE") {
		t.Fatalf("message missing preview body: %q", got)
	}
}

func TestBuildPersistedToolResultMessageFallsBackToBarePathForTinyBudget(t *testing.T) {
	ref := "/tmp/persisted/search_text-tc-1.txt"

	got := buildPersistedToolResultMessage(ref, "tc-1", "search_text", "preview", len(ref))

	if got != ref {
		t.Fatalf("message = %q, want bare path %q", got, ref)
	}
}

func TestApplyAggregateToolResultBudgetReportsPersistenceAndSavings(t *testing.T) {
	fullOutput := strings.Repeat("SEARCH-RESULT-LINE\n", 40)
	store := &toolResultStoreStub{}

	budgeted, report := applyAggregateToolResultBudget(
		stdctx.Background(),
		store,
		[]provider.ToolResult{{ToolUseID: "tc-1", Content: fullOutput}},
		[]provider.ToolCall{{ID: "tc-1", Name: "search_text"}},
		120,
	)

	if len(budgeted) != 1 {
		t.Fatalf("budgeted result count = %d, want 1", len(budgeted))
	}
	if report.OriginalChars != len(fullOutput) {
		t.Fatalf("report.OriginalChars = %d, want %d", report.OriginalChars, len(fullOutput))
	}
	if report.FinalChars != len(budgeted[0].Content) {
		t.Fatalf("report.FinalChars = %d, want %d", report.FinalChars, len(budgeted[0].Content))
	}
	if report.PersistedResults != 1 {
		t.Fatalf("report.PersistedResults = %d, want 1", report.PersistedResults)
	}
	if report.InlineShrunkResults != 0 {
		t.Fatalf("report.InlineShrunkResults = %d, want 0", report.InlineShrunkResults)
	}
	if report.ReplacedResults != 1 {
		t.Fatalf("report.ReplacedResults = %d, want 1", report.ReplacedResults)
	}
	if report.CharsSaved <= 0 {
		t.Fatalf("report.CharsSaved = %d, want > 0", report.CharsSaved)
	}
}

func TestApplyAggregateToolResultBudgetReportsInlineShrinkWhenPersistenceUnavailable(t *testing.T) {
	fullOutput := strings.Repeat("SEARCH-RESULT-LINE\n", 40)
	store := &toolResultStoreStub{err: stdctx.Canceled}

	budgeted, report := applyAggregateToolResultBudget(
		stdctx.Background(),
		store,
		[]provider.ToolResult{{ToolUseID: "tc-1", Content: fullOutput}},
		[]provider.ToolCall{{ID: "tc-1", Name: "search_text"}},
		120,
	)

	if len(budgeted) != 1 {
		t.Fatalf("budgeted result count = %d, want 1", len(budgeted))
	}
	if report.PersistedResults != 0 {
		t.Fatalf("report.PersistedResults = %d, want 0", report.PersistedResults)
	}
	if report.InlineShrunkResults != 1 {
		t.Fatalf("report.InlineShrunkResults = %d, want 1", report.InlineShrunkResults)
	}
	if report.ReplacedResults != 1 {
		t.Fatalf("report.ReplacedResults = %d, want 1", report.ReplacedResults)
	}
	if report.CharsSaved <= 0 {
		t.Fatalf("report.CharsSaved = %d, want > 0", report.CharsSaved)
	}
}
