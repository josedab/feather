// Package queryplanner provides cost-based, self-optimizing query planning
// with adaptive replanning when actual execution costs drift from estimates.
package queryplanner

import (
	"container/list"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// OperationType represents the kind of operation in a query.
type OperationType string

const (
	OpLookup    OperationType = "lookup"
	OpCompute   OperationType = "compute"
	OpAggregate OperationType = "aggregate"
	OpJoin      OperationType = "join"
	OpTransform OperationType = "transform"
)

// Config holds tuning parameters for the query planner.
type Config struct {
	MaxPlanCacheSize        int
	CostDriftThreshold      float64
	EnableAdaptiveReplanning bool
	MaxPlanAlternatives     int
	StatisticsWindowSize    int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxPlanCacheSize:        1000,
		CostDriftThreshold:      0.5,
		EnableAdaptiveReplanning: true,
		MaxPlanAlternatives:     10,
		StatisticsWindowSize:    10000,
	}
}

// Query describes a feature retrieval request to be optimized.
type Query struct {
	ID         string
	Operations []Operation
	Features   []string
	Entities   []string
	Filters    []Filter
}

// Operation is a single unit of work within a query.
type Operation struct {
	Type       OperationType
	Feature    string
	Parameters map[string]interface{}
}

// Filter constrains the rows processed by a query.
type Filter struct {
	Feature  string
	Operator string
	Value    interface{}
}

// ExecutionPlan is the optimized execution strategy for a query.
type ExecutionPlan struct {
	ID             string
	Steps          []PlanStep
	EstimatedCostMs float64
	EstimatedRows  int64
	createdAt      time.Time
}

// PlanStep is one stage of an execution plan.
type PlanStep struct {
	Type         OperationType
	Operation    Operation
	Input        []string
	Output       string
	CostEstimate float64
}

// PlannerStats exposes runtime statistics about the planner.
type PlannerStats struct {
	TotalOptimizations int64
	CacheHits          int64
	CacheMisses        int64
	AvgPlanCostMs      float64
	ReplanCount        int64
	ActivePlans        int
}

// opStats tracks the exponential moving average for a single operation type.
type opStats struct {
	avgCostMs  float64
	avgRows    float64
	sampleCount int64
}

// executionRecord stores execution feedback for adaptive replanning.
type executionRecord struct {
	estimatedCostMs float64
	actualCostMs    float64
	estimatedRows   int64
	actualRows      int64
}

// planCacheEntry is an element stored in the LRU plan cache.
type planCacheEntry struct {
	queryKey string
	plan     *ExecutionPlan
}

// Planner performs cost-based query optimization with adaptive replanning.
type Planner struct {
	cfg Config

	mu       sync.RWMutex
	costModel map[string]*opStats        // keyed by operation type
	records   map[string]*executionRecord // keyed by plan ID

	// LRU plan cache
	cacheMu   sync.Mutex
	cacheMap  map[string]*list.Element
	cacheList *list.List

	// Counters (atomic for lock-free reads).
	totalOptimizations atomic.Int64
	cacheHits          atomic.Int64
	cacheMisses        atomic.Int64
	totalPlanCostMs    atomic.Int64 // stored as microseconds for precision
	replanCount        atomic.Int64
}

// New creates a Planner with the given configuration.
func New(cfg Config) *Planner {
	return &Planner{
		cfg:       cfg,
		costModel: make(map[string]*opStats),
		records:   make(map[string]*executionRecord),
		cacheMap:  make(map[string]*list.Element),
		cacheList: list.New(),
	}
}

// RecordOperationCost feeds observed latency into the cost model using an
// exponential moving average (alpha = 2 / (window + 1)).
func (p *Planner) RecordOperationCost(opType string, durationMs float64, rowCount int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.costModel[opType]
	if !ok {
		s = &opStats{}
		p.costModel[opType] = s
	}

	s.sampleCount++
	if s.sampleCount == 1 {
		s.avgCostMs = durationMs
		s.avgRows = float64(rowCount)
		return
	}

	alpha := 2.0 / (float64(min(s.sampleCount, int64(p.cfg.StatisticsWindowSize))) + 1.0)
	s.avgCostMs = alpha*durationMs + (1-alpha)*s.avgCostMs
	s.avgRows = alpha*float64(rowCount) + (1-alpha)*s.avgRows
}

