package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
)

// ---------------------------------------------------------------------------
// FlightEndpointHandler
// ---------------------------------------------------------------------------

// FlightEndpointHandler exposes production-grade Arrow Flight batch serving endpoints.
type FlightEndpointHandler struct {
	endpoint *arrowflight.FlightServiceEndpoint
}

// NewFlightEndpointHandler creates a new FlightEndpointHandler.
func NewFlightEndpointHandler(endpoint *arrowflight.FlightServiceEndpoint) *FlightEndpointHandler {
	return &FlightEndpointHandler{endpoint: endpoint}
}

// RegisterRoutes registers Flight endpoint API routes.
func (h *FlightEndpointHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/flight/get-batch", h.handleGetBatch)
	mux.HandleFunc("POST /v1/flight/get-records", h.handleGetRecords)
	mux.HandleFunc("POST /v1/flight/put-batch", h.handlePutBatch)
	mux.HandleFunc("POST /v1/flight/schema", h.handleGetSchema)
	mux.HandleFunc("GET /v1/flight/endpoint/stats", h.handleStats)
}

func (h *FlightEndpointHandler) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entities []string `json:"entities"`
		Features []string `json:"features"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	batch, err := h.endpoint.DoGetBatch(r.Context(), req.Entities, req.Features)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, batch)
}

func (h *FlightEndpointHandler) handleGetRecords(w http.ResponseWriter, r *http.Request) {
	var req arrowflight.BatchRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	resp, err := h.endpoint.DoGetRecordBatch(r.Context(), req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *FlightEndpointHandler) handlePutBatch(w http.ResponseWriter, r *http.Request) {
	var batch arrowflight.ColumnarBatch
	if err := strictDecode(r.Body, &batch); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.endpoint.DoPutBatch(r.Context(), &batch)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FlightEndpointHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	var desc arrowflight.FlightDescriptor
	if err := strictDecode(r.Body, &desc); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	schema, err := h.endpoint.GetSchema(r.Context(), desc)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"schema": schema,
	})
}

func (h *FlightEndpointHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.endpoint.Stats())
}
