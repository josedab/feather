package computegraph

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// DeclarativeGraph provides a high-level API for declaring feature
// computation graphs where feature B = f(feature A, feature C) with
// automatic recomputation on upstream changes.
type DeclarativeGraph struct {
	engine      *Engine
	memoizer    *Memoizer
	incremental *IncrementalEngine
	mu          sync.RWMutex
	triggers    map[string][]RecomputeTrigger
	schedules   map[string]*ScheduleEntry
	stats       DeclarativeGraphStats
}

// RecomputeTrigger defines when a derived feature should be recomputed.
type RecomputeTrigger struct {
	Type     TriggerType `json:"type"`
	Source   string      `json:"source,omitempty"`
	Interval string     `json:"interval,omitempty"`
}

// TriggerType classifies recomputation triggers.
type TriggerType string

const (
	TriggerOnChange  TriggerType = "on_change"  // Recompute when upstream changes
	TriggerScheduled TriggerType = "scheduled"  // Recompute on a schedule
	TriggerManual    TriggerType = "manual"      // Recompute only when requested
)

// ScheduleEntry tracks a scheduled recomputation.
type ScheduleEntry struct {
	NodeName string    `json:"node_name"`
	Interval string    `json:"interval"`
	LastRun  time.Time `json:"last_run"`
	NextRun  time.Time `json:"next_run"`
}

// DeclarativeGraphStats tracks graph statistics.
type DeclarativeGraphStats struct {
	TotalNodes           int   `json:"total_nodes"`
	TotalEdges           int   `json:"total_edges"`
	TotalRecomputations  int64 `json:"total_recomputations"`
	CacheHits            int64 `json:"cache_hits"`
	CacheMisses          int64 `json:"cache_misses"`
	AvgRecomputeTimeMs   float64 `json:"avg_recompute_time_ms"`
	totalRecomputeMs     int64
}