// EstimateCost returns the predicted cost in milliseconds for an operation
// based on the current cost model. Unknown operations get a conservative default.
func (p *Planner) EstimateCost(op Operation) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	s, ok := p.costModel[string(op.Type)]
	if !ok {
		return defaultCostMs(op.Type)
	}
	return s.avgCostMs
}

// Optimize produces the lowest-cost ExecutionPlan for the given query.
// It checks the plan cache first, then generates and evaluates alternatives.
func (p *Planner) Optimize(query Query) (*ExecutionPlan, error) {
	if len(query.Operations) == 0 {
		return nil, fmt.Errorf("queryplanner: query has no operations")
	}

	key := queryKey(query)

	// Check cache.
	if plan, ok := p.cacheGet(key); ok {
		p.cacheHits.Add(1)
		p.totalOptimizations.Add(1)
		return plan, nil
	}
	p.cacheMisses.Add(1)

	alternatives := p.generateAlternatives(query)
	if len(alternatives) == 0 {
		return nil, fmt.Errorf("queryplanner: could not generate any execution plan")
	}

	best := alternatives[0]
	for _, alt := range alternatives[1:] {
		if alt.EstimatedCostMs < best.EstimatedCostMs {
			best = alt
		}
	}

	p.cachePut(key, best)
	p.totalOptimizations.Add(1)
	p.totalPlanCostMs.Add(int64(best.EstimatedCostMs * 1000))

	return best, nil
}

// RecordExecutionResult stores actual execution feedback so the planner can
// detect cost drift and trigger replanning.
func (p *Planner) RecordExecutionResult(planID string, actualCostMs float64, actualRows int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	rec, ok := p.records[planID]
	if !ok {
		rec = &executionRecord{}
		p.records[planID] = rec
	}
	rec.actualCostMs = actualCostMs
	rec.actualRows = actualRows

	// Carry over estimated values from the cached plan if available.
	p.cacheMu.Lock()
	for _, elem := range p.cacheMap {
		entry := elem.Value.(*planCacheEntry)
		if entry.plan.ID == planID {
			rec.estimatedCostMs = entry.plan.EstimatedCostMs
			rec.estimatedRows = entry.plan.EstimatedRows
			break
		}
	}
	p.cacheMu.Unlock()
}

// ShouldReplan returns true when the actual execution cost of a plan has
// drifted beyond the configured threshold relative to its estimate.
func (p *Planner) ShouldReplan(planID string) bool {
	if !p.cfg.EnableAdaptiveReplanning {
		return false
	}

	p.mu.RLock()
	rec, ok := p.records[planID]
	p.mu.RUnlock()
	if !ok || rec.estimatedCostMs == 0 {
		return false
	}

	drift := math.Abs(rec.actualCostMs-rec.estimatedCostMs) / rec.estimatedCostMs
	if drift > p.cfg.CostDriftThreshold {
		p.replanCount.Add(1)
		p.invalidatePlan(planID)
		return true
	}
	return false
}

