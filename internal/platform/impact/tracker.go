package impact

import (
	"sort"
	"sync"
	"time"
)

// FeatureUsage represents how a feature is being used.
type FeatureUsage struct {
	Feature      string            `json:"feature"`
	Models       []string          `json:"models"` // Models using this feature
	AccessCount  int64             `json:"access_count"`
	LastAccess   time.Time         `json:"last_access"`
	FirstAccess  time.Time         `json:"first_access"`
	AvgLatencyMs float64           `json:"avg_latency_ms"`
	ErrorCount   int64             `json:"error_count"`
	NullCount    int64             `json:"null_count"`   // Times feature returned null
	Dependencies []string          `json:"dependencies"` // Features this depends on
	Dependents   []string          `json:"dependents"`   // Features depending on this
	Tags         map[string]string `json:"tags"`
	Deprecated   bool              `json:"deprecated"`
	DeprecatedAt *time.Time        `json:"deprecated_at,omitempty"`
	DeprecatedBy string            `json:"deprecated_by,omitempty"`
}

// ModelUsage represents which features a model uses.
type ModelUsage struct {
	ModelID        string            `json:"model_id"`
	ModelVersion   string            `json:"model_version"`
	Features       []string          `json:"features"`
	Environment    string            `json:"environment"` // prod, staging, dev
	Endpoint       string            `json:"endpoint"`
	LastInference  time.Time         `json:"last_inference"`
	InferenceCount int64             `json:"inference_count"`
	AvgLatencyMs   float64           `json:"avg_latency_ms"`
	P99LatencyMs   float64           `json:"p99_latency_ms"`
	ErrorRate      float64           `json:"error_rate"`
	Metadata       map[string]string `json:"metadata"`
}

// ImpactScore measures the impact of a feature.
type ImpactScore struct { //nolint:revive
	Feature          string    `json:"feature"`
	OverallScore     float64   `json:"overall_score"`     // 0-100 overall impact
	UsageScore       float64   `json:"usage_score"`       // Based on access frequency
	ModelCoverage    float64   `json:"model_coverage"`    // % of models using this
	ReliabilityScore float64   `json:"reliability_score"` // Based on error/null rates
	LatencyScore     float64   `json:"latency_score"`     // Based on retrieval speed
	DependencyScore  float64   `json:"dependency_score"`  // Based on dependents
	CriticalPath     bool      `json:"critical_path"`     // Is it in critical model paths
	LastCalculated   time.Time `json:"last_calculated"`
}

// DeprecationRequest represents a request to deprecate a feature.
type DeprecationRequest struct {
	Feature        string    `json:"feature"`
	Reason         string    `json:"reason"`
	RequestedBy    string    `json:"requested_by"`
	RequestedAt    time.Time `json:"requested_at"`
	TargetDate     time.Time `json:"target_date"`
	Replacement    string    `json:"replacement,omitempty"`
	AffectedModels []string  `json:"affected_models"`
	Status         string    `json:"status"` // pending, approved, completed, rejected
}

// ImpactTracker tracks feature usage and impact.
type ImpactTracker struct { //nolint:revive
	featureUsage   map[string]*FeatureUsage
	modelUsage     map[string]*ModelUsage
	impactScores   map[string]*ImpactScore
	deprecations   map[string]*DeprecationRequest
	latencyWindow  []latencyRecord
	maxLatencyHist int
	mu             sync.RWMutex
}

type latencyRecord struct {
	feature   string
	latencyMs float64
	timestamp time.Time
}

// NewImpactTracker creates a new impact tracker.
func NewImpactTracker() *ImpactTracker {
	return &ImpactTracker{
		featureUsage:   make(map[string]*FeatureUsage),
		modelUsage:     make(map[string]*ModelUsage),
		impactScores:   make(map[string]*ImpactScore),
		deprecations:   make(map[string]*DeprecationRequest),
		latencyWindow:  make([]latencyRecord, 0),
		maxLatencyHist: 10000,
	}
}

// RecordAccess records a feature access.
func (t *ImpactTracker) RecordAccess(feature string, latencyMs float64, isError bool, isNull bool) {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.featureUsage[feature]
	if !ok {
		usage = &FeatureUsage{
			Feature:      feature,
			Models:       []string{},
			FirstAccess:  now,
			Dependencies: []string{},
			Dependents:   []string{},
			Tags:         make(map[string]string),
		}
		t.featureUsage[feature] = usage
	}

	usage.AccessCount++
	usage.LastAccess = now

	if isError {
		usage.ErrorCount++
	}
	if isNull {
		usage.NullCount++
	}

	// Update running average latency
	if latencyMs > 0 {
		if usage.AvgLatencyMs == 0 {
			usage.AvgLatencyMs = latencyMs
		} else {
			// Exponential moving average
			usage.AvgLatencyMs = usage.AvgLatencyMs*0.95 + latencyMs*0.05
		}

		t.latencyWindow = append(t.latencyWindow, latencyRecord{
			feature:   feature,
			latencyMs: latencyMs,
			timestamp: now,
		})
		if len(t.latencyWindow) > t.maxLatencyHist {
			t.latencyWindow = t.latencyWindow[1:]
		}
	}
}

