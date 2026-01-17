package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/feather-store/feather/internal/integrations/airflow"
	"github.com/feather-store/feather/internal/integrations/kubeflow"
	"github.com/feather-store/feather/internal/integrations/mlflow"
)

// MLIntegrationsHandler handles MLflow, Kubeflow, and Airflow integration endpoints.
type MLIntegrationsHandler struct {
	mlflow   *mlflow.Tracker
	kubeflow *kubeflow.Manager
	airflow  *airflow.Provider
}

// NewMLIntegrationsHandler creates a new ML integrations handler.
func NewMLIntegrationsHandler(ml *mlflow.Tracker, kf *kubeflow.Manager, af *airflow.Provider) *MLIntegrationsHandler {
	return &MLIntegrationsHandler{
		mlflow:   ml,
		kubeflow: kf,
		airflow:  af,
	}
}

// RegisterRoutes registers all ML integration routes.
func (h *MLIntegrationsHandler) RegisterRoutes(mux *http.ServeMux) {
	// MLflow routes
	mux.HandleFunc("POST /v1/integrations/mlflow/runs", h.handleMLflowStartRun)
	mux.HandleFunc("GET /v1/integrations/mlflow/runs", h.handleMLflowListRuns)
	mux.HandleFunc("GET /v1/integrations/mlflow/runs/{id}", h.handleMLflowGetRun)
	mux.HandleFunc("POST /v1/integrations/mlflow/runs/{id}/end", h.handleMLflowEndRun)
	mux.HandleFunc("POST /v1/integrations/mlflow/runs/{id}/features", h.handleMLflowLogFeatures)
	mux.HandleFunc("POST /v1/integrations/mlflow/runs/{id}/metrics", h.handleMLflowLogMetrics)
	mux.HandleFunc("GET /v1/integrations/mlflow/lineage/{featureId}", h.handleMLflowGetLineage)
	mux.HandleFunc("GET /v1/integrations/mlflow/stats", h.handleMLflowStats)

	// Kubeflow routes
	mux.HandleFunc("POST /v1/integrations/kubeflow/components", h.handleKubeflowRegisterComponent)
	mux.HandleFunc("GET /v1/integrations/kubeflow/components", h.handleKubeflowListComponents)
	mux.HandleFunc("POST /v1/integrations/kubeflow/runs", h.handleKubeflowCreateRun)
	mux.HandleFunc("GET /v1/integrations/kubeflow/runs", h.handleKubeflowListRuns)
	mux.HandleFunc("GET /v1/integrations/kubeflow/runs/{id}", h.handleKubeflowGetRun)
	mux.HandleFunc("GET /v1/integrations/kubeflow/stats", h.handleKubeflowStats)

	// Airflow routes
	mux.HandleFunc("POST /v1/integrations/airflow/operators", h.handleAirflowRegisterOperator)
	mux.HandleFunc("GET /v1/integrations/airflow/operators", h.handleAirflowListOperators)
	mux.HandleFunc("GET /v1/integrations/airflow/operators/{id}", h.handleAirflowGetOperator)
	mux.HandleFunc("POST /v1/integrations/airflow/operators/{id}/enable", h.handleAirflowEnableOperator)
	mux.HandleFunc("POST /v1/integrations/airflow/operators/{id}/disable", h.handleAirflowDisableOperator)
	mux.HandleFunc("GET /v1/integrations/airflow/freshness/{featureId}", h.handleAirflowCheckFreshness)
	mux.HandleFunc("GET /v1/integrations/airflow/stats", h.handleAirflowStats)
}

// --- MLflow handlers ---

func (h *MLIntegrationsHandler) handleMLflowStartRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		ExperimentID string `json:"experiment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	run, err := h.mlflow.StartRun(req.Name, req.ExperimentID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, run)
}

func (h *MLIntegrationsHandler) handleMLflowListRuns(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.mlflow.ListRuns())
}

func (h *MLIntegrationsHandler) handleMLflowGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.mlflow.GetRun(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, run)
}

func (h *MLIntegrationsHandler) handleMLflowEndRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.mlflow.EndRun(id, req.Status); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MLIntegrationsHandler) handleMLflowLogFeatures(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		FeatureIDs []string `json:"feature_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.mlflow.LogFeatureUsage(id, req.FeatureIDs); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MLIntegrationsHandler) handleMLflowLogMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Metrics map[string]float64 `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.mlflow.LogMetrics(id, req.Metrics); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MLIntegrationsHandler) handleMLflowGetLineage(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	writeJSONResponse(r.Context(), w, http.StatusOK, h.mlflow.GetLineage(featureID))
}

func (h *MLIntegrationsHandler) handleMLflowStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.mlflow.Stats())
}

// --- Kubeflow handlers ---

func (h *MLIntegrationsHandler) handleKubeflowRegisterComponent(w http.ResponseWriter, r *http.Request) {
	var comp kubeflow.Component
	if err := json.NewDecoder(r.Body).Decode(&comp); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.kubeflow.RegisterComponent(&comp); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, comp)
}

func (h *MLIntegrationsHandler) handleKubeflowListComponents(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.kubeflow.ListComponents())
}

func (h *MLIntegrationsHandler) handleKubeflowCreateRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		ComponentIDs []string `json:"component_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	run, err := h.kubeflow.CreatePipelineRun(req.Name, req.ComponentIDs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, run)
}

func (h *MLIntegrationsHandler) handleKubeflowListRuns(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.kubeflow.ListPipelineRuns())
}

func (h *MLIntegrationsHandler) handleKubeflowGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.kubeflow.GetPipelineRun(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, run)
}

func (h *MLIntegrationsHandler) handleKubeflowStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.kubeflow.Stats())
}

// --- Airflow handlers ---

func (h *MLIntegrationsHandler) handleAirflowRegisterOperator(w http.ResponseWriter, r *http.Request) {
	var op airflow.DAGOperator
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.airflow.RegisterOperator(&op); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, op)
}

func (h *MLIntegrationsHandler) handleAirflowListOperators(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.airflow.ListOperators())
}

func (h *MLIntegrationsHandler) handleAirflowGetOperator(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	op, err := h.airflow.GetOperator(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, op)
}

func (h *MLIntegrationsHandler) handleAirflowEnableOperator(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.airflow.EnableOperator(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MLIntegrationsHandler) handleAirflowDisableOperator(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.airflow.DisableOperator(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MLIntegrationsHandler) handleAirflowCheckFreshness(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("featureId")
	result, err := h.airflow.CheckFreshness(featureID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *MLIntegrationsHandler) handleAirflowStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.airflow.Stats())
}
