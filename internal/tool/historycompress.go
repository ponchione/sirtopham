package tool

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// HistoryMessage is the minimal interface needed for Phase 2 compression.
// It mirrors the fields from db.Message that compression inspects and modifies.
type HistoryMessage struct {
	Role       string
	Content    sql.NullString
	ToolName   sql.NullString
	ToolUseID  sql.NullString
	TurnNumber int64
}

// HistoryCompressor applies Phase 2 history compression transforms to
// historical tool results during conversation history serialization.
//
// Phase 2 transforms are lossy but recoverable — the LLM can re-read files
// or re-run commands if it needs full content. Compression trades token cost
// for a tool call the LLM probably won't make.
type HistoryCompressor struct {
	// CurrentTurn is the turn number of the in-progress turn. Messages from
	// this turn are NOT compressed (they're actively in use).
	CurrentTurn int64

	// SummarizeAfterTurns controls stale result summarization. Tool results
	// older than (CurrentTurn - SummarizeAfterTurns) are replaced with a
	// one-line summary. Set to 0 to disable.
	SummarizeAfterTurns int
}

// CompressHistory applies Phase 2 transforms to a slice of history messages.
// Messages are processed in order. The returned slice has the same length but
// tool results from prior turns may have compressed content.
//
// The transforms applied (in order):
//  1. Duplicate result elision — older reads of the same file are replaced with pointers
//  2. Stale result summarization — very old results become one-line summaries
//  3. Line-number stripping — file_read line prefixes are removed from historical results
//  4. JSON re-minification — ensures historical JSON results are compact
func (c *HistoryCompressor) CompressHistory(messages []HistoryMessage) []HistoryMessage {
	if len(messages) == 0 {
		return messages
	}

	// Build the duplicate tracking map: (toolName, filePath) → most recent turn.
	// We need this before processing so we know which results to elide.
	latestReads := c.buildLatestReadsMap(messages)

	result := make([]HistoryMessage, len(messages))
	for i, msg := range messages {
		result[i] = msg // shallow copy

		// Only compress tool messages from prior turns.
		if !c.isEligible(msg) {
			continue
		}

		content := msg.Content.String
		toolName := ""
		if msg.ToolName.Valid {
			toolName = msg.ToolName.String
		}

		// Transform 1: Duplicate result elision.
		if elided, ok := c.tryElide(msg, toolName, latestReads); ok {
			result[i].Content = sql.NullString{String: elided, Valid: true}
			continue // Skip further transforms on elided results.
		}

		// Transform 2: Stale result summarization.
		if summarized, ok := c.trySummarize(msg, toolName); ok {
			result[i].Content = sql.NullString{String: summarized, Valid: true}
			continue // Skip further transforms on summarized results.
		}

		// Transform 3: Line-number stripping (file_read only).
		if toolName == "file_read" {
			content = StripLineNumbers(content)
		}

		// Transform 4: JSON re-minification (idempotent).
		content = minifyJSON(content)

		result[i].Content = sql.NullString{String: content, Valid: true}
	}

	return result
}

// isEligible returns true if a message should be considered for Phase 2 compression.
func (c *HistoryCompressor) isEligible(msg HistoryMessage) bool {
	// Only tool messages.
	if msg.Role != "tool" {
		return false
	}
	// Must have content.
	if !msg.Content.Valid || msg.Content.String == "" {
		return false
	}
	// Must be from a prior turn (not the current turn).
	if msg.TurnNumber >= c.CurrentTurn {
		return false
	}
	return true
}

// fileReadHeaderPattern matches the file_read header line.
// Format: "File: {path} ({N} lines)" or "File: {path} (lines {X}-{Y} of {N})"
var fileReadHeaderPattern = regexp.MustCompile(`^File:\s+(.+?)\s+\((\d+)\s+lines?\)`)
var fileReadHeaderRangePattern = regexp.MustCompile(`^File:\s+(.+?)\s+\(lines\s+\d+-\d+\s+of\s+(\d+)\)`)

// lineNumberPattern matches line-number prefixes in file_read output.
// Format: "{N}\t{content}" where N is right-justified with spaces.
var lineNumberPattern = regexp.MustCompile(`^\s*\d+\t`)

