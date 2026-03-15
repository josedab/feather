package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/core/vector"
)

// VectorHandler handles vector similarity search API requests.
type VectorHandler struct {
	store   *vector.Store
	metrics interface {
		RecordHTTPLatency(method, path string, duration time.Duration)
	}
}

// NewVectorHandler creates a new vector handler.
func NewVectorHandler(store *vector.Store, metrics interface {
	RecordHTTPLatency(method, path string, duration time.Duration)
}) *VectorHandler {
	return &VectorHandler{
		store:   store,
		metrics: metrics,
	}
}

// RegisterRoutes registers vector API routes.
func (h *VectorHandler) RegisterRoutes(mux *http.ServeMux) {
	// Index management
	mux.HandleFunc("GET /v1/vectors", h.handleListIndexes)
	mux.HandleFunc("POST /v1/vectors", h.handleCreateIndex)
	mux.HandleFunc("GET /v1/vectors/{index}", h.handleGetIndex)
	mux.HandleFunc("DELETE /v1/vectors/{index}", h.handleDeleteIndex)

	// Vector operations
	mux.HandleFunc("POST /v1/vectors/{index}/upsert", h.handleUpsert)
	mux.HandleFunc("POST /v1/vectors/{index}/search", h.handleSearch)
	mux.HandleFunc("GET /v1/vectors/{index}/{id}", h.handleGetVector)
	mux.HandleFunc("DELETE /v1/vectors/{index}/{id}", h.handleDeleteVector)
	mux.HandleFunc("GET /v1/vectors/{index}/query/{id}", h.handleQueryByID)
}

// CreateIndexRequest represents a request to create a new vector index.
type CreateIndexRequest struct {
	Name         string              `json:"name"`
	Dimension    int                 `json:"dimension"`
	DistanceType vector.DistanceType `json:"distance_type,omitempty"`
}

// UpsertRequest represents a request to upsert vectors.
type UpsertRequest struct {
	Vectors []VectorInput `json:"vectors"`
}

// VectorInput represents a vector to upsert.
type VectorInput struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VectorSearchRequest represents a vector search request.
type VectorSearchRequest struct {
	Vector          []float32 `json:"vector"`
	TopK            int       `json:"top_k"`
	Ef              int       `json:"ef,omitempty"`
	IncludeMetadata bool      `json:"include_metadata,omitempty"`
	IncludeVectors  bool      `json:"include_vectors,omitempty"`
}

// IndexInfo represents information about a vector index.
type IndexInfo struct {
	Name         string              `json:"name"`
	Dimension    int                 `json:"dimension"`
	DistanceType vector.DistanceType `json:"distance_type"`
	Size         int                 `json:"size"`
}

// handleListIndexes handles GET /v1/vectors
func (h *VectorHandler) handleListIndexes(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("GET", "/v1/vectors", time.Since(start))
		}
	}()

	indexes := h.store.ListIndexes()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"indexes": indexes,
	})
}

// handleCreateIndex handles POST /v1/vectors
func (h *VectorHandler) handleCreateIndex(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("POST", "/v1/vectors", time.Since(start))
		}
	}()

	var req CreateIndexRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Dimension <= 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "dimension must be positive")
		return
	}

	idx, err := h.store.CreateIndex(req.Name, req.Dimension, req.DistanceType)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, IndexInfo{
		Name:         idx.Name,
		Dimension:    idx.Dimension,
		DistanceType: idx.DistanceType,
		Size:         idx.Size(),
	})
}

// handleGetIndex handles GET /v1/vectors/{index}
func (h *VectorHandler) handleGetIndex(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("GET", "/v1/vectors/{index}", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	if indexName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name required")
		return
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, IndexInfo{
		Name:         idx.Name,
		Dimension:    idx.Dimension,
		DistanceType: idx.DistanceType,
		Size:         idx.Size(),
	})
}

// handleDeleteIndex handles DELETE /v1/vectors/{index}
func (h *VectorHandler) handleDeleteIndex(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("DELETE", "/v1/vectors/{index}", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	if indexName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name required")
		return
	}

	if err := h.store.DeleteIndex(indexName); err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]bool{"success": true})
}

