package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/parity"
)

// ParityHandler handles online/offline parity API requests.
type ParityHandler struct {
	checker *parity.Checker
}

// NewParityHandler creates a new parity handler.
func NewParityHandler(checker *parity.Checker) *ParityHandler {
	return &ParityHandler{checker: checker}
}

// RegisterRoutes registers parity API routes.
func (h *ParityHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/parity/status", h.handleGetAllStatuses)
	mux.HandleFunc("GET /v1/parity/status/{feature}", h.handleGetStatus)
	mux.HandleFunc("GET /v1/parity/summary", h.handleGetSummary)
	mux.HandleFunc("GET /v1/parity/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/parity/record", h.handleRecordPair)
	mux.HandleFunc("POST /v1/parity/record/batch", h.handleRecordBatch)
	mux.HandleFunc("POST /v1/parity/reset/{feature}", h.handleReset)
}

func (h *ParityHandler) handleGetAllStatuses(w http.ResponseWriter, r *http.Request) {
	statuses := h.checker.GetAllStatuses()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"statuses": statuses,
		"count":    len(statuses),
	})
}

func (h *ParityHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	status := h.checker.GetStatus(feature)
	if status == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "feature not tracked")
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, status)
}

func (h *ParityHandler) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	summary := h.checker.GetSummary()
	h.writeJSON(r.Context(), w, http.StatusOK, summary)
}

func (h *ParityHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	var since time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}
	alerts := h.checker.GetAlerts(since)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

type recordPairRequest struct {
	Feature      string      `json:"feature"`
	EntityKey    string      `json:"entity_key"`
	OnlineValue  interface{} `json:"online_value"`
	OfflineValue interface{} `json:"offline_value"`
}

func (h *ParityHandler) handleRecordPair(w http.ResponseWriter, r *http.Request) {
	var req recordPairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	h.checker.RecordPair(req.Feature, req.EntityKey, req.OnlineValue, req.OfflineValue)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true})
}

type recordBatchRequest struct {
	Pairs []recordPairRequest `json:"pairs"`
}

func (h *ParityHandler) handleRecordBatch(w http.ResponseWriter, r *http.Request) {
	var req recordBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	for _, p := range req.Pairs {
		h.checker.RecordPair(p.Feature, p.EntityKey, p.OnlineValue, p.OfflineValue)
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"recorded": len(req.Pairs),
	})
}

func (h *ParityHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}
	if err := h.checker.Reset(feature); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *ParityHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ParityHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
