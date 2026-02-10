package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/modelserving"
)

// ModelGatewayHandler provides the unified predict endpoint.
type ModelGatewayHandler struct {
	gateway *modelserving.Gateway
}

// NewModelGatewayHandler creates a new model gateway handler.
func NewModelGatewayHandler(gateway *modelserving.Gateway) *ModelGatewayHandler {
	return &ModelGatewayHandler{gateway: gateway}
}

// RegisterRoutes registers model gateway API routes.
func (h *ModelGatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/predict", h.handlePredict)
	mux.HandleFunc("GET /v1/predict/adapters", h.handleListAdapters)
	mux.HandleFunc("POST /v1/predict/ab-config", h.handleSetABConfig)
	mux.HandleFunc("GET /v1/predict/ab-config/{model}", h.handleGetABConfig)
	mux.HandleFunc("GET /v1/predict/stats", h.handleStats)
}

func (h *ModelGatewayHandler) handlePredict(w http.ResponseWriter, r *http.Request) {
	var req modelserving.PredictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ModelID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	resp, err := h.gateway.Predict(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *ModelGatewayHandler) handleListAdapters(w http.ResponseWriter, r *http.Request) {
	adapters := h.gateway.ListAdapters()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"adapters": adapters,
		"total":    len(adapters),
	})
}

func (h *ModelGatewayHandler) handleSetABConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string                `json:"model_id"`
		Config  modelserving.ABConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ModelID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	h.gateway.SetABConfig(req.ModelID, req.Config)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"model_id": req.ModelID,
	})
}

func (h *ModelGatewayHandler) handleGetABConfig(w http.ResponseWriter, r *http.Request) {
	model := r.PathValue("model")
	cfg := h.gateway.GetABConfig(model)
	if cfg == nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, "no AB config for model")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, cfg)
}

func (h *ModelGatewayHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.gateway.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
