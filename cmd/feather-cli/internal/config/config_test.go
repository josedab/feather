package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("expected default server URL http://localhost:8080, got %s", cfg.ServerURL)
	}
	if cfg.OutputFormat != "table" {
		t.Errorf("expected default output format table, got %s", cfg.OutputFormat)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.Verbose {
		t.Error("expected default verbose to be false")
	}
	if cfg.APIKey != "" {
		t.Errorf("expected default API key to be empty, got %s", cfg.APIKey)
	}
}

func TestLoadNoFile(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error loading with no file: %v", err)
	}
	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("expected default server URL, got %s", cfg.ServerURL)
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	content := `server_url: "http://feather:9090"
api_key: "test-key"
output_format: "json"
timeout: 10s
verbose: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerURL != "http://feather:9090" {
		t.Errorf("expected server URL http://feather:9090, got %s", cfg.ServerURL)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("expected API key test-key, got %s", cfg.APIKey)
	}
	if cfg.OutputFormat != "json" {
		t.Errorf("expected output format json, got %s", cfg.OutputFormat)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", cfg.Timeout)
	}
	if !cfg.Verbose {
		t.Error("expected verbose to be true")
	}
}

func TestLoadInvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error loading nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bad.yaml")

	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error loading invalid YAML")
	}
}
