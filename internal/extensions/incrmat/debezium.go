package incrmat

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DebeziumEnvelope represents a Debezium CDC event envelope.
type DebeziumEnvelope struct {
	Schema  json.RawMessage        `json:"schema,omitempty"`
	Payload DebeziumPayload        `json:"payload"`
}

// DebeziumPayload contains the actual change data from Debezium.
type DebeziumPayload struct {
	Before    map[string]interface{} `json:"before"`
	After     map[string]interface{} `json:"after"`
	Source    DebeziumSource         `json:"source"`
	Op        string                 `json:"op"` // c=create, u=update, d=delete, r=read
	TsMs      int64                  `json:"ts_ms"`
	Transaction *DebeziumTxn         `json:"transaction,omitempty"`
}

// DebeziumSource contains metadata about the source of the change.
type DebeziumSource struct {
	Version   string `json:"version"`
	Connector string `json:"connector"`
	Name      string `json:"name"`
	TsMs      int64  `json:"ts_ms"`
	Snapshot  string `json:"snapshot,omitempty"`
	DB        string `json:"db"`
	Schema    string `json:"schema,omitempty"`
	Table     string `json:"table"`
	LSN       int64  `json:"lsn,omitempty"`
	File      string `json:"file,omitempty"` // MySQL binlog file
	Pos       int64  `json:"pos,omitempty"`  // MySQL binlog position
}

// DebeziumTxn provides transaction metadata.
type DebeziumTxn struct {
	ID                  string `json:"id"`
	TotalOrder          int64  `json:"total_order"`
	DataCollectionOrder int64  `json:"data_collection_order"`
}

// ParseDebeziumEvent converts a Debezium envelope into a CDCEvent.
func ParseDebeziumEvent(data []byte, sourceID string) (*CDCEvent, error) {
	var envelope DebeziumEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		// Try parsing as a flat event (non-envelope format)
		var flat CDCEvent
		if jsonErr := json.Unmarshal(data, &flat); jsonErr == nil && flat.EntityID != "" {
			return &flat, nil
		}
		return nil, fmt.Errorf("parsing debezium event: %w", err)
	}

	payload := envelope.Payload

	var op CDCOperation
	switch payload.Op {
	case "c", "r":
		op = OpInsert
	case "u":
		op = OpUpdate
	case "d":
		op = OpDelete
	default:
		return nil, fmt.Errorf("unknown debezium operation: %s", payload.Op)
	}

	// Extract entity ID from primary key fields in the after record
	entityID := extractEntityID(payload.After, payload.Before)

	event := &CDCEvent{
		SourceID:  sourceID,
		Operation: op,
		Table:     payload.Source.Table,
		EntityID:  entityID,
		Before:    payload.Before,
		After:     payload.After,
		Timestamp: time.Unix(0, payload.TsMs*int64(time.Millisecond)),
		LSN:       payload.Source.LSN,
	}

	if payload.Source.Pos > 0 {
		event.LSN = payload.Source.Pos
	}

	return event, nil
}

func extractEntityID(after, before map[string]interface{}) string {
	record := after
	if record == nil {
		record = before
	}
	if record == nil {
		return ""
	}

	// Common primary key field names
	for _, key := range []string{"id", "ID", "entity_id", "pk", "key"} {
		if v, ok := record[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// LSNTracker provides exactly-once semantics by tracking processed LSNs per source.
type LSNTracker struct {
	mu        sync.RWMutex
	positions map[string]int64 // sourceID -> last processed LSN
}

// NewLSNTracker creates a new LSN tracker for exactly-once processing.
func NewLSNTracker() *LSNTracker {
	return &LSNTracker{
		positions: make(map[string]int64),
	}
}

// ShouldProcess returns true if the event hasn't been processed yet.
func (t *LSNTracker) ShouldProcess(sourceID string, lsn int64) bool {
	if lsn == 0 {
		return true // No LSN tracking for events without LSN
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	lastLSN, exists := t.positions[sourceID]
	if !exists {
		return true
	}
	return lsn > lastLSN
}

// Advance records a processed LSN.
func (t *LSNTracker) Advance(sourceID string, lsn int64) {
	if lsn == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if lsn > t.positions[sourceID] {
		t.positions[sourceID] = lsn
	}
}

// GetPosition returns the last processed LSN for a source.
func (t *LSNTracker) GetPosition(sourceID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.positions[sourceID]
}

// GetAllPositions returns all tracked positions.
func (t *LSNTracker) GetAllPositions() map[string]int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]int64, len(t.positions))
	for k, v := range t.positions {
		result[k] = v
	}
	return result
}

// ProcessDebeziumEvent processes a raw Debezium event with exactly-once semantics.
func (m *CDCManager) ProcessDebeziumEvent(data []byte, sourceID string) (*CDCEvent, error) {
	event, err := ParseDebeziumEvent(data, sourceID)
	if err != nil {
		return nil, fmt.Errorf("parsing debezium event: %w", err)
	}

	m.mu.Lock()
	if m.lsnTracker == nil {
		m.lsnTracker = NewLSNTracker()
	}
	m.mu.Unlock()

	// Exactly-once check
	if !m.lsnTracker.ShouldProcess(sourceID, event.LSN) {
		return event, nil // Already processed, skip silently
	}

	if err := m.ProcessCDCEvent(*event); err != nil {
		return nil, err
	}

	m.lsnTracker.Advance(sourceID, event.LSN)
	return event, nil
}

// GetLSNPositions returns the current LSN positions for all sources.
func (m *CDCManager) GetLSNPositions() map[string]int64 {
	m.mu.RLock()
	if m.lsnTracker == nil {
		m.mu.RUnlock()
		return map[string]int64{}
	}
	m.mu.RUnlock()
	return m.lsnTracker.GetAllPositions()
}
