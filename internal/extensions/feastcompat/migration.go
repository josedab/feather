package feastcompat

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FeastConfig represents a Feast feature_store.yaml configuration.
type FeastConfig struct {
	Project                  string           `yaml:"project" json:"project"`
	Registry                 string           `yaml:"registry" json:"registry,omitempty"`
	Provider                 string           `yaml:"provider" json:"provider"`
	OnlineStore              *FeastOnlineStore `yaml:"online_store" json:"online_store,omitempty"`
	OfflineStore             *FeastOfflineStore `yaml:"offline_store" json:"offline_store,omitempty"`
	EntityKeySerializationV2 bool             `yaml:"entity_key_serialization_version" json:"entity_key_serialization_version,omitempty"`
	Flags                    map[string]bool  `yaml:"flags" json:"flags,omitempty"`
}

// FeastOnlineStore describes a Feast online store configuration.
type FeastOnlineStore struct {
	Type string `yaml:"type" json:"type"`
	Path string `yaml:"path" json:"path,omitempty"`
}

// FeastOfflineStore describes a Feast offline store configuration.
type FeastOfflineStore struct {
	Type string `yaml:"type" json:"type"`
}

// FeatherConfig represents the generated Feather configuration.
type FeatherConfig struct {
	Serving   FeatherServing   `yaml:"serving" json:"serving"`
	Storage   FeatherStorage   `yaml:"storage" json:"storage"`
	Schema    FeatherSchema    `yaml:"schema" json:"schema"`
	Ingestion FeatherIngestion `yaml:"ingestion" json:"ingestion"`
	Logging   FeatherLogging   `yaml:"logging" json:"logging"`
}

// FeatherServing configures Feather's serving layer.
type FeatherServing struct {
	HTTP FeatherHTTP `yaml:"http" json:"http"`
	GRPC FeatherGRPC `yaml:"grpc" json:"grpc"`
}

// FeatherHTTP configures HTTP serving.
type FeatherHTTP struct {
	Port         int    `yaml:"port" json:"port"`
	ReadTimeout  string `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout" json:"write_timeout"`
}

// FeatherGRPC configures gRPC serving.
type FeatherGRPC struct {
	Port int `yaml:"port" json:"port"`
}

// FeatherStorage configures Feather storage.
type FeatherStorage struct {
	Hot  FeatherHotStorage  `yaml:"hot" json:"hot"`
	Warm FeatherWarmStorage `yaml:"warm" json:"warm"`
}

// FeatherHotStorage configures the hot (in-memory) tier.
type FeatherHotStorage struct {
	MaxMemory string `yaml:"max_memory" json:"max_memory"`
}

// FeatherWarmStorage configures the warm (persistent) tier.
type FeatherWarmStorage struct {
	Path string `yaml:"path" json:"path"`
}

// FeatherSchema configures the feature schema.
type FeatherSchema struct {
	Groups []FeatherGroupConfig `yaml:"groups" json:"groups"`
}

// FeatherGroupConfig defines a feature group in Feather config.
type FeatherGroupConfig struct {
	Name       string              `yaml:"name" json:"name"`
	EntityType string              `yaml:"entity_type" json:"entity_type"`
	TTL        time.Duration       `yaml:"ttl" json:"ttl,omitempty"`
	Features   []FeatherFeatureConfig `yaml:"features" json:"features"`
}

// FeatherFeatureConfig defines a feature in Feather config.
type FeatherFeatureConfig struct {
	Name     string `yaml:"name" json:"name"`
	DataType string `yaml:"data_type" json:"data_type"`
}

// FeatherIngestion configures data ingestion.
type FeatherIngestion struct {
	HTTP FeatherIngestionHTTP `yaml:"http" json:"http"`
}

// FeatherIngestionHTTP configures HTTP ingestion.
type FeatherIngestionHTTP struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
}

// FeatherLogging configures logging.
type FeatherLogging struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

// MigrationResult holds the outcome of a config migration.
type MigrationResult struct {
	Success      bool           `json:"success"`
	FeatherYAML  string         `json:"feather_yaml"`
	Config       *FeatherConfig `json:"config"`
	Warnings     []string       `json:"warnings,omitempty"`
	Mappings     int            `json:"mappings_created"`
}

// MigrateFromFeastConfig converts a Feast feature_store.yaml to Feather config.
func MigrateFromFeastConfig(feastYAML []byte) (*MigrationResult, error) {
	var feast FeastConfig
	if err := yaml.Unmarshal(feastYAML, &feast); err != nil {
		return nil, fmt.Errorf("parsing feast config: %w", err)
	}

	result := &MigrationResult{Success: true}

	cfg := &FeatherConfig{
		Serving: FeatherServing{
			HTTP: FeatherHTTP{
				Port:         8080,
				ReadTimeout:  "10s",
				WriteTimeout: "30s",
			},
			GRPC: FeatherGRPC{Port: 50051},
		},
		Storage: FeatherStorage{
			Hot:  FeatherHotStorage{MaxMemory: "1GB"},
			Warm: FeatherWarmStorage{Path: ":memory:"},
		},
		Ingestion: FeatherIngestion{
			HTTP: FeatherIngestionHTTP{Enabled: true, Port: 8081},
		},
		Logging: FeatherLogging{Level: "info", Format: "json"},
	}

	// Map online store
	if feast.OnlineStore != nil {
		switch strings.ToLower(feast.OnlineStore.Type) {
		case "sqlite":
			cfg.Storage.Warm.Path = feast.OnlineStore.Path
			if cfg.Storage.Warm.Path == "" {
				cfg.Storage.Warm.Path = "/var/lib/feather/data"
			}
		case "redis":
			result.Warnings = append(result.Warnings,
				"Redis online store mapped to Feather hot tier; consider increasing max_memory")
			cfg.Storage.Hot.MaxMemory = "4GB"
		default:
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Unknown online store type %q; using Feather defaults", feast.OnlineStore.Type))
		}
	}

	// Map project name to a default feature group
	if feast.Project != "" {
		cfg.Schema.Groups = append(cfg.Schema.Groups, FeatherGroupConfig{
			Name:       feast.Project,
			EntityType: feast.Project,
		})
		result.Mappings++
	}

	result.Config = cfg

	// Serialize to YAML
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("serializing feather config: %w", err)
	}
	result.FeatherYAML = string(out)

	return result, nil
}
