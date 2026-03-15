package aggregation

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// Engine computes time-window aggregations.
type Engine struct {
	// Entity -> Feature -> WindowManager
	windows map[string]map[string]*WindowManager
	specs   map[string]*domain.AggregationSpec
	mu      sync.RWMutex
}

// NewEngine creates a new aggregation engine.
func NewEngine() *Engine {
	return &Engine{
		windows: make(map[string]map[string]*WindowManager),
		specs:   make(map[string]*domain.AggregationSpec),
	}
}

// RegisterAggregation registers a feature for aggregation.
func (e *Engine) RegisterAggregation(featureName string, spec *domain.AggregationSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.specs[featureName] = spec
}

// GetSpec returns the aggregation spec for a feature.
func (e *Engine) GetSpec(featureName string) *domain.AggregationSpec {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.specs[featureName]
}

// Update adds a value to the aggregation.
func (e *Engine) Update(entityKey, featureName string, value float64, timestamp time.Time) error {
	wm, err := e.getOrCreateWindowManager(entityKey, featureName)
	if err != nil {
		return err
	}
	wm.Update(value, timestamp)
	return nil
}

// getOrCreateWindowManager returns the WindowManager for an entity/feature pair,
// creating it if necessary.
func (e *Engine) getOrCreateWindowManager(entityKey, featureName string) (*WindowManager, error) {
	e.mu.Lock()
	spec, ok := e.specs[featureName]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("no aggregation spec for feature %s", featureName)
	}

	entityWindows, ok := e.windows[entityKey]
	if !ok {
		entityWindows = make(map[string]*WindowManager)
		e.windows[entityKey] = entityWindows
	}

	wm, ok := entityWindows[featureName]
	if !ok {
		wm = NewWindowManager(spec)
		entityWindows[featureName] = wm
	}
	e.mu.Unlock()

	return wm, nil
}

// Compute returns the aggregated value for a feature.
func (e *Engine) Compute(entityKey, featureName string, function domain.AggFunction) (float64, error) {
	wm, err := e.getWindowManager(entityKey, featureName)
	if err != nil {
		return 0, err
	}
	return wm.Compute(function)
}

// getWindowManager returns the WindowManager for an entity/feature pair.
func (e *Engine) getWindowManager(entityKey, featureName string) (*WindowManager, error) {
	e.mu.RLock()
	entityWindows, ok := e.windows[entityKey]
	if !ok {
		e.mu.RUnlock()
		return nil, domain.ErrEntityNotFound
	}

	wm, ok := entityWindows[featureName]
	if !ok {
		e.mu.RUnlock()
		return nil, domain.ErrFeatureNotFound
	}
	e.mu.RUnlock()

	return wm, nil
}

// ComputeWithSpec computes using the registered spec's function.
func (e *Engine) ComputeWithSpec(entityKey, featureName string) (float64, error) {
	e.mu.RLock()
	spec, ok := e.specs[featureName]
	e.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("no aggregation spec for feature %s", featureName)
	}

	return e.Compute(entityKey, featureName, spec.Function)
}

// EvictInactive removes window managers for entities that haven't been updated
// within maxAge. Returns the number of entity-feature pairs evicted.
func (e *Engine) EvictInactive(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge).UnixNano()
	evicted := 0

	e.mu.Lock()
	defer e.mu.Unlock()

	for entityKey, featureWindows := range e.windows {
		for featureName, wm := range featureWindows {
			wm.mu.RLock()
			lastUpdate := wm.lastUpdateTime
			wm.mu.RUnlock()

			if lastUpdate > 0 && lastUpdate < cutoff {
				delete(featureWindows, featureName)
				evicted++
			}
		}
		if len(featureWindows) == 0 {
			delete(e.windows, entityKey)
		}
	}

	return evicted
}

// WindowCount returns the total number of active entity-feature window managers.
func (e *Engine) WindowCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, featureWindows := range e.windows {
		count += len(featureWindows)
	}
	return count
}

// WindowManager manages sliding windows for a feature.
type WindowManager struct {
	spec           *domain.AggregationSpec
	buckets        *RingBuffer
	bucketSize     time.Duration
	lastUpdateTime int64 // UnixNano of last Update call
	mu             sync.RWMutex
}

