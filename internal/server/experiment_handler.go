package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/experiment"
)

// ExperimentHandler handles experiment API requests.
type ExperimentHandler struct {
	engine *experiment.Engine
}

// NewExperimentHandler creates a new experiment handler.
func NewExperimentHandler(engine *experiment.Engine) *ExperimentHandler {
	return &ExperimentHandler{
		engine: engine,
	}
}

// RegisterRoutes registers experiment API routes.
func (h *ExperimentHandler) RegisterRoutes(mux *http.ServeMux) {
	// Experiment management
	mux.HandleFunc("GET /v1/experiments", h.handleListExperiments)
	mux.HandleFunc("GET /v1/experiments/active", h.handleListActiveExperiments)
	mux.HandleFunc("GET /v1/experiments/{id}", h.handleGetExperiment)
	mux.HandleFunc("POST /v1/experiments", h.handleCreateExperiment)
	mux.HandleFunc("PUT /v1/experiments/{id}", h.handleUpdateExperiment)

	// Experiment lifecycle
	mux.HandleFunc("POST /v1/experiments/{id}/start", h.handleStartExperiment)
	mux.HandleFunc("POST /v1/experiments/{id}/pause", h.handlePauseExperiment)
	mux.HandleFunc("POST /v1/experiments/{id}/stop", h.handleStopExperiment)

	// Assignment and feature flags
	mux.HandleFunc("POST /v1/experiments/{id}/assign", h.handleGetAssignment)
	mux.HandleFunc("POST /v1/features/value", h.handleGetFeatureValue)

	// Tracking
	mux.HandleFunc("POST /v1/experiments/exposure", h.handleTrackExposure)
	mux.HandleFunc("POST /v1/experiments/metric", h.handleTrackMetric)

	// Analysis
	mux.HandleFunc("GET /v1/experiments/{id}/results", h.handleAnalyzeExperiment)
	mux.HandleFunc("GET /v1/features/{featureId}/experiments", h.handleGetExperimentsByFeature)
}

// ExperimentJSON represents an experiment in JSON format.
type ExperimentJSON struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Type           string                 `json:"type"`
	Status         string                 `json:"status"`
	FeatureID      string                 `json:"feature_id,omitempty"`
	Hypothesis     string                 `json:"hypothesis,omitempty"`
	Variants       []VariantJSON          `json:"variants"`
	TargetingRules []TargetingRuleJSON    `json:"targeting_rules,omitempty"`
	Allocation     AllocationConfigJSON   `json:"allocation"`
	Metrics        []MetricConfigJSON     `json:"metrics,omitempty"`
	Schedule       *ScheduleConfigJSON    `json:"schedule,omitempty"`
	Owner          string                 `json:"owner,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      string                 `json:"created_at,omitempty"`
	UpdatedAt      string                 `json:"updated_at,omitempty"`
	StartedAt      string                 `json:"started_at,omitempty"`
	EndedAt        string                 `json:"ended_at,omitempty"`
}

// VariantJSON represents a variant in JSON format.
type VariantJSON struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	IsControl   bool                   `json:"is_control,omitempty"`
	Weight      float64                `json:"weight"`
	Value       interface{}            `json:"value,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// TargetingRuleJSON represents a targeting rule in JSON format.
type TargetingRuleJSON struct {
	ID        string      `json:"id"`
	Attribute string      `json:"attribute"`
	Operator  string      `json:"operator"`
	Value     interface{} `json:"value"`
	Negate    bool        `json:"negate,omitempty"`
}

// AllocationConfigJSON represents allocation config in JSON format.
type AllocationConfigJSON struct {
	Strategy   string  `json:"strategy"`
	Percentage float64 `json:"percentage"`
	Salt       string  `json:"salt,omitempty"`
}

// MetricConfigJSON represents a metric config in JSON format.
type MetricConfigJSON struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Type            string               `json:"type"`
	Query           string               `json:"query,omitempty"`
	SuccessCriteria *SuccessCriteriaJSON `json:"success_criteria,omitempty"`
}

