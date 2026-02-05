package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/migration"
)

// MigrationHandler handles Feast to Feather migration API requests.
type MigrationHandler struct {
	manager *migration.Manager
}

// NewMigrationHandler creates a new migration handler.
func NewMigrationHandler(manager *migration.Manager) *MigrationHandler {
	return &MigrationHandler{
		manager: manager,
	}
}

// RegisterRoutes registers migration routes with the given mux.
func (h *MigrationHandler) RegisterRoutes(mux *http.ServeMux) {
	// Analysis endpoints
	mux.HandleFunc("POST /v1/migration/analyze", h.handleAnalyze)
	mux.HandleFunc("POST /v1/migration/convert/schema", h.handleConvertSchema)
	mux.HandleFunc("POST /v1/migration/convert/config", h.handleConvertConfig)
	mux.HandleFunc("POST /v1/migration/full", h.handleFullMigration)

	// Plan management
	mux.HandleFunc("GET /v1/migration/plans", h.handleListPlans)
	mux.HandleFunc("POST /v1/migration/plans", h.handleCreatePlan)
	mux.HandleFunc("GET /v1/migration/plans/{id}", h.handleGetPlan)
	mux.HandleFunc("DELETE /v1/migration/plans/{id}", h.handleDeletePlan)

	// Job management
	mux.HandleFunc("GET /v1/migration/jobs", h.handleListJobs)
	mux.HandleFunc("GET /v1/migration/jobs/{id}", h.handleGetJob)

	// Stats
	mux.HandleFunc("GET /v1/migration/stats", h.handleStats)
}

// AnalyzeRequest represents a request to analyze a Feast project.
type AnalyzeRequest struct {
	Project *migration.FeastProject `json:"project"`
}

func (h *MigrationHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}

	if req.Project == nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "project is required"))
		return
	}

	report, err := h.manager.AnalyzeProject(req.Project)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("ANALYSIS_FAILED", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(report))
}

func (h *MigrationHandler) handleConvertSchema(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}

	if req.Project == nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "project is required"))
		return
	}

	result, err := h.manager.ConvertSchema(req.Project)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("CONVERSION_FAILED", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(result))
}

func (h *MigrationHandler) handleConvertConfig(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}

	if req.Project == nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "project is required"))
		return
	}

	result, err := h.manager.ConvertConfig(req.Project)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("CONVERSION_FAILED", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(result))
}

func (h *MigrationHandler) handleFullMigration(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}

	if req.Project == nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "project is required"))
		return
	}

	result, err := h.manager.RunFullMigration(req.Project)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("MIGRATION_FAILED", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(result))
}

func (h *MigrationHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.manager.ListPlans()
	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(plans))
}

// CreatePlanRequest represents a request to create a migration plan.
type CreatePlanRequest struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	SourceType   string                  `json:"source_type"`
	TargetGroups []string                `json:"target_groups,omitempty"`
	FieldMapping *migration.FieldMapping `json:"field_mapping,omitempty"`
}

func (h *MigrationHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req CreatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", err.Error()))
		return
	}

	if req.ID == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "id is required"))
		return
	}

	fieldMapping := req.FieldMapping
	if fieldMapping == nil {
		fieldMapping = migration.NewFieldMapping()
	}

	plan := &migration.MigrationPlan{
		ID:           req.ID,
		Name:         req.Name,
		SourceType:   req.SourceType,
		TargetGroups: req.TargetGroups,
		FieldMapping: fieldMapping,
	}

	if err := h.manager.CreatePlan(plan); err != nil {
		h.writeJSON(r.Context(), w, http.StatusConflict, domain.NewErrorResponse("PLAN_EXISTS", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, domain.NewSuccessResponse(plan))
}

func (h *MigrationHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "plan id is required"))
		return
	}

	plan, err := h.manager.GetPlan(id)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, domain.NewErrorResponse("NOT_FOUND", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(plan))
}

func (h *MigrationHandler) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "plan id is required"))
		return
	}

	if err := h.manager.DeletePlan(id); err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, domain.NewErrorResponse("NOT_FOUND", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(map[string]string{"status": "deleted"}))
}

func (h *MigrationHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.manager.ListJobs()
	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(jobs))
}

func (h *MigrationHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeJSON(r.Context(), w, http.StatusBadRequest, domain.NewErrorResponse("INVALID_REQUEST", "job id is required"))
		return
	}

	job, err := h.manager.GetJob(id)
	if err != nil {
		h.writeJSON(r.Context(), w, http.StatusNotFound, domain.NewErrorResponse("NOT_FOUND", err.Error()))
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(job))
}

func (h *MigrationHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, domain.NewSuccessResponse(stats))
}

func (h *MigrationHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
