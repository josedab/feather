package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/tools/playground"
)

// PlaygroundHandler provides HTTP endpoints for the feature playground.
type PlaygroundHandler struct {
	service *playground.Service
}

// NewPlaygroundHandler creates a new playground handler.
func NewPlaygroundHandler(service *playground.Service) *PlaygroundHandler {
	return &PlaygroundHandler{service: service}
}

// RegisterRoutes registers playground API routes.
func (h *PlaygroundHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/playground/summary", h.handleComputeSummary)
	mux.HandleFunc("GET /v1/playground/queries", h.handleListQueries)
	mux.HandleFunc("POST /v1/playground/queries", h.handleSaveQuery)
	mux.HandleFunc("GET /v1/playground/queries/{id}", h.handleGetQuery)
	mux.HandleFunc("DELETE /v1/playground/queries/{id}", h.handleDeleteQuery)
	mux.HandleFunc("GET /v1/playground/datasets", h.handleListDatasets)
	mux.HandleFunc("POST /v1/playground/datasets", h.handleCreateDataset)
	mux.HandleFunc("GET /v1/playground/datasets/{id}", h.handleGetDatasetStatus)
}

func (h *PlaygroundHandler) handleComputeSummary(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}

	var req struct {
		Name     string    `json:"name"`
		Group    string    `json:"group"`
		DataType string    `json:"data_type"`
		Values   []float64 `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	summary := h.service.ComputeSummary(req.Name, req.Group, req.DataType, req.Values)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"summary": summary,
	})
}

func (h *PlaygroundHandler) handleListQueries(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	queries := h.service.ListQueries()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"queries": queries,
		"count":   len(queries),
	})
}

func (h *PlaygroundHandler) handleSaveQuery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	var q playground.SavedQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SaveQuery(&q); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"query":   q,
	})
}

func (h *PlaygroundHandler) handleGetQuery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	q, err := h.service.GetQuery(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "query": q})
}

func (h *PlaygroundHandler) handleDeleteQuery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	if err := h.service.DeleteQuery(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "query deleted"})
}

func (h *PlaygroundHandler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	datasets := h.service.ListDatasets()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"datasets": datasets,
		"count":    len(datasets),
	})
}

func (h *PlaygroundHandler) handleCreateDataset(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	var cfg playground.DatasetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	ds, err := h.service.CreateDataset(&cfg)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "dataset": ds})
}

func (h *PlaygroundHandler) handleGetDatasetStatus(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "playground not configured")
		return
	}
	ds, err := h.service.GetDatasetStatus(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "dataset": ds})
}
