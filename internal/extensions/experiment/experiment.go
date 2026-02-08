// Package experiment provides feature experiment management.
package experiment

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ExperimentType defines the type of experiment.
type ExperimentType string //nolint:revive

// ExperimentType constants.
const (
	ExperimentTypeABTest       ExperimentType = "ab_test"
	ExperimentTypeMultivariate ExperimentType = "multivariate"
	ExperimentTypeFeatureFlag  ExperimentType = "feature_flag"
	ExperimentTypeBandit       ExperimentType = "bandit"
)

// ExperimentStatus defines the status of an experiment.
type ExperimentStatus string //nolint:revive

// ExperimentStatus constants.
const (
	StatusDraft     ExperimentStatus = "draft"
	StatusRunning   ExperimentStatus = "running"
	StatusPaused    ExperimentStatus = "paused"
	StatusCompleted ExperimentStatus = "completed"
	StatusAborted   ExperimentStatus = "aborted"
)

// AllocationStrategy defines how users are assigned to variants.
type AllocationStrategy string

// AllocationStrategy constants.
const (
	AllocationRandom        AllocationStrategy = "random"
	AllocationDeterministic AllocationStrategy = "deterministic"
	AllocationSticky        AllocationStrategy = "sticky"
)

// Experiment represents a feature experiment.
type Experiment struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Type           ExperimentType         `json:"type"`
	Status         ExperimentStatus       `json:"status"`
	FeatureID      string                 `json:"feature_id"`
	Hypothesis     string                 `json:"hypothesis,omitempty"`
	Variants       []Variant              `json:"variants"`
	TargetingRules []TargetingRule        `json:"targeting_rules,omitempty"`
	Allocation     AllocationConfig       `json:"allocation"`
	Metrics        []MetricConfig         `json:"metrics"`
	Schedule       *ScheduleConfig        `json:"schedule,omitempty"`
	Owner          string                 `json:"owner,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	EndedAt        *time.Time             `json:"ended_at,omitempty"`
}

// Variant represents an experiment variant.
type Variant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	IsControl   bool                   `json:"is_control"`
	Weight      float64                `json:"weight"`
	Value       interface{}            `json:"value"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// TargetingRule defines who should be included in the experiment.
type TargetingRule struct {
	ID        string      `json:"id"`
	Attribute string      `json:"attribute"`
	Operator  string      `json:"operator"`
	Value     interface{} `json:"value"`
	Negate    bool        `json:"negate,omitempty"`
}

// AllocationConfig defines how traffic is allocated.
type AllocationConfig struct {
	Strategy   AllocationStrategy `json:"strategy"`
	Percentage float64            `json:"percentage"`
	Salt       string             `json:"salt,omitempty"`
}

// MetricConfig defines a metric to track.
type MetricConfig struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	Query           string           `json:"query,omitempty"`
	SuccessCriteria *SuccessCriteria `json:"success_criteria,omitempty"`
}

// SuccessCriteria defines when a metric is considered successful.
type SuccessCriteria struct {
	MinLift       float64 `json:"min_lift,omitempty"`
	MaxPValue     float64 `json:"max_p_value,omitempty"`
	MinSampleSize int     `json:"min_sample_size,omitempty"`
	Direction     string  `json:"direction,omitempty"` // increase, decrease, any
}

// ScheduleConfig defines experiment scheduling.
type ScheduleConfig struct {
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	RampUpPeriod string     `json:"ramp_up_period,omitempty"`
	MaxDuration  string     `json:"max_duration,omitempty"`
}

// Assignment represents a user's variant assignment.
type Assignment struct {
	ExperimentID string    `json:"experiment_id"`
	VariantID    string    `json:"variant_id"`
	UserID       string    `json:"user_id"`
	Timestamp    time.Time `json:"timestamp"`
	InExperiment bool      `json:"in_experiment"`
}

