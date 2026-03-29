package id

import (
	"regexp"
	"testing"
)

func TestNewProducesUUIDv7Format(t *testing.T) {
	id := New()

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(id) {
		t.Fatalf("generated id %q does not match UUIDv7 format", id)
	}
}

func TestNewGeneratesUniqueIDs(t *testing.T) {
	seen := make(map[string]struct{}, 10000)
	for range 10000 {
		id := New()
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate id generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewMaintainsLexicographicOrdering(t *testing.T) {
	previous := New()
	for i := 0; i < 9999; i++ {
		current := New()
		if current <= previous {
			t.Fatalf("IDs are not strictly increasing: previous=%s current=%s", previous, current)
		}
		previous = current
	}
}
