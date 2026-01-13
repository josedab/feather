// Package governance provides enterprise governance features for Feather.
//
// This package implements comprehensive governance capabilities including:
//   - Audit logging for all data access and modifications
//   - PII detection and classification
//   - Dynamic data masking based on user permissions
//   - Column-level access control
//   - Data residency enforcement
//
// # Audit Logging
//
// All data operations are logged with full context:
//
//	auditor := governance.NewAuditLogger(config)
//	auditor.LogAccess(ctx, &AuditEvent{
//	    Action:    ActionRead,
//	    Resource:  "user:123",
//	    Features:  []string{"email", "phone"},
//	    UserID:    "analyst@company.com",
//	})
package governance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by audit operations.
var (
	ErrAuditLogFull       = errors.New("audit log buffer full")
	ErrAuditLogClosed     = errors.New("audit logger closed")
	ErrInvalidAuditConfig = errors.New("invalid audit configuration")
)

// AuditAction represents the type of audited operation.
type AuditAction string

// AuditAction constants.
const (
	ActionRead        AuditAction = "read"
	ActionWrite       AuditAction = "write"
	ActionDelete      AuditAction = "delete"
	ActionExport      AuditAction = "export"
	ActionImport      AuditAction = "import"
	ActionQuery       AuditAction = "query"
	ActionSchemaRead  AuditAction = "schema_read"
	ActionSchemaWrite AuditAction = "schema_write"
	ActionAdminOp     AuditAction = "admin"
	ActionAuth        AuditAction = "auth"
	ActionAuthFail    AuditAction = "auth_fail"
)

// AuditOutcome represents the result of an audited operation.
type AuditOutcome string

// AuditOutcome constants.
const (
	OutcomeSuccess     AuditOutcome = "success"
	OutcomeFailure     AuditOutcome = "failure"
	OutcomeDenied      AuditOutcome = "denied"
	OutcomePartial     AuditOutcome = "partial"
	OutcomeRateLimited AuditOutcome = "rate_limited"
)

// AuditSeverity indicates the importance level of an audit event.
type AuditSeverity string

// AuditSeverity constants.
const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarning  AuditSeverity = "warning"
	SeverityCritical AuditSeverity = "critical"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	// ID is the unique event identifier.
	ID string `json:"id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Action is the type of operation.
	Action AuditAction `json:"action"`

	// Outcome is the result of the operation.
	Outcome AuditOutcome `json:"outcome"`

	// Severity indicates event importance.
	Severity AuditSeverity `json:"severity"`

	// UserID is the authenticated user.
	UserID string `json:"user_id,omitempty"`

	// TenantID is the tenant context.
	TenantID string `json:"tenant_id,omitempty"`

	// Resource is the target resource (entity key, table, etc.).
	Resource string `json:"resource,omitempty"`

	// Features lists accessed features.
	Features []string `json:"features,omitempty"`

	// RequestID links to the API request.
	RequestID string `json:"request_id,omitempty"`

	// SourceIP is the client IP address.
	SourceIP string `json:"source_ip,omitempty"`

	// UserAgent is the client user agent.
	UserAgent string `json:"user_agent,omitempty"`

	// Method is the HTTP method or gRPC method.
	Method string `json:"method,omitempty"`

	// Path is the API endpoint path.
	Path string `json:"path,omitempty"`

	// Duration is the operation duration.
	Duration time.Duration `json:"duration_ns"`

	// BytesRead is the bytes read during the operation.
	BytesRead int64 `json:"bytes_read,omitempty"`

	// BytesWritten is the bytes written during the operation.
	BytesWritten int64 `json:"bytes_written,omitempty"`

	// RowsAffected is the number of rows affected.
	RowsAffected int64 `json:"rows_affected,omitempty"`

	// Error contains error details if the operation failed.
	Error string `json:"error,omitempty"`

	// PIIAccessed indicates if PII data was accessed.
	PIIAccessed bool `json:"pii_accessed,omitempty"`

	// MaskingApplied indicates if data masking was applied.
	MaskingApplied bool `json:"masking_applied,omitempty"`

	// Metadata contains additional context.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AuditConfig configures the audit logger.
type AuditConfig struct {
	// Enabled enables audit logging.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// LogLevel sets the minimum severity to log.
	LogLevel AuditSeverity `json:"log_level" yaml:"log_level"`

	// BufferSize is the async event buffer size.
	BufferSize int `json:"buffer_size" yaml:"buffer_size"`

	// FlushInterval is how often to flush the buffer.
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`

	// RetentionDays is how long to retain audit logs.
	RetentionDays int `json:"retention_days" yaml:"retention_days"`

	// OutputPath is the file path for audit logs.
	OutputPath string `json:"output_path" yaml:"output_path"`

	// EnableConsole also outputs to console.
	EnableConsole bool `json:"enable_console" yaml:"enable_console"`

	// IncludePII includes PII in audit logs (for compliance).
	IncludePII bool `json:"include_pii" yaml:"include_pii"`

	// SignEvents cryptographically signs events.
	SignEvents bool `json:"sign_events" yaml:"sign_events"`
}