// FeatureDefinition declares a feature in the computation graph.
type FeatureDefinition struct {
	Name        string            `json:"name" yaml:"name"`
	Kind        NodeKind          `json:"kind" yaml:"kind"`
	Expression  string            `json:"expression,omitempty" yaml:"expression,omitempty"`
	Inputs      []string          `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Function    ComputeFunc       `json:"function,omitempty" yaml:"function,omitempty"`
	OutputType  string            `json:"output_type" yaml:"output_type"`
	Triggers    []RecomputeTrigger `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// GraphSpec declares a complete computation graph (distinct from DSL GraphDefinition).
type GraphSpec struct {
	Name        string              `json:"name" yaml:"name"`
	Version     string              `json:"version" yaml:"version"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Features    []FeatureDefinition `json:"features" yaml:"features"`
}

// ApplyGraphResult summarizes the outcome of applying a graph definition.
type ApplyGraphResult struct {
	NodesAdded   int      `json:"nodes_added"`
	NodesUpdated int      `json:"nodes_updated"`
	Errors       []string `json:"errors,omitempty"`
}

// NewDeclarativeGraph creates a new declarative computation graph.
func NewDeclarativeGraph(engine *Engine, memoizer *Memoizer) *DeclarativeGraph {
	incr := NewIncrementalEngine(engine, DefaultIncrementalConfig())
	return &DeclarativeGraph{
		engine:      engine,
		memoizer:    memoizer,
		incremental: incr,
		triggers:    make(map[string][]RecomputeTrigger),
		schedules:   make(map[string]*ScheduleEntry),
	}
}

// ApplyDefinition applies a graph spec, adding or updating nodes.
func (g *DeclarativeGraph) ApplyDefinition(def GraphSpec) *ApplyGraphResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	result := &ApplyGraphResult{}
	for _, fd := range def.Features {
		node := FeatureNode{
			Name:       fd.Name,
			Kind:       fd.Kind,
			Inputs:     fd.Inputs,
			Function:   fd.Function,
			Expression: fd.Expression,
			OutputType: fd.OutputType,
			Metadata:   fd.Metadata,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if _, err := g.engine.GetNode(fd.Name); err == nil {
			if err := g.engine.RemoveNode(fd.Name); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("removing %s: %v", fd.Name, err))
				continue
			}
			result.NodesUpdated++
		} else {
			result.NodesAdded++
		}

		if err := g.engine.AddNode(node); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("adding %s: %v", fd.Name, err))
			result.NodesAdded--
			continue
		}

		if len(fd.Triggers) > 0 {
			g.triggers[fd.Name] = fd.Triggers
			for _, trigger := range fd.Triggers {
				if trigger.Type == TriggerOnChange && trigger.Source != "" {
					g.incremental.OnChange(trigger.Source, func(evt ChangeEvent) {
						g.recompute(fd.Name)
					})
				}
			}
		}
	}

	dag := g.engine.GetDAG()
	g.stats.TotalNodes = len(dag.Nodes)
	g.stats.TotalEdges = len(dag.Edges)
	return result
}

// Compute evaluates a feature node with memoization.
func (g *DeclarativeGraph) Compute(ctx context.Context, name string, inputs map[string]interface{}) (*ComputeResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Build a deterministic cache key by sorting input keys.
	cacheKey := buildCacheKey(name, inputs)
	if cached, ok := g.memoizer.Get(cacheKey); ok {
		g.mu.Lock()
		g.stats.CacheHits++
		g.mu.Unlock()
		if result, ok := cached.(*ComputeResult); ok {
			return result, nil
		}
	}

	g.mu.Lock()
	g.stats.CacheMisses++
	g.mu.Unlock()

	start := time.Now()
	result, err := g.engine.Compute(name, inputs)
	elapsed := time.Since(start).Milliseconds()

	g.mu.Lock()
	g.stats.TotalRecomputations++
	g.stats.totalRecomputeMs += elapsed
	if g.stats.TotalRecomputations > 0 {
		g.stats.AvgRecomputeTimeMs = float64(g.stats.totalRecomputeMs) / float64(g.stats.TotalRecomputations)
	}
	g.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("computing %s: %w", name, err)
	}

	if result != nil {
		g.memoizer.Put(cacheKey, result)
	}
	return result, nil
}

// ComputeAll evaluates all nodes in topological order.
func (g *DeclarativeGraph) ComputeAll(ctx context.Context, inputs map[string]interface{}) (map[string]*ComputeResult, error) {
	order, err := g.engine.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	results := make(map[string]*ComputeResult, len(order.Order))
	for _, name := range order.Order {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}
		result, err := g.Compute(ctx, name, inputs)
		if err != nil {
			return results, fmt.Errorf("computing %s: %w", name, err)
		}
		results[name] = result
	}
	return results, nil
}

// GetExecutionPlan returns the topological execution order.
func (g *DeclarativeGraph) GetExecutionPlan() (*TopologicalOrder, error) {
	return g.engine.TopologicalSort()
}

// GetLineage returns upstream and downstream dependencies for a node.
func (g *DeclarativeGraph) GetLineage(name string) map[string]interface{} {
	upstream, _ := g.engine.GetUpstream(name)
	downstream, _ := g.engine.GetDownstream(name)
	return map[string]interface{}{
		"upstream":   upstream,
		"downstream": downstream,
		"triggers":   g.triggers[name],
	}
}

// InvalidateAndRecompute invalidates a node and recomputes its dependents.
func (g *DeclarativeGraph) InvalidateAndRecompute(name string, inputs map[string]interface{}) (int, error) {
	affected := g.engine.Invalidate(name)
	g.memoizer.Invalidate(name)
	return affected, nil
}

// Stats returns declarative graph statistics.
func (g *DeclarativeGraph) Stats() DeclarativeGraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.stats
}

func (g *DeclarativeGraph) recompute(name string) {
	g.mu.Lock()
	g.stats.TotalRecomputations++
	g.mu.Unlock()
	g.memoizer.Invalidate(name)
	if _, err := g.engine.Compute(name, nil); err != nil {
		slog.Debug("recompute failed", "node", name, "error", err)
	}
}

// buildCacheKey creates a deterministic cache key from node name and inputs.
func buildCacheKey(name string, inputs map[string]interface{}) string {
	if len(inputs) == 0 {
		return name
	}
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		fmt.Fprintf(&b, ":%s=%v", k, inputs[k])
	}
	return b.String()
}