// RegisterModel registers a model and its feature dependencies.
func (t *ImpactTracker) RegisterModel(model *ModelUsage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modelUsage[model.ModelID] = model

	// Update feature->model mappings
	for _, feature := range model.Features {
		usage, ok := t.featureUsage[feature]
		if !ok {
			usage = &FeatureUsage{
				Feature:      feature,
				Models:       []string{},
				Dependencies: []string{},
				Dependents:   []string{},
				Tags:         make(map[string]string),
			}
			t.featureUsage[feature] = usage
		}

		// Add model if not already present
		found := false
		for _, m := range usage.Models {
			if m == model.ModelID {
				found = true
				break
			}
		}
		if !found {
			usage.Models = append(usage.Models, model.ModelID)
		}
	}
}

// RecordInference records a model inference.
func (t *ImpactTracker) RecordInference(modelID string, latencyMs float64, isError bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	model, ok := t.modelUsage[modelID]
	if !ok {
		return
	}

	model.InferenceCount++
	model.LastInference = time.Now()

	// Update average latency
	if latencyMs > 0 {
		if model.AvgLatencyMs == 0 {
			model.AvgLatencyMs = latencyMs
		} else {
			model.AvgLatencyMs = model.AvgLatencyMs*0.95 + latencyMs*0.05
		}
	}

	// Update error rate
	if isError {
		model.ErrorRate = (model.ErrorRate*float64(model.InferenceCount-1) + 1) / float64(model.InferenceCount)
	} else {
		model.ErrorRate = model.ErrorRate * float64(model.InferenceCount-1) / float64(model.InferenceCount)
	}
}

// SetDependencies sets the dependencies for a feature.
func (t *ImpactTracker) SetDependencies(feature string, dependencies []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.featureUsage[feature]
	if !ok {
		usage = &FeatureUsage{
			Feature:      feature,
			Models:       []string{},
			Dependencies: []string{},
			Dependents:   []string{},
			Tags:         make(map[string]string),
		}
		t.featureUsage[feature] = usage
	}

	usage.Dependencies = dependencies

	// Update dependents in dependency features
	for _, dep := range dependencies {
		depUsage, ok := t.featureUsage[dep]
		if !ok {
			depUsage = &FeatureUsage{
				Feature:      dep,
				Models:       []string{},
				Dependencies: []string{},
				Dependents:   []string{},
				Tags:         make(map[string]string),
			}
			t.featureUsage[dep] = depUsage
		}

		// Add as dependent if not already present
		found := false
		for _, d := range depUsage.Dependents {
			if d == feature {
				found = true
				break
			}
		}
		if !found {
			depUsage.Dependents = append(depUsage.Dependents, feature)
		}
	}
}

// GetFeatureUsage returns usage stats for a feature.
func (t *ImpactTracker) GetFeatureUsage(feature string) *FeatureUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.featureUsage[feature]
}

// GetAllFeatureUsage returns all feature usage data.
func (t *ImpactTracker) GetAllFeatureUsage() []*FeatureUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*FeatureUsage, 0, len(t.featureUsage))
	for _, u := range t.featureUsage {
		result = append(result, u)
	}
	return result
}

// GetModelUsage returns usage stats for a model.
func (t *ImpactTracker) GetModelUsage(modelID string) *ModelUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.modelUsage[modelID]
}

// GetAllModelUsage returns all model usage data.
func (t *ImpactTracker) GetAllModelUsage() []*ModelUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*ModelUsage, 0, len(t.modelUsage))
	for _, m := range t.modelUsage {
		result = append(result, m)
	}
	return result
}

