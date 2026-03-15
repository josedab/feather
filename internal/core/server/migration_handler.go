package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/logging"
	"github.com/feather-store/feather/internal/platform/migration"
)

// MigrationHandler handles Feast to Feather migration API requests.
type MigrationHandler struct {
	manager     *migration.Manager
	requireAuth func(http.Handler) http.Handler
}

// NewMigrationHandler creates a new migration handler.
func NewMigrationHandler(manager *migration.Manager) *MigrationHandler {
	return &MigrationHandler{
		manager: manager,
	}
}

// RegisterRoutes registers migration routes with the given mux.
func (h *MigrationHandler) RegisterRoutes(mux *http.ServeMux) {
	wrap := h.requireAuth
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	// Analysis endpoints
	mux.Handle("POST /v1/migration/analyze", wrap(http.HandlerFunc(h.handleAnalyze)))
	mux.Handle("POST /v1/migration/convert/schema", wrap(http.HandlerFunc(h.handleConvertSchema)))
	mux.Handle("POST /v1/migration/convert/config", wrap(http.HandlerFunc(h.handleConvertConfig)))
	mux.Handle("POST /v1/migration/full", wrap(http.HandlerFunc(h.handleFullMigration)))

	// Plan management
	mux.Handle("GET /v1/migration/plans", wrap(http.HandlerFunc(h.handleListPlans)))
	mux.Handle("POST /v1/migration/plans", wrap(http.HandlerFunc(h.handleCreatePlan)))
	mux.Handle("GET /v1/migration/plans/{id}", wrap(http.HandlerFunc(h.handleGetPlan)))
	mux.Handle("DELETE /v1/migration/plans/{id}", wrap(http.HandlerFunc(h.handleDeletePlan)))

	// Job management
	mux.Handle("GET /v1/migration/jobs", wrap(http.HandlerFunc(h.handleListJobs)))
	mux.Handle("GET /v1/migration/jobs/{id}", wrap(http.HandlerFunc(h.handleGetJob)))

	// Stats
	mux.Handle("GET /v1/migration/stats", wrap(http.HandlerFunc(h.handleStats)))
}

// AnalyzeRequest represents a request to analyze a Feast project.
type AnalyzeRequest struct {
	Project *migration.FeastProject `json:"project"`
}

func (h *MigrationHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &req); err != nil {
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
	if err := strictDecode(r.Body, &req); err != nil {
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
		logging.FromContext(ctx, nil).Error("failed to encode JSON response", "error", err)
	}
}
