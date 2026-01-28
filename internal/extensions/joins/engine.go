package joins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JoinType specifies how entities are matched across datasets.
type JoinType int

const (
	Inner JoinType = iota
	Left
	Right
	Full
)

// String returns the string representation of a JoinType.
func (jt JoinType) String() string {
	switch jt {
	case Inner:
		return "inner"
	case Left:
		return "left"
	case Right:
		return "right"
	case Full:
		return "full"
	default:
		return "unknown"
	}
}

// WindowType specifies the windowing strategy for temporal joins.
type WindowType int

const (
	Tumbling WindowType = iota
	Sliding
	Session
)

// FeatureValue is a simplified feature value for join operations.
type FeatureValue struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"`
	Version   int64       `json:"version,omitempty"`
}

// JoinConfig configures a join plan.
type JoinConfig struct {
	LeftEntity  string        `json:"left_entity"`
	RightEntity string        `json:"right_entity"`
	JoinKey     string        `json:"join_key"`
	JoinType    JoinType      `json:"join_type"`
	Window      time.Duration `json:"window"`
	Watermark   time.Duration `json:"watermark"`
}

// JoinPlan represents a configured join operation.
type JoinPlan struct {
	ID        string     `json:"id"`
	Config    JoinConfig `json:"config"`
	CreatedAt time.Time  `json:"created_at"`
	Status    string     `json:"status"`
}

// JoinResult represents a single joined entity.
type JoinResult struct {
	EntityKey     string                   `json:"entity_key"`
	LeftFeatures  map[string]*FeatureValue `json:"left_features,omitempty"`
	RightFeatures map[string]*FeatureValue `json:"right_features,omitempty"`
	JoinedAt      time.Time                `json:"joined_at"`
	Watermark     time.Time                `json:"watermark"`
}

// JoinOutput contains the results of a join execution.
type JoinOutput struct {
	Results           []JoinResult  `json:"results"`
	LeftUnmatched     int           `json:"left_unmatched"`
	RightUnmatched    int           `json:"right_unmatched"`
	ExecutionTime     time.Duration `json:"execution_time"`
	WatermarkPosition time.Time     `json:"watermark_position"`
}

// EngineConfig configures the join engine.
type EngineConfig struct {
	MaxPlans         int           `json:"max_plans"`
	DefaultWindow    time.Duration `json:"default_window"`
	DefaultWatermark time.Duration `json:"default_watermark"`
	MaxBufferSize    int           `json:"max_buffer_size"`
}

// DefaultEngineConfig returns sensible defaults for the join engine.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxPlans:         100,
		DefaultWindow:    5 * time.Minute,
		DefaultWatermark: 1 * time.Minute,
		MaxBufferSize:    10000,
	}
}

// Engine manages join plans and executes temporal joins.
type Engine struct {
	mu     sync.RWMutex
	plans  map[string]*JoinPlan
	config EngineConfig
}

// NewEngine creates a new join engine with the given configuration.
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		plans:  make(map[string]*JoinPlan),
		config: cfg,
	}
}

// CreatePlan validates and stores a new join plan.
func (e *Engine) CreatePlan(cfg JoinConfig) (*JoinPlan, error) {
	if cfg.LeftEntity == "" {
		return nil, fmt.Errorf("creating join plan: left_entity is required")
	}
	if cfg.RightEntity == "" {
		return nil, fmt.Errorf("creating join plan: right_entity is required")
	}
	if cfg.JoinKey == "" {
		return nil, fmt.Errorf("creating join plan: join_key is required")
	}

	if cfg.Window == 0 {
		cfg.Window = e.config.DefaultWindow
	}
	if cfg.Watermark == 0 {
		cfg.Watermark = e.config.DefaultWatermark
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.plans) >= e.config.MaxPlans {
		return nil, fmt.Errorf("creating join plan: max plans (%d) reached", e.config.MaxPlans)
	}

	plan := &JoinPlan{
		ID:        uuid.New().String(),
		Config:    cfg,
		CreatedAt: time.Now(),
		Status:    "active",
	}
	e.plans[plan.ID] = plan
	return plan, nil
}

// GetPlan retrieves a join plan by ID.
func (e *Engine) GetPlan(id string) (*JoinPlan, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plan, ok := e.plans[id]
	if !ok {
		return nil, fmt.Errorf("getting join plan: plan %q not found", id)
	}
	return plan, nil
}

