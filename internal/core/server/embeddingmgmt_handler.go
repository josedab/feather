package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
)

// EmbeddingMgmtHandler handles embedding management API requests.
type EmbeddingMgmtHandler struct {
	manager *embeddingmgmt.Manager
}

// NewEmbeddingMgmtHandler creates a new embedding management handler.
func NewEmbeddingMgmtHandler(manager *embeddingmgmt.Manager) *EmbeddingMgmtHandler {
	return &EmbeddingMgmtHandler{manager: manager}
}

// RegisterRoutes registers embedding management API routes.
func (h *EmbeddingMgmtHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/embeddings/models", h.handleListModels)
	mux.HandleFunc("POST /v1/embeddings/models", h.handleRegisterModel)
	mux.HandleFunc("GET /v1/embeddings/collections", h.handleListCollections)
	mux.HandleFunc("POST /v1/embeddings/collections", h.handleCreateCollection)
	mux.HandleFunc("GET /v1/embeddings/collections/{name}", h.handleGetCollection)
	mux.HandleFunc("DELETE /v1/embeddings/collections/{name}", h.handleDeleteCollection)
	mux.HandleFunc("POST /v1/embeddings/collections/{name}/upsert", h.handleUpsert)
	mux.HandleFunc("GET /v1/embeddings/collections/{name}/{id}", h.handleGetEmbedding)
	mux.HandleFunc("POST /v1/embeddings/collections/{name}/search", h.handleSearch)
	mux.HandleFunc("GET /v1/embeddings/stats", h.handleStats)
}

func (h *EmbeddingMgmtHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	models := h.manager.ListModels()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"models": models})
}

func (h *EmbeddingMgmtHandler) handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var model embeddingmgmt.EmbeddingModel
	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RegisterModel(model); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "model_id": model.ID})
}

func (h *EmbeddingMgmtHandler) handleListCollections(w http.ResponseWriter, r *http.Request) {
	collections := h.manager.ListCollections()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"collections": collections})
}

func (h *EmbeddingMgmtHandler) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string            `json:"name"`
		ModelID  string            `json:"model_id"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	col, err := h.manager.CreateCollection(req.Name, req.ModelID, req.Metadata)
	if err != nil {
		if errors.Is(err, embeddingmgmt.ErrCollectionExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "collection already exists")
			return
		}
		if errors.Is(err, embeddingmgmt.ErrModelNotFound) {
			h.writeError(r.Context(), w, http.StatusBadRequest, "model not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, col)
}

func (h *EmbeddingMgmtHandler) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	col, err := h.manager.GetCollection(name)
	if err != nil {
		if errors.Is(err, embeddingmgmt.ErrCollectionNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "collection not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, col)
}

func (h *EmbeddingMgmtHandler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.manager.DeleteCollection(name); err != nil {
		if errors.Is(err, embeddingmgmt.ErrCollectionNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "collection not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *EmbeddingMgmtHandler) handleUpsert(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var emb embeddingmgmt.Embedding
	if err := json.NewDecoder(r.Body).Decode(&emb); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.Upsert(name, emb); err != nil {
		if errors.Is(err, embeddingmgmt.ErrDimensionMismatch) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *EmbeddingMgmtHandler) handleGetEmbedding(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	emb, err := h.manager.Get(name, id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, emb)
}

func (h *EmbeddingMgmtHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Query []float64 `json:"query"`
		TopK  int       `json:"top_k,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	topK := req.TopK
	if topK <= 0 {
		if k := r.URL.Query().Get("top_k"); k != "" {
			topK, _ = strconv.Atoi(k)
		}
	}

	results, err := h.manager.Search(name, req.Query, topK)
	if err != nil {
		if errors.Is(err, embeddingmgmt.ErrDimensionMismatch) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"results": results, "count": len(results)})
}

func (h *EmbeddingMgmtHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *EmbeddingMgmtHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *EmbeddingMgmtHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
