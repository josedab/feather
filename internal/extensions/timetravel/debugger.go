package timetravel

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrSessionNotFound = errors.New("debug session not found")
	ErrNoSnapshots     = errors.New("no snapshots available")
	ErrInvalidRange    = errors.New("invalid time range")
)

// DebuggerConfig configures the time-travel debugger.
type DebuggerConfig struct {
	MaxSessions     int           `json:"max_sessions"`
	MaxSnapshots    int           `json:"max_snapshots_per_session"`
	DefaultWindow   time.Duration `json:"default_window"`
	RetentionPeriod time.Duration `json:"retention_period"`
}

func DefaultDebuggerConfig() DebuggerConfig {
	return DebuggerConfig{
		MaxSessions:     100,
		MaxSnapshots:    1000,
		DefaultWindow:   24 * time.Hour,
		RetentionPeriod: 30 * 24 * time.Hour,
	}
}

// Debugger provides time-travel debugging capabilities.
type Debugger struct {
	mu       sync.RWMutex
	config   DebuggerConfig
	sessions map[string]*DebugSession
}

// DebugSession represents an active debugging investigation.
type DebugSession struct {
	ID        string      `json:"id"`
	EntityKey string      `json:"entity_key"`
	Features  []string    `json:"features"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time"`
	CreatedAt time.Time   `json:"created_at"`
	Snapshots []*Snapshot `json:"snapshots"`
	Status    string      `json:"status"` // "active", "completed"
}

// Snapshot captures feature values at a specific point in time.
type Snapshot struct {
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

// Comparison holds the result of comparing two time windows.
type Comparison struct {
	EntityKey string        `json:"entity_key"`
	WindowA   TimeWindow    `json:"window_a"`
	WindowB   TimeWindow    `json:"window_b"`
	Diffs     []FeatureDiff `json:"diffs"`
	Anomalies []Anomaly     `json:"anomalies"`
	Summary   string        `json:"summary"`
}

// TimeWindow defines a time range.
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FeatureDiff shows how a feature changed between windows.
type FeatureDiff struct {
	FeatureName string      `json:"feature_name"`
	ValueA      interface{} `json:"value_a"`     // Value in window A
	ValueB      interface{} `json:"value_b"`     // Value in window B
	ChangeType  string      `json:"change_type"` // "increased", "decreased", "changed", "unchanged", "missing"
	ChangePct   float64     `json:"change_pct"`  // Percentage change for numeric values
}

// Anomaly represents a detected anomaly in feature values.
type Anomaly struct {
	FeatureName string      `json:"feature_name"`
	Timestamp   time.Time   `json:"timestamp"`
	Value       interface{} `json:"value"`
	Expected    interface{} `json:"expected"`
	Severity    string      `json:"severity"` // "low", "medium", "high"
	Description string      `json:"description"`
}

// ReplayResult holds the results of replaying feature history.
type ReplayResult struct {
	EntityKey  string      `json:"entity_key"`
	Features   []string    `json:"features"`
	Timeline   []*Snapshot `json:"timeline"`
	StartTime  time.Time   `json:"start_time"`
	EndTime    time.Time   `json:"end_time"`
	PointCount int         `json:"point_count"`
}

// NewDebugger creates a new time-travel debugger with the given configuration.
func NewDebugger(config DebuggerConfig) *Debugger {
	return &Debugger{
		config:   config,
		sessions: make(map[string]*DebugSession),
	}
}

// CreateSession creates a new debug session for investigating feature history.
func (d *Debugger) CreateSession(id, entityKey string, features []string, start, end time.Time) (*DebugSession, error) {
	if end.Before(start) {
		return nil, ErrInvalidRange
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.sessions) >= d.config.MaxSessions {
		return nil, fmt.Errorf("max sessions (%d) reached", d.config.MaxSessions)
	}

	session := &DebugSession{
		ID:        id,
		EntityKey: entityKey,
		Features:  features,
		StartTime: start,
		EndTime:   end,
		CreatedAt: time.Now(),
		Snapshots: make([]*Snapshot, 0),
		Status:    "active",
	}
	d.sessions[id] = session
	return session, nil
}

// GetSession returns a debug session by ID.
func (d *Debugger) GetSession(id string) (*DebugSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	session, ok := d.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// ListSessions returns all debug sessions.
func (d *Debugger) ListSessions() []*DebugSession {
	d.mu.RLock()
	defer d.mu.RUnlock()

	sessions := make([]*DebugSession, 0, len(d.sessions))
	for _, s := range d.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// CloseSession marks a session as completed.
func (d *Debugger) CloseSession(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, ok := d.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	session.Status = "completed"
	return nil
}

// AddSnapshot adds a snapshot to an existing debug session.
func (d *Debugger) AddSnapshot(sessionID string, snapshot *Snapshot) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	session, ok := d.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}

	if len(session.Snapshots) >= d.config.MaxSnapshots {
		return fmt.Errorf("max snapshots (%d) reached for session %s", d.config.MaxSnapshots, sessionID)
	}

	session.Snapshots = append(session.Snapshots, snapshot)
	return nil
}

// Replay returns all snapshots for a session sorted by timestamp.
func (d *Debugger) Replay(sessionID string) (*ReplayResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	session, ok := d.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	if len(session.Snapshots) == 0 {
		return nil, ErrNoSnapshots
	}

	sorted := make([]*Snapshot, len(session.Snapshots))
	copy(sorted, session.Snapshots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	return &ReplayResult{
		EntityKey:  session.EntityKey,
		Features:   session.Features,
		Timeline:   sorted,
		StartTime:  session.StartTime,
		EndTime:    session.EndTime,
		PointCount: len(sorted),
	}, nil
}

// Compare compares feature values between two time windows.
func (d *Debugger) Compare(entityKey string, windowA, windowB TimeWindow, snapshots []*Snapshot) (*Comparison, error) {
	if windowA.End.Before(windowA.Start) || windowB.End.Before(windowB.Start) {
		return nil, ErrInvalidRange
	}

	// Separate snapshots into windows
	var snapsA, snapsB []*Snapshot
	for _, s := range snapshots {
		if !s.Timestamp.Before(windowA.Start) && !s.Timestamp.After(windowA.End) {
			snapsA = append(snapsA, s)
		}
		if !s.Timestamp.Before(windowB.Start) && !s.Timestamp.After(windowB.End) {
			snapsB = append(snapsB, s)
		}
	}

	// Collect all feature names
	featureSet := make(map[string]struct{})
	for _, s := range snapshots {
		for k := range s.Values {
			featureSet[k] = struct{}{}
		}
	}

	// Compute average values per feature in each window
	avgA := computeAverages(snapsA)
	avgB := computeAverages(snapsB)

	// Create diffs
	var diffs []FeatureDiff
	changedCount := 0
	for feature := range featureSet {
		diff := FeatureDiff{FeatureName: feature}
		valA, okA := avgA[feature]
		valB, okB := avgB[feature]

		if !okA && !okB {
			diff.ChangeType = "missing"
		} else if !okA || !okB {
			diff.ChangeType = "missing"
			if okA {
				diff.ValueA = valA
			}
			if okB {
				diff.ValueB = valB
			}
			changedCount++
		} else {
			diff.ValueA = valA
			diff.ValueB = valB
			if valA == valB {
				diff.ChangeType = "unchanged"
			} else if valB > valA {
				diff.ChangeType = "increased"
				changedCount++
				if valA != 0 {
					diff.ChangePct = ((valB - valA) / math.Abs(valA)) * 100
				}
			} else {
				diff.ChangeType = "decreased"
				changedCount++
				if valA != 0 {
					diff.ChangePct = ((valB - valA) / math.Abs(valA)) * 100
				}
			}
		}
		diffs = append(diffs, diff)
	}

	// Sort diffs by feature name for deterministic output
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].FeatureName < diffs[j].FeatureName
	})

	// Detect anomalies across all snapshots
	var anomalies []Anomaly
	for feature := range featureSet {
		detected := d.DetectAnomalies(snapshots, feature)
		anomalies = append(anomalies, detected...)
	}

	summary := fmt.Sprintf("%d features changed, %d anomalies detected", changedCount, len(anomalies))

	return &Comparison{
		EntityKey: entityKey,
		WindowA:   windowA,
		WindowB:   windowB,
		Diffs:     diffs,
		Anomalies: anomalies,
		Summary:   summary,
	}, nil
}