// SuccessCriteriaJSON represents success criteria in JSON format.
type SuccessCriteriaJSON struct {
	MinLift       float64 `json:"min_lift,omitempty"`
	MaxPValue     float64 `json:"max_p_value,omitempty"`
	MinSampleSize int     `json:"min_sample_size,omitempty"`
	Direction     string  `json:"direction,omitempty"`
}

// ScheduleConfigJSON represents schedule config in JSON format.
type ScheduleConfigJSON struct {
	StartTime    string `json:"start_time,omitempty"`
	EndTime      string `json:"end_time,omitempty"`
	RampUpPeriod string `json:"ramp_up_period,omitempty"`
	MaxDuration  string `json:"max_duration,omitempty"`
}

// AssignmentJSON represents an assignment in JSON format.
type AssignmentJSON struct {
	ExperimentID string `json:"experiment_id"`
	VariantID    string `json:"variant_id,omitempty"`
	UserID       string `json:"user_id"`
	Timestamp    string `json:"timestamp"`
	InExperiment bool   `json:"in_experiment"`
}

// handleListExperiments handles GET /v1/experiments
func (h *ExperimentHandler) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experiments := h.engine.ListExperiments()
	response := make([]ExperimentJSON, len(experiments))

	for i, exp := range experiments {
		response[i] = h.experimentToJSON(exp)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"experiments": response,
		"count":       len(response),
	})
}

// handleListActiveExperiments handles GET /v1/experiments/active
func (h *ExperimentHandler) handleListActiveExperiments(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experiments := h.engine.GetActiveExperiments()
	response := make([]ExperimentJSON, len(experiments))

	for i, exp := range experiments {
		response[i] = h.experimentToJSON(exp)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"experiments": response,
		"count":       len(response),
	})
}

// handleGetExperiment handles GET /v1/experiments/{id}
func (h *ExperimentHandler) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	exp, err := h.engine.GetExperiment(experimentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.experimentToJSON(exp))
}

// handleCreateExperiment handles POST /v1/experiments
func (h *ExperimentHandler) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	var req ExperimentJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	exp := h.jsonToExperiment(&req)

	if err := h.engine.CreateExperiment(exp); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":       true,
		"experiment_id": exp.ID,
	})
}

// handleUpdateExperiment handles PUT /v1/experiments/{id}
func (h *ExperimentHandler) handleUpdateExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	var req ExperimentJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = experimentID
	exp := h.jsonToExperiment(&req)

	if err := h.engine.UpdateExperiment(exp); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleStartExperiment handles POST /v1/experiments/{id}/start
func (h *ExperimentHandler) handleStartExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	if err := h.engine.StartExperiment(experimentID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  "running",
	})
}

// handlePauseExperiment handles POST /v1/experiments/{id}/pause
func (h *ExperimentHandler) handlePauseExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	if err := h.engine.PauseExperiment(experimentID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  "paused",
	})
}

// handleStopExperiment handles POST /v1/experiments/{id}/stop
func (h *ExperimentHandler) handleStopExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	var req struct {
		Completed bool `json:"completed"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.engine.StopExperiment(experimentID, req.Completed); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status := "aborted"
	if req.Completed {
		status = "completed"
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// AssignmentRequest represents an assignment request.
type AssignmentRequest struct {
	UserID     string                 `json:"user_id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// handleGetAssignment handles POST /v1/experiments/{id}/assign
func (h *ExperimentHandler) handleGetAssignment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	var req AssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	assignment, err := h.engine.GetAssignment(r.Context(), experimentID, req.UserID, req.Attributes)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, AssignmentJSON{
		ExperimentID: assignment.ExperimentID,
		VariantID:    assignment.VariantID,
		UserID:       assignment.UserID,
		Timestamp:    assignment.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		InExperiment: assignment.InExperiment,
	})
}

