package arrowflight

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Config controls the Arrow Flight server behavior.
type Config struct {
	// MaxBatchSize is the maximum number of rows per Arrow record batch.
	MaxBatchSize int `json:"max_batch_size" yaml:"max_batch_size"`
	// MaxConcurrentStreams limits parallel Flight streams.
	MaxConcurrentStreams int `json:"max_concurrent_streams" yaml:"max_concurrent_streams"`
	// CompressionEnabled enables LZ4 compression for Flight data frames.
	CompressionEnabled bool `json:"compression_enabled" yaml:"compression_enabled"`
	// StreamTimeoutSeconds is the max duration for a single Flight stream.
	StreamTimeoutSeconds int `json:"stream_timeout_seconds" yaml:"stream_timeout_seconds"`
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		MaxBatchSize:         65536,
		MaxConcurrentStreams: 64,
		CompressionEnabled:  true,
		StreamTimeoutSeconds: 300,
	}
}

// DataType represents Arrow-compatible column types.
type DataType string

const (
	DataTypeInt64   DataType = "int64"
	DataTypeFloat64 DataType = "float64"
	DataTypeString  DataType = "string"
	DataTypeBool    DataType = "bool"
	DataTypeBytes   DataType = "bytes"
)

// ColumnSchema describes a single column in an Arrow schema.
type ColumnSchema struct {
	Name     string   `json:"name"`
	Type     DataType `json:"type"`
	Nullable bool     `json:"nullable"`
}

// FlightDescriptor identifies a dataset for Flight operations.
type FlightDescriptor struct {
	Type     string   `json:"type"` // "path" or "cmd"
	Path     []string `json:"path,omitempty"`
	Command  string   `json:"command,omitempty"`
	Entities []string `json:"entities,omitempty"`
	Features []string `json:"features,omitempty"`
}

// FlightTicket is an opaque token referencing a prepared dataset.
type FlightTicket struct {
	ID        string           `json:"id"`
	Desc      FlightDescriptor `json:"descriptor"`
	Schema    []ColumnSchema   `json:"schema"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt time.Time        `json:"expires_at"`
}

// RecordBatch represents a columnar data batch (simplified Arrow format).
type RecordBatch struct {
	Schema  []ColumnSchema           `json:"schema"`
	Rows    int                      `json:"rows"`
	Columns map[string][]interface{} `json:"columns"`
}

// FlightInfo describes available data for a Flight endpoint.
type FlightInfo struct {
	Schema      []ColumnSchema `json:"schema"`
	Descriptor  FlightDescriptor `json:"descriptor"`
	TotalRows   int64          `json:"total_rows"`
	TotalBytes  int64          `json:"total_bytes"`
	Endpoints   []FlightEndpoint `json:"endpoints"`
}

// FlightEndpoint describes where to retrieve Flight data.
type FlightEndpoint struct {
	Ticket   FlightTicket `json:"ticket"`
	Location string       `json:"location"`
}

// PutResult is returned after a DoPut operation.
type PutResult struct {
	RowsWritten   int64  `json:"rows_written"`
	BytesReceived int64  `json:"bytes_received"`
	DurationMs    int64  `json:"duration_ms"`
	EntityKeys    []string `json:"entity_keys"`
}

// ExchangeRequest specifies a bidirectional exchange operation.
type ExchangeRequest struct {
	Descriptor FlightDescriptor `json:"descriptor"`
	Command    string           `json:"command"` // "query", "transform", "aggregate"
	Parameters json.RawMessage  `json:"parameters,omitempty"`
	Data       *RecordBatch     `json:"data,omitempty"`
}

// ExchangeResponse is the result of a DoExchange operation.
type ExchangeResponse struct {
	Data     *RecordBatch    `json:"data,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
	Status   string          `json:"status"`
}

// Stats tracks Flight server performance metrics.
type Stats struct {
	TotalDoGets      int64   `json:"total_do_gets"`
	TotalDoPuts      int64   `json:"total_do_puts"`
	TotalExchanges   int64   `json:"total_exchanges"`
	ActiveStreams     int64   `json:"active_streams"`
	TotalRowsRead    int64   `json:"total_rows_read"`
	TotalRowsWritten int64   `json:"total_rows_written"`
	TotalBytesRead   int64   `json:"total_bytes_read"`
	TotalBytesWritten int64  `json:"total_bytes_written"`
	AvgBatchSize     float64 `json:"avg_batch_size"`
	ErrorCount       int64   `json:"error_count"`
}

// FeatureReader is a callback interface for reading features from the store.
type FeatureReader interface {
	ReadBatch(ctx context.Context, entities []string, features []string) (*RecordBatch, error)
}

// FeatureWriter is a callback interface for writing features to the store.
type FeatureWriter interface {
	WriteBatch(ctx context.Context, batch *RecordBatch) (*PutResult, error)
}

