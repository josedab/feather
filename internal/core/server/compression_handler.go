package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/compression"
)

// CompressionHandler handles compression API requests.
type CompressionHandler struct {
	selector *compression.Selector
}

// NewCompressionHandler creates a new compression handler.
func NewCompressionHandler(selector *compression.Selector) *CompressionHandler {
	return &CompressionHandler{selector: selector}
}

// RegisterRoutes registers compression API routes.
func (h *CompressionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/compression/analyze", h.handleAnalyze)
	mux.HandleFunc("POST /v1/compression/select", h.handleSelectStrategy)
	mux.HandleFunc("POST /v1/compression/compress", h.handleCompress)
	mux.HandleFunc("POST /v1/compression/decompress", h.handleDecompress)
	mux.HandleFunc("GET /v1/compression/reencode/{feature}", h.handleShouldReEncode)
	mux.HandleFunc("GET /v1/compression/stats", h.handleGetStats)
}

// handleAnalyze handles POST /v1/compression/analyze
func (h *CompressionHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Data     []byte `json:"data"`
		DataType string `json:"data_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	stats, err := compression.AnalyzeData(req.Data, req.DataType)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleSelectStrategy handles POST /v1/compression/select
func (h *CompressionHandler) handleSelectStrategy(w http.ResponseWriter, r *http.Request) {
	var stats compression.DataStats
	if err := json.NewDecoder(r.Body).Decode(&stats); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	strategy := compression.SelectStrategy(&stats)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"strategy": strategy,
	})
}

// handleCompress handles POST /v1/compression/compress
func (h *CompressionHandler) handleCompress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Data     []byte              `json:"data"`
		Strategy compression.Strategy `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	block, err := h.selector.Compress(req.Data, req.Strategy)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, block)
}

// handleDecompress handles POST /v1/compression/decompress
func (h *CompressionHandler) handleDecompress(w http.ResponseWriter, r *http.Request) {
	var block compression.CompressedBlock
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	data, err := h.selector.Decompress(&block)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"data": data,
	})
}

// handleShouldReEncode handles GET /v1/compression/reencode/{feature}
func (h *CompressionHandler) handleShouldReEncode(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature name required")
		return
	}

	shouldReEncode, newStrategy := h.selector.ShouldReEncode(feature)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature":        feature,
		"should_reencode": shouldReEncode,
		"new_strategy":   newStrategy,
	})
}

// handleGetStats handles GET /v1/compression/stats
func (h *CompressionHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.selector.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *CompressionHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CompressionHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