// NewWindowManager creates a new window manager.
func NewWindowManager(spec *domain.AggregationSpec) *WindowManager {
	// Calculate bucket size (1 minute buckets for windows < 1 hour, etc.)
	bucketSize := calculateBucketSize(spec.Window)
	if spec.SlideBy > 0 && spec.SlideBy < bucketSize {
		bucketSize = spec.SlideBy
	}
	numBuckets := int(spec.Window/bucketSize) + 1

	return &WindowManager{
		spec:       spec,
		buckets:    NewRingBuffer(numBuckets),
		bucketSize: bucketSize,
	}
}

func calculateBucketSize(window time.Duration) time.Duration {
	switch {
	case window <= time.Hour:
		return time.Minute
	case window <= 24*time.Hour:
		return time.Hour
	default:
		return 24 * time.Hour
	}
}

// Update adds a value to the window.
func (w *WindowManager) Update(value float64, timestamp time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastUpdateTime = timestamp.UnixNano()
	bucketStart := timestamp.Truncate(w.bucketSize).UnixNano()

	// Check if we need to add to existing bucket or create new one
	latest := w.buckets.GetLatest()
	if latest == nil || latest.StartTime != bucketStart {
		// Rotate buckets if needed
		w.maybeRotate(timestamp)

		// Create new bucket
		w.buckets.Push(AggregationBucket{
			StartTime: bucketStart,
			Count:     1,
			Sum:       value,
			Min:       value,
			Max:       value,
			LastValue: value,
		})
	} else {
		// Update existing bucket
		latest.Count++
		latest.Sum += value
		if value < latest.Min {
			latest.Min = value
		}
		if value > latest.Max {
			latest.Max = value
		}
		latest.LastValue = value
	}
}

// maybeRotate removes old buckets outside the window.
func (w *WindowManager) maybeRotate(now time.Time) {
	windowStart := now.Add(-w.spec.Window).UnixNano()

	// Remove buckets older than window
	for w.buckets.Size() > 0 {
		bucket := w.buckets.Get(0) // Always check oldest
		if bucket == nil || bucket.StartTime >= windowStart {
			break
		}
		w.buckets.PopOldest()
	}
}

// Compute returns the aggregated value for the window.
func (w *WindowManager) Compute(function domain.AggFunction) (float64, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	slideBy := w.spec.SlideBy
	if slideBy <= 0 {
		slideBy = w.spec.Window
	}
	windowEnd := time.Now().Truncate(slideBy)
	windowStart := windowEnd.Add(-w.spec.Window).UnixNano()

	var count int64
	var sum float64
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64
	var lastValue float64
	var hasData bool

	w.buckets.Range(func(bucket *AggregationBucket) bool {
		if bucket.StartTime >= windowStart {
			count += bucket.Count
			sum += bucket.Sum
			if bucket.Min < minVal {
				minVal = bucket.Min
			}
			if bucket.Max > maxVal {
				maxVal = bucket.Max
			}
			lastValue = bucket.LastValue
			hasData = true
		}
		return true
	})

	if !hasData {
		return 0, domain.ErrNoData
	}

	switch function {
	case domain.AggCount:
		return float64(count), nil
	case domain.AggSum:
		if math.IsInf(sum, 0) || math.IsNaN(sum) {
			return 0, fmt.Errorf("aggregation overflow: sum is %v", sum)
		}
		return sum, nil
	case domain.AggAvg:
		if count == 0 {
			return 0, domain.ErrNoData
		}
		avg := sum / float64(count)
		if math.IsInf(avg, 0) || math.IsNaN(avg) {
			return 0, fmt.Errorf("aggregation overflow: avg is %v", avg)
		}
		return avg, nil
	case domain.AggMin:
		if minVal == math.MaxFloat64 {
			return 0, domain.ErrNoData
		}
		return minVal, nil
	case domain.AggMax:
		if maxVal == -math.MaxFloat64 {
			return 0, domain.ErrNoData
		}
		return maxVal, nil
	case domain.AggLast:
		return lastValue, nil
	default:
		return 0, fmt.Errorf("unknown aggregation function: %s", function)
	}
}

// GetBucketCount returns the number of active buckets.
func (w *WindowManager) GetBucketCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.buckets.Size()
}
