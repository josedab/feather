package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/backpressure"
)

// BackpressureHandler handles backpressure monitoring API requests.
type BackpressureHandler struct {
	monitor *backpressure.Monitor
}

// NewBackpressureHandler creates a new backpressure handler.
func NewBackpressureHandler(monitor *backpressure.Monitor) *BackpressureHandler {
	return &BackpressureHandler{monitor: monitor}
}

// RegisterRoutes registers backpressure API routes.
func (h *BackpressureHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/backpressure/queue", h.handleRecordQueueDepth)
	mux.HandleFunc("POST /v1/backpressure/latency", h.handleRecordLatency)
	mux.HandleFunc("POST /v1/backpressure/errors", h.handleRecordErrorRate)
	mux.HandleFunc("POST /v1/backpressure/evaluate", h.handleEvaluate)
	mux.HandleFunc("GET /v1/backpressure/level", h.handleGetLevel)
	mux.HandleFunc("GET /v1/backpressure/reports", h.handleGetReports)
	mux.HandleFunc("GET /v1/backpressure/stats", h.handleGetStats)
}

// handleRecordQueueDepth handles POST /v1/backpressure/queue
func (h *BackpressureHandler) handleRecordQueueDepth(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	var req struct {
		Depth float64 `json:"depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.monitor.RecordQueueDepth(req.Depth)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"depth":   req.Depth,
	})
}

// handleRecordLatency handles POST /v1/backpressure/latency
func (h *BackpressureHandler) handleRecordLatency(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	var req struct {
		LatencyMs float64 `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.monitor.RecordLatency(req.LatencyMs)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"latency_ms": req.LatencyMs,
	})
}

// handleRecordErrorRate handles POST /v1/backpressure/errors
func (h *BackpressureHandler) handleRecordErrorRate(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	var req struct {
		Rate float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.monitor.RecordErrorRate(req.Rate)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"rate":    req.Rate,
	})
}

// handleEvaluate handles POST /v1/backpressure/evaluate
func (h *BackpressureHandler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	report := h.monitor.Evaluate()

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// handleGetLevel handles GET /v1/backpressure/level
func (h *BackpressureHandler) handleGetLevel(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	level := h.monitor.GetCurrentLevel()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"level": level,
	})
}

// handleGetReports handles GET /v1/backpressure/reports
func (h *BackpressureHandler) handleGetReports(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	reports := h.monitor.GetReports(limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"reports": reports,
	})
}

// handleGetStats handles GET /v1/backpressure/stats
func (h *BackpressureHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.monitor == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "backpressure monitor not configured")
		return
	}

	stats := h.monitor.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *BackpressureHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *BackpressureHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