// FeatureValueRequest represents a feature value request.
type FeatureValueRequest struct {
	FeatureID    string                 `json:"feature_id"`
	UserID       string                 `json:"user_id"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
	DefaultValue interface{}            `json:"default_value"`
}

// handleGetFeatureValue handles POST /v1/features/value
func (h *ExperimentHandler) handleGetFeatureValue(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	var req FeatureValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" || req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "feature_id and user_id are required")
		return
	}

	value, assignment, err := h.engine.GetFeatureValue(r.Context(), req.FeatureID, req.UserID, req.Attributes, req.DefaultValue)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"feature_id": req.FeatureID,
		"user_id":    req.UserID,
		"value":      value,
	}

	if assignment != nil {
		response["assignment"] = AssignmentJSON{
			ExperimentID: assignment.ExperimentID,
			VariantID:    assignment.VariantID,
			UserID:       assignment.UserID,
			Timestamp:    assignment.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			InExperiment: assignment.InExperiment,
		}
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ExposureEventJSON represents an exposure event in JSON format.
type ExposureEventJSON struct {
	ExperimentID string                 `json:"experiment_id"`
	VariantID    string                 `json:"variant_id"`
	UserID       string                 `json:"user_id"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// handleTrackExposure handles POST /v1/experiments/exposure
func (h *ExperimentHandler) handleTrackExposure(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	var req ExposureEventJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ExperimentID == "" || req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment_id and user_id are required")
		return
	}

	event := &experiment.ExposureEvent{
		ExperimentID: req.ExperimentID,
		VariantID:    req.VariantID,
		UserID:       req.UserID,
		Context:      req.Context,
	}

	h.engine.TrackExposure(event)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
	})
}