// ListPlans returns all join plans.
func (e *Engine) ListPlans() []*JoinPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plans := make([]*JoinPlan, 0, len(e.plans))
	for _, p := range e.plans {
		plans = append(plans, p)
	}
	return plans
}

// DeletePlan removes a join plan by ID.
func (e *Engine) DeletePlan(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.plans[id]; !ok {
		return fmt.Errorf("deleting join plan: plan %q not found", id)
	}
	delete(e.plans, id)
	return nil
}

// ExecuteJoin performs a temporal join using the specified plan.
// leftData and rightData map entity keys to feature name/value pairs.
func (e *Engine) ExecuteJoin(ctx context.Context, planID string, leftData, rightData map[string]map[string]*FeatureValue) (*JoinOutput, error) {
	start := time.Now()

	e.mu.RLock()
	plan, ok := e.plans[planID]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("executing join: plan %q not found", planID)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("executing join: %w", ctx.Err())
	default:
	}

	watermarkCutoff := time.Now().Add(-plan.Config.Watermark)
	windowNanos := plan.Config.Window.Nanoseconds()

	var results []JoinResult
	leftMatched := make(map[string]bool)
	rightMatched := make(map[string]bool)

	// Match left entities against right entities.
	for entityKey, leftFeatures := range leftData {
		if skipByWatermark(leftFeatures, watermarkCutoff) {
			continue
		}

		rightFeatures, hasRight := rightData[entityKey]
		if hasRight && !skipByWatermark(rightFeatures, watermarkCutoff) && withinWindow(leftFeatures, rightFeatures, windowNanos) {
			leftMatched[entityKey] = true
			rightMatched[entityKey] = true
			results = append(results, JoinResult{
				EntityKey:     entityKey,
				LeftFeatures:  leftFeatures,
				RightFeatures: rightFeatures,
				JoinedAt:      time.Now(),
				Watermark:     watermarkCutoff,
			})
		} else if plan.Config.JoinType == Left || plan.Config.JoinType == Full {
			leftMatched[entityKey] = true
			results = append(results, JoinResult{
				EntityKey:    entityKey,
				LeftFeatures: leftFeatures,
				JoinedAt:     time.Now(),
				Watermark:    watermarkCutoff,
			})
		}
	}

	// For right/full joins, emit unmatched right entities.
	if plan.Config.JoinType == Right || plan.Config.JoinType == Full {
		for entityKey, rightFeatures := range rightData {
			if rightMatched[entityKey] {
				continue
			}
			if skipByWatermark(rightFeatures, watermarkCutoff) {
				continue
			}
			results = append(results, JoinResult{
				EntityKey:     entityKey,
				RightFeatures: rightFeatures,
				JoinedAt:      time.Now(),
				Watermark:     watermarkCutoff,
			})
		}
	}

	leftUnmatched := 0
	for entityKey := range leftData {
		if !leftMatched[entityKey] {
			leftUnmatched++
		}
	}
	rightUnmatched := 0
	for entityKey := range rightData {
		if !rightMatched[entityKey] {
			rightUnmatched++
		}
	}

	return &JoinOutput{
		Results:           results,
		LeftUnmatched:     leftUnmatched,
		RightUnmatched:    rightUnmatched,
		ExecutionTime:     time.Since(start),
		WatermarkPosition: watermarkCutoff,
	}, nil
}

// skipByWatermark returns true if all feature timestamps are older than the watermark cutoff.
func skipByWatermark(features map[string]*FeatureValue, cutoff time.Time) bool {
	cutoffUnix := cutoff.UnixMilli()
	for _, fv := range features {
		if fv.Timestamp >= cutoffUnix {
			return false
		}
	}
	return true
}

// withinWindow returns true if any pair of left/right features has timestamps within the window.
func withinWindow(left, right map[string]*FeatureValue, windowNanos int64) bool {
	windowMillis := windowNanos / int64(time.Millisecond)
	for _, lv := range left {
		for _, rv := range right {
			diff := lv.Timestamp - rv.Timestamp
			if diff < 0 {
				diff = -diff
			}
			if diff <= windowMillis {
				return true
			}
		}
	}
	return false
}