// CalculateImpactScore calculates the impact score for a feature.
func (t *ImpactTracker) CalculateImpactScore(feature string) *ImpactScore {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.featureUsage[feature]
	if !ok {
		return nil
	}

	totalModels := len(t.modelUsage)
	if totalModels == 0 {
		totalModels = 1
	}

	// Usage score: logarithmic scale of access count
	usageScore := 0.0
	if usage.AccessCount > 0 {
		usageScore = minFloat64(100.0, 20.0*log10(float64(usage.AccessCount)))
	}

	// Model coverage: percentage of models using this feature
	modelCoverage := float64(len(usage.Models)) / float64(totalModels) * 100

	// Reliability score: based on error and null rates
	reliabilityScore := 100.0
	if usage.AccessCount > 0 {
		errorRate := float64(usage.ErrorCount) / float64(usage.AccessCount)
		nullRate := float64(usage.NullCount) / float64(usage.AccessCount)
		reliabilityScore = maxFloat64(0, 100.0-errorRate*100-nullRate*50)
	}

	// Latency score: lower is better, target < 10ms
	latencyScore := 100.0
	if usage.AvgLatencyMs > 0 {
		if usage.AvgLatencyMs <= 1 {
			latencyScore = 100.0
		} else if usage.AvgLatencyMs <= 10 {
			latencyScore = 90.0
		} else if usage.AvgLatencyMs <= 50 {
			latencyScore = 70.0
		} else if usage.AvgLatencyMs <= 100 {
			latencyScore = 50.0
		} else {
			latencyScore = maxFloat64(0, 100.0-usage.AvgLatencyMs)
		}
	}

	// Dependency score: based on how many features depend on this
	dependencyScore := minFloat64(100.0, float64(len(usage.Dependents))*20)

	// Check if feature is in critical path (used by prod models)
	criticalPath := false
	for _, modelID := range usage.Models {
		if model, ok := t.modelUsage[modelID]; ok {
			if model.Environment == "prod" || model.Environment == "production" {
				criticalPath = true
				break
			}
		}
	}

	// Overall score: weighted average
	overallScore := usageScore*0.25 + modelCoverage*0.25 + reliabilityScore*0.2 + latencyScore*0.15 + dependencyScore*0.15
	if criticalPath {
		overallScore = minFloat64(100, overallScore*1.2)
	}

	score := &ImpactScore{
		Feature:          feature,
		OverallScore:     overallScore,
		UsageScore:       usageScore,
		ModelCoverage:    modelCoverage,
		ReliabilityScore: reliabilityScore,
		LatencyScore:     latencyScore,
		DependencyScore:  dependencyScore,
		CriticalPath:     criticalPath,
		LastCalculated:   time.Now(),
	}

	t.impactScores[feature] = score
	return score
}

// GetImpactScore returns the cached impact score for a feature.
func (t *ImpactTracker) GetImpactScore(feature string) *ImpactScore {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.impactScores[feature]
}

// GetTopFeaturesByImpact returns the top N features by impact score.
func (t *ImpactTracker) GetTopFeaturesByImpact(n int) []*ImpactScore {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Recalculate all scores
	for feature := range t.featureUsage {
		t.calculateImpactScoreUnlocked(feature)
	}

	scores := make([]*ImpactScore, 0, len(t.impactScores))
	for _, s := range t.impactScores {
		scores = append(scores, s)
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].OverallScore > scores[j].OverallScore
	})

	if len(scores) > n {
		scores = scores[:n]
	}

	return scores
}

func (t *ImpactTracker) calculateImpactScoreUnlocked(feature string) *ImpactScore {
	usage, ok := t.featureUsage[feature]
	if !ok {
		return nil
	}

	totalModels := len(t.modelUsage)
	if totalModels == 0 {
		totalModels = 1
	}

	usageScore := 0.0
	if usage.AccessCount > 0 {
		usageScore = minFloat64(100.0, 20.0*log10(float64(usage.AccessCount)))
	}

	modelCoverage := float64(len(usage.Models)) / float64(totalModels) * 100

	reliabilityScore := 100.0
	if usage.AccessCount > 0 {
		errorRate := float64(usage.ErrorCount) / float64(usage.AccessCount)
		nullRate := float64(usage.NullCount) / float64(usage.AccessCount)
		reliabilityScore = maxFloat64(0, 100.0-errorRate*100-nullRate*50)
	}

	latencyScore := 100.0
	if usage.AvgLatencyMs > 0 {
		if usage.AvgLatencyMs <= 1 {
			latencyScore = 100.0
		} else if usage.AvgLatencyMs <= 10 {
			latencyScore = 90.0
		} else if usage.AvgLatencyMs <= 50 {
			latencyScore = 70.0
		} else if usage.AvgLatencyMs <= 100 {
			latencyScore = 50.0
		} else {
			latencyScore = maxFloat64(0, 100.0-usage.AvgLatencyMs)
		}
	}

	dependencyScore := minFloat64(100.0, float64(len(usage.Dependents))*20)

	criticalPath := false
	for _, modelID := range usage.Models {
		if model, ok := t.modelUsage[modelID]; ok {
			if model.Environment == "prod" || model.Environment == "production" {
				criticalPath = true
				break
			}
		}
	}

	overallScore := usageScore*0.25 + modelCoverage*0.25 + reliabilityScore*0.2 + latencyScore*0.15 + dependencyScore*0.15
	if criticalPath {
		overallScore = minFloat64(100, overallScore*1.2)
	}

	score := &ImpactScore{
		Feature:          feature,
		OverallScore:     overallScore,
		UsageScore:       usageScore,
		ModelCoverage:    modelCoverage,
		ReliabilityScore: reliabilityScore,
		LatencyScore:     latencyScore,
		DependencyScore:  dependencyScore,
		CriticalPath:     criticalPath,
		LastCalculated:   time.Now(),
	}

	t.impactScores[feature] = score
	return score
}

