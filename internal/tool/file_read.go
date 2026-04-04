package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

// FileRead implements the file_read tool — reads file contents with optional
// line range selection and returns content with line numbers.
type FileRead struct {
	store readStateStore
}

func NewFileRead(store readStateStore) FileRead {
	return FileRead{store: store}
}

type fileReadInput struct {
	Path      string `json:"path"`
	LineStart *int   `json:"line_start,omitempty"`
	LineEnd   *int   `json:"line_end,omitempty"`
}

func (FileRead) Name() string        { return "file_read" }
func (FileRead) Description() string { return "Read file contents with optional line range" }
func (FileRead) ToolPurity() Purity  { return Pure }

func (FileRead) Schema() json.RawMessage {
	return json.RawMessage(`{
		"name": "file_read",
		"description": "Read file contents, optionally with a line range. Returns content with line numbers.",
		"input_schema": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "File path relative to the project root"
				},
				"line_start": {
					"type": "integer",
					"description": "First line to read (1-indexed, inclusive). Omit to start from line 1."
				},
				"line_end": {
					"type": "integer",
					"description": "Last line to read (1-indexed, inclusive). Omit to read to end of file."
				}
			},
			"required": ["path"]
		}
	}`)
}

func (f FileRead) Execute(ctx context.Context, projectRoot string, input json.RawMessage) (*ToolResult, error) {
	var params fileReadInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Invalid input: %v", err),
			Error:   err.Error(),
		}, nil
	}

	absPath, err := resolvePath(projectRoot, params.Path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Content: err.Error(),
			Error:   err.Error(),
		}, nil
	}

	file, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			msg := fileNotFoundError(projectRoot, params.Path)
			return &ToolResult{Success: false, Content: msg, Error: "file not found"}, nil
		}
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Error reading file: %v", err),
			Error:   err.Error(),
		}, nil
	}
	defer file.Close()

	probe := make([]byte, 8192)
	n, probeErr := file.Read(probe)
	if probeErr != nil && probeErr != io.EOF {
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Error reading file: %v", probeErr),
			Error:   probeErr.Error(),
		}, nil
	}
	probe = probe[:n]

	if isBinaryContent(probe) {
		return &ToolResult{
			Success: false,
			Content: "Binary file detected, cannot display content",
			Error:   "binary file",
		}, nil
	}

	if len(probe) == 0 {
		return &ToolResult{
			Success: true,
			Content: fmt.Sprintf("File: %s (0 lines)\n(empty file)", params.Path),
		}, nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Error seeking file: %v", err),
			Error:   err.Error(),
		}, nil
	}

	kind := readKindFull
	lineStart := 1
	requestedLineEnd := math.MaxInt
	if params.LineStart != nil {
		lineStart = *params.LineStart
		kind = readKindPartial
	}
	if params.LineEnd != nil {
		requestedLineEnd = *params.LineEnd
		kind = readKindPartial
	}

	if lineStart < 1 {
		lineStart = 1
	}
	if requestedLineEnd < 1 {
		requestedLineEnd = 1
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	totalLines := 0
	selectedLines := make([]string, 0)
	for scanner.Scan() {
		totalLines++
		if totalLines >= lineStart && totalLines <= requestedLineEnd {
			selectedLines = append(selectedLines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("Error reading file: %v", err),
			Error:   err.Error(),
		}, nil
	}

	lineEnd := requestedLineEnd
	if lineEnd > totalLines {
		lineEnd = totalLines
	}

	if lineStart > totalLines {
		kind = readKindPartial
		return &ToolResult{
			Success: true,
			Content: fmt.Sprintf("File: %s (%d lines)\n(line_start %d is beyond end of file)", params.Path, totalLines, lineStart),
		}, nil
	}

	maxSelected := lineEnd - lineStart + 1
	if maxSelected < 0 {
		maxSelected = 0
	}
	if len(selectedLines) > maxSelected {
		selectedLines = selectedLines[:maxSelected]
	}

	width := int(math.Log10(float64(lineEnd))) + 1
	if width < 1 {
		width = 1
	}

	var sb strings.Builder
	if lineStart == 1 && lineEnd == totalLines {
		sb.WriteString(fmt.Sprintf("File: %s (%d lines)\n", params.Path, totalLines))
	} else {
		sb.WriteString(fmt.Sprintf("File: %s (lines %d-%d of %d)\n", params.Path, lineStart, lineEnd, totalLines))
	}

	for i, line := range selectedLines {
		lineNum := lineStart + i
		sb.WriteString(fmt.Sprintf("%*d\t%s\n", width, lineNum, line))
	}

	if lineStart == 1 && lineEnd == totalLines {
		kind = readKindFull
	}
	if kind == readKindFull {
		info, err := os.Stat(absPath)
		if err == nil {
			data, readErr := os.ReadFile(absPath)
			if readErr != nil {
				return &ToolResult{
					Success: false,
					Content: fmt.Sprintf("Error reading file for snapshot: %v", readErr),
					Error:   readErr.Error(),
				}, nil
			}
			store := f.store
			if store == nil {
				store = defaultReadStateStore
			}
			_ = store.Put(ctx, snapshotForFile(ctx, absPath, data, info, kind, time.Now()))
		}
	}

	return &ToolResult{
		Success: true,
		Content: sb.String(),
	}, nil
}
