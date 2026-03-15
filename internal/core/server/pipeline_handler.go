package server

import (
	"net/http"
	"sync"

	"github.com/feather-store/feather/internal/tools/pipelinebuilder"
)

// PipelineHandler provides HTTP endpoints for the visual pipeline builder.
type PipelineHandler struct {
	pipelines   map[string]*pipelinebuilder.Pipeline
	transforms  *pipelinebuilder.TransformRegistry
	templates   *pipelinebuilder.TemplateStore
	codegen     *pipelinebuilder.CodeGenerator
	mu          sync.RWMutex
	requireAuth func(http.Handler) http.Handler
}

// NewPipelineHandler creates a new pipeline handler with default components.
func NewPipelineHandler() *PipelineHandler {
	return &PipelineHandler{
		pipelines:  make(map[string]*pipelinebuilder.Pipeline),
		transforms: pipelinebuilder.NewTransformRegistry(),
		templates:  pipelinebuilder.NewTemplateStore(),
		codegen:    pipelinebuilder.NewCodeGenerator(pipelinebuilder.CodeGenConfig{Language: "go", IncludeComments: true}),
	}
}

// RegisterRoutes registers pipeline builder API routes.
func (h *PipelineHandler) RegisterRoutes(mux *http.ServeMux) {
	wrap := h.requireAuth
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle("GET /v1/pipelines/transforms", wrap(http.HandlerFunc(h.handleListTransforms)))
	mux.Handle("GET /v1/pipelines/transforms/{id}", wrap(http.HandlerFunc(h.handleGetTransform)))
	mux.Handle("GET /v1/pipelines/templates", wrap(http.HandlerFunc(h.handleListTemplates)))
	mux.Handle("GET /v1/pipelines/templates/{id}", wrap(http.HandlerFunc(h.handleGetTemplate)))
	mux.Handle("GET /v1/pipelines/stats", wrap(http.HandlerFunc(h.handleStats)))
	mux.Handle("GET /v1/pipelines", wrap(http.HandlerFunc(h.handleListPipelines)))
	mux.Handle("POST /v1/pipelines", wrap(http.HandlerFunc(h.handleCreatePipeline)))
	mux.Handle("GET /v1/pipelines/{id}", wrap(http.HandlerFunc(h.handleGetPipeline)))
	mux.Handle("DELETE /v1/pipelines/{id}", wrap(http.HandlerFunc(h.handleDeletePipeline)))
	mux.Handle("POST /v1/pipelines/{id}/nodes", wrap(http.HandlerFunc(h.handleAddNode)))
	mux.Handle("DELETE /v1/pipelines/{id}/nodes/{nodeId}", wrap(http.HandlerFunc(h.handleRemoveNode)))
	mux.Handle("POST /v1/pipelines/{id}/connect", wrap(http.HandlerFunc(h.handleConnect)))
	mux.Handle("POST /v1/pipelines/{id}/validate", wrap(http.HandlerFunc(h.handleValidate)))
	mux.Handle("POST /v1/pipelines/{id}/codegen", wrap(http.HandlerFunc(h.handleCodegen)))
}

func (h *PipelineHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]*pipelinebuilder.Pipeline, 0, len(h.pipelines))
	for _, p := range h.pipelines {
		list = append(list, p)
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"pipelines": list,
		"count":     len(list),
	})
}

func (h *PipelineHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := pipelinebuilder.NewPipeline(req.Name, req.Description)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "failed to create pipeline: "+err.Error())
		return
	}
	h.mu.Lock()
	h.pipelines[p.ID] = p
	h.mu.Unlock()
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"pipeline": p,
	})
}

func (h *PipelineHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "pipeline": p})
}

func (h *PipelineHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	id := r.PathValue("id")
	if _, ok := h.pipelines[id]; !ok {
		h.mu.Unlock()
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	delete(h.pipelines, id)
	h.mu.Unlock()
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline deleted"})
}

func (h *PipelineHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	var node pipelinebuilder.PipelineNode
	if err := strictDecode(r.Body, &node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := p.AddNode(&node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "node": node})
}

func (h *PipelineHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	if err := p.RemoveNode(r.PathValue("nodeId")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "node removed"})
}

func (h *PipelineHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := p.Connect(req.From, req.To); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "nodes connected"})
}

func (h *PipelineHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	errs := p.Validate()
	hasErrors := false
	for _, e := range errs {
		if e.Severity == "error" {
			hasErrors = true
			break
		}
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"valid":   !hasErrors,
		"errors":  errs,
	})
}

func (h *PipelineHandler) handleCodegen(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	p, ok := h.pipelines[r.PathValue("id")]
	h.mu.RUnlock()
	if !ok {
		writeJSONError(r.Context(), w, http.StatusNotFound, "pipeline not found")
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "go"
	}
	gen := pipelinebuilder.NewCodeGenerator(pipelinebuilder.CodeGenConfig{
		Language:        lang,
		IncludeComments: true,
	})
	code, err := gen.Generate(p, h.transforms)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"language": lang,
		"code":     code,
	})
}

func (h *PipelineHandler) handleListTransforms(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	query := r.URL.Query().Get("q")
	var transforms []*pipelinebuilder.TransformDef
	switch {
	case query != "":
		transforms = h.transforms.Search(query)
	case category != "":
		transforms = h.transforms.ListByCategory(category)
	default:
		transforms = h.transforms.List()
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"transforms": transforms,
		"count":      len(transforms),
	})
}

func (h *PipelineHandler) handleGetTransform(w http.ResponseWriter, r *http.Request) {
	t, err := h.transforms.Get(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "transform": t})
}

func (h *PipelineHandler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	var templates []*pipelinebuilder.Template
	if query != "" {
		templates = h.templates.Search(query)
	} else {
		templates = h.templates.List()
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"templates": templates,
		"count":     len(templates),
	})
}

func (h *PipelineHandler) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := h.templates.Get(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.templates.IncrementUsage(t.ID)
	// Instantiate a new pipeline from the template.
	p, err2 := pipelinebuilder.NewPipeline(t.Pipeline.Name, t.Pipeline.Description)
	if err2 != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "failed to create pipeline: "+err2.Error())
		return
	}
	for id, node := range t.Pipeline.Nodes {
		clone := *node
		p.Nodes[id] = &clone
	}
	p.Tags = t.Tags
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"template": t,
		"pipeline": p,
	})
}

func (h *PipelineHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	statusCounts := make(map[string]int)
	totalNodes := 0
	for _, p := range h.pipelines {
		statusCounts[string(p.Status)]++
		totalNodes += len(p.Nodes)
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"total_pipelines": len(h.pipelines),
		"total_nodes":     totalNodes,
		"by_status":       statusCounts,
		"transforms":      len(h.transforms.List()),
		"templates":       len(h.templates.List()),
	})
}
