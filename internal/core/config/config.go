package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all Feather configuration.
type Config struct {
	Schema    SchemaConfig    `yaml:"schema"`
	Storage   StorageConfig   `yaml:"storage"`
	Ingestion IngestionConfig `yaml:"ingestion"`
	Serving   ServingConfig   `yaml:"serving"`
	Sync      SyncConfig      `yaml:"sync"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Logging   LoggingConfig   `yaml:"logging"`
	Tracing   TracingConfig   `yaml:"tracing"`
	TLS       TLSConfig       `yaml:"tls"`
	UI        UIConfig        `yaml:"ui"`
	DBT       DBTConfig       `yaml:"dbt"`
}

// UIConfig defines feature catalog UI configuration.
type UIConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DBTConfig defines dbt integration configuration.
type DBTConfig struct {
	Enabled           bool              `yaml:"enabled"`
	DefaultEntityType string            `yaml:"default_entity_type"`
	Owner             string            `yaml:"owner"`
	Team              string            `yaml:"team"`
	IncludeSources    bool              `yaml:"include_sources"`
	IncludeMetrics    bool              `yaml:"include_metrics"`
	EntityTypeMapping map[string]string `yaml:"entity_type_mapping"`
}

// TLSConfig defines TLS/HTTPS configuration for all servers.
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	CAFile     string `yaml:"ca_file,omitempty"`
	MinVersion string `yaml:"min_version,omitempty"` // "1.2" or "1.3"
	ClientAuth string `yaml:"client_auth,omitempty"` // "none", "request", "require", "verify"
}

// TracingConfig defines OpenTelemetry tracing configuration.
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Endpoint    string  `yaml:"endpoint"`
	ServiceName string  `yaml:"service_name"`
	SampleRate  float64 `yaml:"sample_rate"`
	Insecure    bool    `yaml:"insecure"`
}

// SchemaConfig defines feature schema configuration.
type SchemaConfig struct {
	Groups []FeatureGroupConfig `yaml:"groups"`
}

// FeatureGroupConfig defines a feature group.
type FeatureGroupConfig struct {
	Name        string          `yaml:"name"`
	EntityType  string          `yaml:"entity_type"`
	TTL         time.Duration   `yaml:"ttl"`
	Features    []FeatureConfig `yaml:"features"`
	Description string          `yaml:"description,omitempty"`
}

// FeatureConfig defines a feature.
type FeatureConfig struct {
	Name        string             `yaml:"name"`
	DataType    string             `yaml:"data_type"`
	Dimensions  []int              `yaml:"dimensions,omitempty"`
	Default     interface{}        `yaml:"default,omitempty"`
	Aggregation *AggregationConfig `yaml:"aggregation,omitempty"`
}

// AggregationConfig defines aggregation settings.
type AggregationConfig struct {
	Function string        `yaml:"function"`
	Window   time.Duration `yaml:"window"`
	SlideBy  time.Duration `yaml:"slide_by,omitempty"`
}

// StorageConfig defines storage configuration.
type StorageConfig struct {
	Hot        HotStorageConfig  `yaml:"hot"`
	Warm       WarmStorageConfig `yaml:"warm"`
	Historical HistoricalConfig  `yaml:"historical"`
}

// HotStorageConfig defines hot tier settings.
type HotStorageConfig struct {
	MaxMemory      string `yaml:"max_memory"`
	EvictionPolicy string `yaml:"eviction_policy"`
}

// WarmStorageConfig defines warm tier settings.
type WarmStorageConfig struct {
	Path         string        `yaml:"path"`
	SyncInterval time.Duration `yaml:"sync_interval"`
}

// HistoricalConfig defines historical storage settings.
type HistoricalConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Retention time.Duration `yaml:"retention"`
}

// IngestionConfig defines ingestion configuration.
type IngestionConfig struct {
	Kafka KafkaIngestionConfig `yaml:"kafka"`
	HTTP  HTTPIngestionConfig  `yaml:"http"`
}

// KafkaIngestionConfig defines Kafka settings.
type KafkaIngestionConfig struct {
	Enabled       bool                `yaml:"enabled"`
	Brokers       []string            `yaml:"brokers"`
	Topic         string              `yaml:"topic"`
	ConsumerGroup string              `yaml:"consumer_group"`
	Security      KafkaSecurityConfig `yaml:"security,omitempty"`
}

// KafkaSecurityConfig defines Kafka authentication settings.
type KafkaSecurityConfig struct {
	Protocol      string `yaml:"protocol,omitempty"`       // "PLAINTEXT", "SSL", "SASL_PLAINTEXT", "SASL_SSL"
	SASLMechanism string `yaml:"sasl_mechanism,omitempty"` // "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	SASLUsername  string `yaml:"sasl_username,omitempty"`
	SASLPassword  string `yaml:"sasl_password,omitempty"`
	SSLCAFile     string `yaml:"ssl_ca_file,omitempty"`
	SSLCertFile   string `yaml:"ssl_cert_file,omitempty"`
	SSLKeyFile    string `yaml:"ssl_key_file,omitempty"`
}

// HTTPIngestionConfig defines HTTP ingestion settings.
type HTTPIngestionConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// ServingConfig defines serving configuration.
type ServingConfig struct {
	GRPC GRPCServingConfig `yaml:"grpc"`
	HTTP HTTPServingConfig `yaml:"http"`
}

// GRPCServingConfig defines gRPC server settings.
type GRPCServingConfig struct {
	Port          int `yaml:"port"`
	MaxConcurrent int `yaml:"max_concurrent"`
}

// HTTPServingConfig defines HTTP server settings.
type HTTPServingConfig struct {
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	TrustedProxies []string      `yaml:"trusted_proxies,omitempty"`
}

// SyncConfig defines sync configuration.
type SyncConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Mode           string        `yaml:"mode"` // "central" or "edge"
	CentralAddress string        `yaml:"central_address"`
	SyncInterval   time.Duration `yaml:"sync_interval"`
	BatchSize      int           `yaml:"batch_size"`
}

// MetricsConfig defines metrics configuration.
type MetricsConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// PrometheusConfig defines Prometheus settings.
type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// LoggingConfig defines logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	return LoadFromBytes(data)
}

// LoadFromBytes parses YAML configuration from raw bytes.
func LoadFromBytes(data []byte) (*Config, error) {
	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv() *Config {
	return &Config{
		Storage: StorageConfig{
			Hot: HotStorageConfig{
				MaxMemory:      getEnv("FEATHER_HOT_MAX_MEMORY", "4GB"),
				EvictionPolicy: getEnv("FEATHER_HOT_EVICTION", "lru"),
			},
			Warm: WarmStorageConfig{
				Path:         getEnv("FEATHER_WARM_PATH", ""),
				SyncInterval: getEnvAsDuration("FEATHER_WARM_SYNC_INTERVAL", time.Second),
			},
			Historical: HistoricalConfig{
				Enabled:   getEnvAsBool("FEATHER_HISTORICAL_ENABLED", false),
				Retention: getEnvAsDuration("FEATHER_HISTORICAL_RETENTION", 30*24*time.Hour),
			},
		},
		Ingestion: IngestionConfig{
			Kafka: KafkaIngestionConfig{
				Enabled:       getEnvAsBool("FEATHER_KAFKA_ENABLED", false),
				Brokers:       getEnvAsSlice("FEATHER_KAFKA_BROKERS", []string{"localhost:9092"}),
				Topic:         getEnv("FEATHER_KAFKA_TOPIC", "feature-updates"),
				ConsumerGroup: getEnv("FEATHER_KAFKA_CONSUMER_GROUP", "feather"),
			},
			HTTP: HTTPIngestionConfig{
				Enabled: getEnvAsBool("FEATHER_HTTP_INGESTION_ENABLED", true),
				Port:    getEnvAsInt("FEATHER_HTTP_INGESTION_PORT", 8081),
			},
		},
		Serving: ServingConfig{
			GRPC: GRPCServingConfig{
				Port:          getEnvAsInt("FEATHER_GRPC_PORT", 50051),
				MaxConcurrent: getEnvAsInt("FEATHER_GRPC_MAX_CONCURRENT", 1000),
			},
			HTTP: HTTPServingConfig{
				Port:         getEnvAsInt("FEATHER_HTTP_PORT", 8080),
				ReadTimeout:  getEnvAsDuration("FEATHER_HTTP_READ_TIMEOUT", 10*time.Second),
				WriteTimeout: getEnvAsDuration("FEATHER_HTTP_WRITE_TIMEOUT", 10*time.Second),
			},
		},
		Metrics: MetricsConfig{
			Prometheus: PrometheusConfig{
				Enabled: getEnvAsBool("FEATHER_PROMETHEUS_ENABLED", true),
				Port:    getEnvAsInt("FEATHER_PROMETHEUS_PORT", 9090),
			},
		},
		Logging: LoggingConfig{
			Level:  getEnv("FEATHER_LOG_LEVEL", "info"),
			Format: getEnv("FEATHER_LOG_FORMAT", "json"),
		},
		Tracing: TracingConfig{
			Enabled:     getEnvAsBool("FEATHER_TRACING_ENABLED", false),
			Endpoint:    getEnv("FEATHER_TRACING_ENDPOINT", "localhost:4317"),
			ServiceName: getEnv("FEATHER_TRACING_SERVICE_NAME", "feather"),
			SampleRate:  getEnvAsFloat("FEATHER_TRACING_SAMPLE_RATE", 0.1),
			Insecure:    getEnvAsBool("FEATHER_TRACING_INSECURE", false),
		},
		TLS: TLSConfig{
			Enabled:    getEnvAsBool("FEATHER_TLS_ENABLED", false),
			CertFile:   getEnv("FEATHER_TLS_CERT_FILE", ""),
			KeyFile:    getEnv("FEATHER_TLS_KEY_FILE", ""),
			CAFile:     getEnv("FEATHER_TLS_CA_FILE", ""),
			MinVersion: getEnv("FEATHER_TLS_MIN_VERSION", "1.2"),
			ClientAuth: getEnv("FEATHER_TLS_CLIENT_AUTH", "none"),
		},
		Sync: SyncConfig{
			Enabled:        getEnvAsBool("FEATHER_SYNC_ENABLED", false),
			Mode:           getEnv("FEATHER_SYNC_MODE", "edge"),
			CentralAddress: getEnv("FEATHER_SYNC_CENTRAL_ADDRESS", ""),
			SyncInterval:   getEnvAsDuration("FEATHER_SYNC_INTERVAL", 5*time.Second),
			BatchSize:      getEnvAsInt("FEATHER_SYNC_BATCH_SIZE", 1000),
		},
		UI: UIConfig{
			Enabled: getEnvAsBool("FEATHER_UI_ENABLED", true),
		},
		DBT: DBTConfig{
			Enabled:           getEnvAsBool("FEATHER_DBT_ENABLED", true),
			DefaultEntityType: getEnv("FEATHER_DBT_DEFAULT_ENTITY_TYPE", "unknown"),
			Owner:             getEnv("FEATHER_DBT_OWNER", ""),
			Team:              getEnv("FEATHER_DBT_TEAM", ""),
			IncludeSources:    getEnvAsBool("FEATHER_DBT_INCLUDE_SOURCES", false),
			IncludeMetrics:    getEnvAsBool("FEATHER_DBT_INCLUDE_METRICS", false),
		},
	}
}

// ParseMemorySize parses a memory size string like "4GB" to bytes.
func ParseMemorySize(s string) (int64, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid memory size: %s", s)
	}

	var multiplier int64 = 1
	suffix := s[len(s)-2:]

	switch suffix {
	case "KB", "kb":
		multiplier = 1024
		s = s[:len(s)-2]
	case "MB", "mb":
		multiplier = 1024 * 1024
		s = s[:len(s)-2]
	case "GB", "gb":
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-2]
	case "TB", "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-2]
	default:
		// Check single letter suffixes
		suffix = s[len(s)-1:]
		switch suffix {
		case "K", "k":
			multiplier = 1024
			s = s[:len(s)-1]
		case "M", "m":
			multiplier = 1024 * 1024
			s = s[:len(s)-1]
		case "G", "g":
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		case "T", "t":
			multiplier = 1024 * 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing memory size: %w", err)
	}

	return val * multiplier, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
		slog.Warn("invalid integer env var, using default", "key", key, "value", val, "default", defaultVal)
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
		slog.Warn("invalid boolean env var, using default", "key", key, "value", val, "default", defaultVal)
	}
	return defaultVal
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		slog.Warn("invalid float env var, using default", "key", key, "value", val, "default", defaultVal)
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
		slog.Warn("invalid duration env var, using default", "key", key, "value", val, "default", defaultVal)
	}
	return defaultVal
}

func getEnvAsSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		// Split by comma and trim whitespace from each element
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config validation: %s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	msg := fmt.Sprintf("%d configuration errors:\n", len(e))
	for _, err := range e {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}
	return msg
}

// Validate validates the configuration and returns any errors.
func (c *Config) Validate() error {
	var errs ValidationErrors

	// Validate storage
	if c.Storage.Hot.MaxMemory != "" {
		if _, err := ParseMemorySize(c.Storage.Hot.MaxMemory); err != nil {
			errs = append(errs, ValidationError{
				Field:   "storage.hot.max_memory",
				Message: fmt.Sprintf("invalid memory size: %v", err),
			})
		}
	}

	if c.Storage.Warm.Path == "" && c.Storage.Historical.Enabled {
		errs = append(errs, ValidationError{
			Field:   "storage.warm.path",
			Message: "path required when historical storage is enabled",
		})
	}

	// Validate serving ports
	if c.Serving.HTTP.Port < 0 || c.Serving.HTTP.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "serving.http.port",
			Message: fmt.Sprintf("invalid port: %d", c.Serving.HTTP.Port),
		})
	}

	if c.Serving.GRPC.Port < 0 || c.Serving.GRPC.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "serving.grpc.port",
			Message: fmt.Sprintf("invalid port: %d", c.Serving.GRPC.Port),
		})
	}

	// Validate Kafka config if enabled
	if c.Ingestion.Kafka.Enabled {
		if len(c.Ingestion.Kafka.Brokers) == 0 {
			errs = append(errs, ValidationError{
				Field:   "ingestion.kafka.brokers",
				Message: "brokers required when Kafka is enabled",
			})
		}
		if c.Ingestion.Kafka.Topic == "" {
			errs = append(errs, ValidationError{
				Field:   "ingestion.kafka.topic",
				Message: "topic required when Kafka is enabled",
			})
		}
		if c.Ingestion.Kafka.ConsumerGroup == "" {
			errs = append(errs, ValidationError{
				Field:   "ingestion.kafka.consumer_group",
				Message: "consumer_group required when Kafka is enabled",
			})
		}
	}

	// Validate HTTP ingestion port if enabled
	if c.Ingestion.HTTP.Enabled {
		if c.Ingestion.HTTP.Port < 0 || c.Ingestion.HTTP.Port > 65535 {
			errs = append(errs, ValidationError{
				Field:   "ingestion.http.port",
				Message: fmt.Sprintf("invalid port: %d", c.Ingestion.HTTP.Port),
			})
		}
	}

	// Validate metrics port if enabled
	if c.Metrics.Prometheus.Enabled {
		if c.Metrics.Prometheus.Port < 0 || c.Metrics.Prometheus.Port > 65535 {
			errs = append(errs, ValidationError{
				Field:   "metrics.prometheus.port",
				Message: fmt.Sprintf("invalid port: %d", c.Metrics.Prometheus.Port),
			})
		}
	}

	// Validate logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
	if c.Logging.Level != "" && !validLevels[c.Logging.Level] {
		errs = append(errs, ValidationError{
			Field:   "logging.level",
			Message: fmt.Sprintf("invalid level '%s', must be one of: debug, info, warn, error", c.Logging.Level),
		})
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if c.Logging.Format != "" && !validFormats[c.Logging.Format] {
		errs = append(errs, ValidationError{
			Field:   "logging.format",
			Message: fmt.Sprintf("invalid format '%s', must be one of: json, text", c.Logging.Format),
		})
	}

	// Validate feature groups
	for i, group := range c.Schema.Groups {
		if group.Name == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("schema.groups[%d].name", i),
				Message: "name is required",
			})
		}
		if group.EntityType == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("schema.groups[%d].entity_type", i),
				Message: "entity_type is required",
			})
		}
		for j, feature := range group.Features {
			if feature.Name == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("schema.groups[%d].features[%d].name", i, j),
					Message: "name is required",
				})
			}
			validTypes := map[string]bool{
				"int64": true, "float64": true, "string": true,
				"bool": true, "bytes": true, "vector": true,
				"timestamp": true,
			}
			if feature.DataType != "" && !validTypes[feature.DataType] {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("schema.groups[%d].features[%d].data_type", i, j),
					Message: fmt.Sprintf("invalid data type '%s'", feature.DataType),
				})
			}
		}
	}

	// Validate TLS configuration
	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			errs = append(errs, ValidationError{
				Field:   "tls.cert_file",
				Message: "certificate file required when TLS is enabled",
			})
		}
		if c.TLS.KeyFile == "" {
			errs = append(errs, ValidationError{
				Field:   "tls.key_file",
				Message: "key file required when TLS is enabled",
			})
		}
		validMinVersions := map[string]bool{"1.2": true, "1.3": true}
		if c.TLS.MinVersion != "" && !validMinVersions[c.TLS.MinVersion] {
			errs = append(errs, ValidationError{
				Field:   "tls.min_version",
				Message: fmt.Sprintf("invalid TLS version '%s', must be '1.2' or '1.3'", c.TLS.MinVersion),
			})
		}
		validClientAuth := map[string]bool{"none": true, "request": true, "require": true, "verify": true}
		if c.TLS.ClientAuth != "" && !validClientAuth[c.TLS.ClientAuth] {
			errs = append(errs, ValidationError{
				Field:   "tls.client_auth",
				Message: fmt.Sprintf("invalid client auth '%s', must be one of: none, request, require, verify", c.TLS.ClientAuth),
			})
		}
	}

	// Check for port conflicts
	ports := make(map[int]string)
	if c.Serving.HTTP.Port > 0 {
		ports[c.Serving.HTTP.Port] = "serving.http.port"
	}
	if c.Serving.GRPC.Port > 0 {
		if existing, ok := ports[c.Serving.GRPC.Port]; ok {
			errs = append(errs, ValidationError{
				Field:   "serving.grpc.port",
				Message: fmt.Sprintf("port %d conflicts with %s", c.Serving.GRPC.Port, existing),
			})
		}
		ports[c.Serving.GRPC.Port] = "serving.grpc.port"
	}
	if c.Ingestion.HTTP.Enabled && c.Ingestion.HTTP.Port > 0 {
		if existing, ok := ports[c.Ingestion.HTTP.Port]; ok {
			errs = append(errs, ValidationError{
				Field:   "ingestion.http.port",
				Message: fmt.Sprintf("port %d conflicts with %s", c.Ingestion.HTTP.Port, existing),
			})
		}
		ports[c.Ingestion.HTTP.Port] = "ingestion.http.port"
	}
	if c.Metrics.Prometheus.Enabled && c.Metrics.Prometheus.Port > 0 {
		if existing, ok := ports[c.Metrics.Prometheus.Port]; ok {
			errs = append(errs, ValidationError{
				Field:   "metrics.prometheus.port",
				Message: fmt.Sprintf("port %d conflicts with %s", c.Metrics.Prometheus.Port, existing),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// BuildTLSConfig creates a *tls.Config from TLSConfig settings.
// Returns nil if TLS is not enabled.
func (c *TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Set minimum TLS version
	switch c.MinVersion {
	case "1.3":
		tlsConfig.MinVersion = tls.VersionTLS13
	case "1.2", "":
		tlsConfig.MinVersion = tls.VersionTLS12
	default:
		return nil, fmt.Errorf("unsupported TLS version: %s", c.MinVersion)
	}

	// Set client authentication mode
	switch c.ClientAuth {
	case "none", "":
		tlsConfig.ClientAuth = tls.NoClientCert
	case "request":
		tlsConfig.ClientAuth = tls.RequestClientCert
	case "require":
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	case "verify":
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		return nil, fmt.Errorf("unsupported client auth mode: %s", c.ClientAuth)
	}

	// Load CA certificate for client verification if specified
	if c.CAFile != "" {
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// LoadCertificate loads the TLS certificate and key from files.
func (c *TLSConfig) LoadCertificate() (tls.Certificate, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return tls.Certificate{}, fmt.Errorf("certificate and key files are required")
	}
	return tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
}
