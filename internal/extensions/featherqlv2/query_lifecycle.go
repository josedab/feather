package featherqlv2

import (
	"fmt"
	"sync"
	"time"
)

// QueryState represents the lifecycle state of a streaming query.
type QueryState string

const (
	QueryPending QueryState = "pending"
	QueryRunning QueryState = "running"
	QueryPaused  QueryState = "paused"
	QueryStopped QueryState = "stopped"
	QueryFailed  QueryState = "failed"
)

// ManagedQuery tracks a streaming query through its lifecycle.
type ManagedQuery struct {
	ID          string     `json:"id"`
	SQL         string     `json:"sql"`
	State       QueryState `json:"state"`
	PipelineID  string     `json:"pipeline_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	EventsIn    int64      `json:"events_in"`
	EventsOut   int64      `json:"events_out"`
	LastEventAt *time.Time `json:"last_event_at,omitempty"`
}

// QueryManagerConfig configures the query lifecycle manager.
type QueryManagerConfig struct {
	MaxQueries int `json:"max_queries" yaml:"max_queries"`
}

// DefaultQueryManagerConfig returns sensible defaults.
func DefaultQueryManagerConfig() QueryManagerConfig {
	return QueryManagerConfig{MaxQueries: 1000}
}

// QueryManager manages streaming query lifecycle.
type QueryManager struct {
	mu      sync.RWMutex
	config  QueryManagerConfig
	queries map[string]*ManagedQuery
	nextID  int
}

// NewQueryManager creates a new query lifecycle manager.
func NewQueryManager(config QueryManagerConfig) *QueryManager {
	if config.MaxQueries == 0 {
		config = DefaultQueryManagerConfig()
	}
	return &QueryManager{
		config:  config,
		queries: make(map[string]*ManagedQuery),
	}
}

// Submit adds a new query for management.
func (m *QueryManager) Submit(sql string, pipelineID string) (*ManagedQuery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queries) >= m.config.MaxQueries {
		return nil, fmt.Errorf("max queries (%d) reached", m.config.MaxQueries)
	}

	m.nextID++
	q := &ManagedQuery{
		ID:         fmt.Sprintf("query-%d", m.nextID),
		SQL:        sql,
		State:      QueryPending,
		PipelineID: pipelineID,
		CreatedAt:  time.Now(),
	}
	m.queries[q.ID] = q
	return q, nil
}

// Start transitions a query to running state.
func (m *QueryManager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return fmt.Errorf("query %s not found", id)
	}
	if q.State != QueryPending && q.State != QueryPaused {
		return fmt.Errorf("query %s cannot be started from state %s", id, q.State)
	}
	q.State = QueryRunning
	now := time.Now()
	q.StartedAt = &now
	return nil
}

// Pause transitions a query to paused state.
func (m *QueryManager) Pause(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return fmt.Errorf("query %s not found", id)
	}
	if q.State != QueryRunning {
		return fmt.Errorf("query %s cannot be paused from state %s", id, q.State)
	}
	q.State = QueryPaused
	return nil
}

// Stop transitions a query to stopped state.
func (m *QueryManager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return fmt.Errorf("query %s not found", id)
	}
	q.State = QueryStopped
	now := time.Now()
	q.StoppedAt = &now
	return nil
}

// Fail marks a query as failed.
func (m *QueryManager) Fail(id string, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return fmt.Errorf("query %s not found", id)
	}
	q.State = QueryFailed
	q.Error = errMsg
	now := time.Now()
	q.StoppedAt = &now
	return nil
}

// RecordEvent tracks event processing metrics.
func (m *QueryManager) RecordEvent(id string, isOutput bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return
	}
	if isOutput {
		q.EventsOut++
	} else {
		q.EventsIn++
	}
	now := time.Now()
	q.LastEventAt = &now
}

// Get returns a managed query by ID.
func (m *QueryManager) Get(id string) (*ManagedQuery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, exists := m.queries[id]
	if !exists {
		return nil, fmt.Errorf("query %s not found", id)
	}
	copy := *q
	return &copy, nil
}

// List returns all managed queries.
func (m *QueryManager) List() []ManagedQuery {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ManagedQuery, 0, len(m.queries))
	for _, q := range m.queries {
		result = append(result, *q)
	}
	return result
}

// Delete removes a query (only stopped/failed queries).
func (m *QueryManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, exists := m.queries[id]
	if !exists {
		return fmt.Errorf("query %s not found", id)
	}
	if q.State == QueryRunning {
		return fmt.Errorf("cannot delete running query %s", id)
	}
	delete(m.queries, id)
	return nil
}

// QueryManagerStats returns aggregate statistics.
type QueryManagerStats struct {
	TotalQueries   int `json:"total_queries"`
	RunningQueries int `json:"running_queries"`
	PausedQueries  int `json:"paused_queries"`
	StoppedQueries int `json:"stopped_queries"`
	FailedQueries  int `json:"failed_queries"`
}

// Stats returns aggregate statistics.
func (m *QueryManager) Stats() QueryManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := QueryManagerStats{TotalQueries: len(m.queries)}
	for _, q := range m.queries {
		switch q.State {
		case QueryRunning:
			stats.RunningQueries++
		case QueryPaused:
			stats.PausedQueries++
		case QueryStopped:
			stats.StoppedQueries++
		case QueryFailed:
			stats.FailedQueries++
		}
	}
	return stats
}
