package consistency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
)

// Errors for consistency checking.
var (
	ErrOfflineSourceNotConfigured = errors.New("offline source not configured")
	ErrFeatureNotFound            = errors.New("feature not found")
	ErrIncompatibleTypes          = errors.New("incompatible types for comparison")
)

// OfflineSource provides access to offline feature values.
type OfflineSource interface {
	// GetFeature retrieves a feature value from offline storage.
	GetFeature(ctx context.Context, entityID string, featureName string) (interface{}, time.Time, error)

	// GetFeaturesBatch retrieves multiple features for multiple entities.
	GetFeaturesBatch(ctx context.Context, entityIDs []string, featureNames []string) (map[string]map[string]interface{}, error)

	// Name returns the name of the offline source.
	Name() string
}

// Result represents the result of a consistency check.
type Result struct {
	EntityID     string      `json:"entity_id"`
	Feature      string      `json:"feature"`
	OnlineValue  interface{} `json:"online_value"`
	OfflineValue interface{} `json:"offline_value"`
	IsConsistent bool        `json:"is_consistent"`
	Difference   *float64    `json:"difference,omitempty"`
	CheckedAt    time.Time   `json:"checked_at"`
	OnlineTime   time.Time   `json:"online_time,omitempty"`
	OfflineTime  time.Time   `json:"offline_time,omitempty"`
	Tolerance    float64     `json:"tolerance,omitempty"`
}

// Report summarizes consistency check results.
type Report struct {
	TotalChecks       int                            `json:"total_checks"`
	ConsistentCount   int                            `json:"consistent_count"`
	InconsistentCount int                            `json:"inconsistent_count"`
	ConsistencyRate   float64                        `json:"consistency_rate"`
	ByFeature         map[string]*FeatureConsistency `json:"by_feature"`
	StartTime         time.Time                      `json:"start_time"`
	EndTime           time.Time                      `json:"end_time"`
	Duration          time.Duration                  `json:"duration"`
}

// FeatureConsistency contains consistency stats for a single feature.
type FeatureConsistency struct {
	Feature           string  `json:"feature"`
	TotalChecks       int     `json:"total_checks"`
	ConsistentCount   int     `json:"consistent_count"`
	InconsistentCount int     `json:"inconsistent_count"`
	ConsistencyRate   float64 `json:"consistency_rate"`
	AvgDifference     float64 `json:"avg_difference,omitempty"`
	MaxDifference     float64 `json:"max_difference,omitempty"`
}

// CheckerConfig configures the consistency checker.
type CheckerConfig struct {
	DefaultTolerance float64            // Default tolerance for numeric comparisons
	Tolerances       map[string]float64 // Per-feature tolerances
	SampleSize       int                // Number of entities to sample
	Concurrency      int                // Number of concurrent checks
	Timeout          time.Duration      // Timeout per check
}

// DefaultConfig returns a default checker configuration.
func DefaultConfig() CheckerConfig {
	return CheckerConfig{
		DefaultTolerance: 0.0001,
		Tolerances:       make(map[string]float64),
		SampleSize:       1000,
		Concurrency:      10,
		Timeout:          30 * time.Second,
	}
}

// Checker performs online/offline consistency checks.
type Checker struct {
	onlineStore   *storage.Store
	offlineSource OfflineSource
	config        CheckerConfig
	results       []*Result
	mu            sync.RWMutex
}

// NewChecker creates a new consistency checker.
func NewChecker(onlineStore *storage.Store, offlineSource OfflineSource, config CheckerConfig) *Checker {
	return &Checker{
		onlineStore:   onlineStore,
		offlineSource: offlineSource,
		config:        config,
		results:       make([]*Result, 0),
	}
}

// SetOfflineSource sets the offline source.
func (c *Checker) SetOfflineSource(source OfflineSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offlineSource = source
}

// CheckFeature checks consistency for a single entity/feature pair.
func (c *Checker) CheckFeature(ctx context.Context, entityID string, featureName string) (*Result, error) {
	c.mu.RLock()
	offlineSource := c.offlineSource
	c.mu.RUnlock()

	if offlineSource == nil {
		return nil, ErrOfflineSourceNotConfigured
	}

	result := &Result{
		EntityID:  entityID,
		Feature:   featureName,
		CheckedAt: time.Now(),
	}

	// Get online value
	onlineValues, err := c.onlineStore.Get(entityID, []string{featureName})
	if err != nil {
		return nil, fmt.Errorf("getting online value: %w", err)
	}

	if fv, ok := onlineValues[featureName]; ok && fv != nil {
		result.OnlineValue = fv.Value
		result.OnlineTime = time.Unix(0, fv.Timestamp)
	}

	// Get offline value
	offlineValue, offlineTime, err := offlineSource.GetFeature(ctx, entityID, featureName)
	if err != nil && !errors.Is(err, ErrFeatureNotFound) {
		return nil, fmt.Errorf("getting offline value: %w", err)
	}

	result.OfflineValue = offlineValue
	result.OfflineTime = offlineTime

	// Compare values
	result.Tolerance = c.getTolerance(featureName)
	result.IsConsistent, result.Difference = c.compareValues(result.OnlineValue, result.OfflineValue, result.Tolerance)

	// Store result
	c.mu.Lock()
	c.results = append(c.results, result)
	if len(c.results) > 10000 {
		c.results = c.results[len(c.results)-10000:]
	}
	c.mu.Unlock()

	return result, nil
}

