package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Best effort — headers already sent.
		_ = err
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON decodes the request body into v. Returns false and writes a 400
// error response if decoding fails.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, logger *slog.Logger) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		logger.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
