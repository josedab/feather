package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/core/logging"
)

// SuccessResponse is the standard response for successful operations.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse is the standard response for errors.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// ItemResponse wraps a single item in a standard response.
type ItemResponse[T any] struct {
	Data T `json:"data"`
}

// ListResponse wraps a list of items with a count.
type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

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
	writeJSONResponse(ctx, w, status, ErrorResponse{Error: message})
}
