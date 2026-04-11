package indexstate

import (
	"testing"
	"time"
)

func TestMarkStaleThenFreshRoundTrip(t *testing.T) {
	projectRoot := t.TempDir()
	staleAt := time.Date(2026, 4, 9, 15, 4, 5, 0, time.UTC)
	if err := MarkStale(projectRoot, "brain_write", staleAt); err != nil {
		t.Fatalf("MarkStale returned error: %v", err)
	}
	state, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load after stale returned error: %v", err)
	}
	if state.Status != StatusStale {
		t.Fatalf("Status = %q, want %q", state.Status, StatusStale)
	}
	if state.StaleSince != staleAt.Format(time.RFC3339) {
		t.Fatalf("StaleSince = %q, want %q", state.StaleSince, staleAt.Format(time.RFC3339))
	}
	if state.StaleReason != "brain_write" {
		t.Fatalf("StaleReason = %q, want brain_write", state.StaleReason)
	}
	if state.LastIndexedAt != "" {
		t.Fatalf("LastIndexedAt = %q, want empty before first reindex", state.LastIndexedAt)
	}

	freshAt := staleAt.Add(2 * time.Hour)
	if err := MarkFresh(projectRoot, freshAt); err != nil {
		t.Fatalf("MarkFresh returned error: %v", err)
	}
	state, err = Load(projectRoot)
	if err != nil {
		t.Fatalf("Load after fresh returned error: %v", err)
	}
	if state.Status != StatusClean {
		t.Fatalf("Status = %q, want %q", state.Status, StatusClean)
	}
	if state.LastIndexedAt != freshAt.Format(time.RFC3339) {
		t.Fatalf("LastIndexedAt = %q, want %q", state.LastIndexedAt, freshAt.Format(time.RFC3339))
	}
	if state.StaleSince != "" {
		t.Fatalf("StaleSince = %q, want empty after MarkFresh", state.StaleSince)
	}
	if state.StaleReason != "" {
		t.Fatalf("StaleReason = %q, want empty after MarkFresh", state.StaleReason)
	}
}

func TestLoadMissingStateReturnsNeverIndexed(t *testing.T) {
	state, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if state.Status != StatusNeverIndexed {
		t.Fatalf("Status = %q, want %q", state.Status, StatusNeverIndexed)
	}
}
