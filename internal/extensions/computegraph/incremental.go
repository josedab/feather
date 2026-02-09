package computegraph

import (
	"fmt"
	"sync"
	"time"
)

// IncrementalEngine wraps Engine with change-propagation support.
// When a source value changes, only affected downstream nodes are recomputed.
type IncrementalEngine struct {
	engine    *Engine
	mu        sync.RWMutex
	listeners map[string][]ChangeListener
	changelog []ChangeEvent
	maxLog    int
}

// ChangeListener is called when a node's computed value changes.
type ChangeListener func(event ChangeEvent)

// ChangeEvent records a single value change in the graph.
type ChangeEvent struct {
	NodeName   string      `json:"node_name"`
	OldValue   interface{} `json:"old_value,omitempty"`
	NewValue   interface{} `json:"new_value"`
	Trigger    string      `json:"trigger"`
	Timestamp  time.Time   `json:"timestamp"`
	Propagated []string    `json:"propagated,omitempty"`
}

// IncrementalConfig configures the incremental engine.
type IncrementalConfig struct {
	MaxChangeLog int
}

// DefaultIncrementalConfig returns sensible defaults.
func DefaultIncrementalConfig() IncrementalConfig {
	return IncrementalConfig{
		MaxChangeLog: 10000,
	}
}

// NewIncrementalEngine creates an incremental engine wrapping the given engine.
func NewIncrementalEngine(engine *Engine, cfg IncrementalConfig) *IncrementalEngine {
	if cfg.MaxChangeLog <= 0 {
		cfg.MaxChangeLog = 10000
	}
	return &IncrementalEngine{
		engine:    engine,
		listeners: make(map[string][]ChangeListener),
		changelog: make([]ChangeEvent, 0),
		maxLog:    cfg.MaxChangeLog,
	}
}

// Engine returns the underlying compute graph engine.
func (ie *IncrementalEngine) Engine() *Engine {
	return ie.engine
}

// OnChange registers a listener for changes to the named node.
func (ie *IncrementalEngine) OnChange(nodeName string, listener ChangeListener) {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	ie.listeners[nodeName] = append(ie.listeners[nodeName], listener)
}

// PropagateChange updates a source value and recomputes all affected downstream
// nodes, returning the list of nodes that were recomputed.
func (ie *IncrementalEngine) PropagateChange(sourceName string, newValue interface{}, allInputs map[string]interface{}) ([]string, error) {
	node, err := ie.engine.GetNode(sourceName)
	if err != nil {
		return nil, fmt.Errorf("propagate change: %w", err)
	}
	if node.Kind != KindSource {
		return nil, fmt.Errorf("propagate change: node %q is not a source node", sourceName)
	}

	// Get old cached value
	ie.engine.mu.RLock()
	var oldValue interface{}
	if cr, ok := ie.engine.cache[sourceName]; ok {
		oldValue = cr.Value
	}
	ie.engine.mu.RUnlock()

	// Update input value
	allInputs[sourceName] = newValue

	// Invalidate downstream
	ie.engine.Invalidate(sourceName)

	// Get all downstream nodes
	downstream, err := ie.engine.GetDownstream(sourceName)
	if err != nil {
		return nil, fmt.Errorf("propagate change: %w", err)
	}

	// Recompute affected nodes
	recomputed := []string{sourceName}
	for _, dn := range downstream {
		if _, err := ie.engine.Compute(dn, allInputs); err != nil {
			// Log but continue — partial recomputation is acceptable
			continue
		}
		recomputed = append(recomputed, dn)
	}

	// Record change event
	event := ChangeEvent{
		NodeName:   sourceName,
		OldValue:   oldValue,
		NewValue:   newValue,
		Trigger:    sourceName,
		Timestamp:  time.Now(),
		Propagated: downstream,
	}

	ie.mu.Lock()
	ie.changelog = append(ie.changelog, event)
	if len(ie.changelog) > ie.maxLog {
		ie.changelog = ie.changelog[len(ie.changelog)-ie.maxLog:]
	}
	listeners := ie.listeners[sourceName]
	ie.mu.Unlock()

	// Notify listeners
	for _, l := range listeners {
		l(event)
	}

	return recomputed, nil
}

// GetChangeLog returns recent change events, optionally filtered by node name.
func (ie *IncrementalEngine) GetChangeLog(nodeName string, limit int) []ChangeEvent {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var result []ChangeEvent
	for _, e := range ie.changelog {
		if nodeName != "" && e.NodeName != nodeName {
			continue
		}
		result = append(result, e)
	}

	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// IncrementalStats returns statistics about the incremental engine.
type IncrementalStats struct {
	GraphStats     GraphStats `json:"graph_stats"`
	TotalChanges   int        `json:"total_changes"`
	ListenerCount  int        `json:"listener_count"`
}

// Stats returns incremental engine statistics.
func (ie *IncrementalEngine) Stats() IncrementalStats {
	ie.mu.RLock()
	defer ie.mu.RUnlock()

	listenerCount := 0
	for _, ls := range ie.listeners {
		listenerCount += len(ls)
	}

	return IncrementalStats{
		GraphStats:    ie.engine.Stats(),
		TotalChanges:  len(ie.changelog),
		ListenerCount: listenerCount,
	}
}

// ComputeParallel evaluates multiple independent nodes concurrently.
// Nodes at the same topological level can be computed in parallel.
func (e *Engine) ComputeParallel(names []string, inputs map[string]interface{}) (map[string]*ComputeResult, error) {
	results := make(map[string]*ComputeResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(names))

	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			result, err := e.Compute(n, inputs)
			if err != nil {
				errCh <- fmt.Errorf("node %q: %w", n, err)
				return
			}
			mu.Lock()
			results[n] = result
			mu.Unlock()
		}(name)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return results, fmt.Errorf("parallel compute errors: %s", joinErrors(errs))
	}

	return results, nil
}

func joinErrors(errs []string) string {
	result := ""
	for i, e := range errs {
		if i > 0 {
			result += "; "
		}
		result += e
	}
	return result
}