// ExposureEvent represents when a user was exposed to a variant.
type ExposureEvent struct {
	ExperimentID string                 `json:"experiment_id"`
	VariantID    string                 `json:"variant_id"`
	UserID       string                 `json:"user_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// MetricEvent represents a metric observation.
type MetricEvent struct {
	ExperimentID string                 `json:"experiment_id"`
	MetricID     string                 `json:"metric_id"`
	UserID       string                 `json:"user_id"`
	VariantID    string                 `json:"variant_id"`
	Value        float64                `json:"value"`
	Timestamp    time.Time              `json:"timestamp"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// ExperimentResults holds analysis results.
type ExperimentResults struct { //nolint:revive
	ExperimentID   string          `json:"experiment_id"`
	Status         string          `json:"status"`
	SampleSize     int             `json:"sample_size"`
	VariantResults []VariantResult `json:"variant_results"`
	Winner         *string         `json:"winner,omitempty"`
	Confidence     float64         `json:"confidence"`
	AnalyzedAt     time.Time       `json:"analyzed_at"`
}

// VariantResult holds results for a single variant.
type VariantResult struct {
	VariantID     string         `json:"variant_id"`
	SampleSize    int            `json:"sample_size"`
	MetricResults []MetricResult `json:"metric_results"`
}

// MetricResult holds results for a single metric.
type MetricResult struct {
	MetricID    string  `json:"metric_id"`
	Mean        float64 `json:"mean"`
	StdDev      float64 `json:"std_dev"`
	Lift        float64 `json:"lift,omitempty"`
	PValue      float64 `json:"p_value,omitempty"`
	Significant bool    `json:"significant"`
	CI95Lower   float64 `json:"ci_95_lower,omitempty"`
	CI95Upper   float64 `json:"ci_95_upper,omitempty"`
}

// Engine manages experiments.
type Engine struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	assignments map[string]map[string]*Assignment // userID -> experimentID -> Assignment
	exposures   []*ExposureEvent
	metrics     []*MetricEvent
	rng         *rand.Rand
}

// NewEngine creates a new experiment engine.
func NewEngine() *Engine {
	return &Engine{
		experiments: make(map[string]*Experiment),
		assignments: make(map[string]map[string]*Assignment),
		exposures:   make([]*ExposureEvent, 0),
		metrics:     make([]*MetricEvent, 0),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}
}

// CreateExperiment creates a new experiment.
func (e *Engine) CreateExperiment(exp *Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if exp.ID == "" {
		return errors.New("experiment ID is required")
	}

	if _, exists := e.experiments[exp.ID]; exists {
		return errors.New("experiment already exists")
	}

	// Validate variants
	if len(exp.Variants) < 2 && exp.Type != ExperimentTypeFeatureFlag {
		return errors.New("at least 2 variants required for A/B test")
	}

	// Validate weights sum to ~1.0
	totalWeight := 0.0
	hasControl := false
	for _, v := range exp.Variants {
		totalWeight += v.Weight
		if v.IsControl {
			hasControl = true
		}
	}

	if exp.Type == ExperimentTypeABTest && !hasControl {
		return errors.New("A/B test must have a control variant")
	}

	// Allow small floating point errors
	if totalWeight < 0.99 || totalWeight > 1.01 {
		return fmt.Errorf("variant weights must sum to 1.0, got %f", totalWeight)
	}

	exp.Status = StatusDraft
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()

	e.experiments[exp.ID] = exp
	return nil
}

// GetExperiment returns an experiment by ID.
func (e *Engine) GetExperiment(id string) (*Experiment, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exp, exists := e.experiments[id]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	return exp, nil
}

// UpdateExperiment updates an experiment.
func (e *Engine) UpdateExperiment(exp *Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.experiments[exp.ID]
	if !exists {
		return errors.New("experiment not found")
	}

	if existing.Status == StatusRunning {
		return errors.New("cannot update running experiment")
	}

	exp.UpdatedAt = time.Now()
	exp.CreatedAt = existing.CreatedAt
	e.experiments[exp.ID] = exp
	return nil
}

