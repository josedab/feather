package backpressure

import (
	"fmt"
	"sync"
	"time"
)

// PressureLevel indicates the current level of backpressure.
type PressureLevel string

// Pressure level constants.
const (
	None     PressureLevel = "none"
	Low      PressureLevel = "low"
	Medium   PressureLevel = "medium"
	High     PressureLevel = "high"
	Critical PressureLevel = "critical"
)

// MetricSample represents a single metric observation.
type MetricSample struct {
	Name      string
	Value     float64
	Timestamp time.Time
}

// PressureReport contains the result of a pressure evaluation.
type PressureReport struct {
	Level             PressureLevel
	QueueDepth        float64
	LatencyP99        float64
	ErrorRate         float64
	Recommendation    string
	ScaleAction       string // "none", "scale_up", "scale_down"
	SuggestedReplicas int
	Timestamp         time.Time
}

// MonitorConfig configures the backpressure monitor.
type MonitorConfig struct {
	// QueueHighWatermark is the queue depth ratio triggering pressure (0-1).
	QueueHighWatermark float64

	// LatencyThresholdMs is the p99 latency threshold in milliseconds.
	LatencyThresholdMs float64

	// ErrorRateThreshold is the error rate threshold (0-1).
	ErrorRateThreshold float64

	// CheckInterval is how often to evaluate pressure.
	CheckInterval time.Duration

	// CooldownPeriod prevents rapid scaling actions.
	CooldownPeriod time.Duration

	// MaxSamples is the maximum number of samples to retain per metric.
	MaxSamples int
}

// DefaultMonitorConfig returns sensible defaults.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		QueueHighWatermark: 0.8,
		LatencyThresholdMs: 100,
		ErrorRateThreshold: 0.05,
		CheckInterval:      30 * time.Second,
		CooldownPeriod:     5 * time.Minute,
		MaxSamples:         1000,
	}
}

// MonitorStats contains monitor statistics.
type MonitorStats struct {
	TotalSamples     int64
	TotalEvaluations int64
	CurrentLevel     string
	ScaleUpEvents    int
	ScaleDownEvents  int
}

// Monitor tracks system metrics and evaluates backpressure levels.
type Monitor struct {
	mu              sync.RWMutex
	config          MonitorConfig
	queueSamples    []MetricSample
	latencySamples  []MetricSample
	errorSamples    []MetricSample
	reports         []PressureReport
	currentLevel    PressureLevel
	lastScaleAction time.Time
	totalSamples    int64
	totalEvals      int64
	scaleUpEvents   int
	scaleDownEvents int
}

// NewMonitor creates a new backpressure monitor.
func NewMonitor(config MonitorConfig) *Monitor {
	if config.MaxSamples == 0 {
		config = DefaultMonitorConfig()
	}

	return &Monitor{
		config:         config,
		queueSamples:   make([]MetricSample, 0),
		latencySamples: make([]MetricSample, 0),
		errorSamples:   make([]MetricSample, 0),
		reports:        make([]PressureReport, 0),
		currentLevel:   None,
	}
}

// RecordQueueDepth records a queue depth observation (0-1 ratio).
func (m *Monitor) RecordQueueDepth(depth float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queueSamples = append(m.queueSamples, MetricSample{
		Name:      "queue_depth",
		Value:     depth,
		Timestamp: time.Now(),
	})
	m.totalSamples++

	if len(m.queueSamples) > m.config.MaxSamples {
		m.queueSamples = m.queueSamples[1:]
	}
}

// RecordLatency records a latency observation in milliseconds.
func (m *Monitor) RecordLatency(latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latencySamples = append(m.latencySamples, MetricSample{
		Name:      "latency_ms",
		Value:     latencyMs,
		Timestamp: time.Now(),
	})
	m.totalSamples++

	if len(m.latencySamples) > m.config.MaxSamples {
		m.latencySamples = m.latencySamples[1:]
	}
}

// RecordErrorRate records an error rate observation (0-1 ratio).
func (m *Monitor) RecordErrorRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorSamples = append(m.errorSamples, MetricSample{
		Name:      "error_rate",
		Value:     rate,
		Timestamp: time.Now(),
	})
	m.totalSamples++

	if len(m.errorSamples) > m.config.MaxSamples {
		m.errorSamples = m.errorSamples[1:]
	}
}

