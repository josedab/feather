package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/feather-store/feather/internal/core/logging"
	"github.com/feather-store/feather/internal/platform/auth"
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
// For 5xx status codes, the original message is logged server-side and a
// generic message is returned to the client to avoid leaking internals.
func writeJSONError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	if status >= 500 {
		logging.FromContext(ctx, nil).Error("internal error", "status", status, "detail", message)
		message = "internal server error"
	}
	writeJSONResponse(ctx, w, status, ErrorResponse{Error: message})
}

// strictDecode decodes JSON from an io.Reader into v, rejecting unknown fields.
func strictDecode(r io.Reader, v interface{}) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// userFromRequest extracts the authenticated user identity from the request context.
// Falls back to "anonymous" if no authenticated identity is available.
func userFromRequest(r *http.Request) string {
	if key := auth.APIKeyFromContext(r.Context()); key != nil {
		return key.Name
	}
	return "anonymous"
}
