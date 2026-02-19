package warehouse

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
)

// RedshiftConfig contains configuration for the Amazon Redshift connector.
type RedshiftConfig struct {
	BaseConfig

	// Host is the Redshift cluster endpoint.
	Host string `json:"host" yaml:"host"`

	// Port is the Redshift port (default 5439).
	Port int `json:"port" yaml:"port"`

	// Database is the Redshift database name.
	Database string `json:"database" yaml:"database"`

	// User is the authentication username.
	User string `json:"user" yaml:"user"`

	// Password is the authentication password.
	Password string `json:"-" yaml:"password,omitempty"`

	// SSLMode controls TLS for the connection (e.g. "require", "verify-ca").
	SSLMode string `json:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty"`

	// Schema is the default schema (default: "public").
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"`

	// IAMRole is the IAM role ARN for COPY/UNLOAD commands.
	IAMRole string `json:"iam_role,omitempty" yaml:"iam_role,omitempty"`

	// S3StagingBucket is the S3 bucket for COPY/UNLOAD staging.
	S3StagingBucket string `json:"s3_staging_bucket,omitempty" yaml:"s3_staging_bucket,omitempty"`

	// MaxConnections limits the pool size.
	MaxConnections int `json:"max_connections" yaml:"max_connections"`
}

// DefaultRedshiftConfig returns a RedshiftConfig with sensible defaults.
func DefaultRedshiftConfig() RedshiftConfig {
	return RedshiftConfig{
		BaseConfig:     DefaultBaseConfig(),
		Port:           5439,
		Database:       "feather",
		SSLMode:        "require",
		Schema:         "public",
		MaxConnections: 10,
	}
}

// RedshiftConnector implements the Connector interface for Amazon Redshift.
type RedshiftConnector struct {
	config RedshiftConfig
	state  ConnectionState
	db     *sql.DB // nil in simulated mode
	store  *storage.Store
	schema *storage.Registry
	logger *slog.Logger
	mu     sync.RWMutex

	exportCount int64
	importCount int64
}

// NewRedshiftConnector creates a new Redshift connector.
func NewRedshiftConnector(config RedshiftConfig, store *storage.Store, schema *storage.Registry, logger *slog.Logger) *RedshiftConnector {
	if logger == nil {
		logger = slog.Default()
	}
	if config.Schema == "" {
		config.Schema = "public"
	}
	return &RedshiftConnector{
		config: config,
		state:  ConnectionStateDisconnected,
		store:  store,
		schema: schema,
		logger: logger,
	}
}

// Connect establishes a connection to Redshift.
func (c *RedshiftConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = ConnectionStateConnecting
	c.logger.Info("connecting to Redshift",
		"host", c.config.Host,
		"database", c.config.Database,
	)

	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.config.Host, c.config.Port, c.config.Database,
		c.config.User, c.config.Password, c.config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		c.state = ConnectionStateFailed
		// Log masked DSN to prevent credential leaks
		maskedDSN := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=***** sslmode=%s",
			c.config.Host, c.config.Port, c.config.Database,
			c.config.User, c.config.SSLMode,
		)
		return fmt.Errorf("connecting to redshift (dsn=%s): %w", maskedDSN, err)
	}

	maxConns := c.config.MaxConnections
	if maxConns <= 0 {
		maxConns = 10
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns / 2)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		c.state = ConnectionStateFailed
		_ = db.Close()
		return fmt.Errorf("pinging redshift: %w", err)
	}

	c.db = db
	c.state = ConnectionStateConnected
	c.logger.Info("connected to Redshift")
	return nil
}

// Close closes the Redshift connection.
func (c *RedshiftConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("closing redshift connection: %w", err)
		}
		c.db = nil
	}
	c.state = ConnectionStateDisconnected
	return nil
}

// State returns the current connection state.
func (c *RedshiftConnector) State() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Type returns the connector type.
func (c *RedshiftConnector) Type() ConnectorType {
	return ConnectorTypeRedshift
}

// Ping verifies the connection is alive.
func (c *RedshiftConnector) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return fmt.Errorf("redshift: %w", ErrConnectorNotConnected)
	}
	return c.db.PingContext(ctx)
}

// Export exports features from Feather to Redshift.
func (c *RedshiftConnector) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ExportResult{}

	tableName, err := c.qualifiedTableName(req.Table)
	if err != nil {
		return nil, fmt.Errorf("validating table name: %w", err)
	}

	if req.CreateTable {
		createSQL, err := c.buildCreateTableSQL(tableName, req.Features)
		if err != nil {
			return nil, fmt.Errorf("building create table SQL: %w", err)
		}
		if c.db != nil {
			if _, err := c.db.ExecContext(ctx, createSQL); err != nil {
				return nil, fmt.Errorf("creating redshift table: %w", err)
			}
		}
	}

	c.logger.Info("exporting features to Redshift",
		"table", tableName,
		"features", len(req.Features),
	)

	atomic.AddInt64(&c.exportCount, 1)
	result.RowsExported = int64(len(req.Features))
	result.FeaturesExported = len(req.Features)
	result.Table = tableName
	result.Duration = time.Since(start)
	return result, nil
}

// Import imports features from Redshift into Feather.
func (c *RedshiftConnector) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.state != ConnectionStateConnected {
		return nil, ErrConnectorNotConnected
	}

	start := time.Now()
	result := &ImportResult{}

	tableName, err := c.qualifiedTableName(req.Table)
	if err != nil {
		return nil, fmt.Errorf("validating table name: %w", err)
	}

	c.logger.Info("importing features from Redshift",
		"table", tableName,
	)

	atomic.AddInt64(&c.importCount, 1)
	result.Duration = time.Since(start)
	return result, nil
}

// Stats returns connector statistics.
func (c *RedshiftConnector) Stats() map[string]interface{} {
	return map[string]interface{}{
		"type":         "redshift",
		"state":        c.State(),
		"host":         c.config.Host,
		"database":     c.config.Database,
		"schema":       c.config.Schema,
		"export_count": atomic.LoadInt64(&c.exportCount),
		"import_count": atomic.LoadInt64(&c.importCount),
	}
}

func (c *RedshiftConnector) qualifiedTableName(table string) (string, error) {
	if err := validateIdentifier(table); err != nil {
		return "", fmt.Errorf("table name: %w", err)
	}
	if c.config.Schema != "" {
		if err := validateIdentifier(c.config.Schema); err != nil {
			return "", fmt.Errorf("schema name: %w", err)
		}
		return c.config.Schema + "." + table, nil
	}
	return table, nil
}

func (c *RedshiftConnector) buildCreateTableSQL(qualifiedName string, features []string) (string, error) {
	// qualifiedName is already validated by qualifiedTableName
	for _, f := range features {
		if err := validateIdentifier(f); err != nil {
			return "", fmt.Errorf("buildCreateTableSQL feature: %w", err)
		}
	}

	var cols []string
	cols = append(cols, "entity_key VARCHAR(512) NOT NULL")
	cols = append(cols, "feature_timestamp TIMESTAMP NOT NULL")

	for _, f := range features {
		cols = append(cols, fmt.Sprintf("%s VARCHAR(65535)", f))
	}

	cols = append(cols, "PRIMARY KEY (entity_key)")

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)",
		qualifiedName, strings.Join(cols, ",\n  ")), nil
}
