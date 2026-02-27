package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/tools/playground"
)

// PlaygroundHandler provides HTTP endpoints for the feature playground.
type PlaygroundHandler struct {
	service     *playground.Service
	requireAuth func(http.Handler) http.Handler
}

// NewPlaygroundHandler creates a new playground handler.
func NewPlaygroundHandler(service *playground.Service) *PlaygroundHandler {
	return &PlaygroundHandler{service: service}
}

// RegisterRoutes registers playground API routes.
func (h *PlaygroundHandler) RegisterRoutes(mux *http.ServeMux) {
	wrap := h.requireAuth
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle("POST /v1/playground/summary", wrap(http.HandlerFunc(h.handleComputeSummary)))
	mux.Handle("GET /v1/playground/queries", wrap(http.HandlerFunc(h.handleListQueries)))
	mux.Handle("POST /v1/playground/queries", wrap(http.HandlerFunc(h.handleSaveQuery)))
	mux.Handle("GET /v1/playground/queries/{id}", wrap(http.HandlerFunc(h.handleGetQuery)))
	mux.Handle("DELETE /v1/playground/queries/{id}", wrap(http.HandlerFunc(h.handleDeleteQuery)))
	mux.Handle("GET /v1/playground/datasets", wrap(http.HandlerFunc(h.handleListDatasets)))
	mux.Handle("POST /v1/playground/datasets", wrap(http.HandlerFunc(h.handleCreateDataset)))
	mux.Handle("GET /v1/playground/datasets/{id}", wrap(http.HandlerFunc(h.handleGetDatasetStatus)))
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
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &q); err != nil {
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
	if err := strictDecode(r.Body, &cfg); err != nil {
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