// DefaultAuditConfig returns the default audit configuration.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:       true,
		LogLevel:      SeverityInfo,
		BufferSize:    10000,
		FlushInterval: 5 * time.Second,
		RetentionDays: 90,
		OutputPath:    "/var/log/feather/audit.log",
		EnableConsole: false,
		IncludePII:    false,
		SignEvents:    false,
	}
}

// AuditLogger handles audit event logging.
type AuditLogger struct {
	mu     sync.RWMutex
	config AuditConfig
	events chan *AuditEvent
	writer io.Writer
	file   *os.File
	logger *slog.Logger
	closed bool
	wg     sync.WaitGroup
	cancel context.CancelFunc

	// Metrics
	eventsLogged  int64
	eventsDropped int64
	bytesWritten  int64

	// Callbacks
	onEvent []func(*AuditEvent)
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(config AuditConfig, logger *slog.Logger) (*AuditLogger, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if config.BufferSize == 0 {
		config.BufferSize = 10000
	}

	al := &AuditLogger{
		config: config,
		events: make(chan *AuditEvent, config.BufferSize),
		logger: logger,
	}

	// Open output file if specified
	if config.OutputPath != "" && config.Enabled {
		file, err := os.OpenFile(config.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("opening audit log file: %w", err)
		}
		al.file = file
		al.writer = file
	}

	// Start background processor
	if config.Enabled {
		ctx, cancel := context.WithCancel(context.Background())
		al.cancel = cancel
		al.wg.Add(1)
		go al.processLoop(ctx)
	}

	return al, nil
}

// Log records an audit event.
func (al *AuditLogger) Log(event *AuditEvent) error {
	if !al.config.Enabled {
		return nil
	}

	al.mu.RLock()
	if al.closed {
		al.mu.RUnlock()
		return ErrAuditLogClosed
	}
	al.mu.RUnlock()

	// Set defaults
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if event.Severity == "" {
		event.Severity = SeverityInfo
	}
	if event.Outcome == "" {
		event.Outcome = OutcomeSuccess
	}

	// Check severity threshold
	if !al.shouldLog(event.Severity) {
		return nil
	}

	// Try to queue event
	select {
	case al.events <- event:
		return nil
	default:
		atomic.AddInt64(&al.eventsDropped, 1)
		return ErrAuditLogFull
	}
}

// LogAccess logs a data access event.
func (al *AuditLogger) LogAccess(ctx context.Context, event *AuditEvent) error {
	if event.Action == "" {
		event.Action = ActionRead
	}
	return al.Log(event)
}

// LogWrite logs a data modification event.
func (al *AuditLogger) LogWrite(ctx context.Context, event *AuditEvent) error {
	if event.Action == "" {
		event.Action = ActionWrite
	}
	return al.Log(event)
}

// LogAuth logs an authentication event.
func (al *AuditLogger) LogAuth(ctx context.Context, userID string, success bool, sourceIP string) error {
	action := ActionAuth
	outcome := OutcomeSuccess
	severity := SeverityInfo

	if !success {
		action = ActionAuthFail
		outcome = OutcomeFailure
		severity = SeverityWarning
	}

	return al.Log(&AuditEvent{
		Action:   action,
		Outcome:  outcome,
		Severity: severity,
		UserID:   userID,
		SourceIP: sourceIP,
	})
}

// LogDenied logs an access denied event.
func (al *AuditLogger) LogDenied(ctx context.Context, event *AuditEvent) error {
	event.Outcome = OutcomeDenied
	event.Severity = SeverityWarning
	return al.Log(event)
}

// LogCritical logs a critical security event.
func (al *AuditLogger) LogCritical(ctx context.Context, event *AuditEvent) error {
	event.Severity = SeverityCritical
	return al.Log(event)
}

// OnEvent registers a callback for audit events.
func (al *AuditLogger) OnEvent(callback func(*AuditEvent)) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.onEvent = append(al.onEvent, callback)
}

// shouldLog checks if an event should be logged based on severity.
func (al *AuditLogger) shouldLog(severity AuditSeverity) bool {
	severityOrder := map[AuditSeverity]int{
		SeverityInfo:     0,
		SeverityWarning:  1,
		SeverityCritical: 2,
	}

	return severityOrder[severity] >= severityOrder[al.config.LogLevel]
}

