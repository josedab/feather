package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/feastcompat"
)

// FeastCompatHandler handles Feast-compatible API requests.
type FeastCompatHandler struct {
	adapter *feastcompat.Adapter
}

// NewFeastCompatHandler creates a new Feast compatibility handler.
func NewFeastCompatHandler(adapter *feastcompat.Adapter) *FeastCompatHandler {
	return &FeastCompatHandler{adapter: adapter}
}

// RegisterRoutes registers Feast-compatible API routes.
func (h *FeastCompatHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feast-compatible endpoints
	mux.HandleFunc("POST /v1/feast/get-online-features", h.handleGetOnlineFeatures)
	mux.HandleFunc("POST /v1/feast/materialize", h.handleMaterialize)

	// Mapping management
	mux.HandleFunc("GET /v1/feast/mappings", h.handleListMappings)
	mux.HandleFunc("POST /v1/feast/mappings", h.handleRegisterMapping)
	mux.HandleFunc("GET /v1/feast/mappings/{view}", h.handleGetMapping)
	mux.HandleFunc("DELETE /v1/feast/mappings/{view}", h.handleDeleteMapping)
	mux.HandleFunc("GET /v1/feast/stats", h.handleStats)
}

func (h *FeastCompatHandler) handleGetOnlineFeatures(w http.ResponseWriter, r *http.Request) {
	var req feastcompat.OnlineFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.adapter.GetOnlineFeatures(req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *FeastCompatHandler) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	var req feastcompat.MaterializeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.adapter.Materialize(req)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *FeastCompatHandler) handleListMappings(w http.ResponseWriter, r *http.Request) {
	mappings := h.adapter.ListMappings()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"mappings": mappings,
		"count":    len(mappings),
	})
}

func (h *FeastCompatHandler) handleRegisterMapping(w http.ResponseWriter, r *http.Request) {
	var mapping feastcompat.FeatureViewMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.adapter.RegisterMapping(mapping); err != nil {
		if errors.Is(err, feastcompat.ErrMappingExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "mapping already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true})
}

func (h *FeastCompatHandler) handleGetMapping(w http.ResponseWriter, r *http.Request) {
	view := r.PathValue("view")
	mapping, err := h.adapter.GetMapping(view)
	if err != nil {
		if errors.Is(err, feastcompat.ErrFeatureViewNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "mapping not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, mapping)
}

func (h *FeastCompatHandler) handleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	view := r.PathValue("view")
	if err := h.adapter.DeleteMapping(view); err != nil {
		if errors.Is(err, feastcompat.ErrFeatureViewNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "mapping not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *FeastCompatHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.adapter.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *FeastCompatHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FeastCompatHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
