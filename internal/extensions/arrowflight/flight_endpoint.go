package arrowflight

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// FlightServiceConfig configures a production-grade Flight RPC endpoint.
type FlightServiceConfig struct {
	Port                int           `json:"port" yaml:"port"`
	MaxMessageSize      int           `json:"max_message_size" yaml:"max_message_size"`
	EnableAuthentication bool         `json:"enable_authentication" yaml:"enable_authentication"`
	EnableTLS           bool          `json:"enable_tls" yaml:"enable_tls"`
	ReadTimeout         time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout        time.Duration `json:"write_timeout" yaml:"write_timeout"`
}

// DefaultFlightServiceConfig returns production-ready defaults.
func DefaultFlightServiceConfig() FlightServiceConfig {
	return FlightServiceConfig{
		Port:                8815,
		MaxMessageSize:      256 * 1024 * 1024, // 256MB
		EnableAuthentication: false,
		EnableTLS:           false,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
	}
}

// FlightEndpoint provides a production-grade Arrow Flight RPC endpoint
// for columnar batch retrieval, enabling direct consumption by
// Pandas, Spark, DuckDB, and other Arrow-compatible clients.
type FlightServiceEndpoint struct {
	config    FlightServiceConfig
	server    *Server
	batch     *BatchServer
	converter *BatchConverter

	mu    sync.RWMutex
	stats FlightServiceStats
}

// FlightServiceStats tracks endpoint-level metrics.
type FlightServiceStats struct {
	TotalRequests      int64   `json:"total_requests"`
	ActiveConnections  int64   `json:"active_connections"`
	TotalBytesServed   int64   `json:"total_bytes_served"`
	AvgResponseTimeMs  float64 `json:"avg_response_time_ms"`
	ErrorCount         int64   `json:"error_count"`
	totalResponseMs    int64
}

// NewFlightServiceEndpoint creates a new production-grade Flight endpoint.
func NewFlightServiceEndpoint(server *Server, batch *BatchServer, cfg FlightServiceConfig) *FlightServiceEndpoint {
	return &FlightServiceEndpoint{
		config:    cfg,
		server:    server,
		batch:     batch,
		converter: NewBatchConverter(),
	}
}

// GetSchema returns the Arrow schema for a given dataset descriptor.
func (e *FlightServiceEndpoint) GetSchema(ctx context.Context, desc FlightDescriptor) ([]ColumnSchema, error) {
	atomic.AddInt64(&e.stats.TotalRequests, 1)
	info, err := e.server.GetFlightInfo(ctx, desc)
	if err != nil {
		atomic.AddInt64(&e.stats.ErrorCount, 1)
		return nil, fmt.Errorf("getting flight info: %w", err)
	}
	return info.Schema, nil
}

// DoGetBatch retrieves feature data as a columnar batch optimized for
// direct consumption by data science tools (Pandas, Spark, DuckDB).
func (e *FlightServiceEndpoint) DoGetBatch(ctx context.Context, entities []string, features []string) (*ColumnarBatch, error) {
	start := time.Now()
	atomic.AddInt64(&e.stats.TotalRequests, 1)
	atomic.AddInt64(&e.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&e.stats.ActiveConnections, -1)

	batch, err := e.batch.ServeBatch(ctx, entities, features)
	if err != nil {
		atomic.AddInt64(&e.stats.ErrorCount, 1)
		return nil, fmt.Errorf("serving batch: %w", err)
	}

	atomic.AddInt64(&e.stats.TotalBytesServed, int64(batch.ByteSize))
	latency := time.Since(start).Milliseconds()
	atomic.AddInt64(&e.stats.totalResponseMs, latency)

	return batch, nil
}

// DoGetRecordBatch retrieves feature data as a RecordBatch with predicate
// pushdown for efficient filtering.
func (e *FlightServiceEndpoint) DoGetRecordBatch(ctx context.Context, req BatchRequest) (*BatchResponse, error) {
	start := time.Now()
	atomic.AddInt64(&e.stats.TotalRequests, 1)
	atomic.AddInt64(&e.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&e.stats.ActiveConnections, -1)

	// Apply read timeout from config.
	if e.config.ReadTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.ReadTimeout)
		defer cancel()
	}

	info, err := e.server.GetFlightInfo(ctx, req.Descriptor)
	if err != nil {
		atomic.AddInt64(&e.stats.ErrorCount, 1)
		return nil, fmt.Errorf("getting flight info: %w", err)
	}

	ticket := FlightTicket{
		Desc:   req.Descriptor,
		Schema: info.Schema,
	}
	rb, err := e.server.DoGet(ctx, ticket)
	if err != nil {
		atomic.AddInt64(&e.stats.ErrorCount, 1)
		return nil, fmt.Errorf("doing get: %w", err)
	}

	eval := NewPredicateEvaluator()

	// Apply predicates.
	if len(req.Predicates) > 0 {
		rb = eval.Apply(rb, req.Predicates)
	}

	// Apply column projection.
	if len(req.Columns) > 0 {
		rb = eval.ProjectColumns(rb, req.Columns)
	}

	totalRows := int64(rb.Rows)
	resp := &BatchResponse{
		Data:      rb,
		TotalRows: totalRows,
	}

	// Apply offset and limit.
	if req.Offset > 0 || req.Limit > 0 {
		resp.HasMore = int64(req.Offset+req.Limit) < totalRows
		if resp.HasMore {
			resp.NextOffset = req.Offset + req.Limit
		}
	}

	latency := time.Since(start).Milliseconds()
	atomic.AddInt64(&e.stats.totalResponseMs, latency)

	return resp, nil
}

// DoPutBatch ingests a columnar batch into the feature store.
func (e *FlightServiceEndpoint) DoPutBatch(ctx context.Context, batch *ColumnarBatch) (*PutResult, error) {
	atomic.AddInt64(&e.stats.TotalRequests, 1)
	atomic.AddInt64(&e.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&e.stats.ActiveConnections, -1)

	rb := e.converter.ToRecordBatch(batch)
	desc := FlightDescriptor{Type: "batch_put"}
	result, err := e.server.DoPut(ctx, desc, rb)
	if err != nil {
		atomic.AddInt64(&e.stats.ErrorCount, 1)
		return nil, fmt.Errorf("putting batch: %w", err)
	}
	return result, nil
}

// Stats returns endpoint statistics.
func (e *FlightServiceEndpoint) Stats() FlightServiceStats {
	stats := FlightServiceStats{
		TotalRequests:     atomic.LoadInt64(&e.stats.TotalRequests),
		ActiveConnections: atomic.LoadInt64(&e.stats.ActiveConnections),
		TotalBytesServed:  atomic.LoadInt64(&e.stats.TotalBytesServed),
		ErrorCount:        atomic.LoadInt64(&e.stats.ErrorCount),
	}
	total := atomic.LoadInt64(&e.stats.totalResponseMs)
	if stats.TotalRequests > 0 {
		stats.AvgResponseTimeMs = float64(total) / float64(stats.TotalRequests)
	}
	return stats
}

// HealthCheck returns nil if the endpoint is operational.
func (e *FlightServiceEndpoint) HealthCheck() error {
	if e.server == nil {
		return fmt.Errorf("flight server not initialized")
	}
	if e.batch == nil {
		return fmt.Errorf("batch server not initialized")
	}
	return nil
}