// StartExperiment starts an experiment.
func (e *Engine) StartExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if exp.Status != StatusDraft && exp.Status != StatusPaused {
		return fmt.Errorf("cannot start experiment in status %s", exp.Status)
	}

	now := time.Now()
	exp.Status = StatusRunning
	exp.StartedAt = &now
	exp.UpdatedAt = now

	return nil
}

// StopExperiment stops an experiment.
func (e *Engine) StopExperiment(id string, completed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if exp.Status != StatusRunning {
		return errors.New("experiment is not running")
	}

	now := time.Now()
	if completed {
		exp.Status = StatusCompleted
	} else {
		exp.Status = StatusAborted
	}
	exp.EndedAt = &now
	exp.UpdatedAt = now

	return nil
}

// PauseExperiment pauses an experiment.
func (e *Engine) PauseExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[id]
	if !exists {
		return errors.New("experiment not found")
	}

	if exp.Status != StatusRunning {
		return errors.New("experiment is not running")
	}

	exp.Status = StatusPaused
	exp.UpdatedAt = time.Now()

	return nil
}

// GetAssignment returns the variant assignment for a user.
func (e *Engine) GetAssignment(ctx context.Context, experimentID, userID string, attributes map[string]interface{}) (*Assignment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp, exists := e.experiments[experimentID]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	// Check if experiment is running
	if exp.Status != StatusRunning {
		return &Assignment{
			ExperimentID: experimentID,
			UserID:       userID,
			InExperiment: false,
			Timestamp:    time.Now(),
		}, nil
	}

	// Check existing assignment (sticky allocation)
	if exp.Allocation.Strategy == AllocationSticky {
		if userAssignments, ok := e.assignments[userID]; ok {
			if assignment, ok := userAssignments[experimentID]; ok {
				return assignment, nil
			}
		}
	}

	// Check targeting rules
	if !e.matchesTargeting(exp, attributes) {
		return &Assignment{
			ExperimentID: experimentID,
			UserID:       userID,
			InExperiment: false,
			Timestamp:    time.Now(),
		}, nil
	}

	// Check if user is in experiment (based on allocation percentage)
	if !e.isInExperiment(exp, userID) {
		return &Assignment{
			ExperimentID: experimentID,
			UserID:       userID,
			InExperiment: false,
			Timestamp:    time.Now(),
		}, nil
	}

	// Assign variant
	variant := e.assignVariant(exp, userID)

	assignment := &Assignment{
		ExperimentID: experimentID,
		VariantID:    variant.ID,
		UserID:       userID,
		Timestamp:    time.Now(),
		InExperiment: true,
	}

	// Store assignment for sticky allocation
	if e.assignments[userID] == nil {
		e.assignments[userID] = make(map[string]*Assignment)
	}
	e.assignments[userID][experimentID] = assignment

	return assignment, nil
}

// GetFeatureValue returns the feature value for a user based on experiment assignment.
func (e *Engine) GetFeatureValue(ctx context.Context, featureID, userID string, attributes map[string]interface{}, defaultValue interface{}) (interface{}, *Assignment, error) {
	e.mu.RLock()
	// Find experiment for this feature
	var targetExp *Experiment
	for _, exp := range e.experiments {
		if exp.FeatureID == featureID && exp.Status == StatusRunning {
			targetExp = exp
			break
		}
	}
	e.mu.RUnlock()

	if targetExp == nil {
		return defaultValue, nil, nil
	}

	assignment, err := e.GetAssignment(ctx, targetExp.ID, userID, attributes)
	if err != nil {
		return defaultValue, nil, err
	}

	if !assignment.InExperiment {
		return defaultValue, assignment, nil
	}

	// Find variant value
	for _, v := range targetExp.Variants {
		if v.ID == assignment.VariantID {
			return v.Value, assignment, nil
		}
	}

	return defaultValue, assignment, nil
}

func (e *Engine) matchesTargeting(exp *Experiment, attributes map[string]interface{}) bool {
	if len(exp.TargetingRules) == 0 {
		return true
	}

	for _, rule := range exp.TargetingRules {
		attrValue, exists := attributes[rule.Attribute]
		if !exists {
			if !rule.Negate {
				return false
			}
			continue
		}

		matches := e.evaluateRule(rule, attrValue)
		if rule.Negate {
			matches = !matches
		}
		if !matches {
			return false
		}
	}

	return true
}

