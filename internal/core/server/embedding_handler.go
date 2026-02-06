package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/embedding"
)

// EmbeddingHandler handles embedding cache API requests.
type EmbeddingHandler struct {
	store    *embedding.Store
	dedup    *embedding.Deduplicator
	version  *embedding.VersionManager
	batch    *embedding.BatchProcessor
	provider embedding.EmbeddingProvider
	logger   *slog.Logger
}

// EmbeddingHandlerConfig configures the embedding handler.
type EmbeddingHandlerConfig struct {
	Store    *embedding.Store
	Dedup    *embedding.Deduplicator
	Version  *embedding.VersionManager
	Batch    *embedding.BatchProcessor
	Provider embedding.EmbeddingProvider
	Logger   *slog.Logger
}

// NewEmbeddingHandler creates a new embedding handler.
func NewEmbeddingHandler(cfg EmbeddingHandlerConfig) *EmbeddingHandler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &EmbeddingHandler{
		store:    cfg.Store,
		dedup:    cfg.Dedup,
		version:  cfg.Version,
		batch:    cfg.Batch,
		provider: cfg.Provider,
		logger:   logger,
	}
}

// RegisterRoutes registers embedding cache API routes.
func (h *EmbeddingHandler) RegisterRoutes(mux *http.ServeMux) {
	// Embedding storage routes
	mux.HandleFunc("GET /v1/embeddings", h.handleListEmbeddings)
	mux.HandleFunc("POST /v1/embeddings", h.handleStoreEmbedding)
	mux.HandleFunc("GET /v1/embeddings/{id}", h.handleGetEmbedding)
	mux.HandleFunc("DELETE /v1/embeddings/{id}", h.handleDeleteEmbedding)

	// Lookup routes
	mux.HandleFunc("GET /v1/embeddings/hash/{hash}", h.handleGetByHash)
	mux.HandleFunc("GET /v1/embeddings/model/{modelID}", h.handleGetByModel)
	mux.HandleFunc("POST /v1/embeddings/lookup", h.handleLookup)

	// Generation routes
	mux.HandleFunc("POST /v1/embeddings/generate", h.handleGenerate)
	mux.HandleFunc("POST /v1/embeddings/batch", h.handleBatch)
	mux.HandleFunc("POST /v1/embeddings/batch/async", h.handleBatchAsync)

	// Model management routes
	mux.HandleFunc("GET /v1/embeddings/models", h.handleListModels)
	mux.HandleFunc("POST /v1/embeddings/models", h.handleRegisterModel)
	mux.HandleFunc("GET /v1/embeddings/models/{modelID}", h.handleGetModel)
	mux.HandleFunc("GET /v1/embeddings/models/{modelID}/versions", h.handleListVersions)
	mux.HandleFunc("POST /v1/embeddings/models/{modelID}/versions", h.handleRegisterVersion)
	mux.HandleFunc("POST /v1/embeddings/models/{modelID}/versions/{version}/deprecate", h.handleDeprecateVersion)

	// Compatibility check
	mux.HandleFunc("POST /v1/embeddings/compatibility", h.handleCheckCompatibility)

	// Stats and management
	mux.HandleFunc("GET /v1/embeddings/stats", h.handleGetStats)
	mux.HandleFunc("POST /v1/embeddings/clear", h.handleClear)
}

// Request/Response types

