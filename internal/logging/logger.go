package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// ContextKey is the type for context keys used by the logger.
type ContextKey string

const (
	// RequestIDKey is the context key for request IDs.
	RequestIDKey ContextKey = "request_id"
	// EntityKeyKey is the context key for entity keys.
	EntityKeyKey ContextKey = "entity_key"
)

// Config holds logger configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output io.Writer
}

// DefaultConfig returns a default logger configuration.
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "json",
		Output: os.Stdout,
	}
}

// New creates a new structured logger.
func New(cfg Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "text":
		handler = slog.NewTextHandler(output, opts)
	default:
		handler = slog.NewJSONHandler(output, opts)
	}

	return slog.New(handler)
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID returns the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WithEntityKey returns a new context with the entity key set.
func WithEntityKey(ctx context.Context, entityKey string) context.Context {
	return context.WithValue(ctx, EntityKeyKey, entityKey)
}

// GetEntityKey returns the entity key from the context.
func GetEntityKey(ctx context.Context) string {
	if v := ctx.Value(EntityKeyKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// FromContext returns a logger with context values added as attributes.
func FromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}

	attrs := make([]any, 0, 4)

	if requestID := GetRequestID(ctx); requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if entityKey := GetEntityKey(ctx); entityKey != "" {
		attrs = append(attrs, "entity_key", entityKey)
	}

	if len(attrs) > 0 {
		return logger.With(attrs...)
	}
	return logger
}

// Component returns a logger with a component name.
func Component(logger *slog.Logger, name string) *slog.Logger {
	return logger.With("component", name)
}
