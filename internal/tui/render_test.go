package tui

import (
	"strings"
	"testing"
)

func TestDashboardRenderIncludesStableFragments(t *testing.T) {
	model := NewModel(newFakeOperator(), Options{RefreshInterval: -1})
	updated, _ := model.Update(model.refreshCmd()())
	got := updated.(Model)

	view := got.View()
	for _, want := range []string{"Dashboard", "project: project", "provider: codex", "auth: not checked", "code index: indexed at 2026-05-01T12:00:00Z commit abc123", "brain index: disabled", "local services: disabled", "chain-1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard view missing %q:\n%s", want, view)
		}
	}
}

func TestReceiptRenderIncludesContent(t *testing.T) {
	model := NewModel(newFakeOperator(), Options{RefreshInterval: -1})
	model.screen = screenReceipts
	updated, _ := model.Update(model.refreshCmd()())
	got := updated.(Model)

	view := got.View()
	for _, want := range []string{"Receipts", "chain: chain-1", "orchestrator", "orchestrator receipt"} {
		if !strings.Contains(view, want) {
			t.Fatalf("receipt view missing %q:\n%s", want, view)
		}
	}
}

func TestChainRenderIncludesFilterStatus(t *testing.T) {
	model := NewModel(newFakeOperator(), Options{RefreshInterval: -1})
	updated, _ := model.Update(model.refreshCmd()())
	got := updated.(Model)
	got.screen = screenChains
	got.chainFilter.Query = "second"
	updated, _ = got.Update(got.refreshCmd()())
	got = updated.(Model)

	view := got.View()
	for _, want := range []string{"Chains", "filter: /second  matches 1/2 chains", "chain-2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("chain view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "chain-1  running") {
		t.Fatalf("chain view rendered filtered-out chain:\n%s", view)
	}
}

func TestReceiptRenderIncludesFilterStatus(t *testing.T) {
	model := NewModel(newFakeOperator(), Options{RefreshInterval: -1})
	model.screen = screenReceipts
	updated, _ := model.Update(model.refreshCmd()())
	got := updated.(Model)
	got.receiptFilter.Query = "step"
	updated, _ = got.Update(got.refreshCmd()())
	got = updated.(Model)

	view := got.View()
	for _, want := range []string{"Receipts", "filter: /step  matches 1/2 receipts", "step 1 coder"} {
		if !strings.Contains(view, want) {
			t.Fatalf("receipt view missing %q:\n%s", want, view)
		}
	}
}
