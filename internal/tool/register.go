package tool

// RegisterFileTools registers all file tools (file_read, file_write, file_edit)
// in the given registry.
func RegisterFileTools(r *Registry) {
	r.Register(FileRead{})
	r.Register(FileWrite{})
	r.Register(FileEdit{})
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