// DetectAnomalies finds anomalies in a time series using z-score analysis.
// Values more than 3 standard deviations from the mean are flagged.
func (d *Debugger) DetectAnomalies(snapshots []*Snapshot, featureName string) []Anomaly {
	var values []float64
	var timestamps []time.Time

	for _, s := range snapshots {
		v, ok := s.Values[featureName]
		if !ok {
			continue
		}
		f, ok := toFloat64(v)
		if !ok {
			continue
		}
		values = append(values, f)
		timestamps = append(timestamps, s.Timestamp)
	}

	if len(values) < 2 {
		return nil
	}

	mean, stddev := meanStddev(values)
	if stddev == 0 {
		return nil
	}

	var anomalies []Anomaly
	for i, v := range values {
		zscore := math.Abs(v-mean) / stddev
		if zscore > 3 {
			severity := "low"
			if zscore > 5 {
				severity = "high"
			} else if zscore > 4 {
				severity = "medium"
			}
			anomalies = append(anomalies, Anomaly{
				FeatureName: featureName,
				Timestamp:   timestamps[i],
				Value:       v,
				Expected:    mean,
				Severity:    severity,
				Description: fmt.Sprintf("value %.2f deviates %.1f stddevs from mean %.2f", v, zscore, mean),
			})
		}
	}
	return anomalies
}

// computeAverages returns the average numeric value per feature across snapshots.
func computeAverages(snapshots []*Snapshot) map[string]float64 {
	sums := make(map[string]float64)
	counts := make(map[string]int)

	for _, s := range snapshots {
		for k, v := range s.Values {
			f, ok := toFloat64(v)
			if !ok {
				continue
			}
			sums[k] += f
			counts[k]++
		}
	}

	avgs := make(map[string]float64)
	for k, sum := range sums {
		avgs[k] = sum / float64(counts[k])
	}
	return avgs
}

// meanStddev calculates the mean and standard deviation of a slice of float64.
func meanStddev(values []float64) (float64, float64) {
	n := float64(len(values))
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / n

	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= n
	return mean, math.Sqrt(variance)
}

// toFloat64 attempts to convert an interface value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
