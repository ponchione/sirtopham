package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ponchione/sodoryard/internal/operator"
)

func filterChains(chains []operator.ChainSummary, query string) []operator.ChainSummary {
	query = normalizeFilterQuery(query)
	if query == "" {
		return append([]operator.ChainSummary(nil), chains...)
	}
	filtered := make([]operator.ChainSummary, 0, len(chains))
	for _, ch := range chains {
		if chainMatchesFilter(ch, query) {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func chainMatchesFilter(ch operator.ChainSummary, query string) bool {
	fields := []string{
		ch.ID,
		ch.Status,
		ch.SourceTask,
		strings.Join(ch.SourceSpecs, " "),
		strconv.Itoa(ch.TotalSteps),
		strconv.Itoa(ch.TotalTokens),
	}
	if ch.CurrentStep != nil {
		fields = append(fields,
			ch.CurrentStep.ID,
			strconv.Itoa(ch.CurrentStep.SequenceNum),
			ch.CurrentStep.Role,
			ch.CurrentStep.Status,
			ch.CurrentStep.Verdict,
			ch.CurrentStep.ReceiptPath,
			strconv.Itoa(ch.CurrentStep.TokensUsed),
		)
	}
	return fieldsMatchFilter(fields, query)
}

func filterReceiptItems(items []receiptItem, query string, loaded *operator.ReceiptView) []receiptItem {
	query = normalizeFilterQuery(query)
	if query == "" {
		return append([]receiptItem(nil), items...)
	}
	filtered := make([]receiptItem, 0, len(items))
	for _, item := range items {
		if receiptItemMatchesFilter(item, query, loaded) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func receiptItemMatchesFilter(item receiptItem, query string, loaded *operator.ReceiptView) bool {
	fields := []string{item.Label, item.Step, item.Path}
	if loadedReceiptMatchesItem(loaded, item) {
		fields = append(fields, loaded.Content)
	}
	return fieldsMatchFilter(fields, query)
}

func loadedReceiptMatchesItem(loaded *operator.ReceiptView, item receiptItem) bool {
	if loaded == nil {
		return false
	}
	if strings.TrimSpace(item.Path) != "" && loaded.Path == item.Path {
		return true
	}
	return loaded.Step == item.Step && loaded.Step != ""
}

func renderFilterStatus(query string, editing bool, visibleCount int, totalCount int, noun string) string {
	value := strings.TrimSpace(query)
	if value == "" {
		value = "none"
	} else {
		value = "/" + query
	}
	if editing {
		if strings.TrimSpace(query) == "" {
			value = "/_"
		} else {
			value += "_"
		}
	}
	return fmt.Sprintf("filter: %s  matches %d/%d %s", value, visibleCount, totalCount, noun)
}

func fieldsMatchFilter(fields []string, query string) bool {
	query = normalizeFilterQuery(query)
	if query == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func normalizeFilterQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}