func (e *Engine) evaluateRule(rule TargetingRule, value interface{}) bool {
	switch rule.Operator {
	case "eq", "equals":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", rule.Value)
	case "neq", "not_equals":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", rule.Value)
	case "contains":
		return containsValue(value, rule.Value)
	case "in":
		return inList(value, rule.Value)
	case "gt":
		return compareNumeric(value, rule.Value) > 0
	case "gte":
		return compareNumeric(value, rule.Value) >= 0
	case "lt":
		return compareNumeric(value, rule.Value) < 0
	case "lte":
		return compareNumeric(value, rule.Value) <= 0
	case "regex":
		return matchesRegex(value, rule.Value)
	default:
		return false
	}
}

func (e *Engine) isInExperiment(exp *Experiment, userID string) bool {
	hash := e.hashUser(userID, exp.Allocation.Salt)
	return hash < exp.Allocation.Percentage
}

func (e *Engine) hashUser(userID, salt string) float64 {
	h := sha256.New()
	h.Write([]byte(userID + salt))
	sum := h.Sum(nil)
	// Use first 8 bytes as uint64
	value := binary.BigEndian.Uint64(sum[:8])
	// Normalize to [0, 1)
	return float64(value) / float64(^uint64(0))
}

func (e *Engine) assignVariant(exp *Experiment, userID string) *Variant {
	switch exp.Allocation.Strategy {
	case AllocationDeterministic, AllocationSticky:
		return e.deterministicAssignment(exp, userID)
	default:
		return e.randomAssignment(exp)
	}
}

func (e *Engine) deterministicAssignment(exp *Experiment, userID string) *Variant {
	hash := e.hashUser(userID, exp.ID+exp.Allocation.Salt)

	cumulative := 0.0
	for i := range exp.Variants {
		cumulative += exp.Variants[i].Weight
		if hash < cumulative {
			return &exp.Variants[i]
		}
	}

	return &exp.Variants[len(exp.Variants)-1]
}

func (e *Engine) randomAssignment(exp *Experiment) *Variant {
	r := e.rng.Float64()

	cumulative := 0.0
	for i := range exp.Variants {
		cumulative += exp.Variants[i].Weight
		if r < cumulative {
			return &exp.Variants[i]
		}
	}

	return &exp.Variants[len(exp.Variants)-1]
}

// TrackExposure records that a user was exposed to a variant.
func (e *Engine) TrackExposure(event *ExposureEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	event.Timestamp = time.Now()
	e.exposures = append(e.exposures, event)
}

// TrackMetric records a metric event.
func (e *Engine) TrackMetric(event *MetricEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	event.Timestamp = time.Now()
	e.metrics = append(e.metrics, event)
}

