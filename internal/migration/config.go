package migration

import (
	"fmt"
	"time"
)

// ConfigMigrator converts Feast configuration to Feather configuration.
type ConfigMigrator struct {
	config ConfigMigratorConfig
}

// ConfigMigratorConfig configures the config migrator.
type ConfigMigratorConfig struct {
	// PreserveComments keeps comments from source config
	PreserveComments bool
	// StrictMode fails on unsupported options
	StrictMode bool
}

// DefaultConfigMigratorConfig returns sensible defaults.
func DefaultConfigMigratorConfig() ConfigMigratorConfig {
	return ConfigMigratorConfig{
		PreserveComments: true,
		StrictMode:       false,
	}
}

// NewConfigMigrator creates a new config migrator.
func NewConfigMigrator(config ConfigMigratorConfig) *ConfigMigrator {
	return &ConfigMigrator{
		config: config,
	}
}

// FeatherConfig represents Feather configuration.
type FeatherConfig struct {
	Server    ServerConfig         `json:"server" yaml:"server"`
	Storage   StorageConfig        `json:"storage" yaml:"storage"`
	Ingestion IngestionConfig      `json:"ingestion,omitempty" yaml:"ingestion,omitempty"`
	Metrics   MetricsConfig        `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Tracing   TracingConfig        `json:"tracing,omitempty" yaml:"tracing,omitempty"`
	Features  []FeatureGroupConfig `json:"features,omitempty" yaml:"features,omitempty"`
}

// ServerConfig configures the Feather server.
type ServerConfig struct {
	HTTPPort     int           `json:"http_port" yaml:"http_port"`
	GRPCPort     int           `json:"grpc_port" yaml:"grpc_port"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
}

// StorageConfig configures storage.
type StorageConfig struct {
	Hot  HotStorageConfig  `json:"hot" yaml:"hot"`
	Warm WarmStorageConfig `json:"warm" yaml:"warm"`
}

// HotStorageConfig configures hot tier storage.
type HotStorageConfig struct {
	MaxMemory  int64         `json:"max_memory" yaml:"max_memory"`
	ShardCount int           `json:"shard_count" yaml:"shard_count"`
	DefaultTTL time.Duration `json:"default_ttl" yaml:"default_ttl"`
}

// WarmStorageConfig configures warm tier storage.
type WarmStorageConfig struct {
	Path          string `json:"path" yaml:"path"`
	MaxSize       int64  `json:"max_size,omitempty" yaml:"max_size,omitempty"`
	RetentionDays int    `json:"retention_days,omitempty" yaml:"retention_days,omitempty"`
}

// IngestionConfig configures data ingestion.
type IngestionConfig struct {
	Kafka KafkaConfig `json:"kafka,omitempty" yaml:"kafka,omitempty"`
	HTTP  HTTPConfig  `json:"http,omitempty" yaml:"http,omitempty"`
}

// KafkaConfig configures Kafka ingestion.
type KafkaConfig struct {
	Enabled       bool     `json:"enabled" yaml:"enabled"`
	Brokers       []string `json:"brokers,omitempty" yaml:"brokers,omitempty"`
	Topic         string   `json:"topic,omitempty" yaml:"topic,omitempty"`
	ConsumerGroup string   `json:"consumer_group,omitempty" yaml:"consumer_group,omitempty"`
}

