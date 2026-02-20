package auditlog

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ActionType identifies the kind of audited operation.
type ActionType string

const (
	ActionRead         ActionType = "read"
	ActionWrite        ActionType = "write"
	ActionDelete       ActionType = "delete"
	ActionSchemaChange ActionType = "schema_change"
	ActionConfigChange ActionType = "config_change"
	ActionAuth         ActionType = "auth"
)

// allActions is the complete set of action types.
var allActions = []ActionType{
	ActionRead, ActionWrite, ActionDelete,
	ActionSchemaChange, ActionConfigChange, ActionAuth,
}

// AuditEntry represents a single auditable operation.
type AuditEntry struct {
	ID           string                 `json:"id"`
	Action       ActionType             `json:"action"`
	Actor        string                 `json:"actor"`
	Resource     string                 `json:"resource"`
	ResourceType string                 `json:"resource_type"`
	Details      map[string]interface{} `json:"details,omitempty"`
	SourceIP     string                 `json:"source_ip"`
	Timestamp    time.Time              `json:"timestamp"`
	Success      bool                   `json:"success"`
	DurationMs   float64                `json:"duration_ms"`
}

// QueryFilter specifies criteria for querying audit entries.
type QueryFilter struct {
	Action    ActionType `json:"action,omitempty"`
	Actor     string     `json:"actor,omitempty"`
	Resource  string     `json:"resource,omitempty"`
	StartTime time.Time  `json:"start_time,omitempty"`
	EndTime   time.Time  `json:"end_time,omitempty"`
	Limit     int        `json:"limit,omitempty"`
}

// ExportFormat specifies the format for audit log exports.
type ExportFormat string

const (
	ExportJSON   ExportFormat = "json"
	ExportCSV    ExportFormat = "csv"
	ExportSyslog ExportFormat = "syslog"
)

// LoggerConfig configures the audit logger.
type LoggerConfig struct {
	MaxEntries     int          `json:"max_entries"`
	RetentionDays  int          `json:"retention_days"`
	EnabledActions []ActionType `json:"enabled_actions"`
	FilePath       string       `json:"file_path,omitempty"`
}

// DefaultLoggerConfig returns sensible defaults with all actions enabled.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		MaxEntries:     10000000,
		RetentionDays:  90,
		EnabledActions: append([]ActionType(nil), allActions...),
	}
}

// LoggerStats holds audit logger statistics.
type LoggerStats struct {
	TotalEntries int64            `json:"total_entries"`
	TotalLogged  int64            `json:"total_logged"`
	OldestEntry  time.Time        `json:"oldest_entry"`
	NewestEntry  time.Time        `json:"newest_entry"`
	ActionCounts map[string]int64 `json:"action_counts"`
}

// Logger provides an immutable, queryable audit log.
type Logger struct {
	mu          sync.RWMutex
	config      LoggerConfig
	entries     []AuditEntry
	entryIndex  map[string]int // ID -> index in entries
	totalLogged atomic.Int64
	file        *os.File
}

// NewLogger creates a new audit logger.
func NewLogger(config LoggerConfig) *Logger {
	l := &Logger{
		config:     config,
		entries:    make([]AuditEntry, 0),
		entryIndex: make(map[string]int),
	}
	if config.FilePath != "" {
		f, err := os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Warn("auditlog: failed to open file, proceeding in-memory only", "path", config.FilePath, "error", err)
		} else {
			l.file = f
		}
	}
	return l
}

// Close flushes and closes the audit log file, if one is open.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// Log records an audit entry.
func (l *Logger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) >= l.config.MaxEntries {
		return fmt.Errorf("max entries (%d) reached: %w", l.config.MaxEntries, ErrAuditLogFull)
	}

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", len(l.entries)+1)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	l.entryIndex[entry.ID] = len(l.entries)
	l.entries = append(l.entries, entry)
	l.totalLogged.Add(1)

	if l.file != nil {
		data, err := json.Marshal(entry)
		if err == nil {
			data = append(data, '\n')
			if _, writeErr := l.file.Write(data); writeErr != nil {
				slog.Error("auditlog: failed to write entry to file", "error", writeErr)
			}
		}
	}

	return nil
}

// Query returns entries matching the given filter.
func (l *Logger) Query(filter QueryFilter) []AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []AuditEntry
	for _, e := range l.entries {
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.Actor != "" && e.Actor != filter.Actor {
			continue
		}
		if filter.Resource != "" && e.Resource != filter.Resource {
			continue
		}
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		results = append(results, e)
		if filter.Limit > 0 && len(results) >= filter.Limit {
			break
		}
	}
	return results
}

// GetEntry retrieves a single audit entry by ID.
func (l *Logger) GetEntry(id string) (*AuditEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	idx, exists := l.entryIndex[id]
	if !exists {
		return nil, fmt.Errorf("entry %s not found", id)
	}

	entry := l.entries[idx]
	return &entry, nil
}

// Export returns audit entries in the specified format.
func (l *Logger) Export(filter QueryFilter, format ExportFormat) (string, error) {
	entries := l.Query(filter)

	switch format {
	case ExportJSON:
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return "", fmt.Errorf("json export: %w", err)
		}
		return string(data), nil

	case ExportCSV:
		var b strings.Builder
		b.WriteString("id,action,actor,resource,resource_type,source_ip,timestamp,success,duration_ms\n")
		for _, e := range entries {
			fmt.Fprintf(&b, "%s,%s,%s,%s,%s,%s,%s,%t,%.2f\n",
				e.ID, e.Action, e.Actor, e.Resource, e.ResourceType,
				e.SourceIP, e.Timestamp.Format(time.RFC3339), e.Success, e.DurationMs)
		}
		return b.String(), nil

	default:
		return "", fmt.Errorf("unsupported format %s", format)
	}
}

// Purge removes entries older than the given time and returns the count deleted.
func (l *Logger) Purge(before time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	var kept []AuditEntry
	purged := 0
	for _, e := range l.entries {
		if e.Timestamp.Before(before) {
			delete(l.entryIndex, e.ID)
			purged++
		} else {
			l.entryIndex[e.ID] = len(kept)
			kept = append(kept, e)
		}
	}
	l.entries = kept
	return purged
}

// Stats returns audit logger statistics.
func (l *Logger) Stats() LoggerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LoggerStats{
		TotalEntries: int64(len(l.entries)),
		TotalLogged:  l.totalLogged.Load(),
		ActionCounts: make(map[string]int64),
	}

	for _, e := range l.entries {
		stats.ActionCounts[string(e.Action)]++
		if stats.OldestEntry.IsZero() || e.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = e.Timestamp
		}
		if stats.NewestEntry.IsZero() || e.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = e.Timestamp
		}
	}

	return stats
}
