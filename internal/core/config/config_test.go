package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Two-letter suffixes
		{"KB lowercase", "100kb", 100 * 1024, false},
		{"KB uppercase", "100KB", 100 * 1024, false},
		{"MB lowercase", "64mb", 64 * 1024 * 1024, false},
		{"MB uppercase", "64MB", 64 * 1024 * 1024, false},
		{"GB lowercase", "4gb", 4 * 1024 * 1024 * 1024, false},
		{"GB uppercase", "4GB", 4 * 1024 * 1024 * 1024, false},
		{"TB lowercase", "1tb", 1 * 1024 * 1024 * 1024 * 1024, false},
		{"TB uppercase", "1TB", 1 * 1024 * 1024 * 1024 * 1024, false},

		// Single-letter suffixes
		{"K lowercase", "512k", 512 * 1024, false},
		{"K uppercase", "512K", 512 * 1024, false},
		{"M lowercase", "128m", 128 * 1024 * 1024, false},
		{"M uppercase", "128M", 128 * 1024 * 1024, false},
		{"G lowercase", "2g", 2 * 1024 * 1024 * 1024, false},
		{"G uppercase", "2G", 2 * 1024 * 1024 * 1024, false},
		{"T lowercase", "2t", 2 * 1024 * 1024 * 1024 * 1024, false},
		{"T uppercase", "2T", 2 * 1024 * 1024 * 1024 * 1024, false},

		// Error cases
		{"too short", "1", 0, true},
		{"invalid number", "abc", 0, true},
		{"empty string", "", 0, true},
		{"negative value", "-1GB", 0, true},
		{"negative small", "-100KB", 0, true},
		{"overflow TB", "9999999999TB", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemorySize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseMemorySize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore environment
	saveEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range saveEnv {
			os.Setenv(e[:indexOf(e, '=')], e[indexOf(e, '=')+1:])
		}
	}()

	os.Clearenv()
	os.Setenv("FEATHER_HTTP_PORT", "9000")
	os.Setenv("FEATHER_GRPC_PORT", "50052")
	os.Setenv("FEATHER_LOG_LEVEL", "debug")
	os.Setenv("FEATHER_KAFKA_ENABLED", "true")
	os.Setenv("FEATHER_TLS_ENABLED", "true")

	cfg := LoadFromEnv()

	if cfg.Serving.HTTP.Port != 9000 {
		t.Errorf("HTTP port = %d, want 9000", cfg.Serving.HTTP.Port)
	}
	if cfg.Serving.GRPC.Port != 50052 {
		t.Errorf("GRPC port = %d, want 50052", cfg.Serving.GRPC.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Log level = %q, want 'debug'", cfg.Logging.Level)
	}
	if !cfg.Ingestion.Kafka.Enabled {
		t.Error("Kafka should be enabled")
	}
	if !cfg.TLS.Enabled {
		t.Error("TLS should be enabled")
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	// Save and restore environment
	saveEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range saveEnv {
			os.Setenv(e[:indexOf(e, '=')], e[indexOf(e, '=')+1:])
		}
	}()

	os.Clearenv()
	cfg := LoadFromEnv()

	if cfg.Serving.HTTP.Port != 8080 {
		t.Errorf("default HTTP port = %d, want 8080", cfg.Serving.HTTP.Port)
	}
	if cfg.Serving.GRPC.Port != 50051 {
		t.Errorf("default GRPC port = %d, want 50051", cfg.Serving.GRPC.Port)
	}
	if cfg.Metrics.Prometheus.Port != 9090 {
		t.Errorf("default Prometheus port = %d, want 9090", cfg.Metrics.Prometheus.Port)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default log level = %q, want 'info'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("default log format = %q, want 'json'", cfg.Logging.Format)
	}
	if cfg.Ingestion.Kafka.Enabled {
		t.Error("Kafka should be disabled by default")
	}
	if cfg.TLS.Enabled {
		t.Error("TLS should be disabled by default")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
serving:
  http:
    port: 8888
    read_timeout: 5s
    write_timeout: 10s
  grpc:
    port: 50050
    max_concurrent: 500

storage:
  hot:
    max_memory: "2GB"
  warm:
    path: "/tmp/feather"

logging:
  level: "debug"
  format: "text"

tls:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error: %v", err)
	}

	if cfg.Serving.HTTP.Port != 8888 {
		t.Errorf("HTTP port = %d, want 8888", cfg.Serving.HTTP.Port)
	}
	if cfg.Serving.GRPC.Port != 50050 {
		t.Errorf("GRPC port = %d, want 50050", cfg.Serving.GRPC.Port)
	}
	if cfg.Storage.Hot.MaxMemory != "2GB" {
		t.Errorf("MaxMemory = %q, want '2GB'", cfg.Storage.Hot.MaxMemory)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Log level = %q, want 'debug'", cfg.Logging.Level)
	}
	if !cfg.TLS.Enabled {
		t.Error("TLS should be enabled")
	}
}