// CheckBatch checks consistency for multiple entity/feature pairs.
func (c *Checker) CheckBatch(ctx context.Context, entityIDs []string, featureNames []string) ([]*Result, error) {
	c.mu.RLock()
	offlineSource := c.offlineSource
	c.mu.RUnlock()

	if offlineSource == nil {
		return nil, ErrOfflineSourceNotConfigured
	}

	// Semaphore for concurrency control
	sem := make(chan struct{}, c.config.Concurrency)
	var wg sync.WaitGroup
	resultsCh := make(chan *Result, len(entityIDs)*len(featureNames))
	errorsCh := make(chan error, 1)

	for _, entityID := range entityIDs {
		for _, featureName := range featureNames {
			wg.Add(1)
			go func(eid, fname string) {
				defer wg.Done()

				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				result, err := c.CheckFeature(ctx, eid, fname)
				if err != nil {
					select {
					case errorsCh <- err:
					default:
					}
					return
				}
				resultsCh <- result
			}(entityID, featureName)
		}
	}

	wg.Wait()
	close(resultsCh)
	close(errorsCh)

	results := make([]*Result, 0, len(entityIDs)*len(featureNames))
	for result := range resultsCh {
		results = append(results, result)
	}

	return results, nil
}

// GenerateReport generates a consistency report from results.
func (c *Checker) GenerateReport(results []*Result) *Report {
	startTime := time.Now()
	if len(results) > 0 {
		startTime = results[0].CheckedAt
	}

	report := &Report{
		TotalChecks: len(results),
		ByFeature:   make(map[string]*FeatureConsistency),
		StartTime:   startTime,
		EndTime:     time.Now(),
	}

	for _, r := range results {
		if r.IsConsistent {
			report.ConsistentCount++
		} else {
			report.InconsistentCount++
		}

		// Update per-feature stats
		fc, ok := report.ByFeature[r.Feature]
		if !ok {
			fc = &FeatureConsistency{Feature: r.Feature}
			report.ByFeature[r.Feature] = fc
		}

		fc.TotalChecks++
		if r.IsConsistent {
			fc.ConsistentCount++
		} else {
			fc.InconsistentCount++
		}

		if r.Difference != nil {
			diff := math.Abs(*r.Difference)
			fc.AvgDifference = (fc.AvgDifference*float64(fc.TotalChecks-1) + diff) / float64(fc.TotalChecks)
			if diff > fc.MaxDifference {
				fc.MaxDifference = diff
			}
		}
	}

	// Calculate rates
	if report.TotalChecks > 0 {
		report.ConsistencyRate = float64(report.ConsistentCount) / float64(report.TotalChecks) * 100
	}

	for _, fc := range report.ByFeature {
		if fc.TotalChecks > 0 {
			fc.ConsistencyRate = float64(fc.ConsistentCount) / float64(fc.TotalChecks) * 100
		}
	}

	report.Duration = report.EndTime.Sub(report.StartTime)

	return report
}