// GetUnusedFeatures returns features not accessed since the given time.
func (t *ImpactTracker) GetUnusedFeatures(since time.Time) []*FeatureUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var unused []*FeatureUsage
	for _, usage := range t.featureUsage {
		if usage.LastAccess.Before(since) || usage.LastAccess.IsZero() {
			unused = append(unused, usage)
		}
	}
	return unused
}

// GetLowImpactFeatures returns features with impact score below threshold.
func (t *ImpactTracker) GetLowImpactFeatures(threshold float64) []*ImpactScore {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Recalculate scores
	for feature := range t.featureUsage {
		t.calculateImpactScoreUnlocked(feature)
	}

	var lowImpact []*ImpactScore
	for _, score := range t.impactScores {
		if score.OverallScore < threshold {
			lowImpact = append(lowImpact, score)
		}
	}

	sort.Slice(lowImpact, func(i, j int) bool {
		return lowImpact[i].OverallScore < lowImpact[j].OverallScore
	})

	return lowImpact
}

// RequestDeprecation creates a deprecation request for a feature.
func (t *ImpactTracker) RequestDeprecation(req *DeprecationRequest) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.featureUsage[req.Feature]
	if !ok {
		return ErrFeatureNotFound
	}

	req.RequestedAt = time.Now()
	req.Status = "pending"
	req.AffectedModels = make([]string, len(usage.Models))
	copy(req.AffectedModels, usage.Models)

	t.deprecations[req.Feature] = req
	return nil
}

// ApproveDeprecation approves a deprecation request.
func (t *ImpactTracker) ApproveDeprecation(feature string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	req, ok := t.deprecations[feature]
	if !ok {
		return ErrDeprecationNotFound
	}

	req.Status = "approved"

	// Mark feature as deprecated
	if usage, ok := t.featureUsage[feature]; ok {
		usage.Deprecated = true
		now := time.Now()
		usage.DeprecatedAt = &now
		usage.DeprecatedBy = req.RequestedBy
	}

	return nil
}

// GetDeprecationRequest returns a deprecation request.
func (t *ImpactTracker) GetDeprecationRequest(feature string) *DeprecationRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.deprecations[feature]
}

// GetAllDeprecations returns all deprecation requests.
func (t *ImpactTracker) GetAllDeprecations() []*DeprecationRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*DeprecationRequest, 0, len(t.deprecations))
	for _, d := range t.deprecations {
		result = append(result, d)
	}
	return result
}

// GetDependencyGraph returns the dependency graph for visualization.
func (t *ImpactTracker) GetDependencyGraph() map[string][]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	graph := make(map[string][]string)
	for feature, usage := range t.featureUsage {
		graph[feature] = make([]string, len(usage.Dependencies))
		copy(graph[feature], usage.Dependencies)
	}
	return graph
}

// GetFeatureLineage traces the full lineage of a feature.
func (t *ImpactTracker) GetFeatureLineage(feature string) *FeatureLineage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	usage, ok := t.featureUsage[feature]
	if !ok {
		return nil
	}

	lineage := &FeatureLineage{
		Feature:    feature,
		Upstream:   t.getUpstream(feature, make(map[string]bool)),
		Downstream: t.getDownstream(feature, make(map[string]bool)),
		Models:     usage.Models,
	}

	return lineage
}

func (t *ImpactTracker) getUpstream(feature string, visited map[string]bool) []string {
	if visited[feature] {
		return nil
	}
	visited[feature] = true

	usage, ok := t.featureUsage[feature]
	if !ok {
		return nil
	}

	upstream := make([]string, 0, len(usage.Dependencies))
	for _, dep := range usage.Dependencies {
		upstream = append(upstream, dep)
		upstream = append(upstream, t.getUpstream(dep, visited)...)
	}
	return upstream
}

