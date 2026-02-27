package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/ftl"
)

// FTLHandler provides HTTP endpoints for the Feature Transformation Language.
type FTLHandler struct {
	compiler *ftl.Compiler
}

// NewFTLHandler creates a new FTL handler.
func NewFTLHandler(compiler *ftl.Compiler) *FTLHandler {
	return &FTLHandler{compiler: compiler}
}

// RegisterRoutes registers FTL API routes.
func (h *FTLHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/ftl/tokenize", h.handleTokenize)
	mux.HandleFunc("POST /v1/ftl/parse", h.handleParse)
	mux.HandleFunc("POST /v1/ftl/compile", h.handleCompile)
	mux.HandleFunc("POST /v1/ftl/execute/{id}", h.handleExecute)
	mux.HandleFunc("POST /v1/ftl/query", h.handleQuery)
	mux.HandleFunc("GET /v1/ftl/pipelines", h.handleListPipelines)
	mux.HandleFunc("GET /v1/ftl/pipelines/{id}", h.handleGetPipeline)
	mux.HandleFunc("DELETE /v1/ftl/pipelines/{id}", h.handleDeletePipeline)
	mux.HandleFunc("GET /v1/ftl/stats", h.handleStats)
}

func (h *FTLHandler) handleTokenize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Query == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	tokens, err := h.compiler.Tokenize(req.Query)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
		"count":  len(tokens),
	})
}

func (h *FTLHandler) handleParse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Query == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	ast, err := h.compiler.Parse(req.Query)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"ast":   ast,
		"query": req.Query,
	})
}

func (h *FTLHandler) handleCompile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Source == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "name and source are required")
		return
	}

	pipeline, err := h.compiler.Compile(req.Name, req.Source)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, pipeline)
}

func (h *FTLHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.compiler.Execute(id, req.Data)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FTLHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string                   `json:"query"`
		Data  []map[string]interface{} `json:"data"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Query == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	result, err := h.compiler.ExecuteQuery(req.Query, req.Data)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FTLHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	pipelines := h.compiler.ListPipelines()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"total":     len(pipelines),
	})
}

func (h *FTLHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pipeline, err := h.compiler.GetPipeline(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, pipeline)
}

func (h *FTLHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.compiler.DeletePipeline(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *FTLHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.compiler.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
