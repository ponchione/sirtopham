package router

import (
	"errors"

	"github.com/ponchione/sirtopham/internal/provider"
)

// errorClass categorizes an error for routing decisions.
type errorClass int

const (
	// errorClassAuth indicates an authentication error (HTTP 401, 403).
	// Auth errors never trigger fallback.
	errorClassAuth errorClass = iota

	// errorClassRetriable indicates a transient error (HTTP 429, 500, 502, 503,
	// or network errors) where a fallback attempt may succeed.
	errorClassRetriable

	// errorClassFatal indicates a non-retriable error that should not trigger
	// fallback (e.g., HTTP 400, context canceled, unknown errors).
	errorClassFatal
)

// classifyError inspects an error and returns its classification for routing
// decisions. Auth errors (401, 403) override the Retriable field and are never
// retried. Retriable ProviderErrors trigger fallback. All other errors are
// fatal and returned immediately.
func classifyError(err error) errorClass {
	var pe *provider.ProviderError
	if !errors.As(err, &pe) {
		return errorClassFatal
	}

	// Auth errors take precedence regardless of the Retriable field.
	if pe.StatusCode == 401 || pe.StatusCode == 403 {
		return errorClassAuth
	}

	if pe.Retriable {
		return errorClassRetriable
	}

	return errorClassFatal
}