// Server implements the Arrow Flight protocol for feature store data transport.
type Server struct {
	config  Config
	tickets sync.Map // map[string]*FlightTicket
	stats   Stats

	reader FeatureReader
	writer FeatureWriter

	mu             sync.RWMutex
	activeStreams   int64
	nextTicketID   int64
}

// NewServer creates a new Arrow Flight server.
func NewServer(cfg Config) *Server {
	return &Server{
		config: cfg,
	}
}

// SetReader sets the feature reader for DoGet operations.
func (s *Server) SetReader(r FeatureReader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reader = r
}

// SetWriter sets the feature writer for DoPut operations.
func (s *Server) SetWriter(w FeatureWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = w
}

// GetFlightInfo returns metadata about a dataset without transferring data.
func (s *Server) GetFlightInfo(ctx context.Context, desc FlightDescriptor) (*FlightInfo, error) {
	if len(desc.Features) == 0 {
		return nil, fmt.Errorf("at least one feature is required")
	}

	schema := make([]ColumnSchema, 0, len(desc.Features)+1)
	schema = append(schema, ColumnSchema{Name: "entity_key", Type: DataTypeString, Nullable: false})
	for _, f := range desc.Features {
		schema = append(schema, ColumnSchema{Name: f, Type: DataTypeFloat64, Nullable: true})
	}

	ticket := s.issueTicket(desc, schema)

	return &FlightInfo{
		Schema:     schema,
		Descriptor: desc,
		TotalRows:  int64(len(desc.Entities)),
		Endpoints: []FlightEndpoint{
			{Ticket: *ticket, Location: "local"},
		},
	}, nil
}

// DoGet retrieves feature data as columnar record batches.
func (s *Server) DoGet(ctx context.Context, ticket FlightTicket) (*RecordBatch, error) {
	if atomic.LoadInt64(&s.activeStreams) >= int64(s.config.MaxConcurrentStreams) {
		return nil, fmt.Errorf("max concurrent streams exceeded (%d)", s.config.MaxConcurrentStreams)
	}
	atomic.AddInt64(&s.activeStreams, 1)
	defer atomic.AddInt64(&s.activeStreams, -1)

	stored, ok := s.tickets.Load(ticket.ID)
	if !ok {
		return nil, fmt.Errorf("ticket %s not found or expired", ticket.ID)
	}
	ft := stored.(*FlightTicket)

	if time.Now().After(ft.ExpiresAt) {
		s.tickets.Delete(ticket.ID)
		return nil, fmt.Errorf("ticket %s has expired", ticket.ID)
	}

	s.mu.RLock()
	reader := s.reader
	s.mu.RUnlock()

	if reader != nil {
		batch, err := reader.ReadBatch(ctx, ft.Desc.Entities, ft.Desc.Features)
		if err != nil {
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			return nil, fmt.Errorf("reading batch: %w", err)
		}
		atomic.AddInt64(&s.stats.TotalDoGets, 1)
		atomic.AddInt64(&s.stats.TotalRowsRead, int64(batch.Rows))
		return batch, nil
	}

	// Fallback: generate empty batch with correct schema
	batch := s.buildEmptyBatch(ft)
	atomic.AddInt64(&s.stats.TotalDoGets, 1)
	return batch, nil
}

// DoPut ingests feature data from columnar record batches.
func (s *Server) DoPut(ctx context.Context, desc FlightDescriptor, batch *RecordBatch) (*PutResult, error) {
	if batch == nil || batch.Rows == 0 {
		return nil, fmt.Errorf("empty batch")
	}

	if batch.Rows > s.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", batch.Rows, s.config.MaxBatchSize)
	}

	atomic.AddInt64(&s.activeStreams, 1)
	defer atomic.AddInt64(&s.activeStreams, -1)

	start := time.Now()

	s.mu.RLock()
	writer := s.writer
	s.mu.RUnlock()

	if writer != nil {
		result, err := writer.WriteBatch(ctx, batch)
		if err != nil {
			atomic.AddInt64(&s.stats.ErrorCount, 1)
			return nil, fmt.Errorf("writing batch: %w", err)
		}
		atomic.AddInt64(&s.stats.TotalDoPuts, 1)
		atomic.AddInt64(&s.stats.TotalRowsWritten, result.RowsWritten)
		return result, nil
	}

	// Fallback: count and acknowledge
	result := &PutResult{
		RowsWritten: int64(batch.Rows),
		DurationMs:  time.Since(start).Milliseconds(),
	}
	atomic.AddInt64(&s.stats.TotalDoPuts, 1)
	atomic.AddInt64(&s.stats.TotalRowsWritten, int64(batch.Rows))
	return result, nil
}

