// Package flink provides integration between Feather and Apache Flink.
//
// This package enables bidirectional synchronization between Feather's
// online feature store and Apache Flink for real-time streaming feature
// computation. It provides sink and source connectors for Flink DataStreams.
//
// # Usage
//
//	connector, err := flink.NewConnector(config, store, schema, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer connector.Close()
//
//	// Process a stream record
//	err = connector.ProcessRecord(ctx, record)
package flink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// Errors returned by the Flink connector.
var (
	ErrConnectorNotInitialized = errors.New("connector not initialized")
	ErrInvalidConfig           = errors.New("invalid configuration")
	ErrStreamClosed            = errors.New("stream closed")
	ErrCheckpointFailed        = errors.New("checkpoint failed")
	ErrRecordInvalid           = errors.New("invalid record")
	ErrProcessingFailed        = errors.New("processing failed")
	ErrBackpressure            = errors.New("backpressure detected")
)

// ConnectorState represents the connector lifecycle state.
type ConnectorState string

// ConnectorState constants.
const (
	StateUninitialized ConnectorState = "uninitialized"
	StateRunning       ConnectorState = "running"
	StatePaused        ConnectorState = "paused"
	StateCheckpointing ConnectorState = "checkpointing"
	StateStopped       ConnectorState = "stopped"
	StateFailed        ConnectorState = "failed"
)

// DeliveryGuarantee specifies the delivery semantics.
type DeliveryGuarantee string

// DeliveryGuarantee constants.
const (
	GuaranteeAtLeastOnce DeliveryGuarantee = "at_least_once"
	GuaranteeAtMostOnce  DeliveryGuarantee = "at_most_once"
	GuaranteeExactlyOnce DeliveryGuarantee = "exactly_once"
)

// CheckpointMode determines checkpoint behavior.
type CheckpointMode string

// CheckpointMode constants.
const (
	CheckpointModeAligned   CheckpointMode = "aligned"
	CheckpointModeUnaligned CheckpointMode = "unaligned"
)

