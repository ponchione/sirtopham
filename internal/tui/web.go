package tui

import (
	"fmt"
	"net/url"
	"strings"
)

const defaultWebBaseURL = "http://localhost:8090"

type webInspectorTarget struct {
	Kind    string
	Label   string
	URL     string
	Command string
}

func normalizeWebBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultWebBaseURL
	}
	return strings.TrimRight(base, "/")
}

func (m Model) webBaseURL() string {
	return normalizeWebBaseURL(m.webBaseURLValue)
}

func (m Model) selectedWebInspectorTarget() (webInspectorTarget, bool) {
	if m.screen == screenReceipts && m.detail != nil && m.receipt != nil {
		chainID := m.detail.Chain.ID
		if strings.TrimSpace(chainID) == "" {
			return webInspectorTarget{}, false
		}
		receiptKey := m.receipt.Step
		if strings.TrimSpace(receiptKey) == "" {
			receiptKey = m.receipt.Path
		}
		targetURL := fmt.Sprintf("%s/chains/%s", m.webBaseURL(), url.PathEscape(chainID))
		if strings.TrimSpace(receiptKey) != "" {
			targetURL += "?receipt=" + url.QueryEscape(receiptKey)
		}
		return webInspectorTarget{
			Kind:    "receipt",
			Label:   m.receipt.Path,
			URL:     targetURL,
			Command: "yard serve",
		}, true
	}

	chainID := m.selectedVisibleChainID()
	if strings.TrimSpace(chainID) == "" {
		return webInspectorTarget{}, false
	}
	return webInspectorTarget{
		Kind:    "chain",
		Label:   chainID,
		URL:     fmt.Sprintf("%s/chains/%s", m.webBaseURL(), url.PathEscape(chainID)),
		Command: "yard serve",
	}, true
}