// HTTPConfig configures HTTP ingestion.
type HTTPConfig struct {
	Enabled   bool `json:"enabled" yaml:"enabled"`
	Port      int  `json:"port,omitempty" yaml:"port,omitempty"`
	RateLimit int  `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
}

// MetricsConfig configures metrics.
type MetricsConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	Port    int  `json:"port,omitempty" yaml:"port,omitempty"`
}

// TracingConfig configures distributed tracing.
type TracingConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Sampler  string `json:"sampler,omitempty" yaml:"sampler,omitempty"`
}

// FeatureGroupConfig represents feature group configuration.
type FeatureGroupConfig struct {
	Name   string            `json:"name" yaml:"name"`
	TTL    time.Duration     `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Source string            `json:"source,omitempty" yaml:"source,omitempty"`
	Tags   map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// ConfigConvertResult contains the results of config conversion.
type ConfigConvertResult struct {
	Config   *FeatherConfig `json:"config"`
	Warnings []string       `json:"warnings,omitempty"`
	Errors   []string       `json:"errors,omitempty"`
}

// ConvertConfig converts a Feast project configuration to Feather configuration.
func (c *ConfigMigrator) ConvertConfig(feast *FeastProject) (*ConfigConvertResult, error) {
	if feast == nil {
		return nil, ErrInvalidFeastSchema
	}

	result := &ConfigConvertResult{
		Config:   &FeatherConfig{},
		Warnings: make([]string, 0),
		Errors:   make([]string, 0),
	}

	// Set server defaults
	result.Config.Server = ServerConfig{
		HTTPPort:     8080,
		GRPCPort:     50051,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Convert online store config
	c.convertOnlineStore(feast.OnlineStore, result)

	// Convert offline store config (for warm tier)
	c.convertOfflineStore(feast.OfflineStore, result)

	// Set storage defaults
	if result.Config.Storage.Hot.MaxMemory == 0 {
		result.Config.Storage.Hot.MaxMemory = 4 * 1024 * 1024 * 1024 // 4GB
	}
	if result.Config.Storage.Hot.ShardCount == 0 {
		result.Config.Storage.Hot.ShardCount = 256
	}
	if result.Config.Storage.Hot.DefaultTTL == 0 {
		result.Config.Storage.Hot.DefaultTTL = 5 * time.Minute
	}
	if result.Config.Storage.Warm.Path == "" {
		result.Config.Storage.Warm.Path = "/var/lib/feather/data"
	}

	// Convert feature views to feature group configs
	for _, fv := range feast.FeatureViews {
		groupConfig := FeatureGroupConfig{
			Name: fv.Name,
			Tags: fv.Tags,
		}
		if fv.TTL != nil {
			groupConfig.TTL = *fv.TTL
		}
		if fv.Source != nil {
			groupConfig.Source = fv.Source.Type
		}
		result.Config.Features = append(result.Config.Features, groupConfig)
	}

	// Set default metrics config
	result.Config.Metrics = MetricsConfig{
		Enabled: true,
		Port:    9090,
	}

	return result, nil
}

func (c *ConfigMigrator) convertOnlineStore(store map[string]interface{}, result *ConfigConvertResult) {
	if store == nil {
		return
	}

	storeType, _ := store["type"].(string)

	switch storeType {
	case "redis":
		// Feather uses built-in hot tier, no Redis needed
		result.Warnings = append(result.Warnings, "Redis online store will be replaced by Feather's built-in hot tier")
		if connStr, ok := store["connection_string"].(string); ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Redis connection '%s' not needed", connStr))
		}

	case "sqlite":
		// Feather uses built-in storage
		result.Warnings = append(result.Warnings, "SQLite online store will be replaced by Feather's built-in storage")

	case "dynamodb":
		result.Warnings = append(result.Warnings, "DynamoDB online store will be replaced by Feather's built-in storage")

	case "datastore":
		result.Warnings = append(result.Warnings, "Datastore online store will be replaced by Feather's built-in storage")

	default:
		if storeType != "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown online store type '%s'", storeType))
		}
	}
}

func (c *ConfigMigrator) convertOfflineStore(store map[string]interface{}, result *ConfigConvertResult) {
	if store == nil {
		return
	}

	storeType, _ := store["type"].(string)

	switch storeType {
	case "file":
		// Map file path to warm tier
		if path, ok := store["path"].(string); ok {
			result.Config.Storage.Warm.Path = path + "/feather"
		}

	case "bigquery":
		result.Warnings = append(result.Warnings, "BigQuery offline store requires Feather warehouse connector (separate setup)")
		if project, ok := store["project"].(string); ok {
			result.Config.Features = append(result.Config.Features, FeatureGroupConfig{
				Name:   "_bigquery_source",
				Source: "bigquery",
				Tags:   map[string]string{"project": project},
			})
		}

	case "snowflake":
		result.Warnings = append(result.Warnings, "Snowflake offline store requires Feather warehouse connector (separate setup)")

	case "redshift":
		result.Warnings = append(result.Warnings, "Redshift offline store requires Feather warehouse connector (separate setup)")

	case "spark":
		result.Warnings = append(result.Warnings, "Spark offline store not directly supported, consider using Feather's export feature")

	default:
		if storeType != "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown offline store type '%s'", storeType))
		}
	}
}

// MigrationReport generates a comprehensive migration report.
type MigrationReport struct {
	ProjectName        string                  `json:"project_name"`
	GeneratedAt        time.Time               `json:"generated_at"`
	SchemaConversion   *SchemaConversionReport `json:"schema_conversion"`
	ConfigConversion   *ConfigConversionReport `json:"config_conversion"`
	DataMigration      *DataMigrationReport    `json:"data_migration,omitempty"`
	Recommendations    []string                `json:"recommendations"`
	EstimatedEffort    string                  `json:"estimated_effort"`
	CompatibilityScore float64                 `json:"compatibility_score"` // 0-100
}

