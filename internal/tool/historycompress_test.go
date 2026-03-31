package tool

import (
	"database/sql"
	"strings"
	"testing"
)

func nullS(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

func emptyNull() sql.NullString {
	return sql.NullString{}
}

// --- StripLineNumbers tests ---

func TestStripLineNumbers_BasicFileRead(t *testing.T) {
	input := "File: main.go (3 lines)\n 1\tpackage main\n 2\t\n 3\tfunc main() {}\n"
	want := "File: main.go (3 lines)\npackage main\n\nfunc main() {}\n"

	got := StripLineNumbers(input)
	if got != want {
		t.Errorf("StripLineNumbers:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestStripLineNumbers_WideLineNumbers(t *testing.T) {
	input := "File: big.go (150 lines)\n  1\tline one\n 10\tline ten\n100\tline hundred\n150\tline last\n"
	want := "File: big.go (150 lines)\nline one\nline ten\nline hundred\nline last\n"

	got := StripLineNumbers(input)
	if got != want {
		t.Errorf("StripLineNumbers:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestStripLineNumbers_RangeHeader(t *testing.T) {
	input := "File: utils.go (lines 10-20 of 100)\n10\tline ten\n11\tline eleven\n"
	want := "File: utils.go (lines 10-20 of 100)\nline ten\nline eleven\n"

	got := StripLineNumbers(input)
	if got != want {
		t.Errorf("StripLineNumbers:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestStripLineNumbers_NoHeader_NoNumbers(t *testing.T) {
	input := "Some random tool output\nwith no line numbers\n"
	got := StripLineNumbers(input)
	if got != input {
		t.Errorf("Expected no change, got: %q", got)
	}
}

func TestStripLineNumbers_EmptyContent(t *testing.T) {
	got := StripLineNumbers("")
	if got != "" {
		t.Errorf("Expected empty, got: %q", got)
	}
}

func TestStripLineNumbers_TabsInContent(t *testing.T) {
	// Content that itself contains tabs should keep them.
	input := "File: Makefile (2 lines)\n1\t\tbuild:\n2\t\t\tgo build\n"
	want := "File: Makefile (2 lines)\n\tbuild:\n\t\tgo build\n"

	got := StripLineNumbers(input)
	if got != want {
		t.Errorf("StripLineNumbers:\n  got:  %q\n  want: %q", got, want)
	}
}

// --- extractFileReadPath tests ---

func TestExtractFileReadPath_Normal(t *testing.T) {
	input := "File: internal/auth/middleware.go (89 lines)\n1\tpackage auth\n"
	path, ok := extractFileReadPath(input)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if path != "internal/auth/middleware.go" {
		t.Errorf("path: expected %q, got %q", "internal/auth/middleware.go", path)
	}
}

func TestExtractFileReadPath_Range(t *testing.T) {
	input := "File: cmd/main.go (lines 1-50 of 200)\n"
	path, ok := extractFileReadPath(input)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if path != "cmd/main.go" {
		t.Errorf("path: expected %q, got %q", "cmd/main.go", path)
	}
}

func TestExtractFileReadPath_NoMatch(t *testing.T) {
	input := "Error: file not found"
	_, ok := extractFileReadPath(input)
	if ok {
		t.Error("expected ok=false for non-file-read content")
	}
}

// --- extractFileReadLineCount tests ---

func TestExtractFileReadLineCount_Normal(t *testing.T) {
	input := "File: main.go (42 lines)\n"
	got := extractFileReadLineCount(input)
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestExtractFileReadLineCount_Range(t *testing.T) {
	input := "File: main.go (lines 1-20 of 150)\n"
	got := extractFileReadLineCount(input)
	if got != 150 {
		t.Errorf("expected 150, got %d", got)
	}
}

// --- Duplicate elision tests ---

func TestCompressHistory_DuplicateElision(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "user", Content: nullS("read main.go"), TurnNumber: 1},
		{Role: "tool", Content: nullS("File: main.go (10 lines)\n 1\tpackage main\n"), ToolName: nullS("file_read"), TurnNumber: 1},
		{Role: "user", Content: nullS("now fix it"), TurnNumber: 2},
		{Role: "tool", Content: nullS("File: main.go (10 lines)\n 1\tpackage main\n"), ToolName: nullS("file_read"), TurnNumber: 2},
	}

	c := &HistoryCompressor{CurrentTurn: 3, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Turn 1 file_read should be elided (turn 2 has a more recent read).
	turn1Content := result[1].Content.String
	if !strings.Contains(turn1Content, "elided") {
		t.Errorf("turn 1 file_read should be elided, got: %q", turn1Content)
	}
	if !strings.Contains(turn1Content, "turn 2") {
		t.Errorf("elision message should reference turn 2, got: %q", turn1Content)
	}

	// Turn 2 file_read should still have content (line numbers stripped but not elided).
	turn2Content := result[3].Content.String
	if strings.Contains(turn2Content, "elided") {
		t.Errorf("turn 2 file_read should NOT be elided, got: %q", turn2Content)
	}
}

func TestCompressHistory_NoElisionForCurrentTurn(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: main.go (3 lines)\n1\tpackage main\n"), ToolName: nullS("file_read"), TurnNumber: 2},
	}

	c := &HistoryCompressor{CurrentTurn: 2, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Should not be modified — it's from the current turn.
	if result[0].Content.String != messages[0].Content.String {
		t.Errorf("current turn message should be unchanged, got: %q", result[0].Content.String)
	}
}

func TestCompressHistory_ShellResultsNotDeduplicated(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("go test ./...\nPASS"), ToolName: nullS("shell"), TurnNumber: 1},
		{Role: "tool", Content: nullS("go test ./...\nPASS"), ToolName: nullS("shell"), TurnNumber: 2},
	}

	c := &HistoryCompressor{CurrentTurn: 3, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Shell results should never be elided.
	for i, msg := range result {
		if strings.Contains(msg.Content.String, "elided") {
			t.Errorf("shell result at index %d should not be elided", i)
		}
	}
}

// --- Stale summarization tests ---

func TestCompressHistory_StaleSummarization(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: old.go (89 lines)\n 1\tpackage old\n 2\t// lots of code\n"), ToolName: nullS("file_read"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 15, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	got := result[0].Content.String
	if !strings.Contains(got, "Historical") {
		t.Errorf("expected historical summary, got: %q", got)
	}
	if !strings.Contains(got, "old.go") {
		t.Errorf("summary should mention file path, got: %q", got)
	}
	if !strings.Contains(got, "89 lines") {
		t.Errorf("summary should mention line count, got: %q", got)
	}
	if !strings.Contains(got, "turn 1") {
		t.Errorf("summary should mention turn number, got: %q", got)
	}
}

func TestCompressHistory_StaleSummarization_Disabled(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: old.go (89 lines)\n1\tpackage old\n"), ToolName: nullS("file_read"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 100, SummarizeAfterTurns: 0}
	result := c.CompressHistory(messages)

	// Should not be summarized when disabled.
	if strings.Contains(result[0].Content.String, "Historical") {
		t.Errorf("summarization should be disabled, got: %q", result[0].Content.String)
	}
}

func TestCompressHistory_StaleSummarization_NotOldEnough(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: recent.go (5 lines)\n1\tpackage recent\n"), ToolName: nullS("file_read"), TurnNumber: 8},
	}

	c := &HistoryCompressor{CurrentTurn: 12, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Turn 8 is 4 turns ago — not old enough (threshold is 10).
	if strings.Contains(result[0].Content.String, "Historical") {
		t.Errorf("result should NOT be summarized (only 4 turns old), got: %q", result[0].Content.String)
	}
}

func TestCompressHistory_StaleSummarization_SearchText(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("Found 5 matches:\nmain.go:10: func main\nmain.go:20: func init\nutil.go:5: func helper\nutil.go:10: func parse\nutil.go:15: func format\n"), ToolName: nullS("search_text"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 15, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	got := result[0].Content.String
	if !strings.Contains(got, "Historical") {
		t.Errorf("expected historical summary, got: %q", got)
	}
	if !strings.Contains(got, "search_text") {
		t.Errorf("summary should mention tool name, got: %q", got)
	}
}

// --- Line-number stripping in compression flow ---

func TestCompressHistory_LineNumberStripping(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: main.go (3 lines)\n 1\tpackage main\n 2\t\n 3\tfunc main() {}\n"), ToolName: nullS("file_read"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 3, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	got := result[0].Content.String
	// Line numbers should be stripped.
	if strings.Contains(got, "\t") && strings.Contains(got, " 1\t") {
		t.Errorf("line numbers should be stripped, got: %q", got)
	}
	// Content should still be present.
	if !strings.Contains(got, "package main") {
		t.Errorf("content should be preserved, got: %q", got)
	}
	// Header should be preserved.
	if !strings.Contains(got, "File: main.go (3 lines)") {
		t.Errorf("header should be preserved, got: %q", got)
	}
}

// --- JSON re-minification in compression flow ---

func TestCompressHistory_JSONReMinification(t *testing.T) {
	// Pretend a tool result that wasn't minified at write time (migration path).
	prettyJSON := "{\n  \"name\": \"test\",\n  \"version\": \"1.0\"\n}"
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS(prettyJSON), ToolName: nullS("some_tool"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 3, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	got := result[0].Content.String
	// Should be minified.
	if strings.Contains(got, "\n") {
		t.Errorf("JSON should be minified, got: %q", got)
	}
	if got != `{"name":"test","version":"1.0"}` {
		t.Errorf("expected minified JSON, got: %q", got)
	}
}

// --- Non-tool messages left alone ---

func TestCompressHistory_NonToolMessagesUnchanged(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "user", Content: nullS("hello world"), TurnNumber: 1},
		{Role: "assistant", Content: nullS("hi there"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 3}
	result := c.CompressHistory(messages)

	for i, msg := range result {
		if msg.Content.String != messages[i].Content.String {
			t.Errorf("message %d should be unchanged: got %q, want %q",
				i, msg.Content.String, messages[i].Content.String)
		}
	}
}

// --- Empty and nil content ---

func TestCompressHistory_EmptyContent(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "tool", Content: emptyNull(), ToolName: nullS("file_read"), TurnNumber: 1},
	}

	c := &HistoryCompressor{CurrentTurn: 3}
	result := c.CompressHistory(messages)

	if result[0].Content.Valid {
		t.Errorf("empty content should remain empty, got: %q", result[0].Content.String)
	}
}

func TestCompressHistory_EmptySlice(t *testing.T) {
	c := &HistoryCompressor{CurrentTurn: 3}
	result := c.CompressHistory(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got: %v", result)
	}
}

// --- Priority: elision wins over summarization ---

func TestCompressHistory_ElisionPriority(t *testing.T) {
	// A very old file read that also has a duplicate — elision should win.
	messages := []HistoryMessage{
		{Role: "tool", Content: nullS("File: main.go (100 lines)\n 1\tpackage main\n"), ToolName: nullS("file_read"), TurnNumber: 1},
		{Role: "tool", Content: nullS("File: main.go (100 lines)\n 1\tpackage main\n"), ToolName: nullS("file_read"), TurnNumber: 5},
	}

	c := &HistoryCompressor{CurrentTurn: 20, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Turn 1 should be elided (not summarized) — elision takes priority.
	got := result[0].Content.String
	if !strings.Contains(got, "elided") {
		t.Errorf("expected elision, got: %q", got)
	}
}

// --- atoiSafe ---

func TestAtoiSafe(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"42", 42},
		{"100", 100},
		{"abc", 0},
		{"12abc", 0},
		{"", 0},
	}
	for _, tc := range tests {
		got := atoiSafe(tc.input)
		if got != tc.want {
			t.Errorf("atoiSafe(%q): expected %d, got %d", tc.input, tc.want, got)
		}
	}
}

// --- Full pipeline integration test ---

func TestCompressHistory_FullPipeline(t *testing.T) {
	messages := []HistoryMessage{
		// Turn 1: user + assistant + file_read (will be elided — re-read in turn 3)
		{Role: "user", Content: nullS("read the config"), TurnNumber: 1},
		{Role: "tool", Content: nullS("File: config.json (5 lines)\n1\t{\n2\t  \"name\": \"test\",\n3\t  \"version\": \"1.0\",\n4\t  \"debug\": true\n5\t}\n"), ToolName: nullS("file_read"), TurnNumber: 1},

		// Turn 2: shell result (not deduplicated, but line numbers N/A)
		{Role: "tool", Content: nullS("go test ./...\nok  pkg 0.5s"), ToolName: nullS("shell"), TurnNumber: 2},

		// Turn 3: re-read of config.json (this is the latest)
		{Role: "tool", Content: nullS("File: config.json (5 lines)\n1\t{\n2\t  \"name\": \"test\",\n3\t  \"version\": \"2.0\",\n4\t  \"debug\": false\n5\t}\n"), ToolName: nullS("file_read"), TurnNumber: 3},

		// Turn 4: current turn (untouched)
		{Role: "tool", Content: nullS("File: main.go (2 lines)\n1\tpackage main\n2\tfunc main() {}\n"), ToolName: nullS("file_read"), TurnNumber: 4},
	}

	c := &HistoryCompressor{CurrentTurn: 4, SummarizeAfterTurns: 10}
	result := c.CompressHistory(messages)

	// Message 0: user — unchanged.
	if result[0].Content.String != "read the config" {
		t.Errorf("user message should be unchanged, got: %q", result[0].Content.String)
	}

	// Message 1: Turn 1 file_read of config.json — should be elided.
	if !strings.Contains(result[1].Content.String, "elided") {
		t.Errorf("turn 1 config.json should be elided, got: %q", result[1].Content.String)
	}

	// Message 2: Shell result — unchanged (no dedup, no line numbers).
	if !strings.Contains(result[2].Content.String, "go test") {
		t.Errorf("shell result should be unchanged, got: %q", result[2].Content.String)
	}

	// Message 3: Turn 3 file_read of config.json — line numbers stripped, JSON not minified
	// (because the JSON is embedded in a file_read format, not pure JSON).
	r3 := result[3].Content.String
	if strings.Contains(r3, "1\t") {
		t.Errorf("turn 3 should have line numbers stripped, got: %q", r3)
	}
	if !strings.Contains(r3, "File: config.json") {
		t.Errorf("turn 3 should preserve header, got: %q", r3)
	}

	// Message 4: Current turn — completely untouched.
	if result[4].Content.String != messages[4].Content.String {
		t.Errorf("current turn should be untouched, got: %q", result[4].Content.String)
	}
}
