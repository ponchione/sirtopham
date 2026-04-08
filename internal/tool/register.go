package tool

import (
	"github.com/ponchione/sirtopham/internal/brain"
	"github.com/ponchione/sirtopham/internal/config"
	"github.com/ponchione/sirtopham/internal/provider"
)

// RegisterFileTools registers all file tools (file_read, file_write, file_edit)
// in the given registry.
func RegisterFileTools(r *Registry) {
	store := newMemoryReadStateStore()
	r.Register(NewFileRead(store))
	r.Register(NewFileWrite(store))
	r.Register(NewFileEdit(store))
}

// RegisterGitTools registers all git tools (git_status, git_diff) in the
// given registry.
func RegisterGitTools(r *Registry) {
	r.Register(GitStatus{})
	r.Register(GitDiff{})
}

// RegisterShellTool registers the shell tool in the given registry.
func RegisterShellTool(r *Registry, config ShellConfig) {
	r.Register(NewShell(config))
}

// RegisterSearchTools registers all search tools (search_text, search_semantic)
// in the given registry. The searcher parameter is the Layer 1 semantic search
// backend — pass nil to omit search_semantic (search_text still registers).
func RegisterSearchTools(r *Registry, searcher SemanticSearcher) {
	r.Register(SearchText{})
	if searcher != nil {
		r.Register(NewSearchSemantic(searcher))
	}
}

// RegisterBrainTools registers all brain tools (brain_search, brain_read,
// brain_write, brain_update, brain_lint) in the given registry. The backend parameter is
// the configured brain backend — pass nil if brain is disabled (tools will
// return guidance messages when invoked).
func RegisterBrainTools(r *Registry, client brain.Backend, cfg config.BrainConfig) {
	RegisterBrainToolsWithProvider(r, client, cfg, nil)
}

// RegisterBrainToolsWithProvider registers brain tools with an optional
// model provider for the explicit opt-in contradictions check in brain_lint.
func RegisterBrainToolsWithProvider(r *Registry, client brain.Backend, cfg config.BrainConfig, llm provider.Provider) {
	r.Register(NewBrainSearch(client, cfg))
	r.Register(NewBrainRead(client, cfg))
	r.Register(NewBrainWrite(client, cfg))
	r.Register(NewBrainUpdate(client, cfg))
	r.Register(NewBrainLintWithProvider(client, cfg, llm))
}