// Evaluate computes the current pressure level and generates a report.
func (m *Monitor) Evaluate() PressureReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalEvals++
	now := time.Now()

	avgQueue := averageRecent(m.queueSamples, 10)
	avgLatency := averageRecent(m.latencySamples, 10)
	avgError := averageRecent(m.errorSamples, 10)

	// Determine pressure level
	level := None
	signals := 0

	if avgQueue > m.config.QueueHighWatermark {
		signals++
	}
	if avgLatency > m.config.LatencyThresholdMs {
		signals++
	}
	if avgError > m.config.ErrorRateThreshold {
		signals++
	}

	switch {
	case signals >= 3:
		level = Critical
	case signals == 2:
		level = High
	case signals == 1:
		level = Medium
	case avgQueue > m.config.QueueHighWatermark*0.5 ||
		avgLatency > m.config.LatencyThresholdMs*0.5:
		level = Low
	}

	m.currentLevel = level

	// Determine scale action
	scaleAction := "none"
	suggestedReplicas := 0
	recommendation := "System operating normally"

	switch level {
	case Critical:
		scaleAction = "scale_up"
		suggestedReplicas = 4
		recommendation = fmt.Sprintf("Critical backpressure: queue=%.2f, latency=%.1fms, errors=%.2f%%. Immediate scale-up recommended.",
			avgQueue, avgLatency, avgError*100)
	case High:
		scaleAction = "scale_up"
		suggestedReplicas = 2
		recommendation = fmt.Sprintf("High backpressure: queue=%.2f, latency=%.1fms. Scale-up recommended.",
			avgQueue, avgLatency)
	case Medium:
		recommendation = fmt.Sprintf("Moderate pressure detected: queue=%.2f, latency=%.1fms. Monitor closely.",
			avgQueue, avgLatency)
	case Low:
		recommendation = "Light pressure detected. No action needed."
	case None:
		// Check if we can scale down
		if avgQueue < m.config.QueueHighWatermark*0.2 &&
			avgLatency < m.config.LatencyThresholdMs*0.2 &&
			len(m.queueSamples) > 5 {
			scaleAction = "scale_down"
			suggestedReplicas = 1
			recommendation = "System underutilized. Consider scaling down."
		}
	}

	// Apply cooldown
	if scaleAction != "none" && now.Sub(m.lastScaleAction) < m.config.CooldownPeriod {
		scaleAction = "none"
		suggestedReplicas = 0
	} else if scaleAction != "none" {
		m.lastScaleAction = now
		if scaleAction == "scale_up" {
			m.scaleUpEvents++
		} else if scaleAction == "scale_down" {
			m.scaleDownEvents++
		}
	}

	report := PressureReport{
		Level:             level,
		QueueDepth:        avgQueue,
		LatencyP99:        avgLatency,
		ErrorRate:         avgError,
		Recommendation:    recommendation,
		ScaleAction:       scaleAction,
		SuggestedReplicas: suggestedReplicas,
		Timestamp:         now,
	}

	m.reports = append(m.reports, report)
	if len(m.reports) > m.config.MaxSamples {
		m.reports = m.reports[1:]
	}

	return report
}

// GetCurrentLevel returns the current pressure level.
func (m *Monitor) GetCurrentLevel() PressureLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.currentLevel
}

// GetReports returns the most recent pressure reports.
func (m *Monitor) GetReports(limit int) []PressureReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.reports) {
		limit = len(m.reports)
	}

	start := len(m.reports) - limit
	result := make([]PressureReport, limit)
	copy(result, m.reports[start:])

	return result
}

// Stats returns monitor statistics.
func (m *Monitor) Stats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MonitorStats{
		TotalSamples:     m.totalSamples,
		TotalEvaluations: m.totalEvals,
		CurrentLevel:     string(m.currentLevel),
		ScaleUpEvents:    m.scaleUpEvents,
		ScaleDownEvents:  m.scaleDownEvents,
	}
}

// averageRecent computes the average of the last N samples.
func averageRecent(samples []MetricSample, n int) float64 {
	if len(samples) == 0 {
		return 0
	}

	if n > len(samples) {
		n = len(samples)
	}

	start := len(samples) - n
	var sum float64
	for _, s := range samples[start:] {
		sum += s.Value
	}

	return sum / float64(n)
}
