package tool

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// NormalizeToolResult applies the Phase 1 write-time normalization pipeline
// to a tool result. The pipeline is:
//
//  1. ANSI escape code stripping (shell only)
//  2. Progress line collapsing (shell only)
//  3. Trailing whitespace cleanup (all)
//  4. JSON minification (when content is valid JSON)
//
// Normalization is lossless — the content is semantically identical before
// and after. The tool name drives tool-specific transforms.
func NormalizeToolResult(toolName string, content string) string {
	if content == "" {
		return content
	}

	// Step 1+2: Shell-specific transforms.
	if toolName == "shell" {
		content = stripANSI(content)
		content = collapseProgressLines(content)
	}

	// Step 3: Whitespace cleanup (all tools).
	content = cleanWhitespace(content)

	// Step 4: JSON minification (all tools).
	content = minifyJSON(content)

	return content
}

// ansiPattern matches all ANSI escape sequences: CSI sequences (ESC[...X),
// OSC sequences (ESC]...ST), and simple two-byte escapes (ESC X).
var ansiPattern = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07\x1b]*(?:\x07|\x1b\\)|\[[0-9;]*m|[()][AB012]|[a-zA-Z])`)

// stripANSI removes all ANSI escape sequences from the content.
// Many CLI tools emit color codes even when stdout is a pipe.
func stripANSI(content string) string {
	return ansiPattern.ReplaceAllString(content, "")
}

// progressPattern matches common progress/status lines from build tools,
// package managers, and test runners. These are lines that typically overwrite
// each other via carriage return or are repetitive status updates.
var progressPattern = regexp.MustCompile(
	`(?i)^\s*(?:` +
		`(?:Compiling|Downloading|Installing|Resolving|Updating|Fetching|Building|Linking|Checking)\s+` + // cargo, npm, pip, go
		`|(?:\d+\s*/\s*\d+\s+)` + // "3/47" style counters
		`|(?:\d+%\s)` + // percentage indicators
		`|(?:\.{3,})` + // progress dots
		`)`,
)

// collapseProgressLines detects runs of progress-style lines and collapses
// them into a single summary. Lines containing \r (carriage return) are also
// treated as progress lines, as they typically overwrite each other.
func collapseProgressLines(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var currentAction string
	var currentCount int

	flush := func() {
		if currentAction != "" && currentCount > 0 {
			result = append(result, collapseProgressSummary(currentAction, currentCount))
			currentAction = ""
			currentCount = 0
		}
	}

	for _, line := range lines {
		// Lines with carriage return: take only the last segment.
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			result = append(result, line)
			continue
		}

		action := detectProgressAction(trimmed)
		if action == "" {
			// Not a progress line.
			flush()
			result = append(result, line)
			continue
		}

		if action == currentAction {
			currentCount++
		} else {
			flush()
			currentAction = action
			currentCount = 1
		}
	}
	flush()

	return strings.Join(result, "\n")
}

// detectProgressAction returns the action keyword if the line matches a
// progress pattern, or empty string if it doesn't.
func detectProgressAction(line string) string {
	actions := []string{
		"Compiling", "Downloading", "Installing", "Resolving",
		"Updating", "Fetching", "Building", "Linking", "Checking",
	}
	lower := strings.ToLower(line)
	for _, action := range actions {
		if strings.Contains(lower, strings.ToLower(action)) {
			return action
		}
	}
	// Percentage or counter patterns.
	if progressPattern.MatchString(line) {
		return "progress"
	}
	return ""
}

// collapseProgressSummary produces the summary line for a run of progress lines.
func collapseProgressSummary(action string, count int) string {
	switch strings.ToLower(action) {
	case "compiling":
		return "[Compiled " + itoa(count) + " crates]"
	case "downloading":
		return "[Downloaded " + itoa(count) + " packages]"
	case "installing":
		return "[Installed " + itoa(count) + " packages]"
	case "fetching":
		return "[Fetched " + itoa(count) + " packages]"
	case "building":
		return "[Built " + itoa(count) + " targets]"
	case "linking":
		return "[Linked " + itoa(count) + " targets]"
	case "checking":
		return "[Checked " + itoa(count) + " crates]"
	case "resolving":
		return "[Resolved " + itoa(count) + " dependencies]"
	case "updating":
		return "[Updated " + itoa(count) + " packages]"
	default:
		return "[" + itoa(count) + " progress lines collapsed]"
	}
}

// itoa is a minimal int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// cleanWhitespace performs three operations:
//  1. Strips trailing whitespace from every line.
//  2. Collapses runs of 3+ empty lines down to 2.
//  3. Trims trailing newlines to at most one.
//
// Content that had no trailing newline retains none.
func cleanWhitespace(content string) string {
	if content == "" {
		return content
	}

	hadTrailingNewline := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")

	// Strip trailing whitespace from each line.
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}

	// Collapse runs of 3+ empty lines down to 2.
	var result []string
	emptyCount := 0
	for _, line := range lines {
		if line == "" {
			emptyCount++
			if emptyCount <= 2 {
				result = append(result, line)
			}
		} else {
			emptyCount = 0
			result = append(result, line)
		}
	}

	// Trim trailing empty lines.
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	out := strings.Join(result, "\n")
	if hadTrailingNewline && out != "" {
		out += "\n"
	}
	return out
}

// minifyJSON compacts the content if it is valid JSON, removing all
// insignificant whitespace. Non-JSON content passes through unchanged.
//
// Uses json.Compact from the standard library — zero external dependencies.
func minifyJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}

	// Quick check: must start with { or [ to be JSON.
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return content
	}

	if !json.Valid([]byte(trimmed)) {
		return content
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(trimmed)); err != nil {
		return content
	}

	compacted := buf.String()

	// Only use the compacted form if it's actually smaller.
	if len(compacted) < len(trimmed) {
		// Preserve trailing newline if the original had one.
		if strings.HasSuffix(content, "\n") {
			return compacted + "\n"
		}
		return compacted
	}
	return content
}
