package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/featherqlv2"
)

// FeatherQLv2Handler handles FeatherQL v2 API requests.
type FeatherQLv2Handler struct {
	engine *featherqlv2.Engine
}

// NewFeatherQLv2Handler creates a new FeatherQL v2 handler.
func NewFeatherQLv2Handler(engine *featherqlv2.Engine) *FeatherQLv2Handler {
	return &FeatherQLv2Handler{engine: engine}
}

// RegisterRoutes registers FeatherQL v2 API routes.
func (h *FeatherQLv2Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/featherql/v2/parse", h.handleParse)
	mux.HandleFunc("POST /v1/featherql/v2/compile", h.handleCompile)
	mux.HandleFunc("POST /v1/featherql/v2/execute", h.handleExecute)
	mux.HandleFunc("GET /v1/featherql/v2/pipelines", h.handleListPipelines)
	mux.HandleFunc("GET /v1/featherql/v2/pipelines/{id}", h.handleGetPipeline)
	mux.HandleFunc("DELETE /v1/featherql/v2/pipelines/{id}", h.handleDeletePipeline)
}

func (h *FeatherQLv2Handler) handleParse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := h.engine.Parse(req.Query)
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *FeatherQLv2Handler) handleCompile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" || req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id and query are required")
		return
	}

	pipeline, err := h.engine.Compile(req.ID, req.Query)
	if err != nil {
		if errors.Is(err, featherqlv2.ErrParseFailed) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, pipeline)
}

func (h *FeatherQLv2Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.engine.Execute(req.Query)
	if err != nil {
		if errors.Is(err, featherqlv2.ErrParseFailed) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *FeatherQLv2Handler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	pipelines := h.engine.ListPipelines()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"count":     len(pipelines),
	})
}

func (h *FeatherQLv2Handler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pipeline, err := h.engine.GetPipeline(id)
	if err != nil {
		if errors.Is(err, featherqlv2.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, pipeline)
}

func (h *FeatherQLv2Handler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.DeletePipeline(id); err != nil {
		if errors.Is(err, featherqlv2.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline deleted"})
}

func (h *FeatherQLv2Handler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *FeatherQLv2Handler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