// AnalyzeExperiment analyzes experiment results.
func (e *Engine) AnalyzeExperiment(experimentID string) (*ExperimentResults, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exp, exists := e.experiments[experimentID]
	if !exists {
		return nil, errors.New("experiment not found")
	}

	results := &ExperimentResults{
		ExperimentID:   experimentID,
		Status:         string(exp.Status),
		AnalyzedAt:     time.Now(),
		VariantResults: make([]VariantResult, len(exp.Variants)),
	}

	// Collect metrics by variant
	variantMetrics := make(map[string]map[string][]float64)
	for _, variant := range exp.Variants {
		variantMetrics[variant.ID] = make(map[string][]float64)
	}

	for _, event := range e.metrics {
		if event.ExperimentID != experimentID {
			continue
		}
		if _, ok := variantMetrics[event.VariantID]; ok {
			variantMetrics[event.VariantID][event.MetricID] = append(
				variantMetrics[event.VariantID][event.MetricID],
				event.Value,
			)
		}
	}

	// Find control variant
	var controlID string
	for _, v := range exp.Variants {
		if v.IsControl {
			controlID = v.ID
			break
		}
	}

	// Calculate results for each variant
	for i, variant := range exp.Variants {
		vr := VariantResult{
			VariantID:     variant.ID,
			MetricResults: make([]MetricResult, 0),
		}

		for _, metric := range exp.Metrics {
			values := variantMetrics[variant.ID][metric.ID]
			vr.SampleSize = len(values)

			mr := MetricResult{
				MetricID: metric.ID,
			}

			if len(values) > 0 {
				mr.Mean = mean(values)
				mr.StdDev = stddev(values)

				// Calculate lift vs control
				if controlID != "" && variant.ID != controlID {
					controlValues := variantMetrics[controlID][metric.ID]
					if len(controlValues) > 0 {
						controlMean := mean(controlValues)
						if controlMean != 0 {
							mr.Lift = (mr.Mean - controlMean) / controlMean * 100
						}

						// Calculate p-value using t-test approximation
						mr.PValue = calculatePValue(values, controlValues)
						mr.Significant = mr.PValue < 0.05

						// Calculate confidence interval
						mr.CI95Lower, mr.CI95Upper = confidenceInterval(values, 0.95)
					}
				}
			}

			vr.MetricResults = append(vr.MetricResults, mr)
		}

		results.VariantResults[i] = vr
		results.SampleSize += vr.SampleSize
	}

	// Determine winner
	results.Winner, results.Confidence = e.determineWinner(results, exp)

	return results, nil
}

func (e *Engine) determineWinner(results *ExperimentResults, exp *Experiment) (*string, float64) {
	if len(results.VariantResults) < 2 {
		return nil, 0
	}

	var bestVariant string
	var bestLift float64
	bestPValue := 1.0

	for _, vr := range results.VariantResults {
		// Skip control
		isControl := false
		for _, v := range exp.Variants {
			if v.ID == vr.VariantID && v.IsControl {
				isControl = true
				break
			}
		}
		if isControl {
			continue
		}

		for _, mr := range vr.MetricResults {
			if mr.Significant && mr.PValue < bestPValue {
				bestVariant = vr.VariantID
				bestLift = mr.Lift
				bestPValue = mr.PValue
			}
		}
	}

	if bestVariant == "" {
		return nil, 0
	}

	confidence := (1 - bestPValue) * 100
	_ = bestLift

	return &bestVariant, confidence
}

// ListExperiments returns all experiments.
func (e *Engine) ListExperiments() []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	experiments := make([]*Experiment, 0, len(e.experiments))
	for _, exp := range e.experiments {
		experiments = append(experiments, exp)
	}

	return experiments
}

// GetActiveExperiments returns running experiments.
func (e *Engine) GetActiveExperiments() []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	experiments := make([]*Experiment, 0)
	for _, exp := range e.experiments {
		if exp.Status == StatusRunning {
			experiments = append(experiments, exp)
		}
	}

	return experiments
}

// GetExperimentsByFeature returns experiments for a feature.
func (e *Engine) GetExperimentsByFeature(featureID string) []*Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	experiments := make([]*Experiment, 0)
	for _, exp := range e.experiments {
		if exp.FeatureID == featureID {
			experiments = append(experiments, exp)
		}
	}

	return experiments
}

// Helper functions

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	sum := 0.0
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	return sqrt(sum / float64(len(values)-1))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func calculatePValue(treatment, control []float64) float64 {
	if len(treatment) < 2 || len(control) < 2 {
		return 1.0
	}

	n1 := float64(len(treatment))
	n2 := float64(len(control))
	m1 := mean(treatment)
	m2 := mean(control)
	s1 := stddev(treatment)
	s2 := stddev(control)

	// Pooled standard error
	se := sqrt(s1*s1/n1 + s2*s2/n2)
	if se == 0 {
		return 1.0
	}

	// t-statistic
	t := (m1 - m2) / se

	// Approximate p-value using normal distribution
	// This is a simplification; real implementation would use t-distribution
	if t < 0 {
		t = -t
	}

	// Approximate two-tailed p-value
	pValue := 2 * normalCDF(-t)
	return pValue
}