// handleUpsert handles POST /v1/vectors/{index}/upsert
func (h *VectorHandler) handleUpsert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("POST", "/v1/vectors/{index}/upsert", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	if indexName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name required")
		return
	}

	var req UpsertRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Vectors) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "vectors required")
		return
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to records
	records := make([]vector.Record, len(req.Vectors))
	for i, v := range req.Vectors {
		records[i] = vector.Record{
			ID:       v.ID,
			Vector:   v.Vector,
			Metadata: v.Metadata,
		}
	}

	if err := idx.UpsertBatch(records); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"upserted": len(req.Vectors),
	})
}

// handleSearch handles POST /v1/vectors/{index}/search
func (h *VectorHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("POST", "/v1/vectors/{index}/search", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	if indexName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name required")
		return
	}

	var req VectorSearchRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Vector) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "vector required")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 10 // Default
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	if idx.Dimension > 0 && len(req.Vector) != idx.Dimension {
		h.writeError(r.Context(), w, http.StatusBadRequest,
			fmt.Sprintf("vector dimension mismatch: query has %d dimensions, index %q expects %d",
				len(req.Vector), indexName, idx.Dimension))
		return
	}

	results, err := idx.Search(req.Vector, req.TopK, &vector.SearchOptions{
		Ef:              req.Ef,
		IncludeMetadata: req.IncludeMetadata,
		IncludeVectors:  req.IncludeVectors,
	})
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetVector handles GET /v1/vectors/{index}/{id}
func (h *VectorHandler) handleGetVector(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("GET", "/v1/vectors/{index}/{id}", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	vectorID := r.PathValue("id")
	if indexName == "" || vectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name and vector id required")
		return
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	record, err := idx.Get(vectorID)
	if err != nil {
		if errors.Is(err, vector.ErrVectorNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "vector not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check if user wants vector in response
	includeVector := r.URL.Query().Get("include_vector")
	if includeVector != "true" {
		record.Vector = nil
	}

	h.writeJSON(r.Context(), w, http.StatusOK, record)
}

// handleDeleteVector handles DELETE /v1/vectors/{index}/{id}
func (h *VectorHandler) handleDeleteVector(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("DELETE", "/v1/vectors/{index}/{id}", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	vectorID := r.PathValue("id")
	if indexName == "" || vectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name and vector id required")
		return
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := idx.Delete(vectorID); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]bool{"success": true})
}

// handleQueryByID handles finding similar vectors to an existing vector
func (h *VectorHandler) handleQueryByID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		if h.metrics != nil {
			h.metrics.RecordHTTPLatency("GET", "/v1/vectors/{index}/query/{id}", time.Since(start))
		}
	}()

	indexName := r.PathValue("index")
	vectorID := r.PathValue("id")
	if indexName == "" || vectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "index name and vector id required")
		return
	}

	topK := 10
	if k := r.URL.Query().Get("top_k"); k != "" {
		if parsed, err := strconv.Atoi(k); err == nil && parsed > 0 {
			topK = parsed
		}
	}

	idx, err := h.store.GetIndex(indexName)
	if err != nil {
		if errors.Is(err, vector.ErrIndexNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "index not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get the source vector
	record, err := idx.Get(vectorID)
	if err != nil {
		if errors.Is(err, vector.ErrVectorNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "vector not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Search for similar vectors
	results, err := idx.Search(record.Vector, topK+1, &vector.SearchOptions{
		IncludeMetadata: r.URL.Query().Get("include_metadata") == "true",
		IncludeVectors:  r.URL.Query().Get("include_vectors") == "true",
	})
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter out the source vector itself
	filtered := make([]vector.SearchResultWithMetadata, 0, len(results))
	for _, res := range results {
		if res.ID != vectorID {
			filtered = append(filtered, res)
		}
	}

	// Limit to topK
	if len(filtered) > topK {
		filtered = filtered[:topK]
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": filtered,
	})
}

func (h *VectorHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *VectorHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
