package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
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

// writeJSONError writes a JSON error response with error code and request ID.
// For 5xx status codes, the original message is logged server-side and a
// generic message is returned to the client to avoid leaking internals.
func writeJSONError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	if status >= 500 {
		logging.FromContext(ctx, nil).Error("internal error", "status", status, "detail", message)
		message = "internal server error"
	}

	resp := ErrorResponse{
		Error: message,
		Code:  httpStatusToErrorCode(status),
	}

	// Include request ID if set by middleware
	if requestID := w.Header().Get("X-Request-ID"); requestID != "" {
		resp.RequestID = requestID
	}

	writeJSONResponse(ctx, w, status, resp)
}

// httpStatusToErrorCode maps HTTP status codes to machine-readable error codes.
func httpStatusToErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusGatewayTimeout:
		return "timeout"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
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

// parsePagination extracts limit and offset from query parameters with defaults and bounds.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// setPaginationHeaders adds standard pagination headers to the response.
func setPaginationHeaders(w http.ResponseWriter, total, limit, offset int, r *http.Request) {
	w.Header().Set("X-Total-Count", strconv.Itoa(total))

	// Build Link header with rel=next and rel=prev
	basePath := r.URL.Path
	var links []string

	if offset+limit < total {
		nextOffset := offset + limit
		links = append(links, fmt.Sprintf("<%s?limit=%d&offset=%d>; rel=\"next\"", basePath, limit, nextOffset))
	}
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, fmt.Sprintf("<%s?limit=%d&offset=%d>; rel=\"prev\"", basePath, limit, prevOffset))
	}

	if len(links) > 0 {
		header := links[0]
		for _, l := range links[1:] {
			header += ", " + l
		}
		w.Header().Set("Link", header)
	}
}
