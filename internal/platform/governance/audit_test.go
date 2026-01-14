package governance

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAuditConfig(t *testing.T) {
	config := DefaultAuditConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, SeverityInfo, config.LogLevel)
	assert.Equal(t, 10000, config.BufferSize)
	assert.Equal(t, 5*time.Second, config.FlushInterval)
	assert.Equal(t, 90, config.RetentionDays)
}

func TestAuditAction_Values(t *testing.T) {
	assert.Equal(t, AuditAction("read"), ActionRead)
	assert.Equal(t, AuditAction("write"), ActionWrite)
	assert.Equal(t, AuditAction("delete"), ActionDelete)
	assert.Equal(t, AuditAction("export"), ActionExport)
	assert.Equal(t, AuditAction("import"), ActionImport)
	assert.Equal(t, AuditAction("query"), ActionQuery)
	assert.Equal(t, AuditAction("auth"), ActionAuth)
	assert.Equal(t, AuditAction("auth_fail"), ActionAuthFail)
}

func TestAuditOutcome_Values(t *testing.T) {
	assert.Equal(t, AuditOutcome("success"), OutcomeSuccess)
	assert.Equal(t, AuditOutcome("failure"), OutcomeFailure)
	assert.Equal(t, AuditOutcome("denied"), OutcomeDenied)
	assert.Equal(t, AuditOutcome("partial"), OutcomePartial)
}

func TestAuditSeverity_Values(t *testing.T) {
	assert.Equal(t, AuditSeverity("info"), SeverityInfo)
	assert.Equal(t, AuditSeverity("warning"), SeverityWarning)
	assert.Equal(t, AuditSeverity("critical"), SeverityCritical)
}

func TestNewAuditLogger(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	require.NotNil(t, logger)

	defer logger.Close()

	assert.Equal(t, config.BufferSize, cap(logger.events))
}

func TestNewAuditLogger_Disabled(t *testing.T) {
	config := AuditConfig{
		Enabled: false,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Should not error when logging to disabled logger
	err = logger.Log(&AuditEvent{Action: ActionRead})
	assert.NoError(t, err)
}

func TestNewAuditLogger_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
		OutputPath:    logPath,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	require.NotNil(t, logger)

	defer logger.Close()

	// Log an event
	err = logger.Log(&AuditEvent{
		Action:   ActionRead,
		UserID:   "user-1",
		Resource: "entity:123",
	})
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Check file was created
	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestAuditLogger_Log(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Log event
	event := &AuditEvent{
		Action:   ActionRead,
		UserID:   "user-1",
		Resource: "entity:123",
		Features: []string{"feature1", "feature2"},
	}

	err = logger.Log(event)
	require.NoError(t, err)

	// Event should have defaults set
	assert.NotEmpty(t, event.ID)
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, SeverityInfo, event.Severity)
	assert.Equal(t, OutcomeSuccess, event.Outcome)
}

func TestAuditLogger_Log_Closed(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)

	// Close logger
	err = logger.Close()
	require.NoError(t, err)

	// Log should fail
	err = logger.Log(&AuditEvent{Action: ActionRead})
	assert.ErrorIs(t, err, ErrAuditLogClosed)
}

func TestAuditLogger_LogAccess(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	event := &AuditEvent{
		UserID:   "user-1",
		Resource: "entity:123",
	}

	err = logger.LogAccess(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, ActionRead, event.Action)
}

func TestAuditLogger_LogWrite(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	event := &AuditEvent{
		UserID:   "user-1",
		Resource: "entity:123",
	}

	err = logger.LogWrite(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, ActionWrite, event.Action)
}

func TestAuditLogger_LogAuth(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Successful auth
	err = logger.LogAuth(context.Background(), "user-1", true, "192.168.1.1")
	require.NoError(t, err)

	// Failed auth
	err = logger.LogAuth(context.Background(), "user-2", false, "192.168.1.2")
	require.NoError(t, err)
}

func TestAuditLogger_LogDenied(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	event := &AuditEvent{
		Action:   ActionRead,
		UserID:   "user-1",
		Resource: "entity:123",
	}

	err = logger.LogDenied(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, OutcomeDenied, event.Outcome)
	assert.Equal(t, SeverityWarning, event.Severity)
}

func TestAuditLogger_LogCritical(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	event := &AuditEvent{
		Action:   ActionAdminOp,
		UserID:   "user-1",
		Resource: "system",
	}

	err = logger.LogCritical(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, SeverityCritical, event.Severity)
}

func TestAuditLogger_OnEvent(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Register callback with synchronization
	var mu sync.Mutex
	var receivedEvent *AuditEvent
	logger.OnEvent(func(e *AuditEvent) {
		mu.Lock()
		receivedEvent = e
		mu.Unlock()
	})

	// Log event
	event := &AuditEvent{
		Action:   ActionRead,
		UserID:   "user-1",
		Resource: "entity:123",
	}
	err = logger.Log(event)
	require.NoError(t, err)

	// Wait for callback
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	assert.NotNil(t, receivedEvent)
	if receivedEvent != nil {
		assert.Equal(t, "user-1", receivedEvent.UserID)
	}
	mu.Unlock()
}

func TestAuditLogger_SeverityFiltering(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
		LogLevel:      SeverityWarning, // Only warning and above
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Info event should be filtered
	infoEvent := &AuditEvent{
		Action:   ActionRead,
		Severity: SeverityInfo,
	}
	err = logger.Log(infoEvent)
	assert.NoError(t, err)

	// Warning event should pass
	warningEvent := &AuditEvent{
		Action:   ActionRead,
		Severity: SeverityWarning,
	}
	err = logger.Log(warningEvent)
	assert.NoError(t, err)
}

func TestAuditLogger_Stats(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Log events
	for i := 0; i < 5; i++ {
		_ = logger.Log(&AuditEvent{Action: ActionRead})
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	stats := logger.Stats()
	assert.GreaterOrEqual(t, stats["events_logged"].(int64), int64(5))
	assert.Equal(t, 100, stats["buffer_capacity"])
}

func TestAuditLogger_Query(t *testing.T) {
	config := AuditConfig{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: time.Second,
	}

	logger, err := NewAuditLogger(config, nil)
	require.NoError(t, err)
	defer logger.Close()

	// Query returns empty (simplified implementation)
	filter := &AuditFilter{
		Actions: []AuditAction{ActionRead},
		Limit:   10,
	}

	events, err := logger.Query(context.Background(), filter)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestAuditEvent_Fields(t *testing.T) {
	event := &AuditEvent{
		ID:             "event-1",
		Action:         ActionRead,
		PIIAccessed:    true,
		MaskingApplied: true,
	}

	assert.Equal(t, "event-1", event.ID)
	assert.Equal(t, ActionRead, event.Action)
	assert.True(t, event.PIIAccessed)
	assert.True(t, event.MaskingApplied)
}

func TestAuditFilter_Fields(t *testing.T) {
	filter := &AuditFilter{
		Actions: []AuditAction{ActionRead, ActionWrite},
		PIIOnly: true,
	}

	assert.Len(t, filter.Actions, 2)
	assert.True(t, filter.PIIOnly)
}

func TestAuditReport_Fields(t *testing.T) {
	report := &AuditReport{
		Period:      "daily",
		TotalEvents: 1000,
	}

	assert.Equal(t, "daily", report.Period)
	assert.Equal(t, int64(1000), report.TotalEvents)
}
