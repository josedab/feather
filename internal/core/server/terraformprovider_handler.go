package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/terraformprovider"
)

// TerraformProviderHandler handles Terraform provider API requests.
type TerraformProviderHandler struct {
	provider *terraformprovider.Provider
}

// NewTerraformProviderHandler creates a new Terraform provider handler.
func NewTerraformProviderHandler(provider *terraformprovider.Provider) *TerraformProviderHandler {
	return &TerraformProviderHandler{
		provider: provider,
	}
}

// RegisterRoutes registers Terraform provider API routes.
func (h *TerraformProviderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/terraform/resources", h.handleListResources)
	mux.HandleFunc("POST /v1/terraform/resources", h.handleCreateResource)
	mux.HandleFunc("GET /v1/terraform/resources/{id}", h.handleReadResource)
	mux.HandleFunc("PUT /v1/terraform/resources/{id}", h.handleUpdateResource)
	mux.HandleFunc("DELETE /v1/terraform/resources/{id}", h.handleDeleteResource)
	mux.HandleFunc("POST /v1/terraform/plan", h.handlePlan)
	mux.HandleFunc("POST /v1/terraform/apply", h.handleApply)
	mux.HandleFunc("POST /v1/terraform/import", h.handleImport)
	mux.HandleFunc("GET /v1/terraform/stats", h.handleGetStats)
}

// handleListResources handles GET /v1/terraform/resources
func (h *TerraformProviderHandler) handleListResources(w http.ResponseWriter, r *http.Request) {
	resType := terraformprovider.ResourceType(r.URL.Query().Get("type"))
	resources := h.provider.ListResources(resType)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"resources": resources,
	})
}

// handleCreateResource handles POST /v1/terraform/resources
func (h *TerraformProviderHandler) handleCreateResource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       string                 `json:"type"`
		ID         string                 `json:"id"`
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.provider.CreateResource(terraformprovider.ResourceType(req.Type), req.ID, req.Attributes)
	if err != nil {
		if errors.Is(err, terraformprovider.ErrResourceExists) {
			h.writeError(r.Context(), w, http.StatusConflict, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, state)
}

// handleReadResource handles GET /v1/terraform/resources/{id}
func (h *TerraformProviderHandler) handleReadResource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "resource id required")
		return
	}

	state, err := h.provider.ReadResource(id)
	if err != nil {
		if errors.Is(err, terraformprovider.ErrResourceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, state)
}

// handleUpdateResource handles PUT /v1/terraform/resources/{id}
func (h *TerraformProviderHandler) handleUpdateResource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "resource id required")
		return
	}

	var req struct {
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.provider.UpdateResource(id, req.Attributes)
	if err != nil {
		if errors.Is(err, terraformprovider.ErrResourceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, state)
}

// handleDeleteResource handles DELETE /v1/terraform/resources/{id}
func (h *TerraformProviderHandler) handleDeleteResource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "resource id required")
		return
	}

	if err := h.provider.DeleteResource(id); err != nil {
		if errors.Is(err, terraformprovider.ErrResourceNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "resource deleted"})
}

// handlePlan handles POST /v1/terraform/plan
func (h *TerraformProviderHandler) handlePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Desired []terraformprovider.ResourceState `json:"desired"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.provider.Plan(req.Desired)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"plan": results,
	})
}

// handleApply handles POST /v1/terraform/apply
func (h *TerraformProviderHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan []terraformprovider.PlanResult `json:"plan"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.provider.Apply(req.Plan)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleImport handles POST /v1/terraform/import
func (h *TerraformProviderHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.provider.ImportResource(terraformprovider.ResourceType(req.Type), req.ID)
	if err != nil {
		if errors.Is(err, terraformprovider.ErrResourceExists) {
			h.writeError(r.Context(), w, http.StatusConflict, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, state)
}

// handleGetStats handles GET /v1/terraform/stats
func (h *TerraformProviderHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.provider.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *TerraformProviderHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *TerraformProviderHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
