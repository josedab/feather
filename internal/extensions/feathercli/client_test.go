package feathercli

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	cfg := DefaultClientConfig()
	c := NewClient(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.config.ServerURL != "http://localhost:8080" {
		t.Errorf("expected default server URL, got %s", c.config.ServerURL)
	}
}

func TestFormatTable(t *testing.T) {
	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"key1", "val1"},
		{"key2", "val2"},
	}

	output := FormatTable(headers, rows)

	if !strings.Contains(output, "Name") {
		t.Error("table output should contain header 'Name'")
	}
	if !strings.Contains(output, "Value") {
		t.Error("table output should contain header 'Value'")
	}
	if !strings.Contains(output, "+") {
		t.Error("table output should contain separator '+'")
	}
	if !strings.Contains(output, "key1") {
		t.Error("table output should contain data 'key1'")
	}
}

func TestFormatCSV(t *testing.T) {
	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"key1", "val1"},
		{"key2", "val2"},
	}

	output := FormatCSV(headers, rows)

	if !strings.HasPrefix(output, "Name,Value\n") {
		t.Error("CSV should start with header line")
	}
	if !strings.Contains(output, "key1,val1") {
		t.Error("CSV should contain data row")
	}
}

func TestParseArgs(t *testing.T) {
	args := []string{"get", "--entity=user1", "--group=features"}
	query, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}
	if query.Entity != "user1" {
		t.Errorf("expected entity=user1, got %s", query.Entity)
	}
	if query.Group != "features" {
		t.Errorf("expected group=features, got %s", query.Group)
	}

	// Empty args
	_, err = ParseArgs(nil)
	if err == nil {
		t.Error("expected error for empty args")
	}
}

func TestGetFeatures(t *testing.T) {
	c := NewClient(DefaultClientConfig())

	query := FeatureQuery{Entity: "user1", Group: "user_features"}
	result, err := c.GetFeatures(query)
	if err != nil {
		t.Fatalf("GetFeatures failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Missing entity should fail
	_, err = c.GetFeatures(FeatureQuery{})
	if err == nil {
		t.Error("expected error for missing entity")
	}
}

func TestGetHealth(t *testing.T) {
	c := NewClient(DefaultClientConfig())

	result, err := c.GetHealth()
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map data")
	}
	if data["status"] != "healthy" {
		t.Errorf("expected healthy status, got %v", data["status"])
	}
}

func TestStats(t *testing.T) {
	c := NewClient(DefaultClientConfig())

	c.GetFeatures(FeatureQuery{Entity: "user1"})
	c.GetHealth()

	stats := c.Stats()
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.SuccessfulRequests != 2 {
		t.Errorf("expected 2 successful requests, got %d", stats.SuccessfulRequests)
	}
}