func normalCDF(x float64) float64 {
	// Approximation of standard normal CDF
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = x * sign / sqrt(2.0)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}

func exp(x float64) float64 {
	// Simple exp approximation
	if x > 20 {
		return 1e9
	}
	if x < -20 {
		return 0
	}

	result := 1.0
	term := 1.0
	for i := 1; i < 20; i++ {
		term *= x / float64(i)
		result += term
	}
	return result
}

func confidenceInterval(values []float64, confidence float64) (float64, float64) {
	if len(values) < 2 {
		return 0, 0
	}

	m := mean(values)
	s := stddev(values)
	n := float64(len(values))

	// Z-score for 95% confidence
	z := 1.96

	margin := z * s / sqrt(n)
	return m - margin, m + margin
}

func containsValue(value, target interface{}) bool {
	str := fmt.Sprintf("%v", value)
	targetStr := fmt.Sprintf("%v", target)
	for i := 0; i <= len(str)-len(targetStr); i++ {
		if str[i:i+len(targetStr)] == targetStr {
			return true
		}
	}
	return false
}

func inList(value, list interface{}) bool {
	valueStr := fmt.Sprintf("%v", value)
	switch l := list.(type) {
	case []interface{}:
		for _, item := range l {
			if fmt.Sprintf("%v", item) == valueStr {
				return true
			}
		}
	case []string:
		for _, item := range l {
			if item == valueStr {
				return true
			}
		}
	}
	return false
}

