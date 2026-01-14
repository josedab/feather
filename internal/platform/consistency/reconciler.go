package consistency

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// ReconcileAction defines what to do when inconsistency is found.
type ReconcileAction string

const (
	// ActionReport only reports the inconsistency.
	ActionReport ReconcileAction = "report"
	// ActionBackfillOnline overwrites online store from offline source.
	ActionBackfillOnline ReconcileAction = "backfill_online"
)

// ReconcileConfig configures the automatic reconciliation.
type ReconcileConfig struct {
	// Action determines what to do on inconsistency.
	Action ReconcileAction
	// Threshold is the consistency rate below which reconciliation triggers.
	Threshold float64
	// CheckInterval is how often to run checks.
	CheckInterval time.Duration
	// MaxReconcilePerRun caps reconciliation operations per cycle.
	MaxReconcilePerRun int
}

// DefaultReconcileConfig returns sensible defaults.
func DefaultReconcileConfig() ReconcileConfig {
	return ReconcileConfig{
		Action:             ActionReport,
		Threshold:          99.0,
		CheckInterval:      5 * time.Minute,
		MaxReconcilePerRun: 100,
	}
}

// ReconcileResult records one reconciliation operation.
type ReconcileResult struct {
	EntityID    string      `json:"entity_id"`
	Feature     string      `json:"feature"`
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	Action      string      `json:"action"`
	Success     bool        `json:"success"`
	Error       string      `json:"error,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// SkewReport provides statistical analysis of online/offline skew.
type SkewReport struct {
	Feature           string  `json:"feature"`
	SampleSize        int     `json:"sample_size"`
	ConsistencyRate   float64 `json:"consistency_rate"`
	MeanAbsDifference float64 `json:"mean_abs_difference"`
	MaxAbsDifference  float64 `json:"max_abs_difference"`
	StdDevDifference  float64 `json:"std_dev_difference"`
	P50Difference     float64 `json:"p50_difference"`
	P95Difference     float64 `json:"p95_difference"`
	P99Difference     float64 `json:"p99_difference"`
}

// Monitor periodically checks consistency and optionally reconciles.
type Monitor struct {
	checker   *Checker
	config    ReconcileConfig
	mu        sync.RWMutex
	history   []ReconcileResult
	reports   []*SkewReport
	stopCh    chan struct{}
}

// NewMonitor creates a consistency monitor.
func NewMonitor(checker *Checker, config ReconcileConfig) *Monitor {
	if config.CheckInterval == 0 {
		config = DefaultReconcileConfig()
	}
	return &Monitor{
		checker: checker,
		config:  config,
		history: make([]ReconcileResult, 0),
		reports: make([]*SkewReport, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start begins periodic consistency monitoring.
func (m *Monitor) Start(ctx context.Context) {
	go m.monitorLoop(ctx)
}

// Stop halts the monitor.
func (m *Monitor) Stop() {
	close(m.stopCh)
}

// GetHistory returns recent reconciliation history.
func (m *Monitor) GetHistory(limit int) []ReconcileResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}
	result := make([]ReconcileResult, limit)
	copy(result, m.history[start:])
	return result
}

// GetSkewReports returns recent skew analysis reports.
func (m *Monitor) GetSkewReports() []*SkewReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SkewReport, len(m.reports))
	copy(result, m.reports)
	return result
}

// AnalyzeSkew computes statistical skew metrics from check results.
func (m *Monitor) AnalyzeSkew(results []*Result) []*SkewReport {
	byFeature := make(map[string][]*Result)
	for _, r := range results {
		byFeature[r.Feature] = append(byFeature[r.Feature], r)
	}

	reports := make([]*SkewReport, 0, len(byFeature))
	for feature, featureResults := range byFeature {
		report := m.computeSkewReport(feature, featureResults)
		reports = append(reports, report)
	}

	m.mu.Lock()
	m.reports = reports
	m.mu.Unlock()

	return reports
}

func (m *Monitor) computeSkewReport(feature string, results []*Result) *SkewReport {
	report := &SkewReport{
		Feature:    feature,
		SampleSize: len(results),
	}

	consistent := 0
	var diffs []float64
	for _, r := range results {
		if r.IsConsistent {
			consistent++
		}
		if r.Difference != nil {
			diffs = append(diffs, math.Abs(*r.Difference))
		}
	}

	if len(results) > 0 {
		report.ConsistencyRate = float64(consistent) / float64(len(results)) * 100
	}

	if len(diffs) > 0 {
		var sum float64
		for _, d := range diffs {
			sum += d
			if d > report.MaxAbsDifference {
				report.MaxAbsDifference = d
			}
		}
		report.MeanAbsDifference = sum / float64(len(diffs))

		// StdDev
		var variance float64
		for _, d := range diffs {
			diff := d - report.MeanAbsDifference
			variance += diff * diff
		}
		report.StdDevDifference = math.Sqrt(variance / float64(len(diffs)))

		// Percentiles (simple sorted-based)
		sorted := make([]float64, len(diffs))
		copy(sorted, diffs)
		sortFloat64s(sorted)
		report.P50Difference = percentile(sorted, 50)
		report.P95Difference = percentile(sorted, 95)
		report.P99Difference = percentile(sorted, 99)
	}

	return report
}

func (m *Monitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// The actual check would be triggered externally with entity/feature lists
		}
	}
}

func sortFloat64s(s []float64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(p) / 100 * float64(len(sorted)-1))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// ReconcileEntry records one item for reconciliation tracking.
func (m *Monitor) RecordReconciliation(result ReconcileResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, result)
	if len(m.history) > 10000 {
		m.history = m.history[len(m.history)-10000:]
	}
}

// GetConfig returns the reconciliation configuration.
func (m *Monitor) GetConfig() ReconcileConfig {
	return m.config
}

// SetAction updates the reconciliation action.
func (m *Monitor) SetAction(action ReconcileAction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Action = action
}

// Summary returns a summary of the monitor status.
func (m *Monitor) Summary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reconciled := 0
	failed := 0
	for _, h := range m.history {
		if h.Success {
			reconciled++
		} else {
			failed++
		}
	}

	summary := map[string]interface{}{
		"action":             string(m.config.Action),
		"threshold":          m.config.Threshold,
		"check_interval":     m.config.CheckInterval.String(),
		"total_reconciled":   reconciled,
		"total_failed":       failed,
		"skew_reports_count": len(m.reports),
	}

	if len(m.history) > 0 {
		summary["last_reconciliation"] = m.history[len(m.history)-1].Timestamp
	}

	return summary
}

// FormatReport returns a human-readable string of the skew report.
func FormatReport(report *SkewReport) string {
	return fmt.Sprintf(
		"Feature: %s | Samples: %d | Consistency: %.1f%% | Mean Diff: %.6f | P99 Diff: %.6f",
		report.Feature, report.SampleSize, report.ConsistencyRate,
		report.MeanAbsDifference, report.P99Difference,
	)
}
