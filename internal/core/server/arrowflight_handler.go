package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
)

// ArrowFlightHandler handles Arrow Flight API requests.
type ArrowFlightHandler struct {
	server *arrowflight.Server
}

// NewArrowFlightHandler creates a new Arrow Flight handler.
func NewArrowFlightHandler(server *arrowflight.Server) *ArrowFlightHandler {
	return &ArrowFlightHandler{server: server}
}

// RegisterRoutes registers Arrow Flight API routes.
func (h *ArrowFlightHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/flight/info", h.handleGetFlightInfo)
	mux.HandleFunc("POST /v1/flight/get", h.handleDoGet)
	mux.HandleFunc("POST /v1/flight/put", h.handleDoPut)
	mux.HandleFunc("POST /v1/flight/exchange", h.handleDoExchange)
	mux.HandleFunc("GET /v1/flight/list", h.handleListFlights)
	mux.HandleFunc("GET /v1/flight/stats", h.handleStats)
}

// handleGetFlightInfo handles POST /v1/flight/info
func (h *ArrowFlightHandler) handleGetFlightInfo(w http.ResponseWriter, r *http.Request) {
	var desc arrowflight.FlightDescriptor
	if err := json.NewDecoder(r.Body).Decode(&desc); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	info, err := h.server.GetFlightInfo(r.Context(), desc)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, info)
}

// handleDoGet handles POST /v1/flight/get
func (h *ArrowFlightHandler) handleDoGet(w http.ResponseWriter, r *http.Request) {
	var ticket arrowflight.FlightTicket
	if err := json.NewDecoder(r.Body).Decode(&ticket); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	batch, err := h.server.DoGet(r.Context(), ticket)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, batch)
}

// handleDoPut handles POST /v1/flight/put
func (h *ArrowFlightHandler) handleDoPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Descriptor arrowflight.FlightDescriptor `json:"descriptor"`
		Batch      arrowflight.RecordBatch      `json:"batch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.server.DoPut(r.Context(), req.Descriptor, &req.Batch)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleDoExchange handles POST /v1/flight/exchange
func (h *ArrowFlightHandler) handleDoExchange(w http.ResponseWriter, r *http.Request) {
	var req arrowflight.ExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.server.DoExchange(r.Context(), req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

// handleListFlights handles GET /v1/flight/list
func (h *ArrowFlightHandler) handleListFlights(w http.ResponseWriter, r *http.Request) {
	flights := h.server.ListFlights()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"flights": flights,
		"count":   len(flights),
	})
}

// handleStats handles GET /v1/flight/stats
func (h *ArrowFlightHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(r.Context(), w, http.StatusOK, h.server.Stats())
}

func (h *ArrowFlightHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ArrowFlightHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