func (t *ImpactTracker) getDownstream(feature string, visited map[string]bool) []string {
	if visited[feature] {
		return nil
	}
	visited[feature] = true

	usage, ok := t.featureUsage[feature]
	if !ok {
		return nil
	}

	downstream := make([]string, 0, len(usage.Dependents))
	for _, dep := range usage.Dependents {
		downstream = append(downstream, dep)
		downstream = append(downstream, t.getDownstream(dep, visited)...)
	}
	return downstream
}

// FeatureLineage represents the full lineage of a feature.
type FeatureLineage struct {
	Feature    string   `json:"feature"`
	Upstream   []string `json:"upstream"`   // Features this depends on (transitively)
	Downstream []string `json:"downstream"` // Features depending on this (transitively)
	Models     []string `json:"models"`     // Models using this feature
}

// ImpactReport provides a summary report.
type ImpactReport struct { //nolint:revive
	GeneratedAt         time.Time             `json:"generated_at"`
	TotalFeatures       int                   `json:"total_features"`
	TotalModels         int                   `json:"total_models"`
	AvgImpactScore      float64               `json:"avg_impact_score"`
	TopFeatures         []*ImpactScore        `json:"top_features"`
	LowImpactFeatures   []*ImpactScore        `json:"low_impact_features"`
	UnusedFeatures      []*FeatureUsage       `json:"unused_features"`
	DeprecatedCount     int                   `json:"deprecated_count"`
	PendingDeprecations []*DeprecationRequest `json:"pending_deprecations"`
	CriticalFeatures    int                   `json:"critical_features"`
}

// GenerateReport generates an impact report.
func (t *ImpactTracker) GenerateReport() *ImpactReport {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Recalculate all scores
	for feature := range t.featureUsage {
		t.calculateImpactScoreUnlocked(feature)
	}

	// Calculate average score
	var totalScore float64
	criticalCount := 0
	for _, score := range t.impactScores {
		totalScore += score.OverallScore
		if score.CriticalPath {
			criticalCount++
		}
	}
	avgScore := 0.0
	if len(t.impactScores) > 0 {
		avgScore = totalScore / float64(len(t.impactScores))
	}

	// Get top features
	scores := make([]*ImpactScore, 0, len(t.impactScores))
	for _, s := range t.impactScores {
		scores = append(scores, s)
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].OverallScore > scores[j].OverallScore
	})
	topFeatures := scores
	if len(topFeatures) > 10 {
		topFeatures = topFeatures[:10]
	}

	// Get low impact features
	var lowImpact []*ImpactScore
	for _, score := range t.impactScores {
		if score.OverallScore < 20 {
			lowImpact = append(lowImpact, score)
		}
	}

	// Get unused features (not accessed in 30 days)
	threshold := time.Now().Add(-30 * 24 * time.Hour)
	var unused []*FeatureUsage
	for _, usage := range t.featureUsage {
		if usage.LastAccess.Before(threshold) || usage.LastAccess.IsZero() {
			unused = append(unused, usage)
		}
	}

	// Count deprecated and pending
	deprecatedCount := 0
	var pending []*DeprecationRequest
	for _, usage := range t.featureUsage {
		if usage.Deprecated {
			deprecatedCount++
		}
	}
	for _, req := range t.deprecations {
		if req.Status == "pending" {
			pending = append(pending, req)
		}
	}

	return &ImpactReport{
		GeneratedAt:         time.Now(),
		TotalFeatures:       len(t.featureUsage),
		TotalModels:         len(t.modelUsage),
		AvgImpactScore:      avgScore,
		TopFeatures:         topFeatures,
		LowImpactFeatures:   lowImpact,
		UnusedFeatures:      unused,
		DeprecatedCount:     deprecatedCount,
		PendingDeprecations: pending,
		CriticalFeatures:    criticalCount,
	}
}

// Errors
var (
	ErrFeatureNotFound     = errorf("feature not found")
	ErrDeprecationNotFound = errorf("deprecation request not found")
)

type impactError string

func (e impactError) Error() string { return string(e) }

func errorf(msg string) impactError { return impactError(msg) }

// Helper functions
func log10(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Manual log10 calculation: log10(x) = ln(x) / ln(10)
	return ln(x) / 2.302585092994046
}

func ln(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton-Raphson approximation for ln
	// For simplicity, use a good approximation
	var result float64
	for x >= 2 {
		result += 0.693147180559945 // ln(2)
		x /= 2
	}
	// For x in [1, 2), use series approximation
	y := (x - 1) / (x + 1)
	y2 := y * y
	term := y
	for i := 1; i < 20; i += 2 {
		result += 2 * term / float64(i)
		term *= y2
	}
	return result
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
