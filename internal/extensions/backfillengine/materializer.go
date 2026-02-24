package backfillengine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Materializer writes feature events with correct timestamps into the
// warm/historical storage tier, preserving temporal ordering for accurate
// point-in-time queries.
type Materializer struct {
	writer       FeatureWriter
	dedup        map[string]bool
	dedupOrder   []string // tracks insertion order for bounded eviction
	maxDedupSize int
	mu           sync.Mutex
	stats        MaterializerStats
}

// MaterializerStats tracks materializer performance.
type MaterializerStats struct {
	EventsMaterialized int64      `json:"events_materialized"`
	DuplicatesSkipped  int64      `json:"duplicates_skipped"`
	Errors             int64      `json:"errors"`
	DedupEvictions     int64      `json:"dedup_evictions"`
	LastMaterializedAt *time.Time `json:"last_materialized_at,omitempty"`
}

// NewMaterializer creates a new point-in-time materializer.
func NewMaterializer(writer FeatureWriter) *Materializer {
	return &Materializer{
		writer:       writer,
		dedup:        make(map[string]bool),
		dedupOrder:   make([]string, 0, 1024),
		maxDedupSize: 100000,
	}
}

// Materialize writes a batch of events to the feature store with their
// original timestamps, ensuring exactly-once semantics via deduplication.
func (m *Materializer) Materialize(ctx context.Context, events []Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, evt := range events {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		dedupKey := fmt.Sprintf("%s:%s:%d", evt.Source, evt.ID, evt.Timestamp.UnixNano())
		if m.dedup[dedupKey] {
			m.stats.DuplicatesSkipped++
			continue
		}

		for featureName, value := range evt.Features {
			if err := m.writer.WriteFeature(ctx, evt.EntityKey, featureName, value, evt.Timestamp); err != nil {
				m.stats.Errors++
				return fmt.Errorf("materializing event %s: %w", evt.ID, err)
			}
		}

		m.dedup[dedupKey] = true
		m.dedupOrder = append(m.dedupOrder, dedupKey)
		m.stats.EventsMaterialized++
		now := time.Now()
		m.stats.LastMaterializedAt = &now

		// Evict oldest entries if dedup window is too large.
		if len(m.dedup) > m.maxDedupSize {
			evictCount := m.maxDedupSize / 10
			for i := 0; i < evictCount && i < len(m.dedupOrder); i++ {
				delete(m.dedup, m.dedupOrder[i])
				m.stats.DedupEvictions++
			}
			m.dedupOrder = m.dedupOrder[evictCount:]
		}
	}

	return m.writer.Flush(ctx)
}

// Stats returns current materializer statistics.
func (m *Materializer) Stats() MaterializerStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}

// Reset clears the deduplication state.
func (m *Materializer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dedup = make(map[string]bool)
	m.stats = MaterializerStats{}
}
