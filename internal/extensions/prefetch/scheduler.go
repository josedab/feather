package prefetch

import (
	"sync"
	"sync/atomic"
	"time"
)

// SchedulerConfig configures the prefetch scheduler.
type SchedulerConfig struct {
	MaxMemoryBudgetMB int           `json:"max_memory_budget_mb"`
	BatchInterval     time.Duration `json:"batch_interval"`
	SamplingRate      float64       `json:"sampling_rate"`
	MaxCandidates     int           `json:"max_candidates"`
	EstBytesPerFeature int64        `json:"est_bytes_per_feature"`
}

// DefaultSchedulerConfig returns sensible defaults.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxMemoryBudgetMB:  256,
		BatchInterval:      100 * time.Millisecond,
		SamplingRate:       0.1,
		MaxCandidates:      50,
		EstBytesPerFeature: 256,
	}
}

// PrefetchAction represents a scheduled prefetch operation.
type PrefetchAction struct {
	EntityKey  string   `json:"entity_key"`
	Features   []string `json:"features"`
	Priority   string   `json:"priority"`
	MemoryEst  int64    `json:"memory_est_bytes"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

// SchedulerStats tracks scheduler performance.
type SchedulerStats struct {
	TotalScheduled    int64   `json:"total_scheduled"`
	TotalExecuted     int64   `json:"total_executed"`
	TotalSkipped      int64   `json:"total_skipped"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryBudgetBytes int64   `json:"memory_budget_bytes"`
	HitRate           float64 `json:"hit_rate"`
	QueueDepth        int     `json:"queue_depth"`
}

// Scheduler manages prefetch execution with memory budgeting and priority.
type Scheduler struct {
	config     SchedulerConfig
	controller *Controller
	mu         sync.RWMutex
	queue      []PrefetchAction
	memoryUsed int64

	totalScheduled atomic.Int64
	totalExecuted  atomic.Int64
	totalSkipped   atomic.Int64
	hits           atomic.Int64
	total          atomic.Int64
}

// NewScheduler creates a new prefetch scheduler.
func NewScheduler(controller *Controller, cfg SchedulerConfig) *Scheduler {
	if cfg.MaxMemoryBudgetMB <= 0 {
		cfg = DefaultSchedulerConfig()
	}
	return &Scheduler{
		config:     cfg,
		controller: controller,
		queue:      make([]PrefetchAction, 0),
	}
}

// Schedule evaluates an entity and queues prefetch actions if predictions
// exceed the threshold and fit within the memory budget.
func (s *Scheduler) Schedule(entityKey string) []PrefetchAction {
	candidates := s.controller.Predict(entityKey)
	if len(candidates) == 0 {
		return nil
	}

	budgetBytes := int64(s.config.MaxMemoryBudgetMB) * 1024 * 1024

	s.mu.Lock()
	defer s.mu.Unlock()

	var actions []PrefetchAction
	for _, c := range candidates {
		if len(actions) >= s.config.MaxCandidates {
			break
		}

		memEst := s.config.EstBytesPerFeature
		if s.memoryUsed+memEst > budgetBytes {
			s.totalSkipped.Add(1)
			continue
		}

		action := PrefetchAction{
			EntityKey:   entityKey,
			Features:    []string{c.Feature},
			Priority:    priorityFromScore(c.Score),
			MemoryEst:   memEst,
			ScheduledAt: time.Now(),
		}

		s.queue = append(s.queue, action)
		s.memoryUsed += memEst
		s.totalScheduled.Add(1)
		actions = append(actions, action)
	}

	// Bound queue size
	if len(s.queue) > s.config.MaxCandidates*10 {
		released := int64(0)
		for _, a := range s.queue[:len(s.queue)-s.config.MaxCandidates*10] {
			released += a.MemoryEst
		}
		s.queue = s.queue[len(s.queue)-s.config.MaxCandidates*10:]
		s.memoryUsed -= released
		if s.memoryUsed < 0 {
			s.memoryUsed = 0
		}
	}

	return actions
}

// Drain returns and clears all queued prefetch actions.
func (s *Scheduler) Drain() []PrefetchAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := make([]PrefetchAction, len(s.queue))
	copy(actions, s.queue)
	s.queue = s.queue[:0]
	s.memoryUsed = 0
	s.totalExecuted.Add(int64(len(actions)))
	return actions
}

// RecordHit records that a prefetched feature was actually used.
func (s *Scheduler) RecordHit() {
	s.hits.Add(1)
	s.total.Add(1)
}

// RecordMiss records that a prefetched feature was not used.
func (s *Scheduler) RecordMiss() {
	s.total.Add(1)
}

// Stats returns scheduler statistics.
func (s *Scheduler) Stats() SchedulerStats {
	s.mu.RLock()
	queueDepth := len(s.queue)
	memUsed := s.memoryUsed
	s.mu.RUnlock()

	hitRate := 0.0
	total := s.total.Load()
	if total > 0 {
		hitRate = float64(s.hits.Load()) / float64(total)
	}

	return SchedulerStats{
		TotalScheduled:    s.totalScheduled.Load(),
		TotalExecuted:     s.totalExecuted.Load(),
		TotalSkipped:      s.totalSkipped.Load(),
		MemoryUsedBytes:   memUsed,
		MemoryBudgetBytes: int64(s.config.MaxMemoryBudgetMB) * 1024 * 1024,
		HitRate:           hitRate,
		QueueDepth:        queueDepth,
	}
}

func priorityFromScore(score float64) string {
	switch {
	case score >= 0.9:
		return "high"
	case score >= 0.7:
		return "medium"
	default:
		return "low"
	}
}

// AccessPatternRingBuffer is a fixed-size ring buffer for access events
// with configurable sampling rate.
type AccessPatternRingBuffer struct {
	mu           sync.Mutex
	buffer       []AccessEvent
	head         int
	size         int
	capacity     int
	samplingRate float64
	sampleCount  int64
}

// AccessEvent records a single feature access.
type AccessEvent struct {
	EntityKey string    `json:"entity_key"`
	Features  []string  `json:"features"`
	Timestamp time.Time `json:"timestamp"`
}

// NewAccessPatternRingBuffer creates a new ring buffer.
func NewAccessPatternRingBuffer(capacity int, samplingRate float64) *AccessPatternRingBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	if samplingRate <= 0 || samplingRate > 1 {
		samplingRate = 1.0
	}
	return &AccessPatternRingBuffer{
		buffer:       make([]AccessEvent, capacity),
		capacity:     capacity,
		samplingRate: samplingRate,
	}
}

// Record adds an event to the ring buffer, respecting the sampling rate.
func (rb *AccessPatternRingBuffer) Record(event AccessEvent) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.sampleCount++
	// Deterministic sampling based on count
	if rb.samplingRate < 1.0 {
		if float64(rb.sampleCount%100)/100.0 >= rb.samplingRate {
			return false
		}
	}

	rb.buffer[rb.head] = event
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
	return true
}

// Snapshot returns a copy of all events in the buffer.
func (rb *AccessPatternRingBuffer) Snapshot() []AccessEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	events := make([]AccessEvent, rb.size)
	if rb.size < rb.capacity {
		copy(events, rb.buffer[:rb.size])
	} else {
		// Ring has wrapped
		n := copy(events, rb.buffer[rb.head:])
		copy(events[n:], rb.buffer[:rb.head])
	}
	return events
}

// Size returns the current number of events in the buffer.
func (rb *AccessPatternRingBuffer) Size() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size
}
