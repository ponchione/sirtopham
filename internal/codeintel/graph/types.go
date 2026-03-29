package graph

// Symbol represents a named code entity in the project source tree.
type Symbol struct {
	ID        string
	Name      string
	Kind      string // function, method, type, interface, class, module
	Language  string
	Package   string
	FilePath  string
	LineStart int
	LineEnd   int
	Signature string
	Exported  bool
	Receiver  string // Go methods only
}

// Edge represents a structural relationship between two symbols.
type Edge struct {
	SourceID   string
	TargetID   string
	EdgeType   string  // CALLS, IMPORTS, IMPLEMENTS, EMBEDS, EXTENDS, INSTANTIATES
	Confidence float64
	SourceLine int
	Metadata   string
}

// BoundarySymbol is an external symbol that terminates blast radius queries.
type BoundarySymbol struct {
	ID       string
	Name     string
	Kind     string
	Language string
	Package  string
}

// AnalysisResult holds the output of a language analyzer.
type AnalysisResult struct {
	Symbols         []Symbol
	Edges           []Edge
	BoundarySymbols []BoundarySymbol
}

// Merge combines another AnalysisResult into this one.
func (r *AnalysisResult) Merge(other *AnalysisResult) {
	r.Symbols = append(r.Symbols, other.Symbols...)
	r.Edges = append(r.Edges, other.Edges...)
	r.BoundarySymbols = append(r.BoundarySymbols, other.BoundarySymbols...)
}
