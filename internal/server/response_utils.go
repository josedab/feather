package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/logging"
)

// writeJSONResponse writes a JSON response with proper error handling.
// If encoding fails, the error is logged.
func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.FromContext(context.Background(), nil).Error("failed to encode JSON response", "error", err)
	}
}

// writeJSONError writes a JSON error response with proper error handling.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSONResponse(w, status, map[string]string{"error": message})
}
