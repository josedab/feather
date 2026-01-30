package incrmat

import (
	"fmt"
	"sync"
	"time"
)

// CDCSourceType identifies the type of CDC source.
type CDCSourceType string

const (
	CDCPostgreSQL CDCSourceType = "postgresql"
	CDCMySQL      CDCSourceType = "mysql"
	CDCMongoDB    CDCSourceType = "mongodb"
	CDCGeneric    CDCSourceType = "generic"
	CDCKafka      CDCSourceType = "kafka"
)

// CDCOperation identifies the type of data mutation.
type CDCOperation string

const (
	OpInsert CDCOperation = "INSERT"
	OpUpdate CDCOperation = "UPDATE"
	OpDelete CDCOperation = "DELETE"
)

// CDCSourceConfig configures a CDC source connection.
type CDCSourceConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          CDCSourceType     `json:"type"`
	ConnectionURL string            `json:"connection_url,omitempty"`
	Database      string            `json:"database,omitempty"`
	Table         string            `json:"table,omitempty"`
	Topic         string            `json:"topic,omitempty"`
	SlotName      string            `json:"slot_name,omitempty"` // PostgreSQL replication slot
	FeatureGroup  string            `json:"feature_group"`
	FieldMapping  map[string]string `json:"field_mapping,omitempty"`
	PollInterval  time.Duration     `json:"poll_interval_ns,omitempty"`
	BatchSize     int               `json:"batch_size,omitempty"`
	Enabled       bool              `json:"enabled"`
}