// StripLineNumbers removes line-number prefixes from file_read tool output.
// The header line ("File: path (N lines)") is preserved. Lines that don't
// match the line-number pattern are left unchanged.
func StripLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	var result []string
	for i, line := range lines {
		// Preserve the header line (first line).
		if i == 0 && (fileReadHeaderPattern.MatchString(line) || fileReadHeaderRangePattern.MatchString(line)) {
			result = append(result, line)
			continue
		}

		// Strip line-number prefix if present.
		if loc := lineNumberPattern.FindStringIndex(line); loc != nil {
			// The tab is at loc[1]-1; content starts after the tab.
			tabIdx := strings.Index(line, "\t")
			if tabIdx >= 0 {
				result = append(result, line[tabIdx+1:])
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// extractFileReadPath extracts the file path from a file_read tool result header.
// Returns the path and true if found, empty string and false otherwise.
func extractFileReadPath(content string) (string, bool) {
	// Check first line only.
	firstLine := content
	if idx := strings.Index(content, "\n"); idx >= 0 {
		firstLine = content[:idx]
	}

	if m := fileReadHeaderPattern.FindStringSubmatch(firstLine); len(m) >= 2 {
		return m[1], true
	}
	if m := fileReadHeaderRangePattern.FindStringSubmatch(firstLine); len(m) >= 2 {
		return m[1], true
	}
	return "", false
}

// extractFileReadLineCount extracts the line count from a file_read header.
func extractFileReadLineCount(content string) int {
	firstLine := content
	if idx := strings.Index(content, "\n"); idx >= 0 {
		firstLine = content[:idx]
	}

	if m := fileReadHeaderPattern.FindStringSubmatch(firstLine); len(m) >= 3 {
		return atoiSafe(m[2])
	}
	if m := fileReadHeaderRangePattern.FindStringSubmatch(firstLine); len(m) >= 3 {
		return atoiSafe(m[2])
	}
	return 0
}

// readKey uniquely identifies a file read for deduplication.
type readKey struct {
	toolName string
	filePath string
}

// buildLatestReadsMap scans all messages and records the most recent turn
// number for each (tool_name, file_path) pair that is eligible for deduplication.
func (c *HistoryCompressor) buildLatestReadsMap(messages []HistoryMessage) map[readKey]int64 {
	latest := make(map[readKey]int64)

	for _, msg := range messages {
		if msg.Role != "tool" || !msg.Content.Valid || !msg.ToolName.Valid {
			continue
		}

		toolName := msg.ToolName.String
		if !isDeduplicable(toolName) {
			continue
		}

		filePath, ok := extractToolPath(toolName, msg.Content.String)
		if !ok {
			continue
		}

		key := readKey{toolName: toolName, filePath: filePath}
		if msg.TurnNumber > latest[key] {
			latest[key] = msg.TurnNumber
		}
	}

	return latest
}

// isDeduplicable returns true if the tool's results can be deduplicated.
// Only file_read and git_diff are eligible — shell and search results may
// produce different output at different times.
func isDeduplicable(toolName string) bool {
	return toolName == "file_read" || toolName == "git_diff"
}

// extractToolPath extracts the file/resource path from a tool result for deduplication.
func extractToolPath(toolName string, content string) (string, bool) {
	switch toolName {
	case "file_read":
		return extractFileReadPath(content)
	case "git_diff":
		// git_diff results don't have a standard path header — skip for now.
		return "", false
	}
	return "", false
}

// tryElide checks if this tool result is a duplicate (same file was read in a
// later turn) and returns the elision message if so.
func (c *HistoryCompressor) tryElide(msg HistoryMessage, toolName string, latestReads map[readKey]int64) (string, bool) {
	if !isDeduplicable(toolName) {
		return "", false
	}

	filePath, ok := extractToolPath(toolName, msg.Content.String)
	if !ok {
		return "", false
	}

	key := readKey{toolName: toolName, filePath: filePath}
	latestTurn, exists := latestReads[key]
	if !exists {
		return "", false
	}

	// Only elide if a more recent read exists in a later turn.
	if latestTurn > msg.TurnNumber {
		return fmt.Sprintf("[%s result elided — same file was read again in turn %d. Content from the later read is in history.]",
			toolName, latestTurn), true
	}

	return "", false
}

// trySummarize checks if this tool result is old enough to be summarized.
func (c *HistoryCompressor) trySummarize(msg HistoryMessage, toolName string) (string, bool) {
	if c.SummarizeAfterTurns <= 0 {
		return "", false
	}

	// Only summarize file_read and search_text results.
	if toolName != "file_read" && toolName != "search_text" {
		return "", false
	}

	age := c.CurrentTurn - msg.TurnNumber
	if age < int64(c.SummarizeAfterTurns) {
		return "", false
	}

	switch toolName {
	case "file_read":
		filePath, ok := extractFileReadPath(msg.Content.String)
		if !ok {
			filePath = "(unknown file)"
		}
		lineCount := extractFileReadLineCount(msg.Content.String)
		if lineCount > 0 {
			return fmt.Sprintf("[Historical: file_read of %s returned %d lines at turn %d]",
				filePath, lineCount, msg.TurnNumber), true
		}
		return fmt.Sprintf("[Historical: file_read of %s at turn %d]",
			filePath, msg.TurnNumber), true

	case "search_text":
		// Count result lines for the summary.
		lineCount := strings.Count(msg.Content.String, "\n")
		return fmt.Sprintf("[Historical: search_text returned %d lines at turn %d]",
			lineCount, msg.TurnNumber), true
	}

	return "", false
}

// atoiSafe converts a string to int, returning 0 on failure.
func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
