package tool

import (
	"strings"
	"testing"
)

// --- stripANSI ---

func TestStripANSI_ColorCodes(t *testing.T) {
	input := "\x1b[31mERROR\x1b[0m: something failed"
	got := stripANSI(input)
	want := "ERROR: something failed"
	if got != want {
		t.Errorf("stripANSI = %q, want %q", got, want)
	}
}

func TestStripANSI_BoldAndColors(t *testing.T) {
	input := "\x1b[1m\x1b[32mPASS\x1b[0m tests/auth_test.go"
	got := stripANSI(input)
	want := "PASS tests/auth_test.go"
	if got != want {
		t.Errorf("stripANSI = %q, want %q", got, want)
	}
}

func TestStripANSI_NoEscapes(t *testing.T) {
	input := "plain text with no escapes"
	got := stripANSI(input)
	if got != input {
		t.Errorf("stripANSI changed plain text: %q", got)
	}
}

func TestStripANSI_MultiplePerLine(t *testing.T) {
	input := "\x1b[33mWARN\x1b[0m: \x1b[36mhttp\x1b[0m: timeout"
	got := stripANSI(input)
	want := "WARN: http: timeout"
	if got != want {
		t.Errorf("stripANSI = %q, want %q", got, want)
	}
}

func TestStripANSI_CursorMovement(t *testing.T) {
	// CSI sequences for cursor positioning: ESC[2J (clear screen), ESC[H (home)
	input := "\x1b[2J\x1b[HWelcome"
	got := stripANSI(input)
	want := "Welcome"
	if got != want {
		t.Errorf("stripANSI = %q, want %q", got, want)
	}
}

// --- collapseProgressLines ---

func TestCollapseProgressLines_CargoBuild(t *testing.T) {
	lines := []string{
		"   Compiling proc-macro2 v1.0.78",
		"   Compiling unicode-ident v1.0.12",
		"   Compiling quote v1.0.35",
		"   Compiling syn v2.0.48",
		"   Compiling serde_derive v1.0.196",
		"   Compiling serde v1.0.196",
		"   Compiling tokio v1.35.1",
	}
	input := strings.Join(lines, "\n")
	got := collapseProgressLines(input)
	want := "[Compiled 7 crates]"
	if got != want {
		t.Errorf("collapseProgressLines =\n%s\nwant:\n%s", got, want)
	}
}

func TestCollapseProgressLines_MixedWithNormal(t *testing.T) {
	input := "Starting build\n" +
		"   Compiling foo v0.1.0\n" +
		"   Compiling bar v0.2.0\n" +
		"   Compiling baz v0.3.0\n" +
		"    Finished dev [unoptimized] in 4.2s"
	got := collapseProgressLines(input)
	if !strings.Contains(got, "Starting build") {
		t.Error("lost 'Starting build' line")
	}
	if !strings.Contains(got, "[Compiled 3 crates]") {
		t.Errorf("missing collapsed summary, got:\n%s", got)
	}
	if !strings.Contains(got, "Finished dev") {
		t.Error("lost 'Finished' line")
	}
}

func TestCollapseProgressLines_CarriageReturn(t *testing.T) {
	// Lines overwriting each other via \r
	input := "Downloading: 10%\rDownloading: 50%\rDownloading: 100%\nDone!"
	got := collapseProgressLines(input)
	if !strings.Contains(got, "[Downloaded") {
		t.Errorf("expected collapsed downloading, got:\n%s", got)
	}
	if !strings.Contains(got, "Done!") {
		t.Error("lost 'Done!' line")
	}
}

func TestCollapseProgressLines_NoProgress(t *testing.T) {
	input := "go test ./...\nok  \tgithub.com/example/pkg\t0.015s\nFAIL\tgithub.com/example/other\t0.022s"
	got := collapseProgressLines(input)
	if got != input {
		t.Errorf("collapseProgressLines modified non-progress output:\n%s", got)
	}
}

func TestCollapseProgressLines_Downloading(t *testing.T) {
	lines := []string{
		"Downloading express@4.18.2",
		"Downloading lodash@4.17.21",
		"Downloading typescript@5.3.0",
	}
	input := strings.Join(lines, "\n")
	got := collapseProgressLines(input)
	if !strings.Contains(got, "[Downloaded 3 packages]") {
		t.Errorf("expected collapsed downloads, got:\n%s", got)
	}
}

func TestCollapseProgressLines_Installing(t *testing.T) {
	lines := []string{
		"Installing express@4.18.2",
		"Installing lodash@4.17.21",
	}
	input := strings.Join(lines, "\n")
	got := collapseProgressLines(input)
	if !strings.Contains(got, "[Installed 2 packages]") {
		t.Errorf("expected collapsed installs, got:\n%s", got)
	}
}

// --- cleanWhitespace ---

func TestCleanWhitespace_TrailingSpaces(t *testing.T) {
	input := "hello   \nworld\t\t\n"
	got := cleanWhitespace(input)
	want := "hello\nworld\n"
	if got != want {
		t.Errorf("cleanWhitespace = %q, want %q", got, want)
	}
}

func TestCleanWhitespace_CollapsesEmptyLines(t *testing.T) {
	input := "a\n\n\n\n\nb\n"
	got := cleanWhitespace(input)
	want := "a\n\n\nb\n"
	if got != want {
		t.Errorf("cleanWhitespace = %q, want %q", got, want)
	}
}

func TestCleanWhitespace_TrimsTrailingNewlines(t *testing.T) {
	input := "hello\n\n\n\n"
	got := cleanWhitespace(input)
	want := "hello\n"
	if got != want {
		t.Errorf("cleanWhitespace = %q, want %q", got, want)
	}
}