// SchemaConversionReport reports on schema conversion.
type SchemaConversionReport struct {
	TotalEntities      int      `json:"total_entities"`
	TotalFeatureViews  int      `json:"total_feature_views"`
	TotalFeatures      int      `json:"total_features"`
	ConvertedGroups    int      `json:"converted_groups"`
	UnsupportedTypes   []string `json:"unsupported_types,omitempty"`
	RequiresManualWork []string `json:"requires_manual_work,omitempty"`
}

// ConfigConversionReport reports on config conversion.
type ConfigConversionReport struct {
	SourceProvider     string   `json:"source_provider"`
	OnlineStoreType    string   `json:"online_store_type"`
	OfflineStoreType   string   `json:"offline_store_type"`
	UnsupportedOptions []string `json:"unsupported_options,omitempty"`
}

// DataMigrationReport reports on data migration status.
type DataMigrationReport struct {
	TotalRecords      int64  `json:"total_records"`
	MigratedRecords   int64  `json:"migrated_records"`
	FailedRecords     int64  `json:"failed_records"`
	EstimatedDuration string `json:"estimated_duration"`
}

// GenerateMigrationReport creates a comprehensive migration analysis.
func GenerateMigrationReport(project *FeastProject, schemaResult *ConvertResult, configResult *ConfigConvertResult) *MigrationReport {
	report := &MigrationReport{
		ProjectName:     project.Name,
		GeneratedAt:     time.Now(),
		Recommendations: make([]string, 0),
	}

	// Schema report
	totalFeatures := 0
	for _, fv := range project.FeatureViews {
		totalFeatures += len(fv.Features)
	}
	for _, odfv := range project.OnDemandFeatureViews {
		totalFeatures += len(odfv.Features)
	}

	report.SchemaConversion = &SchemaConversionReport{
		TotalEntities:     len(project.Entities),
		TotalFeatureViews: len(project.FeatureViews) + len(project.OnDemandFeatureViews),
		TotalFeatures:     totalFeatures,
		ConvertedGroups:   len(schemaResult.FeatureGroups),
	}

	if len(schemaResult.Errors) > 0 {
		report.SchemaConversion.RequiresManualWork = schemaResult.Errors
	}

	// Config report
	onlineType := "unknown"
	offlineType := "unknown"
	if project.OnlineStore != nil {
		if t, ok := project.OnlineStore["type"].(string); ok {
			onlineType = t
		}
	}
	if project.OfflineStore != nil {
		if t, ok := project.OfflineStore["type"].(string); ok {
			offlineType = t
		}
	}

	report.ConfigConversion = &ConfigConversionReport{
		SourceProvider:   project.Provider,
		OnlineStoreType:  onlineType,
		OfflineStoreType: offlineType,
	}

	if configResult != nil && len(configResult.Warnings) > 0 {
		report.ConfigConversion.UnsupportedOptions = configResult.Warnings
	}

	// Generate recommendations
	if len(project.OnDemandFeatureViews) > 0 {
		report.Recommendations = append(report.Recommendations,
			"On-demand feature views with UDFs require manual migration to Feather transforms")
	}

	if offlineType == "bigquery" || offlineType == "snowflake" || offlineType == "redshift" {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("Configure Feather warehouse connector for %s integration", offlineType))
	}

	if len(project.FeatureServices) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Feature services can be recreated using Feather's API for grouped feature retrieval")
	}

	// Calculate compatibility score
	totalItems := float64(len(project.Entities) + len(project.FeatureViews) + len(project.OnDemandFeatureViews))
	convertedItems := float64(len(schemaResult.FeatureGroups))
	if totalItems > 0 {
		report.CompatibilityScore = (convertedItems / totalItems) * 100
	} else {
		report.CompatibilityScore = 100
	}

	// Subtract for warnings
	warningPenalty := float64(len(schemaResult.Warnings)+len(configResult.Warnings)) * 2
	report.CompatibilityScore = max(0, report.CompatibilityScore-warningPenalty)

	// Estimate effort
	if report.CompatibilityScore >= 90 {
		report.EstimatedEffort = "Low (1-2 hours)"
	} else if report.CompatibilityScore >= 70 {
		report.EstimatedEffort = "Medium (1-2 days)"
	} else if report.CompatibilityScore >= 50 {
		report.EstimatedEffort = "High (1 week)"
	} else {
		report.EstimatedEffort = "Very High (2+ weeks)"
	}

	return report
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