func TestLoadFromFile_EnvExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Save and restore environment
	saveEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range saveEnv {
			os.Setenv(e[:indexOf(e, '=')], e[indexOf(e, '=')+1:])
		}
	}()

	os.Setenv("FEATHER_TEST_PORT", "9999")

	configContent := `
serving:
  http:
    port: ${FEATHER_TEST_PORT}
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error: %v", err)
	}

	if cfg.Serving.HTTP.Port != 9999 {
		t.Errorf("HTTP port = %d, want 9999", cfg.Serving.HTTP.Port)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		Serving: ServingConfig{
			HTTP: HTTPServingConfig{Port: 8080},
			GRPC: GRPCServingConfig{Port: 50051},
		},
		Ingestion: IngestionConfig{
			HTTP: HTTPIngestionConfig{Enabled: true, Port: 8081},
		},
		Metrics: MetricsConfig{
			Prometheus: PrometheusConfig{Enabled: true, Port: 9090},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Storage: StorageConfig{
			Hot: HotStorageConfig{MaxMemory: "4GB"},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "HTTP port too high",
			config: Config{
				Serving: ServingConfig{
					HTTP: HTTPServingConfig{Port: 70000},
				},
			},
		},
		{
			name: "GRPC port negative",
			config: Config{
				Serving: ServingConfig{
					GRPC: GRPCServingConfig{Port: -1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_PortConflict(t *testing.T) {
	cfg := &Config{
		Serving: ServingConfig{
			HTTP: HTTPServingConfig{Port: 8080},
			GRPC: GRPCServingConfig{Port: 8080}, // Conflict!
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected port conflict error")
	}

	// Check that the error mentions the conflict
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		found := false
		for _, verr := range verrs {
			if verr.Field == "serving.grpc.port" {
				found = true
			}
		}
		if !found {
			t.Error("expected error for grpc port conflict")
		}
	}
}

func TestConfig_Validate_InvalidLogging(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"invalid level", "verbose", "json"},
		{"invalid format", "info", "xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Logging: LoggingConfig{
					Level:  tt.level,
					Format: tt.format,
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_KafkaEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  KafkaIngestionConfig
		wantErr bool
	}{
		{
			name: "valid kafka config",
			config: KafkaIngestionConfig{
				Enabled:       true,
				Brokers:       []string{"localhost:9092"},
				Topic:         "features",
				ConsumerGroup: "feather",
			},
			wantErr: false,
		},
		{
			name: "missing brokers",
			config: KafkaIngestionConfig{
				Enabled:       true,
				Brokers:       []string{},
				Topic:         "features",
				ConsumerGroup: "feather",
			},
			wantErr: true,
		},
		{
			name: "missing topic",
			config: KafkaIngestionConfig{
				Enabled:       true,
				Brokers:       []string{"localhost:9092"},
				Topic:         "",
				ConsumerGroup: "feather",
			},
			wantErr: true,
		},
		{
			name: "missing consumer group",
			config: KafkaIngestionConfig{
				Enabled:       true,
				Brokers:       []string{"localhost:9092"},
				Topic:         "features",
				ConsumerGroup: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Ingestion: IngestionConfig{
					Kafka: tt.config,
				},
			}

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_Validate_TLS(t *testing.T) {
	tests := []struct {
		name    string
		config  TLSConfig
		wantErr bool
	}{
		{
			name: "valid TLS config",
			config: TLSConfig{
				Enabled:    true,
				CertFile:   "/path/to/cert.pem",
				KeyFile:    "/path/to/key.pem",
				MinVersion: "1.2",
				ClientAuth: "none",
			},
			wantErr: false,
		},
		{
			name: "missing cert file",
			config: TLSConfig{
				Enabled:  true,
				CertFile: "",
				KeyFile:  "/path/to/key.pem",
			},
			wantErr: true,
		},
		{
			name: "missing key file",
			config: TLSConfig{
				Enabled:  true,
				CertFile: "/path/to/cert.pem",
				KeyFile:  "",
			},
			wantErr: true,
		},
		{
			name: "invalid min version",
			config: TLSConfig{
				Enabled:    true,
				CertFile:   "/path/to/cert.pem",
				KeyFile:    "/path/to/key.pem",
				MinVersion: "1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid client auth",
			config: TLSConfig{
				Enabled:    true,
				CertFile:   "/path/to/cert.pem",
				KeyFile:    "/path/to/key.pem",
				ClientAuth: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{TLS: tt.config}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_Validate_FeatureGroups(t *testing.T) {
	tests := []struct {
		name    string
		groups  []FeatureGroupConfig
		wantErr bool
	}{
		{
			name: "valid feature group",
			groups: []FeatureGroupConfig{
				{
					Name:       "user_features",
					EntityType: "user",
					Features: []FeatureConfig{
						{Name: "score", DataType: "float64"},
						{Name: "rank", DataType: "int64"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing group name",
			groups: []FeatureGroupConfig{
				{
					Name:       "",
					EntityType: "user",
				},
			},
			wantErr: true,
		},
		{
			name: "missing entity type",
			groups: []FeatureGroupConfig{
				{
					Name:       "user_features",
					EntityType: "",
				},
			},
			wantErr: true,
		},
		{
			name: "missing feature name",
			groups: []FeatureGroupConfig{
				{
					Name:       "user_features",
					EntityType: "user",
					Features: []FeatureConfig{
						{Name: "", DataType: "float64"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid data type",
			groups: []FeatureGroupConfig{
				{
					Name:       "user_features",
					EntityType: "user",
					Features: []FeatureConfig{
						{Name: "score", DataType: "invalid_type"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Schema: SchemaConfig{
					Groups: tt.groups,
				},
			}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_Validate_InvalidMemorySize(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			Hot: HotStorageConfig{
				MaxMemory: "invalid",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for invalid memory size")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "serving.http.port",
		Message: "invalid port",
	}

	want := "config validation: serving.http.port: invalid port"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name   string
		errors ValidationErrors
		want   string
	}{
		{
			name:   "empty errors",
			errors: ValidationErrors{},
			want:   "",
		},
		{
			name: "single error",
			errors: ValidationErrors{
				{Field: "foo", Message: "bar"},
			},
			want: "config validation: foo: bar",
		},
		{
			name: "multiple errors",
			errors: ValidationErrors{
				{Field: "foo", Message: "bar"},
				{Field: "baz", Message: "qux"},
			},
			want: "2 configuration errors:\n  - foo: bar\n  - baz: qux\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errors.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", tt.errors.Error(), tt.want)
			}
		})
	}
}

func TestTLSConfig_BuildTLSConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     TLSConfig
		wantNil    bool
		wantErr    bool
		wantMinVer uint16
	}{
		{
			name:    "disabled",
			config:  TLSConfig{Enabled: false},
			wantNil: true,
		},
		{
			name: "TLS 1.2",
			config: TLSConfig{
				Enabled:    true,
				MinVersion: "1.2",
			},
			wantMinVer: 0x0303, // TLS 1.2
		},
		{
			name: "TLS 1.3",
			config: TLSConfig{
				Enabled:    true,
				MinVersion: "1.3",
			},
			wantMinVer: 0x0304, // TLS 1.3
		},
		{
			name: "default version",
			config: TLSConfig{
				Enabled:    true,
				MinVersion: "",
			},
			wantMinVer: 0x0303, // TLS 1.2
		},
		{
			name: "invalid version",
			config: TLSConfig{
				Enabled:    true,
				MinVersion: "1.1",
			},
			wantErr: true,
		},
		{
			name: "client auth none",
			config: TLSConfig{
				Enabled:    true,
				ClientAuth: "none",
			},
		},
		{
			name: "client auth request",
			config: TLSConfig{
				Enabled:    true,
				ClientAuth: "request",
			},
		},
		{
			name: "client auth require",
			config: TLSConfig{
				Enabled:    true,
				ClientAuth: "require",
			},
		},
		{
			name: "client auth verify",
			config: TLSConfig{
				Enabled:    true,
				ClientAuth: "verify",
			},
		},
		{
			name: "invalid client auth",
			config: TLSConfig{
				Enabled:    true,
				ClientAuth: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := tt.config.BuildTLSConfig()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if tlsCfg != nil {
					t.Error("expected nil tls.Config")
				}
				return
			}

			if tt.wantMinVer != 0 && tlsCfg.MinVersion != tt.wantMinVer {
				t.Errorf("MinVersion = %x, want %x", tlsCfg.MinVersion, tt.wantMinVer)
			}
		})
	}
}

func TestTLSConfig_LoadCertificate_MissingFiles(t *testing.T) {
	config := TLSConfig{
		Enabled:  true,
		CertFile: "",
		KeyFile:  "",
	}

	_, err := config.LoadCertificate()
	if err == nil {
		t.Error("expected error for missing cert/key files")
	}
}

func TestGetEnvHelpers(t *testing.T) {
	// Save and restore environment
	saveEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range saveEnv {
			os.Setenv(e[:indexOf(e, '=')], e[indexOf(e, '=')+1:])
		}
	}()

	os.Clearenv()

	t.Run("getEnv", func(t *testing.T) {
		if got := getEnv("NONEXISTENT", "default"); got != "default" {
			t.Errorf("getEnv() = %q, want 'default'", got)
		}
		os.Setenv("TEST_VAR", "value")
		if got := getEnv("TEST_VAR", "default"); got != "value" {
			t.Errorf("getEnv() = %q, want 'value'", got)
		}
	})

	t.Run("getEnvAsInt", func(t *testing.T) {
		if got := getEnvAsInt("NONEXISTENT", 42); got != 42 {
			t.Errorf("getEnvAsInt() = %d, want 42", got)
		}
		os.Setenv("INT_VAR", "100")
		if got := getEnvAsInt("INT_VAR", 42); got != 100 {
			t.Errorf("getEnvAsInt() = %d, want 100", got)
		}
		os.Setenv("INVALID_INT", "not_a_number")
		if got := getEnvAsInt("INVALID_INT", 42); got != 42 {
			t.Errorf("getEnvAsInt() with invalid = %d, want 42", got)
		}
	})

	t.Run("getEnvAsBool", func(t *testing.T) {
		if got := getEnvAsBool("NONEXISTENT", true); !got {
			t.Errorf("getEnvAsBool() = %v, want true", got)
		}
		os.Setenv("BOOL_VAR", "false")
		if got := getEnvAsBool("BOOL_VAR", true); got {
			t.Errorf("getEnvAsBool() = %v, want false", got)
		}
		os.Setenv("INVALID_BOOL", "maybe")
		if got := getEnvAsBool("INVALID_BOOL", true); !got {
			t.Errorf("getEnvAsBool() with invalid = %v, want true", got)
		}
	})

	t.Run("getEnvAsFloat", func(t *testing.T) {
		if got := getEnvAsFloat("NONEXISTENT", 3.14); got != 3.14 {
			t.Errorf("getEnvAsFloat() = %v, want 3.14", got)
		}
		os.Setenv("FLOAT_VAR", "2.71")
		if got := getEnvAsFloat("FLOAT_VAR", 3.14); got != 2.71 {
			t.Errorf("getEnvAsFloat() = %v, want 2.71", got)
		}
		os.Setenv("INVALID_FLOAT", "not_a_float")
		if got := getEnvAsFloat("INVALID_FLOAT", 3.14); got != 3.14 {
			t.Errorf("getEnvAsFloat() with invalid = %v, want 3.14", got)
		}
	})

	t.Run("getEnvAsDuration", func(t *testing.T) {
		defaultDur := 5 * time.Second
		if got := getEnvAsDuration("NONEXISTENT", defaultDur); got != defaultDur {
			t.Errorf("getEnvAsDuration() = %v, want %v", got, defaultDur)
		}
		os.Setenv("DUR_VAR", "10s")
		if got := getEnvAsDuration("DUR_VAR", defaultDur); got != 10*time.Second {
			t.Errorf("getEnvAsDuration() = %v, want 10s", got)
		}
		os.Setenv("INVALID_DUR", "not_a_duration")
		if got := getEnvAsDuration("INVALID_DUR", defaultDur); got != defaultDur {
			t.Errorf("getEnvAsDuration() with invalid = %v, want %v", got, defaultDur)
		}
	})

	t.Run("getEnvAsSlice", func(t *testing.T) {
		defaultSlice := []string{"default"}
		got := getEnvAsSlice("NONEXISTENT", defaultSlice)
		if len(got) != 1 || got[0] != "default" {
			t.Errorf("getEnvAsSlice() = %v, want %v", got, defaultSlice)
		}
		os.Setenv("SLICE_VAR", "value1")
		got = getEnvAsSlice("SLICE_VAR", defaultSlice)
		if len(got) != 1 || got[0] != "value1" {
			t.Errorf("getEnvAsSlice() = %v, want [value1]", got)
		}
	})
}

// Helper function to find index of character in string
func TestLoadFromBytes_RejectsUnknownFields(t *testing.T) {
	yaml := []byte(`
