package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/llmstore"
)

// LLMStoreHandler handles LLM feature store API requests.
type LLMStoreHandler struct {
	store *llmstore.Store
}

// NewLLMStoreHandler creates a new LLM store handler.
func NewLLMStoreHandler(store *llmstore.Store) *LLMStoreHandler {
	return &LLMStoreHandler{store: store}
}

// RegisterRoutes registers LLM store API routes.
func (h *LLMStoreHandler) RegisterRoutes(mux *http.ServeMux) {
	// Prompts
	mux.HandleFunc("GET /v1/llm/prompts", h.handleListPrompts)
	mux.HandleFunc("POST /v1/llm/prompts", h.handleCreatePrompt)
	mux.HandleFunc("GET /v1/llm/prompts/{id}", h.handleGetPrompt)
	mux.HandleFunc("PUT /v1/llm/prompts/{id}", h.handleUpdatePrompt)
	mux.HandleFunc("DELETE /v1/llm/prompts/{id}", h.handleDeletePrompt)

	// Embeddings
	mux.HandleFunc("POST /v1/llm/embeddings", h.handleStoreEmbedding)
	mux.HandleFunc("GET /v1/llm/embeddings/{id}", h.handleGetEmbedding)
	mux.HandleFunc("POST /v1/llm/embeddings/search", h.handleSearchSimilar)

	// RAG Pipelines
	mux.HandleFunc("GET /v1/llm/rag/pipelines", h.handleListPipelines)
	mux.HandleFunc("POST /v1/llm/rag/pipelines", h.handleCreatePipeline)
	mux.HandleFunc("GET /v1/llm/rag/pipelines/{id}", h.handleGetPipeline)
	mux.HandleFunc("POST /v1/llm/rag/query", h.handleRAGQuery)

	// Stats
	mux.HandleFunc("GET /v1/llm/store/stats", h.handleStats)
}

func (h *LLMStoreHandler) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	prompts := h.store.ListPrompts()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"prompts": prompts,
		"count":   len(prompts),
	})
}

func (h *LLMStoreHandler) handleCreatePrompt(w http.ResponseWriter, r *http.Request) {
	var p llmstore.PromptTemplate
	if err := strictDecode(r.Body, &p); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.store.CreatePrompt(p)
	if err != nil {
		if errors.Is(err, llmstore.ErrPromptExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "prompt already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *LLMStoreHandler) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.GetPrompt(id)
	if err != nil {
		if errors.Is(err, llmstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, p)
}

func (h *LLMStoreHandler) handleUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p llmstore.PromptTemplate
	if err := strictDecode(r.Body, &p); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id

	updated, err := h.store.UpdatePrompt(p)
	if err != nil {
		if errors.Is(err, llmstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, updated)
}

func (h *LLMStoreHandler) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeletePrompt(id); err != nil {
		if errors.Is(err, llmstore.ErrPromptNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "prompt not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "prompt deleted"})
}

func (h *LLMStoreHandler) handleStoreEmbedding(w http.ResponseWriter, r *http.Request) {
	var e llmstore.Embedding
	if err := strictDecode(r.Body, &e); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	stored, err := h.store.StoreEmbedding(e)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, stored)
}

func (h *LLMStoreHandler) handleGetEmbedding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	e, err := h.store.GetEmbedding(id)
	if err != nil {
		if errors.Is(err, llmstore.ErrEmbeddingNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "embedding not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, e)
}

type searchSimilarReq struct {
	Vector   []float64 `json:"vector"`
	TopK     int       `json:"top_k,omitempty"`
	MinScore float64   `json:"min_score,omitempty"`
}

func (h *LLMStoreHandler) handleSearchSimilar(w http.ResponseWriter, r *http.Request) {
	var req searchSimilarReq
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.store.SearchSimilar(req.Vector, req.TopK, req.MinScore)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

func (h *LLMStoreHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	pipelines := h.store.ListPipelines()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"count":     len(pipelines),
	})
}

func (h *LLMStoreHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	var p llmstore.RAGPipeline
	if err := strictDecode(r.Body, &p); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.store.CreatePipeline(p)
	if err != nil {
		if errors.Is(err, llmstore.ErrPipelineExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *LLMStoreHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.store.GetPipeline(id)
	if err != nil {
		if errors.Is(err, llmstore.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, p)
}

func (h *LLMStoreHandler) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	var req llmstore.RAGRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.store.QueryRAG(req)
	if err != nil {
		if errors.Is(err, llmstore.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *LLMStoreHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *LLMStoreHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *LLMStoreHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