// Stats returns a snapshot of planner statistics.
func (p *Planner) Stats() PlannerStats {
	total := p.totalOptimizations.Load()
	var avg float64
	if total > 0 {
		avg = float64(p.totalPlanCostMs.Load()) / (float64(total) * 1000)
	}

	p.cacheMu.Lock()
	active := p.cacheList.Len()
	p.cacheMu.Unlock()

	return PlannerStats{
		TotalOptimizations: total,
		CacheHits:          p.cacheHits.Load(),
		CacheMisses:        p.cacheMisses.Load(),
		AvgPlanCostMs:      avg,
		ReplanCount:        p.replanCount.Load(),
		ActivePlans:        active,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// generateAlternatives creates up to MaxPlanAlternatives candidate plans by
// exploring different operation orderings.
func (p *Planner) generateAlternatives(query Query) []*ExecutionPlan {
	var plans []*ExecutionPlan

	// Baseline plan: operations in the order given.
	base := p.buildPlan(query, query.Operations)
	plans = append(plans, base)

	// Try pushing filters earlier (predicate pushdown).
	if len(query.Filters) > 0 {
		reordered := p.reorderWithFilterPushdown(query.Operations)
		if !sameOrder(reordered, query.Operations) {
			plans = append(plans, p.buildPlan(query, reordered))
		}
	}

	// Try sorting operations by estimated ascending cost (greedy).
	sorted := p.sortByCost(query.Operations)
	if !sameOrder(sorted, query.Operations) {
		plans = append(plans, p.buildPlan(query, sorted))
	}

	// Cap at MaxPlanAlternatives.
	if len(plans) > p.cfg.MaxPlanAlternatives {
		plans = plans[:p.cfg.MaxPlanAlternatives]
	}
	return plans
}

func (p *Planner) buildPlan(query Query, ops []Operation) *ExecutionPlan {
	steps := make([]PlanStep, 0, len(ops))
	var totalCost float64
	var totalRows int64

	for i, op := range ops {
		cost := p.EstimateCost(op)
		rows := p.estimateRows(op)
		step := PlanStep{
			Type:         op.Type,
			Operation:    op,
			Input:        stepInputs(i, ops),
			Output:       fmt.Sprintf("step_%d", i),
			CostEstimate: cost,
		}
		steps = append(steps, step)
		totalCost += cost
		totalRows += rows
	}

	return &ExecutionPlan{
		ID:              fmt.Sprintf("plan_%s_%d", query.ID, time.Now().UnixNano()),
		Steps:           steps,
		EstimatedCostMs: totalCost,
		EstimatedRows:   totalRows,
		createdAt:       time.Now(),
	}
}

func (p *Planner) estimateRows(op Operation) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if s, ok := p.costModel[string(op.Type)]; ok && s.avgRows > 0 {
		return int64(s.avgRows)
	}
	return 100 // conservative default
}

// reorderWithFilterPushdown moves lookup operations before compute/aggregate.
func (p *Planner) reorderWithFilterPushdown(ops []Operation) []Operation {
	out := make([]Operation, 0, len(ops))
	var rest []Operation
	for _, op := range ops {
		if op.Type == OpLookup {
			out = append(out, op)
		} else {
			rest = append(rest, op)
		}
	}
	return append(out, rest...)
}

func (p *Planner) sortByCost(ops []Operation) []Operation {
	sorted := make([]Operation, len(ops))
	copy(sorted, ops)
	// Simple insertion sort; the slice is typically small.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && p.EstimateCost(sorted[j]) < p.EstimateCost(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func stepInputs(idx int, ops []Operation) []string {
	if idx == 0 {
		return nil
	}
	return []string{fmt.Sprintf("step_%d", idx-1)}
}

func sameOrder(a, b []Operation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || a[i].Feature != b[i].Feature {
			return false
		}
	}
	return true
}

func defaultCostMs(op OperationType) float64 {
	switch op {
	case OpLookup:
		return 0.5
	case OpCompute:
		return 1.0
	case OpAggregate:
		return 2.0
	case OpJoin:
		return 5.0
	case OpTransform:
		return 1.5
	default:
		return 2.0
	}
}

// queryKey produces a deterministic cache key for a query.
func queryKey(q Query) string {
	return fmt.Sprintf("%s:%v:%v:%v", q.ID, q.Features, q.Entities, len(q.Operations))
}

// ---------------------------------------------------------------------------
// LRU plan cache
// ---------------------------------------------------------------------------

func (p *Planner) cacheGet(key string) (*ExecutionPlan, bool) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	elem, ok := p.cacheMap[key]
	if !ok {
		return nil, false
	}
	p.cacheList.MoveToFront(elem)
	return elem.Value.(*planCacheEntry).plan, true
}

func (p *Planner) cachePut(key string, plan *ExecutionPlan) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	if elem, ok := p.cacheMap[key]; ok {
		p.cacheList.MoveToFront(elem)
		elem.Value.(*planCacheEntry).plan = plan
		return
	}

	entry := &planCacheEntry{queryKey: key, plan: plan}
	elem := p.cacheList.PushFront(entry)
	p.cacheMap[key] = elem

	// Evict LRU entries when the cache exceeds its limit.
	for p.cacheList.Len() > p.cfg.MaxPlanCacheSize {
		back := p.cacheList.Back()
		if back == nil {
			break
		}
		p.cacheList.Remove(back)
		delete(p.cacheMap, back.Value.(*planCacheEntry).queryKey)
	}
}

func (p *Planner) invalidatePlan(planID string) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	for key, elem := range p.cacheMap {
		if elem.Value.(*planCacheEntry).plan.ID == planID {
			p.cacheList.Remove(elem)
			delete(p.cacheMap, key)
			return
		}
	}
}