// GetResults returns recent consistency check results.
func (c *Checker) GetResults(feature string, since time.Time, limit int) []*Result {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var filtered []*Result
	for i := len(c.results) - 1; i >= 0 && len(filtered) < limit; i-- {
		r := c.results[i]
		if r.CheckedAt.Before(since) {
			break
		}
		if feature == "" || r.Feature == feature {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// GetInconsistencies returns only inconsistent results.
func (c *Checker) GetInconsistencies(feature string, since time.Time, limit int) []*Result {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var filtered []*Result
	for i := len(c.results) - 1; i >= 0 && len(filtered) < limit; i-- {
		r := c.results[i]
		if r.CheckedAt.Before(since) {
			break
		}
		if !r.IsConsistent && (feature == "" || r.Feature == feature) {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func (c *Checker) getTolerance(feature string) float64 {
	if tol, ok := c.config.Tolerances[feature]; ok {
		return tol
	}
	return c.config.DefaultTolerance
}

func (c *Checker) compareValues(online, offline interface{}, tolerance float64) (bool, *float64) {
	// Both nil
	if online == nil && offline == nil {
		return true, nil
	}

	// One nil
	if online == nil || offline == nil {
		return false, nil
	}

	// Same value (exact match)
	if online == offline {
		return true, nil
	}

	// Numeric comparison with tolerance
	onlineNum, onlineIsNum := toFloat(online)
	offlineNum, offlineIsNum := toFloat(offline)

	if onlineIsNum && offlineIsNum {
		diff := onlineNum - offlineNum
		isConsistent := math.Abs(diff) <= tolerance
		return isConsistent, &diff
	}

	// String comparison
	onlineStr, onlineIsStr := online.(string)
	offlineStr, offlineIsStr := offline.(string)

	if onlineIsStr && offlineIsStr {
		return onlineStr == offlineStr, nil
	}

	// Deep equality for other types
	onlineJSON, _ := json.Marshal(online)
	offlineJSON, _ := json.Marshal(offline)
	return string(onlineJSON) == string(offlineJSON), nil
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// HTTPOfflineSource implements OfflineSource using an HTTP endpoint.
type HTTPOfflineSource struct {
	name     string
	endpoint string
	client   *http.Client
	headers  map[string]string
}

// NewHTTPOfflineSource creates an HTTP-based offline source.
func NewHTTPOfflineSource(name, endpoint string, headers map[string]string) *HTTPOfflineSource {
	return &HTTPOfflineSource{
		name:     name,
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: headers,
	}
}

// Name identifies the offline source.
func (s *HTTPOfflineSource) Name() string {
	return s.name
}

// GetFeature fetches a single feature from the offline service.
func (s *HTTPOfflineSource) GetFeature(ctx context.Context, entityID string, featureName string) (interface{}, time.Time, error) {
	url := fmt.Sprintf("%s/features/%s/%s", s.endpoint, entityID, featureName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, time.Time{}, err
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, time.Time{}, ErrFeatureNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, time.Time{}, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var result struct {
		Value     interface{} `json:"value"`
		Timestamp string      `json:"timestamp"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, time.Time{}, err
	}

	timestamp, _ := time.Parse(time.RFC3339, result.Timestamp)
	return result.Value, timestamp, nil
}

// GetFeaturesBatch fetches multiple features in one request.
func (s *HTTPOfflineSource) GetFeaturesBatch(ctx context.Context, entityIDs []string, featureNames []string) (map[string]map[string]interface{}, error) {
	// Batch endpoint
	url := fmt.Sprintf("%s/features/batch", s.endpoint)

	payload := map[string]interface{}{
		"entity_ids": entityIDs,
		"features":   featureNames,
	}

	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	var result map[string]map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// InMemoryOfflineSource is an in-memory offline source for testing.
type InMemoryOfflineSource struct {
	name  string
	data  map[string]map[string]interface{}
	times map[string]map[string]time.Time
	mu    sync.RWMutex
}

// NewInMemoryOfflineSource creates an in-memory offline source.
func NewInMemoryOfflineSource(name string) *InMemoryOfflineSource {
	return &InMemoryOfflineSource{
		name:  name,
		data:  make(map[string]map[string]interface{}),
		times: make(map[string]map[string]time.Time),
	}
}

// Name identifies the offline source.
func (s *InMemoryOfflineSource) Name() string {
	return s.name
}

// SetFeature sets a feature value in the offline source.
func (s *InMemoryOfflineSource) SetFeature(entityID, featureName string, value interface{}, timestamp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data[entityID] == nil {
		s.data[entityID] = make(map[string]interface{})
		s.times[entityID] = make(map[string]time.Time)
	}
	s.data[entityID][featureName] = value
	s.times[entityID][featureName] = timestamp
}

// GetFeature returns a value from in-memory data.
func (s *InMemoryOfflineSource) GetFeature(ctx context.Context, entityID string, featureName string) (interface{}, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entityData, ok := s.data[entityID]
	if !ok {
		return nil, time.Time{}, ErrFeatureNotFound
	}

	value, ok := entityData[featureName]
	if !ok {
		return nil, time.Time{}, ErrFeatureNotFound
	}

	timestamp := s.times[entityID][featureName]
	return value, timestamp, nil
}

// GetFeaturesBatch returns values from in-memory data.
func (s *InMemoryOfflineSource) GetFeaturesBatch(ctx context.Context, entityIDs []string, featureNames []string) (map[string]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for _, entityID := range entityIDs {
		entityData, ok := s.data[entityID]
		if !ok {
			continue
		}

		result[entityID] = make(map[string]interface{})
		for _, featureName := range featureNames {
			if value, ok := entityData[featureName]; ok {
				result[entityID][featureName] = value
			}
		}
	}

	return result, nil
}
