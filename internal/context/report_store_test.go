package context

import "testing"

func TestCollectIncludedChunkKeysIncludesBrainResults(t *testing.T) {
	report := &ContextAssemblyReport{
		BrainResults: []BrainHit{
			{DocumentPath: "notes/runtime.md", Included: true},
			{DocumentPath: "notes/ignored.md", Included: false},
		},
	}

	included := collectIncludedChunkKeys(report)
	if len(included) != 1 || included[0] != "notes/runtime.md" {
		t.Fatalf("included = %#v, want [notes/runtime.md]", included)
	}
}

func TestCollectExcludedChunkKeysIncludesBrainResults(t *testing.T) {
	report := &ContextAssemblyReport{
		BrainResults: []BrainHit{
			{DocumentPath: "notes/runtime.md", ExclusionReason: "budget"},
			{DocumentPath: "notes/empty.md", ExclusionReason: ""},
		},
	}

	excluded, reasons := collectExcludedChunkKeys(report)
	if len(excluded) != 1 || excluded[0] != "notes/runtime.md" {
		t.Fatalf("excluded = %#v, want [notes/runtime.md]", excluded)
	}
	if got := reasons["notes/runtime.md"]; got != "budget" {
		t.Fatalf("reasons[notes/runtime.md] = %q, want budget", got)
	}
}
