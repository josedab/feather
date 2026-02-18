package warehouse

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/core/domain"
)

func TestRedshiftConnector_Type(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	if c.Type() != ConnectorTypeRedshift {
		t.Errorf("expected redshift, got %s", c.Type())
	}
}

func TestRedshiftConnector_DefaultState(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	if c.State() != ConnectionStateDisconnected {
		t.Errorf("expected disconnected, got %s", c.State())
	}
}

func TestRedshiftConnector_PingNotConnected(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	err := c.Ping(context.Background())
	if err == nil {
		t.Error("expected error for ping when not connected")
	}
}

func TestRedshiftConnector_ExportNotConnected(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	_, err := c.Export(context.Background(), &ExportRequest{})
	if err == nil {
		t.Error("expected error for export when not connected")
	}
}

func TestRedshiftConnector_ImportNotConnected(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	_, err := c.Import(context.Background(), &ImportRequest{})
	if err == nil {
		t.Error("expected error for import when not connected")
	}
}

func TestRedshiftConnector_Stats(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	stats := c.Stats()
	if stats["type"] != "redshift" {
		t.Errorf("expected type 'redshift', got %v", stats["type"])
	}
	if stats["database"] != "feather" {
		t.Errorf("expected database 'feather', got %v", stats["database"])
	}
}

func TestRedshiftConnector_Close(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	err := c.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if c.State() != ConnectionStateDisconnected {
		t.Errorf("expected disconnected after close")
	}
}

func TestRedshiftTypeMapping(t *testing.T) {
	tests := []struct {
		redshiftType string
		featherType  domain.DataType
	}{
		{"BIGINT", domain.DataTypeInt64},
		{"DOUBLE PRECISION", domain.DataTypeFloat64},
		{"VARCHAR", domain.DataTypeString},
		{"BOOLEAN", domain.DataTypeBool},
		{"TIMESTAMP", domain.DataTypeTimestamp},
		{"SUPER", domain.DataTypeVector},
	}

	for _, tt := range tests {
		ft := mapRedshiftTypeToFeature(tt.redshiftType)
		if ft != tt.featherType {
			t.Errorf("mapRedshiftTypeToFeature(%s) = %v, want %v", tt.redshiftType, ft, tt.featherType)
		}
	}
}

func TestRedshiftConnector_BuildCreateTableSQL(t *testing.T) {
	c := NewRedshiftConnector(DefaultRedshiftConfig(), nil, nil, slog.Default())
	sqlStr, err := c.buildCreateTableSQL("features", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sqlStr == "" {
		t.Error("expected non-empty SQL")
	}
	if !strings.Contains(sqlStr, "CREATE TABLE") {
		t.Error("expected CREATE TABLE in SQL")
	}
}
