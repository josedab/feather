package server

import (
	"encoding/base64"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/multimodal"
)

// MultiModalHandler provides HTTP endpoints for multi-modal feature storage.
type MultiModalHandler struct {
	store      *multimodal.MultiModalStore
	embeddings *multimodal.EmbeddingIndex
}

// NewMultiModalHandler creates a new multi-modal handler.
func NewMultiModalHandler(store *multimodal.MultiModalStore, embeddings *multimodal.EmbeddingIndex) *MultiModalHandler {
	return &MultiModalHandler{store: store, embeddings: embeddings}
}

// RegisterRoutes registers multi-modal API routes.
func (h *MultiModalHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/multimodal/blobs", h.handleStoreBlob)
	mux.HandleFunc("GET /v1/multimodal/blobs/{id}", h.handleGetBlob)
	mux.HandleFunc("GET /v1/multimodal/blobs/{id}/metadata", h.handleGetBlobMetadata)
	mux.HandleFunc("DELETE /v1/multimodal/blobs/{id}", h.handleDeleteBlob)
	mux.HandleFunc("GET /v1/multimodal/blobs", h.handleListBlobs)
	mux.HandleFunc("GET /v1/multimodal/search", h.handleSearchBlobs)
	mux.HandleFunc("POST /v1/multimodal/embeddings", h.handleAddEmbedding)
	mux.HandleFunc("GET /v1/multimodal/embeddings/{blobId}", h.handleGetEmbedding)
	mux.HandleFunc("POST /v1/multimodal/embeddings/search", h.handleSearchEmbeddings)
	mux.HandleFunc("GET /v1/multimodal/stats", h.handleStats)
}

func (h *MultiModalHandler) handleStoreBlob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Modality    string            `json:"modality"`
		ContentType string            `json:"content_type"`
		Data        string            `json:"data"` // base64
		Tags        map[string]string `json:"tags"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid base64 data")
		return
	}

	meta, err := h.store.Store(multimodal.ModalityType(req.Modality), req.ContentType, data, req.Tags)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true, "metadata": meta,
	})
}

func (h *MultiModalHandler) handleGetBlob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, meta, err := h.store.Get(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"data":     base64.StdEncoding.EncodeToString(data),
		"metadata": meta,
	})
}

func (h *MultiModalHandler) handleGetBlobMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := h.store.GetMetadata(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "metadata": meta,
	})
}

func (h *MultiModalHandler) handleDeleteBlob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.Delete(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "message": "blob deleted",
	})
}

func (h *MultiModalHandler) handleListBlobs(w http.ResponseWriter, r *http.Request) {
	modality := multimodal.ModalityType(r.URL.Query().Get("modality"))
	blobs := h.store.List(modality)

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "blobs": blobs, "count": len(blobs),
	})
}

func (h *MultiModalHandler) handleSearchBlobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query parameter 'q' required")
		return
	}

	results := h.store.Search(q)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "results": results, "count": len(results),
	})
}

func (h *MultiModalHandler) handleAddEmbedding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BlobID string    `json:"blob_id"`
		Vector []float64 `json:"vector"`
		Model  string    `json:"model"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.embeddings.Add(req.BlobID, req.Vector, req.Model); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true, "message": "embedding added",
	})
}

func (h *MultiModalHandler) handleGetEmbedding(w http.ResponseWriter, r *http.Request) {
	blobID := r.PathValue("blobId")
	entry, err := h.embeddings.Get(blobID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "embedding": entry,
	})
}

func (h *MultiModalHandler) handleSearchEmbeddings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query []float64 `json:"query"`
		TopK  int       `json:"top_k"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TopK <= 0 {
		req.TopK = 10
	}

	results := h.embeddings.Search(req.Query, req.TopK)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "results": results, "count": len(results),
	})
}

func (h *MultiModalHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"store_stats":     h.store.Stats(),
		"embedding_stats": h.embeddings.Stats(),
	})
}
