package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/rag"
)

// RAGHandler handles RAG pipeline API requests.
type RAGHandler struct {
	pipeline *rag.Pipeline
}

// NewRAGHandler creates a new RAG handler.
func NewRAGHandler(pipeline *rag.Pipeline) *RAGHandler {
	return &RAGHandler{pipeline: pipeline}
}

// RegisterRoutes registers RAG API routes.
func (h *RAGHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/rag/documents", h.handleIngestDocument)
	mux.HandleFunc("GET /v1/rag/documents", h.handleListDocuments)
	mux.HandleFunc("GET /v1/rag/documents/{id}", h.handleGetDocument)
	mux.HandleFunc("DELETE /v1/rag/documents/{id}", h.handleDeleteDocument)
	mux.HandleFunc("POST /v1/rag/retrieve", h.handleRetrieve)
	mux.HandleFunc("GET /v1/rag/stats", h.handleStats)
}

func (h *RAGHandler) handleIngestDocument(w http.ResponseWriter, r *http.Request) {
	var doc rag.Document
	if err := strictDecode(r.Body, &doc); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.pipeline.Ingest(r.Context(), &doc); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "id": doc.ID})
}

func (h *RAGHandler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	docs := h.pipeline.ListDocuments(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"documents": docs})
}

func (h *RAGHandler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "document id required")
		return
	}
	doc, err := h.pipeline.GetDocument(r.Context(), id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, doc)
}

func (h *RAGHandler) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "document id required")
		return
	}
	if err := h.pipeline.Delete(r.Context(), id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

type retrieveRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (h *RAGHandler) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	var req retrieveRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query required")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 10
	}
	result, err := h.pipeline.Retrieve(r.Context(), req.Query, req.TopK)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *RAGHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.pipeline.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *RAGHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *RAGHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