// CDCEvent represents a captured change from a source database.
type CDCEvent struct {
	SourceID  string                 `json:"source_id"`
	Operation CDCOperation           `json:"operation"`
	Table     string                 `json:"table"`
	EntityID  string                 `json:"entity_id"`
	Before    map[string]interface{} `json:"before,omitempty"`
	After     map[string]interface{} `json:"after,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	LSN       int64                  `json:"lsn,omitempty"` // Log Sequence Number
	Version   int64                  `json:"version"`
}

// CDCSourceStatus represents the health of a CDC source.
type CDCSourceStatus struct {
	SourceID      string    `json:"source_id"`
	Connected     bool      `json:"connected"`
	LastEventAt   time.Time `json:"last_event_at"`
	EventsCapture int64     `json:"events_captured"`
	Lag           int64     `json:"lag"`
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
}

// CDCStats holds aggregate CDC statistics.
type CDCStats struct {
	TotalSources   int              `json:"total_sources"`
	ActiveSources  int              `json:"active_sources"`
	TotalCaptured  int64            `json:"total_captured"`
	TotalProcessed int64            `json:"total_processed"`
	TotalErrors    int64            `json:"total_errors"`
	ByOperation    map[string]int64 `json:"by_operation"`
	BySource       map[string]int64 `json:"by_source"`
}

// CDCManager manages CDC sources and feeds change events to the Engine.
type CDCManager struct {
	mu         sync.RWMutex
	engine     *Engine
	sources    map[string]*CDCSourceConfig
	statuses   map[string]*CDCSourceStatus
	events     []CDCEvent
	maxEvents  int
	stats      CDCStats
	lsnTracker *LSNTracker
}

// NewCDCManager creates a new CDC manager linked to an Engine.
func NewCDCManager(engine *Engine, maxEvents int) *CDCManager {
	if maxEvents <= 0 {
		maxEvents = 100000
	}
	return &CDCManager{
		engine:    engine,
		sources:   make(map[string]*CDCSourceConfig),
		statuses:  make(map[string]*CDCSourceStatus),
		maxEvents: maxEvents,
		stats: CDCStats{
			ByOperation: make(map[string]int64),
			BySource:    make(map[string]int64),
		},
	}
}

// RegisterSource adds a CDC source configuration.
func (m *CDCManager) RegisterSource(src CDCSourceConfig) error {
	if src.ID == "" || src.Name == "" {
		return fmt.Errorf("id and name are required")
	}
	if src.FeatureGroup == "" {
		return fmt.Errorf("feature_group is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sources[src.ID]; exists {
		return fmt.Errorf("source %s already exists", src.ID)
	}

	if src.BatchSize <= 0 {
		src.BatchSize = 1000
	}
	if src.PollInterval <= 0 {
		src.PollInterval = time.Second
	}

	m.sources[src.ID] = &src
	m.statuses[src.ID] = &CDCSourceStatus{
		SourceID:  src.ID,
		Connected: src.Enabled,
	}
	m.stats.TotalSources++
	if src.Enabled {
		m.stats.ActiveSources++
	}
	return nil
}

// RemoveSource removes a CDC source.
func (m *CDCManager) RemoveSource(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	src, exists := m.sources[id]
	if !exists {
		return fmt.Errorf("source %s not found", id)
	}
	if src.Enabled {
		m.stats.ActiveSources--
	}
	m.stats.TotalSources--
	delete(m.sources, id)
	delete(m.statuses, id)
	return nil
}

// ListSources returns all registered CDC sources.
func (m *CDCManager) ListSources() []CDCSourceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]CDCSourceConfig, 0, len(m.sources))
	for _, s := range m.sources {
		result = append(result, *s)
	}
	return result
}

// GetSourceStatus returns the status of a CDC source.
func (m *CDCManager) GetSourceStatus(id string) (*CDCSourceStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.statuses[id]
	if !exists {
		return nil, fmt.Errorf("source %s not found", id)
	}
	return status, nil
}

// ProcessCDCEvent processes a single CDC event through the engine.
func (m *CDCManager) ProcessCDCEvent(event CDCEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	src, exists := m.sources[event.SourceID]
	if !exists {
		return fmt.Errorf("source %s not found", event.SourceID)
	}

	// Store event
	if len(m.events) >= m.maxEvents {
		m.events = m.events[1:]
	}
	m.events = append(m.events, event)

	// Update stats
	m.stats.TotalCaptured++
	m.stats.ByOperation[string(event.Operation)]++
	m.stats.BySource[event.SourceID]++

	// Update source status
	if status, ok := m.statuses[event.SourceID]; ok {
		status.LastEventAt = event.Timestamp
		status.EventsCapture++
	}

	// Determine changed fields from CDC event
	changedFields := make([]string, 0)
	if event.After != nil {
		for k := range event.After {
			if src.FieldMapping != nil {
				if mapped, ok := src.FieldMapping[k]; ok {
					changedFields = append(changedFields, mapped)
					continue
				}
			}
			changedFields = append(changedFields, k)
		}
	}

	// Forward as a ChangeEvent to the engine
	changeEvent := ChangeEvent{
		EntityID:      event.EntityID,
		FeatureGroup:  src.FeatureGroup,
		ChangedFields: changedFields,
		Timestamp:     event.Timestamp,
		Version:       event.Version,
	}
	m.engine.RecordChange(changeEvent)
	m.stats.TotalProcessed++

	return nil
}

// ProcessBatch processes multiple CDC events in a batch.
func (m *CDCManager) ProcessBatch(events []CDCEvent) (int, int, error) {
	processed := 0
	errCount := 0
	for _, event := range events {
		if err := m.ProcessCDCEvent(event); err != nil {
			errCount++
			m.mu.Lock()
			m.stats.TotalErrors++
			if status, ok := m.statuses[event.SourceID]; ok {
				status.ErrorCount++
				status.LastError = err.Error()
			}
			m.mu.Unlock()
		} else {
			processed++
		}
	}
	return processed, errCount, nil
}

// GetRecentEvents returns the most recent CDC events.
func (m *CDCManager) GetRecentEvents(limit int) []CDCEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	result := make([]CDCEvent, limit)
	copy(result, m.events[start:])
	return result
}

// Stats returns aggregate CDC statistics.
func (m *CDCManager) Stats() CDCStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
