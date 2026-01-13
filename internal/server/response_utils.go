package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/logging"
)

// writeJSONResponse writes a JSON response with proper error handling.
// If encoding fails, the error is logged.
func writeJSONResponse(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.FromContext(ctx, nil).Error("failed to encode JSON response", "error", err)
	}
}

// writeJSONError writes a JSON error response with proper error handling.
func writeJSONError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONResponse(ctx, w, status, map[string]string{"error": message})
}