// DoExchange performs bidirectional data exchange for interactive queries.
func (s *Server) DoExchange(ctx context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	atomic.AddInt64(&s.activeStreams, 1)
	defer atomic.AddInt64(&s.activeStreams, -1)
	atomic.AddInt64(&s.stats.TotalExchanges, 1)

	switch req.Command {
	case "query":
		return s.handleQueryExchange(ctx, req)
	case "transform":
		return s.handleTransformExchange(ctx, req)
	case "aggregate":
		return s.handleAggregateExchange(ctx, req)
	default:
		return nil, fmt.Errorf("unknown exchange command: %s", req.Command)
	}
}

// ListFlights returns available Flight endpoints.
func (s *Server) ListFlights() []FlightInfo {
	var flights []FlightInfo
	s.tickets.Range(func(_, value interface{}) bool {
		ft := value.(*FlightTicket)
		if time.Now().Before(ft.ExpiresAt) {
			flights = append(flights, FlightInfo{
				Schema:     ft.Schema,
				Descriptor: ft.Desc,
				Endpoints: []FlightEndpoint{
					{Ticket: *ft, Location: "local"},
				},
			})
		}
		return true
	})
	return flights
}

// Stats returns current server performance metrics.
func (s *Server) Stats() Stats {
	stats := Stats{
		TotalDoGets:       atomic.LoadInt64(&s.stats.TotalDoGets),
		TotalDoPuts:       atomic.LoadInt64(&s.stats.TotalDoPuts),
		TotalExchanges:    atomic.LoadInt64(&s.stats.TotalExchanges),
		ActiveStreams:     atomic.LoadInt64(&s.activeStreams),
		TotalRowsRead:    atomic.LoadInt64(&s.stats.TotalRowsRead),
		TotalRowsWritten: atomic.LoadInt64(&s.stats.TotalRowsWritten),
		TotalBytesRead:   atomic.LoadInt64(&s.stats.TotalBytesRead),
		TotalBytesWritten: atomic.LoadInt64(&s.stats.TotalBytesWritten),
		ErrorCount:       atomic.LoadInt64(&s.stats.ErrorCount),
	}
	totalOps := stats.TotalDoGets + stats.TotalDoPuts
	if totalOps > 0 {
		stats.AvgBatchSize = float64(stats.TotalRowsRead+stats.TotalRowsWritten) / float64(totalOps)
	}
	return stats
}

func (s *Server) issueTicket(desc FlightDescriptor, schema []ColumnSchema) *FlightTicket {
	id := fmt.Sprintf("ft-%d", atomic.AddInt64(&s.nextTicketID, 1))
	ticket := &FlightTicket{
		ID:        id,
		Desc:      desc,
		Schema:    schema,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(s.config.StreamTimeoutSeconds) * time.Second),
	}
	s.tickets.Store(id, ticket)
	return ticket
}

func (s *Server) buildEmptyBatch(ft *FlightTicket) *RecordBatch {
	columns := make(map[string][]interface{})
	for _, col := range ft.Schema {
		columns[col.Name] = []interface{}{}
	}
	return &RecordBatch{
		Schema:  ft.Schema,
		Rows:    0,
		Columns: columns,
	}
}

func (s *Server) handleQueryExchange(_ context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	if len(req.Descriptor.Features) == 0 {
		return nil, fmt.Errorf("query exchange requires features")
	}

	s.mu.RLock()
	reader := s.reader
	s.mu.RUnlock()

	if reader != nil && len(req.Descriptor.Entities) > 0 {
		batch, err := reader.ReadBatch(context.Background(), req.Descriptor.Entities, req.Descriptor.Features)
		if err != nil {
			return nil, fmt.Errorf("query exchange: %w", err)
		}
		return &ExchangeResponse{Data: batch, Status: "ok"}, nil
	}

	return &ExchangeResponse{
		Status: "ok",
		Data:   &RecordBatch{Rows: 0, Columns: map[string][]interface{}{}},
	}, nil
}

func (s *Server) handleTransformExchange(_ context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	if req.Data == nil {
		return nil, fmt.Errorf("transform exchange requires input data")
	}
	// Pass-through transform (identity) — real implementation would apply column transforms
	return &ExchangeResponse{Data: req.Data, Status: "ok"}, nil
}

func (s *Server) handleAggregateExchange(_ context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	if req.Data == nil {
		return nil, fmt.Errorf("aggregate exchange requires input data")
	}

	resultCols := make(map[string][]interface{})
	for colName, colData := range req.Data.Columns {
		if len(colData) == 0 {
			continue
		}
		// Compute basic count aggregate
		resultCols[colName+"_count"] = []interface{}{len(colData)}
	}

	return &ExchangeResponse{
		Data: &RecordBatch{
			Rows:    1,
			Columns: resultCols,
		},
		Status: "ok",
	}, nil
}
