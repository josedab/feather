package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/incrmat"
)

// CDCHandler handles CDC (Change Data Capture) API requests.
type CDCHandler struct {
	manager *incrmat.CDCManager
	engine  *incrmat.Engine
}

// NewCDCHandler creates a new CDC handler.
func NewCDCHandler(engine *incrmat.Engine) *CDCHandler {
	return &CDCHandler{
		manager: incrmat.NewCDCManager(engine, 100000),
		engine:  engine,
	}
}

// RegisterRoutes registers CDC API routes.
func (h *CDCHandler) RegisterRoutes(mux *http.ServeMux) {
	// CDC Sources
	mux.HandleFunc("GET /v1/cdc/sources", h.handleListSources)
	mux.HandleFunc("POST /v1/cdc/sources", h.handleRegisterSource)
	mux.HandleFunc("GET /v1/cdc/sources/{id}/status", h.handleSourceStatus)
	mux.HandleFunc("DELETE /v1/cdc/sources/{id}", h.handleRemoveSource)

	// CDC Events
	mux.HandleFunc("POST /v1/cdc/events", h.handleProcessEvent)
	mux.HandleFunc("POST /v1/cdc/events/batch", h.handleProcessBatch)
	mux.HandleFunc("POST /v1/cdc/events/debezium", h.handleDebeziumEvent)
	mux.HandleFunc("GET /v1/cdc/events/recent", h.handleRecentEvents)

	// Materialization trigger
	mux.HandleFunc("POST /v1/cdc/materialize", h.handleMaterialize)

	// Stats
	mux.HandleFunc("GET /v1/cdc/stats", h.handleStats)
	mux.HandleFunc("GET /v1/cdc/positions", h.handlePositions)
}

func (h *CDCHandler) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources := h.manager.ListSources()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	})
}

func (h *CDCHandler) handleRegisterSource(w http.ResponseWriter, r *http.Request) {
	var src incrmat.CDCSourceConfig
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.RegisterSource(src); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "source registered"})
}

func (h *CDCHandler) handleSourceStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, err := h.manager.GetSourceStatus(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, status)
}

func (h *CDCHandler) handleRemoveSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.RemoveSource(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "source removed"})
}

func (h *CDCHandler) handleProcessEvent(w http.ResponseWriter, r *http.Request) {
	var event incrmat.CDCEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.ProcessCDCEvent(event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "event processed"})
}

func (h *CDCHandler) handleProcessBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Events []incrmat.CDCEvent `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	processed, errCount, _ := h.manager.ProcessBatch(req.Events)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"processed": processed,
		"errors":    errCount,
		"total":     len(req.Events),
	})
}

func (h *CDCHandler) handleRecentEvents(w http.ResponseWriter, r *http.Request) {
	events := h.manager.GetRecentEvents(100)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

func (h *CDCHandler) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	results := h.engine.Materialize()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results":      results,
		"materialized": len(results),
	})
}

func (h *CDCHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	cdcStats := h.manager.Stats()
	engineStats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"cdc":    cdcStats,
		"engine": engineStats,
	})
}

func (h *CDCHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CDCHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

func (h *CDCHandler) handleDebeziumEvent(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source_id")
	if sourceID == "" {
		sourceID = r.Header.Get("X-CDC-Source-ID")
	}
	if sourceID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "source_id query param or X-CDC-Source-ID header required")
		return
	}

	body := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	event, err := h.manager.ProcessDebeziumEvent(body, sourceID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"event":   event,
	})
}

func (h *CDCHandler) handlePositions(w http.ResponseWriter, r *http.Request) {
	positions := h.manager.GetLSNPositions()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"positions": positions,
	})
}
