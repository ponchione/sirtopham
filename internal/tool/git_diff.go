package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// GitDiff implements the git_diff tool — returns unified diff output for
// working tree, staged, or ref-to-ref comparisons.
type GitDiff struct{}

type gitDiffInput struct {
	Ref1   string `json:"ref1,omitempty"`
	Ref2   string `json:"ref2,omitempty"`
	Staged bool   `json:"staged,omitempty"`
	Path   string `json:"path,omitempty"`
}

func (GitDiff) Name() string { return "git_diff" }
func (GitDiff) Description() string {
	return "Show unified diff for working tree, staged, or ref-to-ref changes"
}
func (GitDiff) ToolPurity() Purity { return Pure }

func (GitDiff) Schema() json.RawMessage {
	return json.RawMessage(`{
		"name": "git_diff",
		"description": "Show unified diff. Modes: working tree (default), staged (staged=true), single ref (ref1), or ref-to-ref (ref1+ref2). Use path to scope to a specific file.",
		"input_schema": {
			"type": "object",
			"properties": {
				"ref1": {
					"type": "string",
					"description": "First git ref (commit, branch, tag). Omit for working tree diff."
				},
				"ref2": {
					"type": "string",
					"description": "Second git ref. Only valid with ref1. Diffs ref1..ref2."
				},
				"staged": {
					"type": "boolean",
					"description": "If true, show staged (cached) changes. Default: false."
				},
				"path": {
					"type": "string",
					"description": "Restrict diff to a specific file or directory path."
				}
			}
		}
	}`)
}

func (GitDiff) Execute(ctx context.Context, projectRoot string, input json.RawMessage) (*ToolResult, error) {
	var params gitDiffInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return &ToolResult{
				Success: false,
				Content: fmt.Sprintf("Invalid input: %v", err),
				Error:   err.Error(),
			}, nil
		}
	}

	gitPath, err := lookupCommandPath("git")
	if err != nil {
		return &ToolResult{
			Success: false,
			Content: "git is required but not found in PATH",
			Error:   "git not found",
		}, nil
	}

	// Build diff command args.
	args := []string{"diff"}

	if params.Staged && params.Ref1 == "" {
		args = append(args, "--cached")
	}
	if params.Ref2 != "" && params.Ref1 == "" {
		return &ToolResult{
			Success: false,
			Content: "ref2 requires ref1. Provide ref1 for ref-to-ref comparisons, or omit both refs for a working tree diff.",
			Error:   "invalid_ref_args",
		}, nil
	}
	if result := validateGitDiffRefParams(params); result != nil {
		return result, nil
	}

	if params.Ref1 != "" {
		if result := verifyGitRef(ctx, gitPath, projectRoot, params.Ref1); result != nil {
			return result, nil
		}
		args = append(args, params.Ref1)
	}
	if params.Ref2 != "" {
		if result := verifyGitRef(ctx, gitPath, projectRoot, params.Ref2); result != nil {
			return result, nil
		}
		args = append(args, params.Ref2)
	}

	if params.Path != "" {
		args = append(args, "--", params.Path)
	}

	output, err := runGitCommand(ctx, gitPath, projectRoot, args...)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not a git repository") {
			return &ToolResult{
				Success: false,
				Content: "Not a git repository (or any parent up to filesystem root)",
				Error:   "not a git repo",
			}, nil
		}
		// Check for bad ref.
		if strings.Contains(errMsg, "unknown revision") || strings.Contains(errMsg, "bad revision") {
			ref := params.Ref1
			if params.Ref2 != "" {
				ref = params.Ref1 + ".." + params.Ref2
			}
			return &ToolResult{
				Success: false,
				Content: fmt.Sprintf("Ref '%s' not found. Use git_status to see available branches and recent commits.", ref),
				Error:   "unknown ref",
			}, nil
		}
		return &ToolResult{
			Success: false,
			Content: fmt.Sprintf("git diff failed: %s", errMsg),
			Error:   errMsg,
		}, nil
	}

	if strings.TrimSpace(output) == "" {
		return &ToolResult{
			Success: true,
			Content: "No differences found",
		}, nil
	}

	return &ToolResult{
		Success: true,
		Content: strings.TrimRight(output, "\n"),
	}, nil
}

func validateGitDiffRefParams(params gitDiffInput) *ToolResult {
	for _, candidate := range []struct {
		label string
		ref   string
	}{
		{label: "ref1", ref: params.Ref1},
		{label: "ref2", ref: params.Ref2},
	} {
		ref := strings.TrimSpace(candidate.ref)
		if ref == "" {
			continue
		}
		if strings.HasPrefix(ref, "-") {
			return invalidGitRefResult(candidate.label, ref, "git refs cannot start with '-'")
		}
		if strings.Contains(ref, "..") {
			return invalidGitRefResult(candidate.label, ref, "git refs cannot contain '..'")
		}
		if strings.Contains(ref, " ") || strings.Contains(ref, "\t") || strings.Contains(ref, "\n") {
			return invalidGitRefResult(candidate.label, ref, "git refs cannot contain whitespace")
		}
		for _, r := range ref {
			if unicode.IsControl(r) {
				return invalidGitRefResult(candidate.label, ref, "git refs cannot contain control characters")
			}
		}
	}
	return nil
}

func invalidGitRefResult(label, ref, reason string) *ToolResult {
	return &ToolResult{
		Success: false,
		Content: fmt.Sprintf("Invalid %s %q: %s.", label, ref, reason),
		Error:   "invalid_ref",
	}
}

func verifyGitRef(ctx context.Context, gitPath, projectRoot, ref string) *ToolResult {
	_, err := runGitCommand(ctx, gitPath, projectRoot, "rev-parse", "--verify", "--quiet", ref+"^{object}")
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "not a git repository") {
		return &ToolResult{
			Success: false,
			Content: "Not a git repository (or any parent up to filesystem root)",
			Error:   "not a git repo",
		}
	}
	return &ToolResult{
		Success: false,
		Content: fmt.Sprintf("Ref '%s' not found. Use git_status to see available branches and recent commits.", ref),
		Error:   "unknown ref",
	}
}
