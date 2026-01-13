package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNew_DefaultJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("expected JSON format with message, got: %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("expected key-value pair in output, got: %s", output)
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "debug",
		Format: "text",
		Output: &buf,
	}

	logger := New(cfg)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected text format with message, got: %s", output)
	}
}

func TestNew_Levels(t *testing.T) {
	tests := []struct {
		level    string
		logDebug bool
		logInfo  bool
	}{
		{"debug", true, true},
		{"info", false, true},
		{"warn", false, false},
		{"error", false, false},
		{"unknown", false, true}, // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			cfg := Config{
				Level:  tt.level,
				Format: "text",
				Output: &buf,
			}

			logger := New(cfg)
			logger.Debug("debug message")
			logger.Info("info message")

			output := buf.String()
			hasDebug := strings.Contains(output, "debug message")
			hasInfo := strings.Contains(output, "info message")

			if hasDebug != tt.logDebug {
				t.Errorf("level %s: debug logging = %v, want %v", tt.level, hasDebug, tt.logDebug)
			}
			if hasInfo != tt.logInfo {
				t.Errorf("level %s: info logging = %v, want %v", tt.level, hasInfo, tt.logInfo)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Level != "info" {
		t.Errorf("expected default level info, got %s", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("expected default format json, got %s", cfg.Format)
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "req-123"

	ctx = WithRequestID(ctx, requestID)
	got := GetRequestID(ctx)

	if got != requestID {
		t.Errorf("GetRequestID() = %q, want %q", got, requestID)
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	ctx := context.Background()
	got := GetRequestID(ctx)

	if got != "" {
		t.Errorf("GetRequestID() on empty context = %q, want empty", got)
	}
}

func TestWithEntityKey(t *testing.T) {
	ctx := context.Background()
	entityKey := "user:456"

	ctx = WithEntityKey(ctx, entityKey)
	got := GetEntityKey(ctx)

	if got != entityKey {
		t.Errorf("GetEntityKey() = %q, want %q", got, entityKey)
	}
}

func TestGetEntityKey_Empty(t *testing.T) {
	ctx := context.Background()
	got := GetEntityKey(ctx)

	if got != "" {
		t.Errorf("GetEntityKey() on empty context = %q, want empty", got)
	}
}

func TestFromContext_WithValues(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}
	logger := New(cfg)

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-789")
	ctx = WithEntityKey(ctx, "user:abc")

	contextLogger := FromContext(ctx, logger)
	contextLogger.Info("test")

	output := buf.String()
	if !strings.Contains(output, `"request_id":"req-789"`) {
		t.Errorf("expected request_id in output, got: %s", output)
	}
	if !strings.Contains(output, `"entity_key":"user:abc"`) {
		t.Errorf("expected entity_key in output, got: %s", output)
	}
}

func TestFromContext_NilLogger(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx, nil)

	// Should not panic and return a valid logger
	if logger == nil {
		t.Error("FromContext with nil logger should return default logger")
	}
}

func TestComponent(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "json",
		Output: &buf,
	}
	logger := New(cfg)

	componentLogger := Component(logger, "storage")
	componentLogger.Info("test")

	output := buf.String()
	if !strings.Contains(output, `"component":"storage"`) {
		t.Errorf("expected component in output, got: %s", output)
	}
}
