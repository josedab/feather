package migration

import (
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

func TestNewConfigMigrator(t *testing.T) {
	config := DefaultConfigMigratorConfig()
	migrator := NewConfigMigrator(config)

	if migrator == nil {
		t.Fatal("Expected migrator to be non-nil")
	}
}

func TestDefaultConfigMigratorConfig(t *testing.T) {
	config := DefaultConfigMigratorConfig()

	if !config.PreserveComments {
		t.Error("Expected PreserveComments to be true")
	}
	if config.StrictMode {
		t.Error("Expected StrictMode to be false")
	}
}

func TestConfigMigrator_ConvertConfig(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	ttl := 10 * time.Minute
	project := &FeastProject{
		Name:     "test_project",
		Provider: "local",
		OnlineStore: map[string]interface{}{
			"type": "sqlite",
			"path": "/tmp/feast.db",
		},
		OfflineStore: map[string]interface{}{
			"type": "file",
			"path": "/tmp/data",
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:   "user_features",
				TTL:    &ttl,
				Online: true,
				Tags:   map[string]string{"team": "ml"},
				Source: &FeastDataSource{Type: "file"},
			},
		},
	}

	result, err := migrator.ConvertConfig(project)
	if err != nil {
		t.Fatalf("ConvertConfig failed: %v", err)
	}

	if result.Config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	// Check server defaults
	if result.Config.Server.HTTPPort != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", result.Config.Server.HTTPPort)
	}
	if result.Config.Server.GRPCPort != 50051 {
		t.Errorf("Expected gRPC port 50051, got %d", result.Config.Server.GRPCPort)
	}

	// Check storage defaults
	if result.Config.Storage.Hot.MaxMemory != 4*1024*1024*1024 {
		t.Errorf("Expected 4GB max memory, got %d", result.Config.Storage.Hot.MaxMemory)
	}
	if result.Config.Storage.Hot.ShardCount != 256 {
		t.Errorf("Expected 256 shards, got %d", result.Config.Storage.Hot.ShardCount)
	}

	// Check feature groups
	if len(result.Config.Features) == 0 {
		t.Error("Expected at least one feature group config")
	} else {
		fg := result.Config.Features[0]
		if fg.Name != "user_features" {
			t.Errorf("Expected feature group 'user_features', got '%s'", fg.Name)
		}
		if fg.TTL != ttl {
			t.Errorf("Expected TTL %v, got %v", ttl, fg.TTL)
		}
	}
}

func TestConfigMigrator_ConvertConfig_Nil(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	_, err := migrator.ConvertConfig(nil)
	if err != ErrInvalidFeastSchema {
		t.Errorf("Expected ErrInvalidFeastSchema, got %v", err)
	}
}

func TestConfigMigrator_ConvertConfig_Redis(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	project := &FeastProject{
		Name: "test_project",
		OnlineStore: map[string]interface{}{
			"type":              "redis",
			"connection_string": "redis://localhost:6379",
		},
	}

	result, _ := migrator.ConvertConfig(project)

	// Should have warning about Redis replacement
	hasRedisWarning := false
	for _, w := range result.Warnings {
		if contains(w, "Redis") {
			hasRedisWarning = true
			break
		}
	}
	if !hasRedisWarning {
		t.Error("Expected warning about Redis replacement")
	}
}

func TestConfigMigrator_ConvertConfig_BigQuery(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	project := &FeastProject{
		Name: "test_project",
		OfflineStore: map[string]interface{}{
			"type":    "bigquery",
			"project": "my-gcp-project",
		},
	}

	result, _ := migrator.ConvertConfig(project)

	// Should have warning about BigQuery
	hasBQWarning := false
	for _, w := range result.Warnings {
		if contains(w, "BigQuery") {
			hasBQWarning = true
			break
		}
	}
	if !hasBQWarning {
		t.Error("Expected warning about BigQuery connector")
	}
}

func TestConfigMigrator_ConvertConfig_Snowflake(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	project := &FeastProject{
		Name: "test_project",
		OfflineStore: map[string]interface{}{
			"type": "snowflake",
		},
	}

	result, _ := migrator.ConvertConfig(project)

	hasSnowflakeWarning := false
	for _, w := range result.Warnings {
		if contains(w, "Snowflake") {
			hasSnowflakeWarning = true
			break
		}
	}
	if !hasSnowflakeWarning {
		t.Error("Expected warning about Snowflake connector")
	}
}

func TestConfigMigrator_ConvertConfig_DynamoDB(t *testing.T) {
	migrator := NewConfigMigrator(DefaultConfigMigratorConfig())

	project := &FeastProject{
		Name: "test_project",
		OnlineStore: map[string]interface{}{
			"type": "dynamodb",
		},
	}

	result, _ := migrator.ConvertConfig(project)

	hasDynamoWarning := false
	for _, w := range result.Warnings {
		if contains(w, "DynamoDB") {
			hasDynamoWarning = true
			break
		}
	}
	if !hasDynamoWarning {
		t.Error("Expected warning about DynamoDB replacement")
	}
}