func TestCleanWhitespace_PreservesDoubleNewlines(t *testing.T) {
	input := "a\n\nb\n"
	got := cleanWhitespace(input)
	if got != input {
		t.Errorf("cleanWhitespace modified valid double newline: %q", got)
	}
}

func TestCleanWhitespace_EmptyString(t *testing.T) {
	got := cleanWhitespace("")
	if got != "" {
		t.Errorf("cleanWhitespace of empty = %q", got)
	}
}

// --- minifyJSON ---

func TestMinifyJSON_PrettyPrinted(t *testing.T) {
	input := `{
    "name": "my-project",
    "version": "1.0.0",
    "dependencies": {
        "express": "^4.18.2"
    }
}
`
	got := minifyJSON(input)
	want := `{"name":"my-project","version":"1.0.0","dependencies":{"express":"^4.18.2"}}` + "\n"
	if got != want {
		t.Errorf("minifyJSON =\n%s\nwant:\n%s", got, want)
	}
}

func TestMinifyJSON_Array(t *testing.T) {
	input := `[
    "a",
    "b",
    "c"
]`
	got := minifyJSON(input)
	want := `["a","b","c"]`
	if got != want {
		t.Errorf("minifyJSON = %q, want %q", got, want)
	}
}

func TestMinifyJSON_ArrayWithNewline(t *testing.T) {
	input := "[\"a\",\n    \"b\"\n]\n"
	got := minifyJSON(input)
	want := `["a","b"]` + "\n"
	if got != want {
		t.Errorf("minifyJSON = %q, want %q", got, want)
	}
}

func TestMinifyJSON_AlreadyCompact(t *testing.T) {
	input := `{"name":"test","version":"1.0.0"}` + "\n"
	got := minifyJSON(input)
	// Already compact — should pass through unchanged.
	if got != input {
		t.Errorf("minifyJSON modified already-compact JSON: %q", got)
	}
}

func TestMinifyJSON_NotJSON(t *testing.T) {
	input := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	got := minifyJSON(input)
	if got != input {
		t.Errorf("minifyJSON modified non-JSON content")
	}
}

func TestMinifyJSON_EmptyString(t *testing.T) {
	got := minifyJSON("")
	if got != "" {
		t.Errorf("minifyJSON of empty = %q", got)
	}
}

func TestMinifyJSON_PlainText(t *testing.T) {
	input := "this is just plain text"
	got := minifyJSON(input)
	if got != input {
		t.Errorf("minifyJSON modified plain text")
	}
}

// --- NormalizeToolResult (pipeline integration) ---

func TestNormalizeToolResult_ShellWithANSIAndProgress(t *testing.T) {
	input := "\x1b[32m   Compiling\x1b[0m foo v0.1.0\n" +
		"\x1b[32m   Compiling\x1b[0m bar v0.2.0\n" +
		"\x1b[32m   Compiling\x1b[0m baz v0.3.0\n" +
		"    Finished dev [unoptimized] in 2.1s\n"

	got := NormalizeToolResult("shell", input)

	if strings.Contains(got, "\x1b") {
		t.Error("ANSI codes not stripped")
	}
	if !strings.Contains(got, "[Compiled 3 crates]") {
		t.Errorf("progress not collapsed, got:\n%s", got)
	}
	if !strings.Contains(got, "Finished") {
		t.Error("lost Finished line")
	}
}

func TestNormalizeToolResult_FileReadJSON(t *testing.T) {
	input := `{
    "name": "test",
    "version": "1.0.0"
}
`
	got := NormalizeToolResult("file_read", input)
	want := `{"name":"test","version":"1.0.0"}` + "\n"
	if got != want {
		t.Errorf("NormalizeToolResult(file_read) = %q, want %q", got, want)
	}
}

func TestNormalizeToolResult_FileReadCode(t *testing.T) {
	// Go source code should not be minified.
	input := "package main\n\nfunc main() {}\n"
	got := NormalizeToolResult("file_read", input)
	if got != input {
		t.Errorf("NormalizeToolResult(file_read) modified Go code: %q", got)
	}
}

func TestNormalizeToolResult_ShellNoProgress(t *testing.T) {
	input := "ok  \tgithub.com/example/pkg\t0.015s\n"
	got := NormalizeToolResult("shell", input)
	// Should strip trailing whitespace but not add progress summary.
	if strings.Contains(got, "[") && strings.Contains(got, "collapsed") {
		t.Errorf("incorrectly detected progress lines in:\n%s", got)
	}
}

func TestNormalizeToolResult_EmptyContent(t *testing.T) {
	got := NormalizeToolResult("shell", "")
	if got != "" {
		t.Errorf("NormalizeToolResult of empty = %q", got)
	}
}

func TestNormalizeToolResult_SearchText(t *testing.T) {
	// search_text output — should get whitespace cleanup but not shell transforms.
	input := "internal/auth/service.go:42:  func Login()   \ninternal/auth/service.go:58:  func Logout()   \n"
	got := NormalizeToolResult("search_text", input)
	// Trailing spaces should be stripped.
	if strings.Contains(got, "   \n") {
		t.Error("trailing whitespace not cleaned in search_text result")
	}
}

func TestNormalizeToolResult_WhitespaceCleanupOrder(t *testing.T) {
	// Verify that whitespace cleanup happens before JSON minification.
	// Input is JSON with trailing spaces and extra newlines.
	input := "{\n    \"a\": 1   \n}\n\n\n\n"
	got := NormalizeToolResult("file_read", input)
	want := `{"a":1}` + "\n"
	if got != want {
		t.Errorf("NormalizeToolResult = %q, want %q", got, want)
	}
}

// --- itoa ---

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{999, "999"},
	}
	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