func compareNumeric(a, b interface{}) int {
	aFloat := toFloat(a)
	bFloat := toFloat(b)
	if aFloat < bFloat {
		return -1
	}
	if aFloat > bFloat {
		return 1
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

func matchesRegex(value, pattern interface{}) bool {
	// Simplified regex matching - real implementation would use regexp package
	return containsValue(value, pattern)
}

// AutoDecisionConfig configures automatic experiment decisioning.
type AutoDecisionConfig struct {
	Enabled           bool    `json:"enabled"`
	MinSampleSize     int     `json:"min_sample_size"`
	MaxPValue         float64 `json:"max_p_value"`
	MinRunDuration    time.Duration `json:"min_run_duration"`
	StopOnSignificant bool    `json:"stop_on_significant"`
}

// DefaultAutoDecisionConfig returns sensible defaults.
func DefaultAutoDecisionConfig() AutoDecisionConfig {
	return AutoDecisionConfig{
		Enabled:           true,
		MinSampleSize:     100,
		MaxPValue:         0.05,
		MinRunDuration:    24 * time.Hour,
		StopOnSignificant: true,
	}
}

// AutoDecisionResult holds the result of an auto-decision check.
type AutoDecisionResult struct {
	ExperimentID    string  `json:"experiment_id"`
	ShouldComplete  bool    `json:"should_complete"`
	Winner          *string `json:"winner,omitempty"`
	Confidence      float64 `json:"confidence"`
	Reason          string  `json:"reason"`
	SampleSize      int     `json:"sample_size"`
	RunningDuration string  `json:"running_duration"`
}

// CheckAutoDecision evaluates whether an experiment should be auto-completed
// based on statistical significance and minimum sample size requirements.
func (e *Engine) CheckAutoDecision(experimentID string, config AutoDecisionConfig) (*AutoDecisionResult, error) {
	e.mu.RLock()
	exp, exists := e.experiments[experimentID]
	if !exists {
		e.mu.RUnlock()
		return nil, errors.New("experiment not found")
	}
	if exp.Status != StatusRunning {
		e.mu.RUnlock()
		return &AutoDecisionResult{
			ExperimentID:   experimentID,
			ShouldComplete: false,
			Reason:         "experiment is not running",
		}, nil
	}
	if exp.StartedAt == nil {
		e.mu.RUnlock()
		return &AutoDecisionResult{
			ExperimentID:   experimentID,
			ShouldComplete: false,
			Reason:         "experiment has no start time",
		}, nil
	}
	e.mu.RUnlock()

	runDuration := time.Since(*exp.StartedAt)
	result := &AutoDecisionResult{
		ExperimentID:    experimentID,
		RunningDuration: runDuration.String(),
	}

	if runDuration < config.MinRunDuration {
		result.Reason = fmt.Sprintf("minimum run duration not met (%.0fh < %.0fh)", runDuration.Hours(), config.MinRunDuration.Hours())
		return result, nil
	}

	analysis, err := e.AnalyzeExperiment(experimentID)
	if err != nil {
		return nil, fmt.Errorf("analyzing experiment: %w", err)
	}

	result.SampleSize = analysis.SampleSize
	if analysis.SampleSize < config.MinSampleSize {
		result.Reason = fmt.Sprintf("minimum sample size not met (%d < %d)", analysis.SampleSize, config.MinSampleSize)
		return result, nil
	}

	if analysis.Winner != nil && analysis.Confidence >= (1-config.MaxPValue)*100 {
		result.ShouldComplete = true
		result.Winner = analysis.Winner
		result.Confidence = analysis.Confidence
		result.Reason = "statistical significance reached"
	} else {
		result.Reason = "no statistically significant winner yet"
	}

	return result, nil
}

// FeatureImpact tracks the cumulative impact of experiments on a feature.
type FeatureImpact struct {
	FeatureID        string             `json:"feature_id"`
	TotalExperiments int                `json:"total_experiments"`
	CompletedCount   int                `json:"completed_count"`
	ActiveCount      int                `json:"active_count"`
	CumulativeLift   float64            `json:"cumulative_lift"`
	ExperimentImpacts []ExperimentImpact `json:"experiment_impacts"`
}

// ExperimentImpact records the impact of one experiment on a feature.
type ExperimentImpact struct {
	ExperimentID string    `json:"experiment_id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Winner       *string   `json:"winner,omitempty"`
	Lift         float64   `json:"lift"`
	Confidence   float64   `json:"confidence"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
}

// GetFeatureImpact returns the cumulative experiment impact for a feature.
func (e *Engine) GetFeatureImpact(featureID string) (*FeatureImpact, error) {
	experiments := e.GetExperimentsByFeature(featureID)
	if len(experiments) == 0 {
		return nil, fmt.Errorf("no experiments found for feature %q", featureID)
	}

	impact := &FeatureImpact{
		FeatureID:         featureID,
		TotalExperiments:  len(experiments),
		ExperimentImpacts: make([]ExperimentImpact, 0, len(experiments)),
	}

	for _, exp := range experiments {
		ei := ExperimentImpact{
			ExperimentID: exp.ID,
			Name:         exp.Name,
			Status:       string(exp.Status),
		}

		switch exp.Status {
		case StatusRunning:
			impact.ActiveCount++
		case StatusCompleted:
			impact.CompletedCount++
			if exp.EndedAt != nil {
				ei.CompletedAt = *exp.EndedAt
			}
		}

		analysis, err := e.AnalyzeExperiment(exp.ID)
		if err == nil && analysis.Winner != nil {
			ei.Winner = analysis.Winner
			ei.Confidence = analysis.Confidence
			for _, vr := range analysis.VariantResults {
				if vr.VariantID == *analysis.Winner {
					for _, mr := range vr.MetricResults {
						if mr.Significant {
							ei.Lift = mr.Lift
							impact.CumulativeLift += mr.Lift
							break
						}
					}
					break
				}
			}
		}

		impact.ExperimentImpacts = append(impact.ExperimentImpacts, ei)
	}

	return impact, nil
}

// GetExperimentSummary returns a high-level summary of all experiments.
func (e *Engine) GetExperimentSummary() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	total := len(e.experiments)
	byStatus := make(map[string]int)
	features := make(map[string]bool)

	for _, exp := range e.experiments {
		byStatus[string(exp.Status)]++
		if exp.FeatureID != "" {
			features[exp.FeatureID] = true
		}
	}

	return map[string]interface{}{
		"total_experiments":  total,
		"by_status":          byStatus,
		"features_tested":    len(features),
		"total_exposures":    len(e.exposures),
		"total_metric_events": len(e.metrics),
	}
}