// EmbeddingRequest represents an embedding storage request.
type EmbeddingRequest struct {
	ID           string                 `json:"id,omitempty"`
	Content      string                 `json:"content"`
	Vector       []float32              `json:"vector,omitempty"`
	ModelID      string                 `json:"model_id"`
	ModelVersion string                 `json:"model_version,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// GenerateRequest represents an embedding generation request.
type GenerateRequest struct {
	Content      string `json:"content"`
	ModelID      string `json:"model_id"`
	ModelVersion string `json:"model_version,omitempty"`
	UseCache     bool   `json:"use_cache"`
}

// BatchGenerateRequest represents a batch embedding generation request.
type BatchGenerateRequest struct {
	Contents     []string `json:"contents"`
	ModelID      string   `json:"model_id"`
	ModelVersion string   `json:"model_version,omitempty"`
	UseCache     bool     `json:"use_cache"`
	Priority     int      `json:"priority,omitempty"`
}

// LookupRequest represents an embedding lookup request.
type LookupRequest struct {
	Contents []string `json:"contents"`
	ModelID  string   `json:"model_id"`
}

// ModelRegistrationRequest represents a model registration request.
type ModelRegistrationRequest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Provider    string                 `json:"provider"`
	Dimension   int                    `json:"dimension"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// VersionRegistrationRequest represents a version registration request.
type VersionRegistrationRequest struct {
	Version    string   `json:"version"`
	Dimension  int      `json:"dimension"`
	Compatible []string `json:"compatible,omitempty"`
	IsDefault  bool     `json:"is_default,omitempty"`
}

// CompatibilityRequest represents a compatibility check request.
type CompatibilityRequest struct {
	ModelID     string `json:"model_id"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
}

// handleListEmbeddings handles GET /v1/embeddings
func (h *EmbeddingHandler) handleListEmbeddings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	modelID := r.URL.Query().Get("model_id")
	limit := 100
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	embeddings, err := h.store.List(r.Context(), modelID, limit, offset)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"embeddings": embeddings,
		"count":      len(embeddings),
	})
}

// handleStoreEmbedding handles POST /v1/embeddings
func (h *EmbeddingHandler) handleStoreEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	var req EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Vector) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "vector is required")
		return
	}
	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	contentHash := ""
	if h.dedup != nil && req.Content != "" {
		contentHash = h.dedup.HashContent(req.Content, req.ModelID)
	}

	emb := &embedding.Embedding{
		ID:           req.ID,
		Content:      req.Content,
		ContentHash:  contentHash,
		Vector:       req.Vector,
		Dimension:    len(req.Vector),
		ModelID:      req.ModelID,
		ModelVersion: req.ModelVersion,
		Metadata:     req.Metadata,
	}

	if err := h.store.Put(r.Context(), emb); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":      true,
		"embedding_id": emb.ID,
	})
}

// handleGetEmbedding handles GET /v1/embeddings/{id}
func (h *EmbeddingHandler) handleGetEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	embeddingID := r.PathValue("id")
	if embeddingID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "embedding ID required")
		return
	}

	emb, err := h.store.Get(r.Context(), embeddingID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, emb)
}

// handleDeleteEmbedding handles DELETE /v1/embeddings/{id}
func (h *EmbeddingHandler) handleDeleteEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	embeddingID := r.PathValue("id")
	if embeddingID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "embedding ID required")
		return
	}

	if err := h.store.Delete(r.Context(), embeddingID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetByHash handles GET /v1/embeddings/hash/{hash}
func (h *EmbeddingHandler) handleGetByHash(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	hash := r.PathValue("hash")
	if hash == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "hash required")
		return
	}

	emb, err := h.store.GetByHash(r.Context(), hash)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, emb)
}

// handleGetByModel handles GET /v1/embeddings/model/{modelID}
func (h *EmbeddingHandler) handleGetByModel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	modelID := r.PathValue("modelID")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model ID required")
		return
	}

	embeddings, err := h.store.GetByModel(r.Context(), modelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"embeddings": embeddings,
		"count":      len(embeddings),
		"model_id":   modelID,
	})
}

// handleLookup handles POST /v1/embeddings/lookup
func (h *EmbeddingHandler) handleLookup(w http.ResponseWriter, r *http.Request) {
	if h.dedup == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "deduplicator not configured")
		return
	}

	var req LookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Contents) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contents is required")
		return
	}
	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	results := make([]map[string]interface{}, len(req.Contents))
	hits := 0

	for i, content := range req.Contents {
		emb, found := h.dedup.CheckDuplicate(r.Context(), content, req.ModelID)
		results[i] = map[string]interface{}{
			"content": content,
			"found":   found,
		}
		if found {
			results[i]["embedding"] = emb
			hits++
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results":    results,
		"total":      len(req.Contents),
		"cache_hits": hits,
	})
}

// handleGenerate handles POST /v1/embeddings/generate
func (h *EmbeddingHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding provider not configured")
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "content is required")
		return
	}
	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	// Check cache first
	if req.UseCache && h.dedup != nil {
		if emb, found := h.dedup.CheckDuplicate(r.Context(), req.Content, req.ModelID); found {
			h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
				"embedding":  emb,
				"cache_hit":  true,
				"api_called": false,
			})
			return
		}
	}

	// Generate embedding
	vectors, err := h.provider.GenerateEmbeddings(r.Context(), []string{req.Content}, req.ModelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(vectors) == 0 {
		h.writeError(r.Context(), w, http.StatusInternalServerError, "no embeddings generated")
		return
	}

	contentHash := ""
	if h.dedup != nil {
		contentHash = h.dedup.HashContent(req.Content, req.ModelID)
	}

	emb := &embedding.Embedding{
		Content:      req.Content,
		ContentHash:  contentHash,
		Vector:       vectors[0],
		Dimension:    len(vectors[0]),
		ModelID:      req.ModelID,
		ModelVersion: req.ModelVersion,
	}

	// Store in cache
	if h.store != nil {
		if err := h.store.Put(r.Context(), emb); err != nil {
			slog.Warn("failed to cache embedding", "error", err)
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"embedding":  emb,
		"cache_hit":  false,
		"api_called": true,
	})
}

// handleBatch handles POST /v1/embeddings/batch
func (h *EmbeddingHandler) handleBatch(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "batch processor not configured")
		return
	}

	var req BatchGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Contents) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contents is required")
		return
	}
	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	batchReq := &embedding.BatchRequest{
		Contents:     req.Contents,
		ModelID:      req.ModelID,
		ModelVersion: req.ModelVersion,
		Priority:     req.Priority,
	}

	result, err := h.batch.ProcessSync(r.Context(), batchReq)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"embeddings": result.Embeddings,
		"count":      len(result.Embeddings),
		"cache_hits": result.CacheHits,
		"api_calls":  result.APICalls,
		"duration":   result.Duration.String(),
	})
}

// handleBatchAsync handles POST /v1/embeddings/batch/async
func (h *EmbeddingHandler) handleBatchAsync(w http.ResponseWriter, r *http.Request) {
	if h.batch == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "batch processor not configured")
		return
	}

	var req BatchGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Contents) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contents is required")
		return
	}
	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}

	batchReq := &embedding.BatchRequest{
		Contents:     req.Contents,
		ModelID:      req.ModelID,
		ModelVersion: req.ModelVersion,
		Priority:     req.Priority,
	}

	if err := h.batch.Submit(batchReq); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusAccepted, map[string]interface{}{
		"success":    true,
		"request_id": batchReq.ID,
		"queued":     len(req.Contents),
	})
}

// handleListModels handles GET /v1/embeddings/models
func (h *EmbeddingHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	models := h.version.ListModels()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// handleRegisterModel handles POST /v1/embeddings/models
func (h *EmbeddingHandler) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	var req ModelRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Dimension <= 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dimension must be positive")
		return
	}

	model := &embedding.ModelInfo{
		ID:          req.ID,
		Name:        req.Name,
		Provider:    req.Provider,
		Dimension:   req.Dimension,
		MaxTokens:   req.MaxTokens,
		Description: req.Description,
		Metadata:    req.Metadata,
	}

	if err := h.version.RegisterModel(model); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"model_id": req.ID,
	})
}

// handleGetModel handles GET /v1/embeddings/models/{modelID}
func (h *EmbeddingHandler) handleGetModel(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	modelID := r.PathValue("modelID")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model ID required")
		return
	}

	model, err := h.version.GetModel(modelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, model)
}

// handleListVersions handles GET /v1/embeddings/models/{modelID}/versions
func (h *EmbeddingHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	modelID := r.PathValue("modelID")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model ID required")
		return
	}

	versions, err := h.version.ListVersions(modelID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"versions": versions,
		"count":    len(versions),
		"model_id": modelID,
	})
}

// handleRegisterVersion handles POST /v1/embeddings/models/{modelID}/versions
func (h *EmbeddingHandler) handleRegisterVersion(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	modelID := r.PathValue("modelID")
	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model ID required")
		return
	}

	var req VersionRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Version == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version is required")
		return
	}

	version := &embedding.ModelVersion{
		Version:    req.Version,
		ModelID:    modelID,
		Dimension:  req.Dimension,
		Compatible: req.Compatible,
		IsDefault:  req.IsDefault,
	}

	if err := h.version.RegisterVersion(version); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"model_id": modelID,
		"version":  req.Version,
	})
}

// handleDeprecateVersion handles POST /v1/embeddings/models/{modelID}/versions/{version}/deprecate
func (h *EmbeddingHandler) handleDeprecateVersion(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	modelID := r.PathValue("modelID")
	version := r.PathValue("version")

	if modelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model ID required")
		return
	}
	if version == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "version required")
		return
	}

	if err := h.version.DeprecateVersion(modelID, version); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"model_id": modelID,
		"version":  version,
	})
}

// handleCheckCompatibility handles POST /v1/embeddings/compatibility
func (h *EmbeddingHandler) handleCheckCompatibility(w http.ResponseWriter, r *http.Request) {
	if h.version == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "version manager not configured")
		return
	}

	var req CompatibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ModelID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "model_id is required")
		return
	}
	if req.FromVersion == "" || req.ToVersion == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "from_version and to_version are required")
		return
	}

	compatible, err := h.version.CheckCompatibility(req.ModelID, req.FromVersion, req.ToVersion)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"compatible":   compatible,
		"model_id":     req.ModelID,
		"from_version": req.FromVersion,
		"to_version":   req.ToVersion,
	})
}

// handleGetStats handles GET /v1/embeddings/stats
func (h *EmbeddingHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})

	if h.store != nil {
		stats["store"] = h.store.Stats()
	}
	if h.dedup != nil {
		stats["dedup"] = h.dedup.Stats()
	}
	if h.version != nil {
		stats["version"] = h.version.Stats()
	}
	if h.batch != nil {
		stats["batch"] = h.batch.Stats()
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleClear handles POST /v1/embeddings/clear
func (h *EmbeddingHandler) handleClear(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "embedding store not configured")
		return
	}

	if err := h.store.Clear(r.Context()); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (h *EmbeddingHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *EmbeddingHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
