package analysis

import "time"

const ContradictionsCheck = "contradictions"

type Document struct {
	Path         string
	Content      string
	Frontmatter  map[string]any
	Tags         []string
	Wikilinks    []string
	UpdatedAt    time.Time
	HasUpdatedAt bool
	Title        string
}

type LintOptions struct {
	Scope           string
	Checks          []string
	StaleAfter      time.Duration
	OrphanAllowlist []string
	Universe        []Document
}

type LintReport struct {
	Scope    string
	Checks   []string
	Summary  LintSummary
	Findings LintFindings
}

type LintSummary struct {
	Documents                  int
	Orphans                    int
	DeadLinks                  int
	StaleReferences            int
	MissingPages               int
	Contradictions             int
	ContradictionPairsExamined int
	SingletonTags              int
	SimilarTagPairs            int
	UntaggedDocuments          int
}

type LintFindings struct {
	Orphans         []OrphanFinding
	DeadLinks       []DeadLinkFinding
	StaleReferences []StaleReferenceFinding
	MissingPages    []MissingPageFinding
	Contradictions  []ContradictionFinding
	TagHygiene      TagHygieneFindings
}

type OrphanFinding struct {
	Path string
}

type DeadLinkFinding struct {
	Source string
	Target string
}

type StaleReferenceFinding struct {
	Source          string
	Target          string
	SourceUpdatedAt time.Time
	TargetUpdatedAt time.Time
	AgeDelta        time.Duration
}

type MissingPageFinding struct {
	Target string
	Count  int
}

type ContradictionFinding struct {
	Left       string
	Right      string
	Summary    string
	Confidence string
}

type TagHygieneFindings struct {
	SingletonTags     []SingletonTagFinding
	SimilarTagPairs   []SimilarTagPairFinding
	UntaggedDocuments []UntaggedDocumentFinding
}

type SingletonTagFinding struct {
	Tag   string
	Paths []string
}

type SimilarTagPairFinding struct {
	Left  string
	Right string
}

type UntaggedDocumentFinding struct {
	Path string
}
