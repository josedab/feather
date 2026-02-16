package main

import (
	"testing"

	"github.com/feather-store/feather/internal/core/config"
)

func TestBuildEnabledFeatures(t *testing.T) {
	cfg := &config.Config{}
	features := buildEnabledFeatures(cfg)

	if features == nil {
		t.Fatal("buildEnabledFeatures returned nil")
	}

	// Verify core features are always enabled
	requiredFeatures := []string{
		"groups", "backfill", "streaming", "catalog",
		"auth", "ml", "transform", "cache",
	}
	for _, name := range requiredFeatures {
		if !features[name] {
			t.Errorf("expected feature %q to be enabled", name)
		}
	}
}

func TestBuildEnabledFeatures_UIEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.UI.Enabled = true
	features := buildEnabledFeatures(cfg)
	if !features["ui"] {
		t.Error("expected ui feature to be enabled when cfg.UI.Enabled is true")
	}
}

func TestBuildEnabledFeatures_UIDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.UI.Enabled = false
	features := buildEnabledFeatures(cfg)
	if features["ui"] {
		t.Error("expected ui feature to be disabled when cfg.UI.Enabled is false")
	}
}

func TestBuildEnabledFeatures_DBTEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.DBT.Enabled = true
	features := buildEnabledFeatures(cfg)
	if !features["dbt"] {
		t.Error("expected dbt feature to be enabled when cfg.DBT.Enabled is true")
	}
}

func TestBuildEnabledFeatures_DBTDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.DBT.Enabled = false
	features := buildEnabledFeatures(cfg)
	if features["dbt"] {
		t.Error("expected dbt feature to be disabled when cfg.DBT.Enabled is false")
	}
}

func TestLoadEmbeddedConfig(t *testing.T) {
	cfg, err := config.LoadFromBytes(defaultConfigData)
	if err != nil {
		t.Fatalf("failed to load embedded config: %v", err)
	}

	if cfg.Serving.HTTP.Port == 0 {
		t.Error("expected non-zero HTTP port")
	}
	if cfg.Serving.GRPC.Port == 0 {
		t.Error("expected non-zero gRPC port")
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("embedded config validation failed: %v", err)
	}
}

func TestLoadEmbeddedConfig_SchemaGroups(t *testing.T) {
	cfg, err := config.LoadFromBytes(defaultConfigData)
	if err != nil {
		t.Fatalf("failed to load embedded config: %v", err)
	}

	if len(cfg.Schema.Groups) == 0 {
		t.Error("expected at least one schema group in embedded config")
	}

	for _, g := range cfg.Schema.Groups {
		if g.Name == "" {
			t.Error("expected non-empty group name")
		}
		if len(g.Features) == 0 {
			t.Errorf("expected features in group %s", g.Name)
		}
	}
}

func TestRunServerSetup(t *testing.T) {
	cfg, err := config.LoadFromBytes(defaultConfigData)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Disable external dependencies for test
	cfg.Ingestion.Kafka.Enabled = false
	cfg.Tracing.Enabled = false
	cfg.Storage.Warm.Path = "" // in-memory

	// Ensure the feature map is populated
	features := buildEnabledFeatures(cfg)
	if len(features) == 0 {
		t.Error("expected non-empty features map")
	}

	// Verify config validates
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config validation failed: %v", err)
	}
}
