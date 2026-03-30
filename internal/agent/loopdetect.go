package agent

import (
	"encoding/json"
	"sort"

	"github.com/ponchione/sirtopham/internal/provider"
)

// loopDetector tracks tool call patterns across iterations to detect when the
// LLM is stuck repeating the same action. It compares tool call signatures
// (name + canonicalized JSON arguments) across consecutive iterations.
type loopDetector struct {
	threshold int
	// history stores the canonicalized tool call signature set for each iteration.
	// Index corresponds to iteration number - 1.
	history [][]string
}

// newLoopDetector creates a loop detector with the given threshold.
// A threshold of 0 or negative disables detection.
func newLoopDetector(threshold int) *loopDetector {
	return &loopDetector{
		threshold: threshold,
	}
}

// record stores the tool calls for the current iteration. Must be called
// in iteration order.
func (d *loopDetector) record(calls []provider.ToolCall) {
	if d == nil {
		return
	}
	sigs := make([]string, 0, len(calls))
	for _, tc := range calls {
		sigs = append(sigs, toolCallSignature(tc.Name, tc.Input))
	}
	sort.Strings(sigs)
	d.history = append(d.history, sigs)
}

// isLooping returns true if the most recent N iterations (where N = threshold)
// all have identical tool call signature sets. Must be called after record()
// for the current iteration.
func (d *loopDetector) isLooping() bool {
	if d == nil || d.threshold <= 1 || len(d.history) < d.threshold {
		return false
	}

	// Compare the last `threshold` entries.
	latest := d.history[len(d.history)-1]
	for i := len(d.history) - 2; i >= len(d.history)-d.threshold; i-- {
		if !signaturesEqual(latest, d.history[i]) {
			return false
		}
	}
	return true
}

// toolCallSignature produces a canonical string key for a tool call:
// "tool_name:" + canonicalized JSON arguments.
func toolCallSignature(name string, input json.RawMessage) string {
	canonical := canonicalizeJSON(input)
	return name + ":" + canonical
}

// canonicalizeJSON normalizes a JSON value by unmarshalling and re-marshalling
// with sorted keys. This ensures {"a":1,"b":2} and {"b":2,"a":1} produce the
// same string. Returns the original string on parse error.
func canonicalizeJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}

	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw)
	}

	canonical, err := json.Marshal(sortKeys(parsed))
	if err != nil {
		return string(raw)
	}
	return string(canonical)
}

// sortKeys recursively processes a parsed JSON value to ensure maps have
// sorted keys when re-marshalled. Go's json.Marshal already sorts map keys
// for map[string]interface{}, so this mainly ensures the value is in the
// right shape.
func sortKeys(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		sorted := make(map[string]interface{}, len(val))
		for k, v := range val {
			sorted[k] = sortKeys(v)
		}
		return sorted
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = sortKeys(v)
		}
		return result
	default:
		return v
	}
}

// signaturesEqual compares two sorted signature slices for equality.
func signaturesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
