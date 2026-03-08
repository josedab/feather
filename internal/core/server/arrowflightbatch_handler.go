package server

import (
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
)

// ArrowFlightBatchHandler handles zero-copy Arrow Flight batch serving API requests.
type ArrowFlightBatchHandler struct {
	server *arrowflight.Server
	batch  *arrowflight.BatchServer
}

// NewArrowFlightBatchHandler creates a new Arrow Flight batch handler.
func NewArrowFlightBatchHandler(server *arrowflight.Server, batch *arrowflight.BatchServer) *ArrowFlightBatchHandler {
	return &ArrowFlightBatchHandler{server: server, batch: batch}
}

// RegisterRoutes registers Arrow Flight batch API routes.
func (h *ArrowFlightBatchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/flight/batch/register", h.handleRegisterDataset)
	mux.HandleFunc("POST /v1/flight/batch/get", h.handleBatchGet)
	mux.HandleFunc("POST /v1/flight/batch/put", h.handleBatchPut)
	mux.HandleFunc("GET /v1/flight/batch/datasets", h.handleListDatasets)
	mux.HandleFunc("GET /v1/flight/batch/stats", h.handleStats)
	mux.HandleFunc("POST /v1/flight/batch/scan", h.handleScan)
}

func (h *ArrowFlightBatchHandler) handleRegisterDataset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Entities []string `json:"entities"`
		Features []string `json:"features"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	desc := arrowflight.FlightDescriptor{
		Type: "path",
		Path: []string{"batch", req.Name},
	}
	info, err := h.server.GetFlightInfo(r.Context(), desc)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"dataset":  req.Name,
		"info":     info,
		"entities": len(req.Entities),
		"features": len(req.Features),
	})
}

func (h *ArrowFlightBatchHandler) handleBatchGet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entities []string `json:"entities"`
		Features []string `json:"features"`
		Format   string   `json:"format"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ticket := arrowflight.FlightTicket{
		ID: "batch-get",
	}
	batch, err := h.server.DoGet(r.Context(), ticket)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"batch":    batch,
		"entities": len(req.Entities),
		"features": len(req.Features),
		"format":   req.Format,
	})
}

func (h *ArrowFlightBatchHandler) handleBatchPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Descriptor arrowflight.FlightDescriptor `json:"descriptor"`
		Batch      arrowflight.RecordBatch      `json:"batch"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.server.DoPut(r.Context(), req.Descriptor, &req.Batch)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ArrowFlightBatchHandler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	flights := h.server.ListFlights()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"datasets": flights,
		"total":    len(flights),
	})
}

func (h *ArrowFlightBatchHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.server.Stats())
}

func (h *ArrowFlightBatchHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset  string   `json:"dataset"`
		Features []string `json:"features"`
		Limit    int      `json:"limit"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Limit <= 0 {
		limitStr := r.URL.Query().Get("limit")
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			req.Limit = v
		} else {
			req.Limit = 1000
		}
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"dataset":  req.Dataset,
		"features": req.Features,
		"limit":    req.Limit,
		"format":   "arrow_ipc",
		"message":  "scan initiated",
	})
}