// processLoop processes queued audit events.
func (al *AuditLogger) processLoop(ctx context.Context) {
	defer al.wg.Done()

	flushTicker := time.NewTicker(al.config.FlushInterval)
	defer flushTicker.Stop()

	var batch []*AuditEvent
	batchSize := 100

	for {
		select {
		case <-ctx.Done():
			// Flush remaining events
			al.flushBatch(batch)
			return

		case event := <-al.events:
			batch = append(batch, event)
			if len(batch) >= batchSize {
				al.flushBatch(batch)
				batch = batch[:0]
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				al.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch writes a batch of events.
func (al *AuditLogger) flushBatch(batch []*AuditEvent) {
	al.mu.RLock()
	callbacks := al.onEvent
	al.mu.RUnlock()

	for _, event := range batch {
		// Write to file
		if al.writer != nil {
			data, err := json.Marshal(event)
			if err != nil {
				al.logger.Error("marshaling audit event", "error", err)
				continue
			}
			data = append(data, '\n')

			n, err := al.writer.Write(data)
			if err != nil {
				al.logger.Error("writing audit event", "error", err)
			} else {
				atomic.AddInt64(&al.bytesWritten, int64(n))
			}
		}

		// Console output
		if al.config.EnableConsole {
			al.logger.Info("audit",
				"action", event.Action,
				"outcome", event.Outcome,
				"user", event.UserID,
				"resource", event.Resource,
			)
		}

		// Callbacks
		for _, cb := range callbacks {
			cb(event)
		}

		atomic.AddInt64(&al.eventsLogged, 1)
	}

	// Sync file
	if al.file != nil {
		_ = al.file.Sync()
	}
}

// Query searches audit logs (simplified in-memory for demo).
func (al *AuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	// In production, this would query a database or log storage
	// For now, return empty results
	return []*AuditEvent{}, nil
}

// Stats returns audit logger statistics.
func (al *AuditLogger) Stats() map[string]interface{} {
	return map[string]interface{}{
		"events_logged":   atomic.LoadInt64(&al.eventsLogged),
		"events_dropped":  atomic.LoadInt64(&al.eventsDropped),
		"bytes_written":   atomic.LoadInt64(&al.bytesWritten),
		"buffer_size":     len(al.events),
		"buffer_capacity": cap(al.events),
	}
}

// Close shuts down the audit logger.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	if al.closed {
		al.mu.Unlock()
		return nil
	}
	al.closed = true
	al.mu.Unlock()

	if al.cancel != nil {
		al.cancel()
	}
	al.wg.Wait()

	if al.file != nil {
		return al.file.Close()
	}

	return nil
}

// AuditFilter defines criteria for querying audit logs.
type AuditFilter struct {
	// StartTime is the earliest event time.
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime is the latest event time.
	EndTime *time.Time `json:"end_time,omitempty"`

	// Actions filters by action types.
	Actions []AuditAction `json:"actions,omitempty"`

	// Outcomes filters by outcome types.
	Outcomes []AuditOutcome `json:"outcomes,omitempty"`

	// UserIDs filters by user IDs.
	UserIDs []string `json:"user_ids,omitempty"`

	// TenantIDs filters by tenant IDs.
	TenantIDs []string `json:"tenant_ids,omitempty"`

	// Resources filters by resources.
	Resources []string `json:"resources,omitempty"`

	// PIIOnly filters for PII access events.
	PIIOnly bool `json:"pii_only,omitempty"`

	// Limit is the maximum results to return.
	Limit int `json:"limit,omitempty"`

	// Offset is the result offset for pagination.
	Offset int `json:"offset,omitempty"`
}

// AuditReport contains aggregated audit statistics.
type AuditReport struct {
	// Period is the report time period.
	Period string `json:"period"`

	// StartTime is the report start time.
	StartTime time.Time `json:"start_time"`

	// EndTime is the report end time.
	EndTime time.Time `json:"end_time"`

	// TotalEvents is the total event count.
	TotalEvents int64 `json:"total_events"`

	// EventsByAction breaks down events by action.
	EventsByAction map[AuditAction]int64 `json:"events_by_action"`

	// EventsByOutcome breaks down events by outcome.
	EventsByOutcome map[AuditOutcome]int64 `json:"events_by_outcome"`

	// UniqueUsers is the count of unique users.
	UniqueUsers int64 `json:"unique_users"`

	// PIIAccessCount is the count of PII access events.
	PIIAccessCount int64 `json:"pii_access_count"`

	// DeniedCount is the count of denied access events.
	DeniedCount int64 `json:"denied_count"`

	// TopUsers lists the most active users.
	TopUsers []UserActivity `json:"top_users,omitempty"`

	// TopResources lists the most accessed resources.
	TopResources []ResourceActivity `json:"top_resources,omitempty"`
}

// UserActivity tracks user audit activity.
type UserActivity struct {
	UserID      string    `json:"user_id"`
	EventCount  int64     `json:"event_count"`
	LastAccess  time.Time `json:"last_access"`
	PIIAccesses int64     `json:"pii_accesses"`
}

// ResourceActivity tracks resource audit activity.
type ResourceActivity struct {
	Resource    string    `json:"resource"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	UniqueUsers int64     `json:"unique_users"`
}