// Config contains configuration for the Flink connector.
type Config struct {
	// JobManagerAddress is the Flink JobManager address.
	JobManagerAddress string `json:"job_manager_address" yaml:"job_manager_address"`

	// TaskSlots is the number of task slots to use.
	TaskSlots int `json:"task_slots" yaml:"task_slots"`

	// Parallelism is the default parallelism for operators.
	Parallelism int `json:"parallelism" yaml:"parallelism"`

	// BufferSize is the size of the internal buffer.
	BufferSize int `json:"buffer_size" yaml:"buffer_size"`

	// FlushInterval is the interval for flushing buffered records.
	FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"`

	// DeliveryGuarantee specifies delivery semantics.
	DeliveryGuarantee DeliveryGuarantee `json:"delivery_guarantee" yaml:"delivery_guarantee"`

	// CheckpointInterval is the interval between checkpoints.
	CheckpointInterval time.Duration `json:"checkpoint_interval" yaml:"checkpoint_interval"`

	// CheckpointTimeout is the maximum time for checkpoint completion.
	CheckpointTimeout time.Duration `json:"checkpoint_timeout" yaml:"checkpoint_timeout"`

	// CheckpointMode determines checkpoint behavior.
	CheckpointMode CheckpointMode `json:"checkpoint_mode" yaml:"checkpoint_mode"`

	// MaxRetries for transient failures.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryBackoff is the initial backoff between retries.
	RetryBackoff time.Duration `json:"retry_backoff" yaml:"retry_backoff"`

	// EnableMetrics enables Flink metrics reporting.
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`

	// MetricsPort is the port for metrics HTTP server.
	MetricsPort int `json:"metrics_port" yaml:"metrics_port"`

	// WatermarkStrategy determines watermark generation.
	WatermarkStrategy string `json:"watermark_strategy" yaml:"watermark_strategy"`

	// MaxOutOfOrderness is the maximum allowed out-of-orderness for watermarks.
	MaxOutOfOrderness time.Duration `json:"max_out_of_orderness" yaml:"max_out_of_orderness"`

	// IdleTimeout is the timeout for marking a source as idle.
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// DefaultConfig returns the default Flink connector configuration.
func DefaultConfig() Config {
	return Config{
		JobManagerAddress:  "localhost:8081",
		TaskSlots:          4,
		Parallelism:        4,
		BufferSize:         10000,
		FlushInterval:      100 * time.Millisecond,
		DeliveryGuarantee:  GuaranteeAtLeastOnce,
		CheckpointInterval: 10 * time.Second,
		CheckpointTimeout:  60 * time.Second,
		CheckpointMode:     CheckpointModeAligned,
		MaxRetries:         3,
		RetryBackoff:       100 * time.Millisecond,
		EnableMetrics:      true,
		MetricsPort:        9250,
		WatermarkStrategy:  "bounded_out_of_orderness",
		MaxOutOfOrderness:  5 * time.Second,
		IdleTimeout:        time.Minute,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.BufferSize <= 0 {
		c.BufferSize = 10000
	}
	if c.Parallelism <= 0 {
		c.Parallelism = 4
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 100 * time.Millisecond
	}
	if c.CheckpointInterval <= 0 {
		c.CheckpointInterval = 10 * time.Second
	}
	if c.DeliveryGuarantee == "" {
		c.DeliveryGuarantee = GuaranteeAtLeastOnce
	}
	return nil
}

// StreamRecord represents a record in the Flink stream.
type StreamRecord struct {
	// EntityKey is the entity identifier.
	EntityKey string `json:"entity_key"`

	// Features contains feature values.
	Features map[string]interface{} `json:"features"`

	// Timestamp is the event time.
	Timestamp time.Time `json:"timestamp"`

	// Watermark is the current watermark (for source records).
	Watermark *time.Time `json:"watermark,omitempty"`

	// Key is the partition key.
	Key string `json:"key,omitempty"`

	// Metadata contains additional record metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Checkpoint represents a checkpoint state.
type Checkpoint struct {
	// ID is the checkpoint identifier.
	ID int64 `json:"id"`

	// Timestamp is when the checkpoint was triggered.
	Timestamp time.Time `json:"timestamp"`

	// ProcessedRecords is the count of processed records.
	ProcessedRecords int64 `json:"processed_records"`

	// LastWatermark is the last watermark at checkpoint time.
	LastWatermark time.Time `json:"last_watermark"`

	// State contains connector-specific state.
	State map[string]interface{} `json:"state"`

	// Completed indicates if checkpoint is complete.
	Completed bool `json:"completed"`

	// Duration is the checkpoint duration.
	Duration time.Duration `json:"duration"`
}

// SinkConfig contains configuration for the Flink sink.
type SinkConfig struct {
	// EntityColumn is the field containing entity keys.
	EntityColumn string `json:"entity_column"`

	// FeatureColumns maps stream fields to feature names.
	FeatureColumns map[string]string `json:"feature_columns"`

	// TimestampColumn is the field containing event time.
	TimestampColumn string `json:"timestamp_column,omitempty"`

	// BatchSize is the number of records per batch write.
	BatchSize int `json:"batch_size"`

	// MaxWaitTime is the maximum time to wait before flushing.
	MaxWaitTime time.Duration `json:"max_wait_time"`

	// ValidateSchema validates records against schema.
	ValidateSchema bool `json:"validate_schema"`
}

// SourceConfig contains configuration for the Flink source.
type SourceConfig struct {
	// Features to emit from the source.
	Features []string `json:"features"`

	// Entities to emit (empty for all).
	Entities []string `json:"entities,omitempty"`

	// PollInterval is the interval for polling updates.
	PollInterval time.Duration `json:"poll_interval"`

	// IncludeTimestamp includes feature timestamps.
	IncludeTimestamp bool `json:"include_timestamp"`

	// StartFromLatest starts from latest features (vs historical).
	StartFromLatest bool `json:"start_from_latest"`
}

// ConnectorMetrics tracks connector performance.
type ConnectorMetrics struct {
	RecordsProcessed   int64         `json:"records_processed"`
	RecordsFailed      int64         `json:"records_failed"`
	BytesProcessed     int64         `json:"bytes_processed"`
	CheckpointsCreated int64         `json:"checkpoints_created"`
	CheckpointsFailed  int64         `json:"checkpoints_failed"`
	CurrentWatermark   time.Time     `json:"current_watermark"`
	ProcessingLatency  time.Duration `json:"processing_latency"`
	BufferUtilization  float64       `json:"buffer_utilization"`
	BackpressureEvents int64         `json:"backpressure_events"`
	LastProcessedAt    time.Time     `json:"last_processed_at"`
}

// Connector provides Flink integration for the Feather feature store.
type Connector struct {
	mu     sync.RWMutex
	config Config
	store  *storage.Store
	schema storage.SchemaRegistry
	state  ConnectorState
	logger *slog.Logger

	// Internal state
	buffer        []StreamRecord
	watermark     time.Time
	checkpoint    *Checkpoint
	checkpointID  int64
	metricsServer *http.Server

	// Metrics
	metrics ConnectorMetrics

	// Channels
	recordCh  chan StreamRecord
	flushCh   chan struct{}
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewConnector creates a new Flink connector.
func NewConnector(config Config, store *storage.Store, schema storage.SchemaRegistry, logger *slog.Logger) (*Connector, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidConfig)
	}

	if logger == nil {
		logger = slog.Default()
	}

	c := &Connector{
		config:    config,
		store:     store,
		schema:    schema,
		state:     StateUninitialized,
		logger:    logger,
		buffer:    make([]StreamRecord, 0, config.BufferSize),
		recordCh:  make(chan StreamRecord, config.BufferSize),
		flushCh:   make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}

	return c, nil
}

// Start starts the connector.
func (c *Connector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.state == StateRunning {
		c.mu.Unlock()
		return nil
	}
	c.state = StateRunning
	c.mu.Unlock()

	// Start background workers
	go c.processLoop(ctx)
	go c.flushLoop(ctx)

	// Start metrics server if enabled
	if c.config.EnableMetrics {
		go c.startMetricsServer()
	}

	c.logger.Info("flink connector started",
		"parallelism", c.config.Parallelism,
		"buffer_size", c.config.BufferSize,
		"delivery_guarantee", c.config.DeliveryGuarantee,
	)

	return nil
}

// Stop stops the connector.
func (c *Connector) Stop() error {
	c.mu.Lock()
	if c.state == StateStopped {
		c.mu.Unlock()
		return nil
	}
	c.state = StateStopped
	c.mu.Unlock()

	close(c.stopCh)

	// Wait for graceful shutdown
	select {
	case <-c.stoppedCh:
	case <-time.After(30 * time.Second):
		c.logger.Warn("force stopping connector after timeout")
	}

	// Flush remaining records
	c.flushBuffer()

	// Stop metrics server
	if c.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.metricsServer.Shutdown(ctx); err != nil {
			c.logger.Error("metrics server shutdown failed", "error", err)
		}
	}

	c.logger.Info("flink connector stopped",
		"records_processed", c.metrics.RecordsProcessed,
		"records_failed", c.metrics.RecordsFailed,
	)

	return nil
}

// Close closes the connector.
func (c *Connector) Close() error {
	return c.Stop()
}

// State returns the current connector state.
func (c *Connector) State() ConnectorState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// ProcessRecord processes a single stream record.
func (c *Connector) ProcessRecord(ctx context.Context, record StreamRecord) error {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if state != StateRunning {
		return ErrConnectorNotInitialized
	}

	// Validate record
	if record.EntityKey == "" {
		return fmt.Errorf("%w: entity_key is required", ErrRecordInvalid)
	}

	// Apply watermark
	if record.Watermark != nil {
		c.updateWatermark(*record.Watermark)
	}

	// Send to processing channel
	select {
	case c.recordCh <- record:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		atomic.AddInt64(&c.metrics.BackpressureEvents, 1)
		return ErrBackpressure
	}
}

// ProcessBatch processes multiple records.
func (c *Connector) ProcessBatch(ctx context.Context, records []StreamRecord) error {
	for _, record := range records {
		if err := c.ProcessRecord(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (c *Connector) processLoop(ctx context.Context) {
	defer close(c.stoppedCh)

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case record := <-c.recordCh:
			c.addToBuffer(record)
		}
	}
}

func (c *Connector) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.flushBuffer()
		case <-c.flushCh:
			c.flushBuffer()
		}
	}
}

func (c *Connector) addToBuffer(record StreamRecord) {
	c.mu.Lock()
	c.buffer = append(c.buffer, record)
	bufLen := len(c.buffer)
	c.mu.Unlock()

	// Trigger flush if buffer is full
	if bufLen >= c.config.BufferSize {
		select {
		case c.flushCh <- struct{}{}:
		default:
		}
	}
}

func (c *Connector) flushBuffer() {
	c.mu.Lock()
	if len(c.buffer) == 0 {
		c.mu.Unlock()
		return
	}

	records := c.buffer
	c.buffer = make([]StreamRecord, 0, c.config.BufferSize)
	c.mu.Unlock()

	start := time.Now()

	// Process records in batch
	for _, record := range records {
		if err := c.writeRecord(record); err != nil {
			atomic.AddInt64(&c.metrics.RecordsFailed, 1)
			c.logger.Error("failed to write record",
				"entity", record.EntityKey,
				"error", err,
			)
			continue
		}
		atomic.AddInt64(&c.metrics.RecordsProcessed, 1)
	}

	latency := time.Since(start)
	c.mu.Lock()
	c.metrics.ProcessingLatency = latency
	c.metrics.LastProcessedAt = time.Now()
	c.mu.Unlock()
}

func (c *Connector) writeRecord(record StreamRecord) error {
	timestamp := record.Timestamp.UnixNano()
	if timestamp == 0 {
		timestamp = time.Now().UnixNano()
	}

	features := make(map[string]*domain.FeatureValue)
	for name, value := range record.Features {
		features[name] = &domain.FeatureValue{
			Value:     value,
			Timestamp: timestamp,
		}
	}

	return c.store.Put(record.EntityKey, features)
}

func (c *Connector) updateWatermark(wm time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if wm.After(c.watermark) {
		c.watermark = wm
		c.metrics.CurrentWatermark = wm
	}
}

// TriggerCheckpoint triggers a new checkpoint.
func (c *Connector) TriggerCheckpoint(ctx context.Context) (*Checkpoint, error) {
	c.mu.Lock()
	if c.state != StateRunning {
		c.mu.Unlock()
		return nil, ErrConnectorNotInitialized
	}
	c.state = StateCheckpointing
	checkpointID := atomic.AddInt64(&c.checkpointID, 1)
	c.mu.Unlock()

	start := time.Now()

	// Flush buffer before checkpoint
	c.flushBuffer()

	checkpoint := &Checkpoint{
		ID:               checkpointID,
		Timestamp:        time.Now(),
		ProcessedRecords: atomic.LoadInt64(&c.metrics.RecordsProcessed),
		LastWatermark:    c.watermark,
		State: map[string]interface{}{
			"buffer_size":     len(c.buffer),
			"checkpoint_mode": c.config.CheckpointMode,
		},
	}

	// Wait for checkpoint to complete with timeout
	select {
	case <-ctx.Done():
		atomic.AddInt64(&c.metrics.CheckpointsFailed, 1)
		c.mu.Lock()
		c.state = StateRunning
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(c.config.CheckpointTimeout):
		atomic.AddInt64(&c.metrics.CheckpointsFailed, 1)
		c.mu.Lock()
		c.state = StateRunning
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: timeout after %v", ErrCheckpointFailed, c.config.CheckpointTimeout)
	default:
		checkpoint.Completed = true
		checkpoint.Duration = time.Since(start)
	}

	atomic.AddInt64(&c.metrics.CheckpointsCreated, 1)

	c.mu.Lock()
	c.checkpoint = checkpoint
	c.state = StateRunning
	c.mu.Unlock()

	c.logger.Info("checkpoint completed",
		"checkpoint_id", checkpoint.ID,
		"records", checkpoint.ProcessedRecords,
		"duration", checkpoint.Duration,
	)

	return checkpoint, nil
}

// RestoreFromCheckpoint restores connector state from a checkpoint.
func (c *Connector) RestoreFromCheckpoint(checkpoint *Checkpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("%w: checkpoint is nil", ErrCheckpointFailed)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkpoint = checkpoint
	c.watermark = checkpoint.LastWatermark
	c.checkpointID = checkpoint.ID

	c.logger.Info("restored from checkpoint",
		"checkpoint_id", checkpoint.ID,
		"watermark", checkpoint.LastWatermark,
	)

	return nil
}

// GetLastCheckpoint returns the last completed checkpoint.
func (c *Connector) GetLastCheckpoint() *Checkpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.checkpoint
}

// Metrics returns connector metrics.
func (c *Connector) Metrics() ConnectorMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	bufLen := len(c.buffer)
	utilization := float64(bufLen) / float64(c.config.BufferSize)

	return ConnectorMetrics{
		RecordsProcessed:   atomic.LoadInt64(&c.metrics.RecordsProcessed),
		RecordsFailed:      atomic.LoadInt64(&c.metrics.RecordsFailed),
		BytesProcessed:     atomic.LoadInt64(&c.metrics.BytesProcessed),
		CheckpointsCreated: atomic.LoadInt64(&c.metrics.CheckpointsCreated),
		CheckpointsFailed:  atomic.LoadInt64(&c.metrics.CheckpointsFailed),
		CurrentWatermark:   c.metrics.CurrentWatermark,
		ProcessingLatency:  c.metrics.ProcessingLatency,
		BufferUtilization:  utilization,
		BackpressureEvents: atomic.LoadInt64(&c.metrics.BackpressureEvents),
		LastProcessedAt:    c.metrics.LastProcessedAt,
	}
}

// CurrentWatermark returns the current watermark.
func (c *Connector) CurrentWatermark() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.watermark
}

func (c *Connector) startMetricsServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", c.handleMetrics)
	mux.HandleFunc("/health", c.handleHealth)

	addr := fmt.Sprintf(":%d", c.config.MetricsPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		c.logger.Error("failed to start metrics server", "error", err)
		return
	}

	c.metricsServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	c.logger.Info("metrics server started", "port", c.config.MetricsPort)

	if err := c.metricsServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		c.logger.Error("metrics server error", "error", err)
	}
}

func (c *Connector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := c.Metrics()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		c.logger.Error("failed to encode metrics", "error", err)
	}
}

func (c *Connector) handleHealth(w http.ResponseWriter, r *http.Request) {
	state := c.State()
	status := "healthy"
	httpStatus := http.StatusOK

	if state != StateRunning {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"state":  state,
	}); err != nil {
		c.logger.Error("failed to encode health status", "error", err)
	}
}

// Sink writes stream records to the feature store.
type Sink struct {
	connector *Connector
	config    SinkConfig
	logger    *slog.Logger
}

// NewSink creates a new Flink sink.
func NewSink(connector *Connector, config SinkConfig) (*Sink, error) {
	if config.EntityColumn == "" {
		return nil, fmt.Errorf("%w: entity_column is required", ErrInvalidConfig)
	}
	if len(config.FeatureColumns) == 0 {
		return nil, fmt.Errorf("%w: feature_columns is required", ErrInvalidConfig)
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 1000
	}
	if config.MaxWaitTime <= 0 {
		config.MaxWaitTime = 100 * time.Millisecond
	}

	return &Sink{
		connector: connector,
		config:    config,
		logger:    connector.logger,
	}, nil
}

// Write writes a record to Feather.
func (s *Sink) Write(ctx context.Context, data map[string]interface{}) error {
	entityKey, ok := data[s.config.EntityColumn].(string)
	if !ok {
		return fmt.Errorf("%w: missing or invalid entity column", ErrRecordInvalid)
	}

	record := StreamRecord{
		EntityKey: entityKey,
		Features:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Extract timestamp if specified
	if s.config.TimestampColumn != "" {
		if ts, ok := data[s.config.TimestampColumn]; ok {
			switch t := ts.(type) {
			case time.Time:
				record.Timestamp = t
			case string:
				if parsed, err := time.Parse(time.RFC3339, t); err == nil {
					record.Timestamp = parsed
				}
			case int64:
				record.Timestamp = time.Unix(0, t)
			}
		}
	}

	// Map feature columns
	for srcCol, featureName := range s.config.FeatureColumns {
		if val, ok := data[srcCol]; ok {
			record.Features[featureName] = val
		}
	}

	return s.connector.ProcessRecord(ctx, record)
}

// Source represents a Flink source for reading features.
type Source struct {
	connector *Connector
	config    SourceConfig
	logger    *slog.Logger
	running   atomic.Bool
	stopCh    chan struct{}
}

// NewSource creates a new Flink source.
func NewSource(connector *Connector, config SourceConfig) (*Source, error) {
	if len(config.Features) == 0 {
		return nil, fmt.Errorf("%w: features is required", ErrInvalidConfig)
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}

	return &Source{
		connector: connector,
		config:    config,
		logger:    connector.logger,
		stopCh:    make(chan struct{}),
	}, nil
}

// Read reads features and sends them to the output channel.
func (s *Source) Read(ctx context.Context, output chan<- StreamRecord) error {
	s.running.Store(true)
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			if err := s.pollFeatures(ctx, output); err != nil {
				s.logger.Error("failed to poll features", "error", err)
			}
		}
	}
}

func (s *Source) pollFeatures(ctx context.Context, output chan<- StreamRecord) error {
	entities := s.config.Entities
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		features, err := s.connector.store.Get(entity, s.config.Features)
		if err != nil {
			continue
		}

		record := StreamRecord{
			EntityKey: entity,
			Features:  make(map[string]interface{}),
			Timestamp: time.Now(),
		}

		var maxTimestamp int64
		for name, fv := range features {
			record.Features[name] = fv.Value
			if fv.Timestamp > maxTimestamp {
				maxTimestamp = fv.Timestamp
			}
		}

		if s.config.IncludeTimestamp {
			record.Timestamp = time.Unix(0, maxTimestamp)
		}

		select {
		case output <- record:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// Stop stops the source.
func (s *Source) Stop() {
	if s.running.CompareAndSwap(true, false) {
		close(s.stopCh)
	}
}

// GenerateFlinkJobCode generates a Flink job skeleton in Java.
func GenerateFlinkJobCode(sinkConfig SinkConfig) string {
	code := `import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.datastream.DataStream;

public class FeatherFeatureJob {
    public static void main(String[] args) throws Exception {
        StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();

        // Configure checkpointing
        env.enableCheckpointing(10000); // 10 second intervals
        env.getCheckpointConfig().setCheckpointTimeout(60000);

        // Create source (e.g., Kafka)
        DataStream<FeatureRecord> source = env
            .addSource(new FlinkKafkaConsumer<>("features-topic", new FeatureDeserializer(), properties));

        // Process and write to Feather
        source
            .keyBy(record -> record.getEntityKey())
            .process(new FeatureProcessor())
            .addSink(new FeatherSink(featherConfig));

        env.execute("Feather Feature Pipeline");
    }
}
`
	return code
}

// GeneratePyFlinkCode generates a PyFlink job skeleton.
func GeneratePyFlinkCode(sinkConfig SinkConfig) string {
	code := `from pyflink.datastream import StreamExecutionEnvironment
from pyflink.common import WatermarkStrategy
from pyflink.datastream.connectors import FlinkKafkaConsumer

def main():
    env = StreamExecutionEnvironment.get_execution_environment()

    # Enable checkpointing
    env.enable_checkpointing(10000)  # 10 second intervals

    # Configure parallelism
    env.set_parallelism(4)

    # Create Kafka source
    kafka_source = FlinkKafkaConsumer(
        topics='features-topic',
        deserialization_schema=JsonRowDeserializationSchema.builder().build(),
        properties=kafka_properties
    )

    # Create stream
    stream = env.add_source(kafka_source)

    # Process features
    processed = stream \
        .key_by(lambda x: x['entity_key']) \
        .process(FeatureProcessor())

    # Write to Feather
    processed.add_sink(FeatherSink(feather_config))

    env.execute("Feather Feature Pipeline")

if __name__ == "__main__":
    main()
`
	return code
}
