package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/llm"
)

// LLMHandler handles LLM feature pipeline requests.
type LLMHandler struct {
	pipeline *llm.Pipeline
}

// NewLLMHandler creates a new LLM handler.
func NewLLMHandler(pipeline *llm.Pipeline) *LLMHandler {
	return &LLMHandler{pipeline: pipeline}
}

// RegisterRoutes registers LLM routes.
func (h *LLMHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/llm/embed", h.handleEmbed)
	mux.HandleFunc("POST /v1/llm/embed/chunks", h.handleEmbedChunks)
	mux.HandleFunc("POST /v1/llm/features", h.handleCreateFeature)
	mux.HandleFunc("POST /v1/llm/features/batch", h.handleCreateFeaturesBatch)
	mux.HandleFunc("GET /v1/llm/stats", h.handleStats)
	mux.HandleFunc("POST /v1/llm/cache/clear", h.handleClearCache)
}

// EmbedRequest is the request for embedding text.
type EmbedRequest struct {
	Text string `json:"text"`
}

// EmbedResponse is the response for embedding text.
type EmbedResponse struct {
	Embedding   []float32 `json:"embedding"`
	Dimension   int       `json:"dimension"`
	ModelID     string    `json:"model_id"`
	ContentHash string    `json:"content_hash"`
}

func (h *LLMHandler) handleEmbed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req EmbedRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.Text == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Text is required")
		return
	}

	embedding, err := h.pipeline.Embed(ctx, req.Text)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := EmbedResponse{
		Embedding:   embedding,
		Dimension:   len(embedding),
		ModelID:     h.pipeline.Stats().ProviderModelID,
		ContentHash: llm.ContentHash(req.Text),
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

// EmbedChunksRequest is the request for embedding text with chunk info.
type EmbedChunksRequest struct {
	Text string `json:"text"`
}

// EmbedChunksResponse is the response with chunk embeddings.
type EmbedChunksResponse struct {
	Chunks  []ChunkEmbeddingResponse `json:"chunks"`
	ModelID string                   `json:"model_id"`
}

// ChunkEmbeddingResponse represents a chunk with its embedding.
type ChunkEmbeddingResponse struct {
	Text      string    `json:"text"`
	Index     int       `json:"index"`
	StartChar int       `json:"start_char"`
	EndChar   int       `json:"end_char"`
	Embedding []float32 `json:"embedding"`
}

func (h *LLMHandler) handleEmbedChunks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req EmbedChunksRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.Text == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Text is required")
		return
	}

	chunks, err := h.pipeline.EmbedChunks(ctx, req.Text)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	respChunks := make([]ChunkEmbeddingResponse, len(chunks))
	for i, ce := range chunks {
		respChunks[i] = ChunkEmbeddingResponse{
			Text:      ce.Chunk.Text,
			Index:     ce.Chunk.Index,
			StartChar: ce.Chunk.StartChar,
			EndChar:   ce.Chunk.EndChar,
			Embedding: ce.Embedding,
		}
	}

	resp := EmbedChunksResponse{
		Chunks:  respChunks,
		ModelID: h.pipeline.Stats().ProviderModelID,
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

// CreateFeatureRequest is the request for creating an embedding feature.
type CreateFeatureRequest struct {
	EntityKey   string `json:"entity_key"`
	FeatureName string `json:"feature_name"`
	Text        string `json:"text"`
}

// CreateFeatureResponse is the response for creating an embedding feature.
type CreateFeatureResponse struct {
	EntityKey   string    `json:"entity_key"`
	FeatureName string    `json:"feature_name"`
	Dimension   int       `json:"dimension"`
	ModelID     string    `json:"model_id"`
	ContentHash string    `json:"content_hash"`
	Timestamp   time.Time `json:"timestamp"`
}

func (h *LLMHandler) handleCreateFeature(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateFeatureRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.EntityKey == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity_key is required")
		return
	}
	if req.FeatureName == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "feature_name is required")
		return
	}
	if req.Text == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "text is required")
		return
	}

	result, err := h.pipeline.Process(ctx, req.EntityKey, req.FeatureName, req.Text)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := CreateFeatureResponse{
		EntityKey:   result.EntityKey,
		FeatureName: result.FeatureName,
		Dimension:   result.Dimension,
		ModelID:     result.ModelID,
		ContentHash: result.ContentHash,
		Timestamp:   result.Timestamp,
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

// CreateFeaturesBatchRequest is the request for batch feature creation.
type CreateFeaturesBatchRequest struct {
	EntityKey string            `json:"entity_key"`
	Features  map[string]string `json:"features"` // feature_name -> text
}

// CreateFeaturesBatchResponse is the response for batch feature creation.
type CreateFeaturesBatchResponse struct {
	EntityKey string                  `json:"entity_key"`
	Features  []CreateFeatureResponse `json:"features"`
}

func (h *LLMHandler) handleCreateFeaturesBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateFeaturesBatchRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.EntityKey == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity_key is required")
		return
	}
	if len(req.Features) == 0 {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "features is required")
		return
	}

	results, err := h.pipeline.ProcessBatch(ctx, req.EntityKey, req.Features)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	features := make([]CreateFeatureResponse, len(results))
	for i, result := range results {
		features[i] = CreateFeatureResponse{
			EntityKey:   result.EntityKey,
			FeatureName: result.FeatureName,
			Dimension:   result.Dimension,
			ModelID:     result.ModelID,
			ContentHash: result.ContentHash,
			Timestamp:   result.Timestamp,
		}
	}

	resp := CreateFeaturesBatchResponse{
		EntityKey: req.EntityKey,
		Features:  features,
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *LLMHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.pipeline.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *LLMHandler) handleClearCache(w http.ResponseWriter, r *http.Request) {
	h.pipeline.ClearCache()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{
		"status": "cache_cleared",
	})
}