storrage:
  hot:
    max_memory: 1GB
`)
	_, err := LoadFromBytes(yaml)
	if err == nil {
		t.Fatal("expected error for unknown field 'storrage', got nil")
	}
}

func TestLoadFromBytes_AcceptsKnownFields(t *testing.T) {
	yaml := []byte(`
storage:
  hot:
    max_memory: 1GB
`)
	cfg, err := LoadFromBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error for valid config: %v", err)
	}
	if cfg.Storage.Hot.MaxMemory != "1GB" {
		t.Errorf("expected max_memory=1GB, got %s", cfg.Storage.Hot.MaxMemory)
	}
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func TestConfig_Validate_TracingSampleRate(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		wantErr    bool
	}{
		{"valid zero", 0.0, false},
		{"valid mid", 0.5, false},
		{"valid one", 1.0, false},
		{"negative", -0.1, true},
		{"too high", 1.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Tracing: TracingConfig{
					Enabled:    true,
					SampleRate: tt.sampleRate,
					Endpoint:   "localhost:4317",
				},
			}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_Validate_TracingEndpointRequired(t *testing.T) {
	cfg := &Config{
		Tracing: TracingConfig{
			Enabled:    true,
			SampleRate: 0.5,
			Endpoint:   "",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing tracing endpoint")
	}
}

func TestConfig_Validate_NegativeTimeouts(t *testing.T) {
	cfg := &Config{
		Serving: ServingConfig{
			HTTP: HTTPServingConfig{
				Port:        8080,
				ReadTimeout: -1 * time.Second,
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for negative read timeout")
	}

	cfg2 := &Config{
		Serving: ServingConfig{
			HTTP: HTTPServingConfig{
				Port:         8080,
				WriteTimeout: -1 * time.Second,
			},
		},
	}
	err = cfg2.Validate()
	if err == nil {
		t.Error("expected validation error for negative write timeout")
	}
}

func TestConfig_Validate_NegativeMaxConcurrent(t *testing.T) {
	cfg := &Config{
		Serving: ServingConfig{
			GRPC: GRPCServingConfig{
				Port:          50051,
				MaxConcurrent: -1,
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for negative max concurrent")
	}
}

func TestConfig_Validate_SyncConfig(t *testing.T) {
	cfg := &Config{
		Sync: SyncConfig{
			Enabled:      true,
			BatchSize:    0,
			SyncInterval: 5 * time.Second,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for zero batch size")
	}

	cfg2 := &Config{
		Sync: SyncConfig{
			Enabled:      true,
			BatchSize:    100,
			SyncInterval: 0,
		},
	}
	err = cfg2.Validate()
	if err == nil {
		t.Error("expected validation error for zero sync interval")
	}
}
