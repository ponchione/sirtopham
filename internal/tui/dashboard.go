package tui

import (
	"fmt"
	"strings"

	"github.com/ponchione/sodoryard/internal/operator"
)

func (m Model) renderDashboard() string {
	lines := []string{
		m.styles.title.Render("Dashboard"),
		fmt.Sprintf("project: %s", valueOrUnknown(m.status.ProjectName)),
		fmt.Sprintf("root: %s", valueOrUnknown(m.status.ProjectRoot)),
		fmt.Sprintf("provider: %s", valueOrUnknown(m.status.Provider)),
		fmt.Sprintf("model: %s", valueOrUnknown(m.status.Model)),
		fmt.Sprintf("auth: %s", valueOrUnknown(m.status.AuthStatus)),
		fmt.Sprintf("code index: %s", renderIndexStatus(m.status.CodeIndex)),
		fmt.Sprintf("brain index: %s", renderIndexStatus(m.status.BrainIndex)),
		fmt.Sprintf("local services: %s", valueOrUnknown(m.status.LocalServicesStatus)),
		fmt.Sprintf("active chains: %d", m.status.ActiveChains),
		"",
		m.styles.title.Render("Recent chains"),
	}
	if len(m.chains) == 0 {
		lines = append(lines, m.styles.subtle.Render("No chains found."))
	} else {
		for i, ch := range m.chains {
			if i >= 8 {
				break
			}
			task := ch.SourceTask
			if task == "" && len(ch.SourceSpecs) > 0 {
				task = strings.Join(ch.SourceSpecs, ", ")
			}
			lines = append(lines, fmt.Sprintf("%s  %s  steps=%d tokens=%d  %s", ch.ID, ch.Status, ch.TotalSteps, ch.TotalTokens, trimOneLine(task, 48)))
		}
	}
	if len(m.status.Warnings) > 0 {
		lines = append(lines, "", m.styles.title.Render("Warnings"))
		for _, warning := range m.status.Warnings {
			lines = append(lines, m.styles.error.Render(warning.Message))
		}
	}
	if m.err != nil {
		lines = append(lines, "", m.styles.error.Render(m.err.Error()))
	}
	return strings.Join(lines, "\n")
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func renderIndexStatus(status operator.RuntimeIndexStatus) string {
	parts := []string{valueOrUnknown(status.Status)}
	if strings.TrimSpace(status.LastIndexedAt) != "" {
		parts = append(parts, "at "+status.LastIndexedAt)
	}
	if strings.TrimSpace(status.LastIndexedCommit) != "" {
		parts = append(parts, "commit "+status.LastIndexedCommit)
	}
	if strings.TrimSpace(status.StaleSince) != "" {
		parts = append(parts, "stale since "+status.StaleSince)
	}
	if strings.TrimSpace(status.StaleReason) != "" {
		parts = append(parts, status.StaleReason)
	}
	return strings.Join(parts, " ")
}