func TestGenerateMigrationReport(t *testing.T) {
	project := &FeastProject{
		Name: "test_project",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name: "user_features",
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
					{Name: "balance", ValueType: FeastTypeDouble},
				},
			},
		},
		OnDemandFeatureViews: []FeastOnDemandFeatureView{
			{
				Name: "derived_features",
				Features: []FeastFeature{
					{Name: "score", ValueType: FeastTypeDouble},
				},
				UDF: "lambda x: x",
			},
		},
		FeatureServices: []FeastFeatureService{
			{Name: "user_service"},
		},
	}

	schemaResult := &ConvertResult{
		FeatureGroups: make([]domain.FeatureGroup, 2),
		Warnings:      []string{"UDF warning"},
	}

	configResult := &ConfigConvertResult{
		Config:   &FeatherConfig{},
		Warnings: []string{"Store warning"},
	}

	report := GenerateMigrationReport(project, schemaResult, configResult)

	if report.ProjectName != "test_project" {
		t.Errorf("Expected project name 'test_project', got '%s'", report.ProjectName)
	}

	if report.SchemaConversion.TotalEntities != 1 {
		t.Errorf("Expected 1 entity, got %d", report.SchemaConversion.TotalEntities)
	}

	if report.SchemaConversion.TotalFeatureViews != 2 {
		t.Errorf("Expected 2 feature views, got %d", report.SchemaConversion.TotalFeatureViews)
	}

	if report.SchemaConversion.TotalFeatures != 3 {
		t.Errorf("Expected 3 total features, got %d", report.SchemaConversion.TotalFeatures)
	}

	// Should have recommendations for on-demand views and feature services
	if len(report.Recommendations) == 0 {
		t.Error("Expected recommendations")
	}

	if report.CompatibilityScore <= 0 {
		t.Error("Expected positive compatibility score")
	}

	if report.EstimatedEffort == "" {
		t.Error("Expected effort estimate")
	}
}

func TestGenerateMigrationReport_HighCompatibility(t *testing.T) {
	project := &FeastProject{
		Name: "simple_project",
		FeatureViews: []FeastFeatureView{
			{Name: "features", Features: []FeastFeature{{Name: "x", ValueType: FeastTypeDouble}}},
		},
	}

	schemaResult := &ConvertResult{
		FeatureGroups: make([]domain.FeatureGroup, 1),
		Warnings:      []string{},
	}

	configResult := &ConfigConvertResult{
		Config:   &FeatherConfig{},
		Warnings: []string{},
	}

	report := GenerateMigrationReport(project, schemaResult, configResult)

	if report.CompatibilityScore < 90 {
		t.Errorf("Expected high compatibility score, got %f", report.CompatibilityScore)
	}

	if report.EstimatedEffort != "Low (1-2 hours)" {
		t.Errorf("Expected low effort, got '%s'", report.EstimatedEffort)
	}
}

func TestGenerateMigrationReport_LowCompatibility(t *testing.T) {
	project := &FeastProject{
		Name: "complex_project",
		FeatureViews: []FeastFeatureView{
			{Name: "fv1"}, {Name: "fv2"}, {Name: "fv3"}, {Name: "fv4"}, {Name: "fv5"},
		},
	}

	schemaResult := &ConvertResult{
		FeatureGroups: make([]domain.FeatureGroup, 1), // Only 1 converted out of 5
		Errors:        []string{"error1", "error2", "error3", "error4"},
		Warnings:      []string{"w1", "w2", "w3", "w4", "w5", "w6", "w7", "w8", "w9", "w10"},
	}

	configResult := &ConfigConvertResult{
		Config:   &FeatherConfig{},
		Warnings: []string{"w1", "w2", "w3", "w4", "w5", "w6", "w7", "w8", "w9", "w10"},
	}

	report := GenerateMigrationReport(project, schemaResult, configResult)

	if report.CompatibilityScore > 50 {
		t.Errorf("Expected low compatibility score, got %f", report.CompatibilityScore)
	}
}

func TestFeatherConfig_Serialization(t *testing.T) {
	config := &FeatherConfig{
		Server: ServerConfig{
			HTTPPort:     8080,
			GRPCPort:     50051,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{
			Hot: HotStorageConfig{
				MaxMemory:  4 * 1024 * 1024 * 1024,
				ShardCount: 256,
				DefaultTTL: 5 * time.Minute,
			},
			Warm: WarmStorageConfig{
				Path: "/var/lib/feather/data",
			},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    9090,
		},
	}

	if config.Server.HTTPPort != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", config.Server.HTTPPort)
	}
	if config.Storage.Hot.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5m, got %v", config.Storage.Hot.DefaultTTL)
	}
}

func TestConfigConversionReport(t *testing.T) {
	report := &ConfigConversionReport{
		SourceProvider:     "local",
		OnlineStoreType:    "sqlite",
		OfflineStoreType:   "file",
		UnsupportedOptions: []string{"custom_option"},
	}

	if report.SourceProvider != "local" {
		t.Errorf("Expected provider 'local', got '%s'", report.SourceProvider)
	}
	if len(report.UnsupportedOptions) != 1 {
		t.Errorf("Expected 1 unsupported option, got %d", len(report.UnsupportedOptions))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
