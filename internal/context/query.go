package context

import (
	"regexp"
	"strings"
)

var sentenceSplitPattern = regexp.MustCompile(`(?:[.!?]+\s+|[.!?]+$)`)
var nonQueryCharacterPattern = regexp.MustCompile(`[^a-z0-9\s]+`)
var underscoreIdentifierPattern = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b`)
var dotNotationPattern = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+\b`)
var statusCodePattern = regexp.MustCompile(`\b(?:200|201|204|400|401|403|404|409|422|429|500|502|503)\b`)
var httpMethodPattern = regexp.MustCompile(`\b(?:GET|POST|PUT|DELETE|PATCH)\b`)

var fillerPhrases = []string{
	"hey",
	"can you",
	"please",
	"i think",
	"i want you to",
	"could you",
	"help me",
	"let's",
	"lets",
}

var technicalDomainTerms = []string{
	"middleware",
	"handler",
	"router",
	"schema",
	"migration",
	"query",
	"endpoint",
	"service",
	"repository",
	"controller",
	"factory",
	"adapter",
}

// HeuristicQueryExtractor implements the v0.1 deterministic semantic-query builder.
//
// It converts the current turn plus analyzer output into up to three semantic
// search queries without performing any I/O or model calls.
type HeuristicQueryExtractor struct{}

// ExtractQueries builds up to three distinct semantic queries from the current
// message and any momentum metadata already present on ContextNeeds.
func (HeuristicQueryExtractor) ExtractQueries(message string, needs *ContextNeeds) []string {
	cleanedQueries := cleanedMessageQueries(message, needs)
	technicalQuery := technicalKeywordQuery(message, needs)
	momentumQuery := momentumEnhancedQuery(cleanedQueries, needs)

	var queries []string
	for _, query := range cleanedQueries {
		appendUniqueQuery(&queries, query)
		if len(queries) == 3 {
			return queries
		}
	}

	appendUniqueQuery(&queries, technicalQuery)
	if len(queries) == 3 {
		return queries
	}

	appendUniqueQuery(&queries, momentumQuery)
	if len(queries) > 3 {
		return queries[:3]
	}
	return queries
}

func cleanedMessageQueries(message string, needs *ContextNeeds) []string {
	sentences := splitIntoSentences(message)
	if len(sentences) == 0 {
		sentences = []string{message}
	}

	var queries []string
	for _, sentence := range sentences {
		query := cleanedSentenceQuery(sentence, needs)
		appendUniqueQuery(&queries, query)
		if len(queries) == 2 {
			break
		}
	}
	return queries
}

func splitIntoSentences(message string) []string {
	parts := sentenceSplitPattern.Split(message, -1)
	var sentences []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sentences = append(sentences, part)
	}
	return sentences
}

func cleanedSentenceQuery(sentence string, needs *ContextNeeds) string {
	query := strings.ToLower(sentence)
	for _, phrase := range fillerPhrases {
		query = replaceWholePhrase(query, phrase)
	}
	query = removeExplicitEntities(query, needs)
	query = nonQueryCharacterPattern.ReplaceAllString(query, " ")
	query = collapseWhitespace(query)
	return capWords(query, 50)
}

func technicalKeywordQuery(message string, needs *ContextNeeds) string {
	exclusions := explicitEntityExclusions(needs)
	var terms []string
	seen := make(map[string]struct{})

	appendTermMatches(&terms, seen, underscoreIdentifierPattern.FindAllString(message, -1), exclusions)
	appendTermMatches(&terms, seen, dotNotationPattern.FindAllString(message, -1), exclusions)
	appendTermMatches(&terms, seen, httpMethodPattern.FindAllString(message, -1), exclusions)
	appendTermMatches(&terms, seen, statusCodePattern.FindAllString(message, -1), exclusions)

	for _, token := range strings.Fields(message) {
		clean := strings.Trim(token, "`\"'()[]{}.,!?;:")
		if clean == "" {
			continue
		}
		if looksLikeCodeIdentifier(clean) {
			appendTechnicalTerm(&terms, seen, clean, exclusions)
		}
	}

	lowerMessage := strings.ToLower(message)
	for _, term := range technicalDomainTerms {
		if strings.Contains(lowerMessage, term) {
			appendTechnicalTerm(&terms, seen, term, exclusions)
		}
	}

	return strings.Join(terms, " ")
}

func appendTermMatches(dst *[]string, seen map[string]struct{}, matches []string, exclusions map[string]struct{}) {
	for _, match := range matches {
		appendTechnicalTerm(dst, seen, match, exclusions)
	}
}

func appendTechnicalTerm(dst *[]string, seen map[string]struct{}, term string, exclusions map[string]struct{}) {
	term = strings.TrimSpace(term)
	if term == "" {
		return
	}
	lower := strings.ToLower(term)
	if _, excluded := exclusions[lower]; excluded {
		return
	}
	if _, exists := seen[lower]; exists {
		return
	}
	seen[lower] = struct{}{}
	*dst = append(*dst, term)
}

func momentumEnhancedQuery(cleanedQueries []string, needs *ContextNeeds) string {
	if needs == nil || needs.MomentumModule == "" || len(cleanedQueries) == 0 {
		return ""
	}
	return needs.MomentumModule + " " + cleanedQueries[0]
}

func explicitEntityExclusions(needs *ContextNeeds) map[string]struct{} {
	exclusions := make(map[string]struct{})
	if needs == nil {
		return exclusions
	}
	for _, value := range needs.ExplicitFiles {
		exclusions[strings.ToLower(value)] = struct{}{}
	}
	for _, value := range needs.ExplicitSymbols {
		exclusions[strings.ToLower(value)] = struct{}{}
	}
	return exclusions
}

func removeExplicitEntities(query string, needs *ContextNeeds) string {
	for value := range explicitEntityExclusions(needs) {
		query = strings.ReplaceAll(query, value, " ")
	}
	return query
}

func replaceWholePhrase(input string, phrase string) string {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(phrase) + `\b`)
	return pattern.ReplaceAllString(input, " ")
}

func collapseWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func capWords(input string, limit int) string {
	fields := strings.Fields(input)
	if len(fields) <= limit {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:limit], " ")
}

func appendUniqueQuery(dst *[]string, query string) {
	query = collapseWhitespace(strings.TrimSpace(query))
	if query == "" {
		return
	}
	for _, existing := range *dst {
		if existing == query {
			return
		}
	}
	*dst = append(*dst, query)
}
