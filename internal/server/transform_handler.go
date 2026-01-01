package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/storage"
	"github.com/feather-store/feather/internal/transform"
)

// TransformHandler handles feature transformation API requests.
type TransformHandler struct {
	pipeline *transform.Pipeline
	dsl      *transform.DSL
}

// NewTransformHandler creates a new transform handler.
func NewTransformHandler(store *storage.Store) *TransformHandler {
	pipeline := transform.NewPipeline(store)
	return &TransformHandler{
		pipeline: pipeline,
		dsl:      transform.NewDSL(pipeline),
	}
}

// RegisterRoutes registers transform API routes.
func (h *TransformHandler) RegisterRoutes(mux *http.ServeMux) {
	// Transform management
	mux.HandleFunc("GET /v1/transforms", h.handleListTransforms)
	mux.HandleFunc("POST /v1/transforms", h.handleRegisterTransform)
	mux.HandleFunc("GET /v1/transforms/{name}", h.handleGetTransform)
	mux.HandleFunc("DELETE /v1/transforms/{name}", h.handleUnregisterTransform)

	// DSL definition
	mux.HandleFunc("POST /v1/transforms/dsl", h.handleDefineFromDSL)

	// Execution
	mux.HandleFunc("POST /v1/transforms/{name}/execute", h.handleExecute)
	mux.HandleFunc("POST /v1/transforms/{name}/execute-store", h.handleExecuteAndStore)
	mux.HandleFunc("POST /v1/transforms/chain", h.handleExecuteChain)
}

// TransformRequest represents a transform registration request.
type TransformRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"`
	Expression  string                 `json:"expression,omitempty"`
	Inputs      []string               `json:"inputs"`
	Output      string                 `json:"output"`
	OutputType  string                 `json:"output_type,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Mode        string                 `json:"mode,omitempty"`
}

// DSLRequest represents a DSL definition request.
type DSLRequest struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// ExecuteRequest represents a transform execution request.
type ExecuteRequest struct {
	EntityID string `json:"entity_id"`
}

// ChainExecuteRequest represents a chain execution request.
type ChainExecuteRequest struct {
	OutputFeature string `json:"output_feature"`
	EntityID      string `json:"entity_id"`
}

// handleListTransforms handles GET /v1/transforms
func (h *TransformHandler) handleListTransforms(w http.ResponseWriter, r *http.Request) {
	transforms := h.pipeline.ListTransforms()

	result := make([]map[string]interface{}, len(transforms))
	for i, t := range transforms {
		result[i] = map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"type":        t.Type,
			"expression":  t.Expression,
			"inputs":      t.Inputs,
			"output":      t.Output,
			"enabled":     t.Enabled,
			"mode":        t.Mode,
			"created_at":  t.CreatedAt,
			"updated_at":  t.UpdatedAt,
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transforms": result,
		"count":      len(result),
	})
}

// handleRegisterTransform handles POST /v1/transforms
func (h *TransformHandler) handleRegisterTransform(w http.ResponseWriter, r *http.Request) {
	var req TransformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Type == "" {
		h.writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	if len(req.Inputs) == 0 {
		h.writeError(w, http.StatusBadRequest, "inputs are required")
		return
	}

	if req.Output == "" {
		h.writeError(w, http.StatusBadRequest, "output is required")
		return
	}

	t := &transform.Transform{
		Name:        req.Name,
		Description: req.Description,
		Type:        transform.TransformType(req.Type),
		Expression:  req.Expression,
		Inputs:      req.Inputs,
		Output:      req.Output,
		Config:      req.Config,
	}

	if req.Mode != "" {
		t.Mode = transform.ExecutionMode(req.Mode)
	} else {
		t.Mode = transform.ModeOnRead
	}

	if err := h.pipeline.RegisterTransform(t); err != nil {
		if err == transform.ErrDependencyCycle {
			h.writeError(w, http.StatusConflict, "dependency cycle detected")
			return
		}
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"name":    req.Name,
		"type":    req.Type,
		"output":  req.Output,
	})
}

// handleGetTransform handles GET /v1/transforms/{name}
func (h *TransformHandler) handleGetTransform(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "transform name required")
		return
	}

	t, err := h.pipeline.GetTransform(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        t.Name,
		"description": t.Description,
		"type":        t.Type,
		"expression":  t.Expression,
		"inputs":      t.Inputs,
		"output":      t.Output,
		"output_type": t.OutputType,
		"config":      t.Config,
		"enabled":     t.Enabled,
		"mode":        t.Mode,
		"created_at":  t.CreatedAt,
		"updated_at":  t.UpdatedAt,
	})
}

// handleUnregisterTransform handles DELETE /v1/transforms/{name}
func (h *TransformHandler) handleUnregisterTransform(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "transform name required")
		return
	}

	if err := h.pipeline.UnregisterTransform(name); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleDefineFromDSL handles POST /v1/transforms/dsl
func (h *TransformHandler) handleDefineFromDSL(w http.ResponseWriter, r *http.Request) {
	var req DSLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Expression == "" {
		h.writeError(w, http.StatusBadRequest, "expression is required")
		return
	}

	if err := h.dsl.Define(req.Name, req.Expression); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get the created transform to return details
	t, _ := h.pipeline.GetTransform(req.Name)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"name":       req.Name,
		"type":       t.Type,
		"output":     t.Output,
		"inputs":     t.Inputs,
		"expression": t.Expression,
	})
}

// handleExecute handles POST /v1/transforms/{name}/execute
func (h *TransformHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "transform name required")
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" {
		h.writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}

	result, err := h.pipeline.Execute(r.Context(), name, req.EntityID)
	if err != nil {
		if err == transform.ErrTransformNotFound {
			h.writeError(w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"transform": name,
		"entity_id": req.EntityID,
		"result":    result,
	})
}

// handleExecuteAndStore handles POST /v1/transforms/{name}/execute-store
func (h *TransformHandler) handleExecuteAndStore(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "transform name required")
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EntityID == "" {
		h.writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}

	if err := h.pipeline.ExecuteAndStore(r.Context(), name, req.EntityID); err != nil {
		if err == transform.ErrTransformNotFound {
			h.writeError(w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get the transform to return the output feature name
	t, _ := h.pipeline.GetTransform(name)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"transform":      name,
		"entity_id":      req.EntityID,
		"output_feature": t.Output,
		"stored":         true,
	})
}

// handleExecuteChain handles POST /v1/transforms/chain
func (h *TransformHandler) handleExecuteChain(w http.ResponseWriter, r *http.Request) {
	var req ChainExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OutputFeature == "" {
		h.writeError(w, http.StatusBadRequest, "output_feature is required")
		return
	}

	if req.EntityID == "" {
		h.writeError(w, http.StatusBadRequest, "entity_id is required")
		return
	}

	result, err := h.pipeline.ExecuteChain(r.Context(), req.OutputFeature, req.EntityID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"output_feature": req.OutputFeature,
		"entity_id":      req.EntityID,
		"result":         result,
	})
}

func (h *TransformHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TransformHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