// MetricEventJSON represents a metric event in JSON format.
type MetricEventJSON struct {
	ExperimentID string                 `json:"experiment_id"`
	MetricID     string                 `json:"metric_id"`
	UserID       string                 `json:"user_id"`
	VariantID    string                 `json:"variant_id"`
	Value        float64                `json:"value"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// handleTrackMetric handles POST /v1/experiments/metric
func (h *ExperimentHandler) handleTrackMetric(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	var req MetricEventJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ExperimentID == "" || req.MetricID == "" || req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment_id, metric_id, and user_id are required")
		return
	}

	event := &experiment.MetricEvent{
		ExperimentID: req.ExperimentID,
		MetricID:     req.MetricID,
		UserID:       req.UserID,
		VariantID:    req.VariantID,
		Value:        req.Value,
		Properties:   req.Properties,
	}

	h.engine.TrackMetric(event)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
	})
}

// handleAnalyzeExperiment handles GET /v1/experiments/{id}/results
func (h *ExperimentHandler) handleAnalyzeExperiment(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	experimentID := r.PathValue("id")
	if experimentID == "" {
		h.writeError(w, http.StatusBadRequest, "experiment ID required")
		return
	}

	results, err := h.engine.AnalyzeExperiment(experimentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, results)
}

// handleGetExperimentsByFeature handles GET /v1/experiments/feature/{featureId}
func (h *ExperimentHandler) handleGetExperimentsByFeature(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(w, http.StatusServiceUnavailable, "experiment engine not configured")
		return
	}

	featureID := r.PathValue("featureId")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	experiments := h.engine.GetExperimentsByFeature(featureID)
	response := make([]ExperimentJSON, len(experiments))

	for i, exp := range experiments {
		response[i] = h.experimentToJSON(exp)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"feature_id":  featureID,
		"experiments": response,
		"count":       len(response),
	})
}

func (h *ExperimentHandler) experimentToJSON(exp *experiment.Experiment) ExperimentJSON {
	j := ExperimentJSON{
		ID:          exp.ID,
		Name:        exp.Name,
		Description: exp.Description,
		Type:        string(exp.Type),
		Status:      string(exp.Status),
		FeatureID:   exp.FeatureID,
		Hypothesis:  exp.Hypothesis,
		Variants:    make([]VariantJSON, len(exp.Variants)),
		Allocation: AllocationConfigJSON{
			Strategy:   string(exp.Allocation.Strategy),
			Percentage: exp.Allocation.Percentage,
			Salt:       exp.Allocation.Salt,
		},
		Metrics:   make([]MetricConfigJSON, len(exp.Metrics)),
		Owner:     exp.Owner,
		Tags:      exp.Tags,
		Metadata:  exp.Metadata,
		CreatedAt: exp.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: exp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	for i, v := range exp.Variants {
		j.Variants[i] = VariantJSON{
			ID:          v.ID,
			Name:        v.Name,
			Description: v.Description,
			IsControl:   v.IsControl,
			Weight:      v.Weight,
			Value:       v.Value,
			Config:      v.Config,
		}
	}

	for i, rule := range exp.TargetingRules {
		j.TargetingRules = append(j.TargetingRules, TargetingRuleJSON{
			ID:        rule.ID,
			Attribute: rule.Attribute,
			Operator:  rule.Operator,
			Value:     rule.Value,
			Negate:    rule.Negate,
		})
		_ = i
	}

	for i, m := range exp.Metrics {
		j.Metrics[i] = MetricConfigJSON{
			ID:    m.ID,
			Name:  m.Name,
			Type:  m.Type,
			Query: m.Query,
		}
		if m.SuccessCriteria != nil {
			j.Metrics[i].SuccessCriteria = &SuccessCriteriaJSON{
				MinLift:       m.SuccessCriteria.MinLift,
				MaxPValue:     m.SuccessCriteria.MaxPValue,
				MinSampleSize: m.SuccessCriteria.MinSampleSize,
				Direction:     m.SuccessCriteria.Direction,
			}
		}
	}

	if exp.StartedAt != nil {
		j.StartedAt = exp.StartedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if exp.EndedAt != nil {
		j.EndedAt = exp.EndedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return j
}

func (h *ExperimentHandler) jsonToExperiment(j *ExperimentJSON) *experiment.Experiment {
	exp := &experiment.Experiment{
		ID:          j.ID,
		Name:        j.Name,
		Description: j.Description,
		Type:        experiment.ExperimentType(j.Type),
		FeatureID:   j.FeatureID,
		Hypothesis:  j.Hypothesis,
		Variants:    make([]experiment.Variant, len(j.Variants)),
		Allocation: experiment.AllocationConfig{
			Strategy:   experiment.AllocationStrategy(j.Allocation.Strategy),
			Percentage: j.Allocation.Percentage,
			Salt:       j.Allocation.Salt,
		},
		Metrics:  make([]experiment.MetricConfig, len(j.Metrics)),
		Owner:    j.Owner,
		Tags:     j.Tags,
		Metadata: j.Metadata,
	}

	for i, v := range j.Variants {
		exp.Variants[i] = experiment.Variant{
			ID:          v.ID,
			Name:        v.Name,
			Description: v.Description,
			IsControl:   v.IsControl,
			Weight:      v.Weight,
			Value:       v.Value,
			Config:      v.Config,
		}
	}

	for _, rule := range j.TargetingRules {
		exp.TargetingRules = append(exp.TargetingRules, experiment.TargetingRule{
			ID:        rule.ID,
			Attribute: rule.Attribute,
			Operator:  rule.Operator,
			Value:     rule.Value,
			Negate:    rule.Negate,
		})
	}

	for i, m := range j.Metrics {
		exp.Metrics[i] = experiment.MetricConfig{
			ID:    m.ID,
			Name:  m.Name,
			Type:  m.Type,
			Query: m.Query,
		}
		if m.SuccessCriteria != nil {
			exp.Metrics[i].SuccessCriteria = &experiment.SuccessCriteria{
				MinLift:       m.SuccessCriteria.MinLift,
				MaxPValue:     m.SuccessCriteria.MaxPValue,
				MinSampleSize: m.SuccessCriteria.MinSampleSize,
				Direction:     m.SuccessCriteria.Direction,
			}
		}
	}

	return exp
}

func (h *ExperimentHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(w, status, data)
}

func (h *ExperimentHandler) writeError(w http.ResponseWriter, status int, message string) {
	writeJSONError(w, status, message)
}
