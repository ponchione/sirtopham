package context

import (
	"strings"
	"testing"
)

func TestMarkdownSerializerGroupsChunksAnnotatesSeenFilesAndIsDeterministic(t *testing.T) {
	serializer := MarkdownSerializer{}
	result := &BudgetResult{
		SelectedFileResults: []FileResult{{
			FilePath: "internal/auth/middleware.go",
			Content:  "package auth\n\nfunc AuthMiddleware() {}\n",
		}},
		SelectedRAGHits: []RAGHit{
			{
				ChunkID:     "chunk-1",
				FilePath:    "internal/auth/service.go",
				Name:        "ValidateToken",
				Description: "Validates JWT tokens.",
				Body:        "func ValidateToken(token string) error { return nil }",
				Language:    "go",
				LineStart:   10,
				LineEnd:     20,
			},
			{
				ChunkID:     "chunk-2",
				FilePath:    "internal/auth/service.go",
				Name:        "GenerateToken",
				Description: "Generates JWT tokens.",
				Body:        "func GenerateToken(userID string) (string, error) { return \"\", nil }",
				Language:    "go",
				LineStart:   22,
				LineEnd:     35,
			},
		},
		SelectedGraphHits: []GraphHit{{
			ChunkID:          "graph-1",
			SymbolName:       "AuthHandler",
			FilePath:         "internal/auth/handler.go",
			RelationshipType: "upstream",
			Depth:            1,
		}},
		SelectedBrainHits: []BrainHit{{
			DocumentPath: "notes/auth.md",
			Title:        "Auth decisions",
			Snippet:      "Use the vault note for durable auth rationale.",
			MatchScore:   0.83,
			MatchMode:    "keyword",
		}},
		ConventionText: "Tests use table-driven style.\nErrors wrap context.",
		GitContext:     "abc123 fix auth\ndef456 add tests",
	}

	content, err := serializer.Serialize(result, seenFilesStub{path: "internal/auth/service.go", turn: 2})
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}

	if strings.Count(content, "### internal/auth/service.go") != 1 {
		t.Fatalf("content had %d grouped headers for service.go, want 1\n%s", strings.Count(content, "### internal/auth/service.go"), content)
	}
	if !strings.Contains(content, "[previously viewed in turn 2]") {
		t.Fatalf("content missing previously-viewed annotation\n%s", content)
	}
	if !strings.Contains(content, "```go") {
		t.Fatalf("content missing go code fence\n%s", content)
	}
	if !strings.Contains(content, "## Project Conventions") {
		t.Fatalf("content missing conventions section\n%s", content)
	}
	if !strings.Contains(content, "- Tests use table-driven style.") {
		t.Fatalf("content missing convention bullet\n%s", content)
	}
	if !strings.Contains(content, "## Recent Changes (last 2 commits)") {
		t.Fatalf("content missing git section\n%s", content)
	}
	if !strings.Contains(content, "## Structural Context") {
		t.Fatalf("content missing structural context section\n%s", content)
	}
	if !strings.Contains(content, "## Project Brain") {
		t.Fatalf("content missing project brain section\n%s", content)
	}
	if !strings.Contains(content, "notes/auth.md") {
		t.Fatalf("content missing brain path\n%s", content)
	}
	if strings.Count(content, "```")%2 != 0 {
		t.Fatalf("content has unbalanced code fences\n%s", content)
	}
	if strings.Index(content, "Validates JWT tokens.") > strings.Index(content, "func ValidateToken") {
		t.Fatalf("description appeared after code\n%s", content)
	}

	again, err := serializer.Serialize(result, seenFilesStub{path: "internal/auth/service.go", turn: 2})
	if err != nil {
		t.Fatalf("second Serialize returned error: %v", err)
	}
	if content != again {
		t.Fatalf("serializer output was not deterministic\nfirst:\n%s\n\nsecond:\n%s", content, again)
	}
}

func TestMarkdownSerializerHandlesEmptyBudgetResult(t *testing.T) {
	serializer := MarkdownSerializer{}

	content, err := serializer.Serialize(&BudgetResult{}, nil)
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	if strings.TrimSpace(content) != "" {
		t.Fatalf("content = %q, want empty", content)
	}
}
