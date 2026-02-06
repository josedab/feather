package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/cloudstorage"
)

// CloudStorageHandler handles cloud storage API requests.
type CloudStorageHandler struct {
	store *cloudstorage.ObjectStore
}

// NewCloudStorageHandler creates a new cloud storage handler.
func NewCloudStorageHandler(store *cloudstorage.ObjectStore) *CloudStorageHandler {
	return &CloudStorageHandler{
		store: store,
	}
}

// RegisterRoutes registers cloud storage API routes.
func (h *CloudStorageHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/storage/objects/{key}", h.handlePutObject)
	mux.HandleFunc("GET /v1/storage/objects/{key}", h.handleGetObject)
	mux.HandleFunc("DELETE /v1/storage/objects/{key}", h.handleDeleteObject)
	mux.HandleFunc("GET /v1/storage/objects", h.handleListObjects)
	mux.HandleFunc("HEAD /v1/storage/objects/{key}", h.handleHeadObject)
	mux.HandleFunc("POST /v1/storage/objects/copy", h.handleCopyObject)
	mux.HandleFunc("GET /v1/storage/bucket/stats", h.handleGetBucketStats)
	mux.HandleFunc("GET /v1/storage/stats", h.handleGetStats)
}

// handlePutObject handles PUT /v1/storage/objects/{key}
func (h *CloudStorageHandler) handlePutObject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "object key required")
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "failed to read request body")
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.store.Put(key, data, contentType, nil); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "object stored"})
}

// handleGetObject handles GET /v1/storage/objects/{key}
func (h *CloudStorageHandler) handleGetObject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "object key required")
		return
	}

	data, info, err := h.store.Get(key)
	if err != nil {
		if errors.Is(err, cloudstorage.ErrObjectNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("ETag", info.ETag)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.Debug("failed to write cloud storage object", "error", err)
	}
}

// handleDeleteObject handles DELETE /v1/storage/objects/{key}
func (h *CloudStorageHandler) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "object key required")
		return
	}

	if err := h.store.Delete(key); err != nil {
		if errors.Is(err, cloudstorage.ErrObjectNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "object deleted"})
}

// handleListObjects handles GET /v1/storage/objects
func (h *CloudStorageHandler) handleListObjects(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")

	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	objects := h.store.List(prefix, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"objects": objects,
	})
}

// handleHeadObject handles HEAD /v1/storage/objects/{key}
func (h *CloudStorageHandler) handleHeadObject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if h.store.Exists(key) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// handleCopyObject handles POST /v1/storage/objects/copy
func (h *CloudStorageHandler) handleCopyObject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Src == "" || req.Dst == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "src and dst keys required")
		return
	}

	if err := h.store.Copy(req.Src, req.Dst); err != nil {
		if errors.Is(err, cloudstorage.ErrObjectNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "object copied"})
}

// handleGetBucketStats handles GET /v1/storage/bucket/stats
func (h *CloudStorageHandler) handleGetBucketStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.ListBucketStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleGetStats handles GET /v1/storage/stats
func (h *CloudStorageHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *CloudStorageHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *CloudStorageHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
