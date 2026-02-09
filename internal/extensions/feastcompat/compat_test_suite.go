package feastcompat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CompatTestSuite runs Feast SDK compatibility tests against the Feather gateway.
type CompatTestSuite struct {
	mu          sync.Mutex
	gateway     *Gateway
	results     []CompatTestResult
	startedAt   time.Time
	completedAt time.Time
}

// CompatTestResult holds the result of a single compatibility test.
type CompatTestResult struct {
	Name       string        `json:"name"`
	Category   string        `json:"category"` // "online", "push", "apply", "materialize"
	Passed     bool          `json:"passed"`
	Duration   time.Duration `json:"duration_ns"`
	Error      string        `json:"error,omitempty"`
	Request    interface{}   `json:"request,omitempty"`
	Response   interface{}   `json:"response,omitempty"`
}

// CompatTestSummary summarizes the test suite results.
type CompatTestSummary struct {
	TotalTests  int                `json:"total_tests"`
	Passed      int                `json:"passed"`
	Failed      int                `json:"failed"`
	ByCategory  map[string]int     `json:"by_category"`
	Duration    time.Duration      `json:"duration_ns"`
	Results     []CompatTestResult `json:"results"`
	StartedAt   time.Time          `json:"started_at"`
	CompletedAt time.Time          `json:"completed_at"`
}

// NewCompatTestSuite creates a new compatibility test suite.
func NewCompatTestSuite(gw *Gateway) *CompatTestSuite {
	return &CompatTestSuite{gateway: gw}
}

// RunAll runs all Feast compatibility tests.
func (s *CompatTestSuite) RunAll() *CompatTestSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.startedAt = time.Now()
	s.results = nil

	tests := []struct {
		name     string
		category string
		fn       func() error
	}{
		{"get_online_features_empty", "online", s.testGetOnlineFeaturesEmpty},
		{"get_online_features_with_entity", "online", s.testGetOnlineFeaturesWithEntity},
		{"push_single_feature", "push", s.testPushSingleFeature},
		{"push_batch", "push", s.testPushBatch},
		{"apply_feature_view", "apply", s.testApplyFeatureView},
		{"apply_feature_service", "apply", s.testApplyFeatureService},
		{"list_feature_views", "apply", s.testListFeatureViews},
		{"materialize_incremental", "materialize", s.testMaterializeIncremental},
		{"saved_dataset_operations", "dataset", s.testSavedDatasetOps},
	}

	for _, tt := range tests {
		start := time.Now()
		err := tt.fn()
		result := CompatTestResult{
			Name:     tt.name,
			Category: tt.category,
			Passed:   err == nil,
			Duration: time.Since(start),
		}
		if err != nil {
			result.Error = err.Error()
		}
		s.results = append(s.results, result)
	}

	s.completedAt = time.Now()
	return s.buildSummary()
}

func (s *CompatTestSuite) buildSummary() *CompatTestSummary {
	summary := &CompatTestSummary{
		TotalTests:  len(s.results),
		ByCategory:  make(map[string]int),
		Results:     s.results,
		StartedAt:   s.startedAt,
		CompletedAt: s.completedAt,
		Duration:    s.completedAt.Sub(s.startedAt),
	}

	for _, r := range s.results {
		if r.Passed {
			summary.Passed++
		} else {
			summary.Failed++
		}
		summary.ByCategory[r.Category]++
	}

	return summary
}

func (s *CompatTestSuite) testGetOnlineFeaturesEmpty() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := OnlineFeatureRequest{
		Features: []string{"nonexistent"},
		Entities: map[string][]interface{}{},
	}
	_, err := s.gateway.GetOnlineFeatures(req)
	// Should return empty, not error
	if err != nil {
		return fmt.Errorf("get_online_features with empty entities should not error: %v", err)
	}
	return nil
}

func (s *CompatTestSuite) testGetOnlineFeaturesWithEntity() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := OnlineFeatureRequest{
		Features: []string{"test_feature"},
		Entities: map[string][]interface{}{"entity_id": {"test_entity_1"}},
	}
	resp, err := s.gateway.GetOnlineFeatures(req)
	if err != nil {
		return fmt.Errorf("get_online_features failed: %v", err)
	}
	if resp == nil {
		return fmt.Errorf("expected response, got nil")
	}
	return nil
}

func (s *CompatTestSuite) testPushSingleFeature() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := PushRequest{
		PushSourceName: "test_source",
		DfData: []map[string]interface{}{
			{"entity_id": "e1", "feature_val": 42.0},
		},
	}
	_, err := s.gateway.Push(req)
	return err
}

func (s *CompatTestSuite) testPushBatch() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := PushRequest{
		PushSourceName: "test_source",
		DfData: []map[string]interface{}{
			{"entity_id": "e1", "feature_val": 1.0},
			{"entity_id": "e2", "feature_val": 2.0},
			{"entity_id": "e3", "feature_val": 3.0},
		},
	}
	_, err := s.gateway.Push(req)
	return err
}

func (s *CompatTestSuite) testApplyFeatureView() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := ApplyRequest{
		FeatureViews: []FeatureViewDef{
			{
				Name: "test_apply_view",
				Entities: []string{"user"},
				Schema: []FieldDef{
					{Name: "score", DType: "FLOAT64"},
				},
			},
		},
	}
	_, err := s.gateway.Apply(req)
	return err
}

func (s *CompatTestSuite) testApplyFeatureService() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	req := ApplyRequest{
		FeatureServices: []FeatureServiceDef{
			{
				Name: "test_service",
				FeatureViews: []string{"test_view"},
			},
		},
	}
	_, err := s.gateway.Apply(req)
	return err
}

func (s *CompatTestSuite) testListFeatureViews() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	views := s.gateway.ListFeatureViewMappings()
	_ = views // No error expected
	return nil
}

func (s *CompatTestSuite) testMaterializeIncremental() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	// Materialize incremental returns stats about materialized features
	stats := s.gateway.MaterializeIncremental(time.Now())
	if stats == nil {
		return fmt.Errorf("materialize returned nil stats")
	}
	return nil
}

func (s *CompatTestSuite) testSavedDatasetOps() error {
	if s.gateway == nil {
		return fmt.Errorf("gateway not configured")
	}
	datasets := s.gateway.ListSavedDatasets()
	_ = datasets
	return nil
}

// RegisterCompatRoutes registers compatibility test endpoints.
func (s *CompatTestSuite) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/feast/compat/test", s.handleRunTests)
	mux.HandleFunc("GET /v1/feast/compat/results", s.handleGetResults)
}

func (s *CompatTestSuite) handleRunTests(w http.ResponseWriter, r *http.Request) {
	summary := s.RunAll()
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if summary.Failed > 0 {
		status = http.StatusExpectationFailed
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *CompatTestSuite) handleGetResults(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	summary := s.buildSummary()
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}
