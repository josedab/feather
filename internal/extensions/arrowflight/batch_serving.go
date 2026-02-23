package arrowflight

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// BatchConfig controls batch serving behavior.
type BatchConfig struct {
	MaxBatchSize   int  `json:"max_batch_size" yaml:"max_batch_size"`
	MaxConcurrency int  `json:"max_concurrency" yaml:"max_concurrency"`
	EnableStats    bool `json:"enable_stats" yaml:"enable_stats"`
}

// DefaultBatchConfig returns production-ready defaults.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:   10000,
		MaxConcurrency: 4,
		EnableStats:    true,
	}
}

// BatchStats tracks batch serving performance metrics.
type BatchStats struct {
	TotalBatches   int64   `json:"total_batches"`
	TotalRows      int64   `json:"total_rows"`
	TotalBytes     int64   `json:"total_bytes"`
	AvgBatchSize   float64 `json:"avg_batch_size"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	ErrorCount     int64   `json:"error_count"`
	TotalLatencyMs int64   `json:"total_latency_ms"`
}

// BatchServer wraps the existing Server to provide optimized batch serving
// with parallel column construction and throughput metrics.
type BatchServer struct {
	server    *Server
	config    BatchConfig
	converter *BatchConverter

	stats struct {
		totalBatches   int64
		totalRows      int64
		totalBytes     int64
		totalLatencyMs int64
		errorCount     int64
	}
}

// NewBatchServer creates a batch server wrapping the given Flight server.
func NewBatchServer(server *Server, cfg BatchConfig) *BatchServer {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = DefaultBatchConfig().MaxBatchSize
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = DefaultBatchConfig().MaxConcurrency
	}
	return &BatchServer{
		server:    server,
		config:    cfg,
		converter: NewBatchConverter(),
	}
}

// ServeBatch retrieves features for the given entities and converts them to columnar format.
// It uses parallel column construction when concurrency is configured.
func (bs *BatchServer) ServeBatch(ctx context.Context, entities []string, features []string) (*ColumnarBatch, error) {
	if len(entities) == 0 {
		return nil, fmt.Errorf("at least one entity is required")
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("at least one feature is required")
	}

	start := time.Now()

	// Split into sub-batches if needed
	batches, err := bs.fetchInBatches(ctx, entities, features)
	if err != nil {
		atomic.AddInt64(&bs.stats.errorCount, 1)
		return nil, fmt.Errorf("fetching batches: %w", err)
	}

	// Merge sub-batches into a single columnar batch
	result, err := bs.mergeBatches(batches, features)
	if err != nil {
		atomic.AddInt64(&bs.stats.errorCount, 1)
		return nil, fmt.Errorf("merging batches: %w", err)
	}

	if bs.config.EnableStats {
		latencyMs := time.Since(start).Milliseconds()
		atomic.AddInt64(&bs.stats.totalBatches, 1)
		atomic.AddInt64(&bs.stats.totalRows, int64(result.RowCount))
		atomic.AddInt64(&bs.stats.totalBytes, result.ByteSize)
		atomic.AddInt64(&bs.stats.totalLatencyMs, latencyMs)
	}

	return result, nil
}

func (bs *BatchServer) fetchInBatches(ctx context.Context, entities []string, features []string) ([]*RecordBatch, error) {
	// Split entities into chunks based on MaxBatchSize
	chunks := splitEntities(entities, bs.config.MaxBatchSize)

	type result struct {
		batch *RecordBatch
		err   error
		idx   int
	}

	results := make([]result, len(chunks))
	sem := make(chan struct{}, bs.config.MaxConcurrency)
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, ents []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			desc := FlightDescriptor{
				Type:     "path",
				Entities: ents,
				Features: features,
			}

			info, err := bs.server.GetFlightInfo(ctx, desc)
			if err != nil {
				results[idx] = result{err: err, idx: idx}
				return
			}

			if len(info.Endpoints) == 0 {
				results[idx] = result{err: fmt.Errorf("no endpoints available"), idx: idx}
				return
			}

			batch, err := bs.server.DoGet(ctx, info.Endpoints[0].Ticket)
			results[idx] = result{batch: batch, err: err, idx: idx}
		}(i, chunk)
	}

	wg.Wait()

	batches := make([]*RecordBatch, 0, len(chunks))
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		if r.batch != nil {
			batches = append(batches, r.batch)
		}
	}

	return batches, nil
}

func (bs *BatchServer) mergeBatches(batches []*RecordBatch, features []string) (*ColumnarBatch, error) {
	if len(batches) == 0 {
		schema := make([]ColumnSchema, 0, len(features)+1)
		schema = append(schema, ColumnSchema{Name: "entity_key", Type: DataTypeString, Nullable: false})
		for _, f := range features {
			schema = append(schema, ColumnSchema{Name: f, Type: DataTypeFloat64, Nullable: true})
		}
		return &ColumnarBatch{
			Columns:  make([]*Column, 0),
			RowCount: 0,
			Schema:   schema,
			Metadata: map[string]string{},
		}, nil
	}

	if len(batches) == 1 {
		return bs.converter.FromRecordBatch(batches[0]), nil
	}

	// Merge multiple batches: use schema from first batch
	schema := batches[0].Schema
	totalRows := 0
	for _, b := range batches {
		totalRows += b.Rows
	}

	merged := make(map[string][]interface{}, len(schema))
	for _, cs := range schema {
		merged[cs.Name] = make([]interface{}, 0, totalRows)
	}

	for _, b := range batches {
		for colName, colData := range b.Columns {
			merged[colName] = append(merged[colName], colData...)
		}
	}

	rb := &RecordBatch{
		Schema:  schema,
		Rows:    totalRows,
		Columns: merged,
	}

	return bs.converter.FromRecordBatch(rb), nil
}

// Stats returns current batch serving performance metrics.
func (bs *BatchServer) Stats() BatchStats {
	totalBatches := atomic.LoadInt64(&bs.stats.totalBatches)
	totalRows := atomic.LoadInt64(&bs.stats.totalRows)
	totalLatencyMs := atomic.LoadInt64(&bs.stats.totalLatencyMs)

	stats := BatchStats{
		TotalBatches:   totalBatches,
		TotalRows:      totalRows,
		TotalBytes:     atomic.LoadInt64(&bs.stats.totalBytes),
		ErrorCount:     atomic.LoadInt64(&bs.stats.errorCount),
		TotalLatencyMs: totalLatencyMs,
	}

	if totalBatches > 0 {
		stats.AvgBatchSize = float64(totalRows) / float64(totalBatches)
		stats.AvgLatencyMs = float64(totalLatencyMs) / float64(totalBatches)
	}

	return stats
}

func splitEntities(entities []string, maxSize int) [][]string {
	if maxSize <= 0 || len(entities) <= maxSize {
		return [][]string{entities}
	}

	var chunks [][]string
	for i := 0; i < len(entities); i += maxSize {
		end := i + maxSize
		if end > len(entities) {
			end = len(entities)
		}
		chunks = append(chunks, entities[i:end])
	}
	return chunks
}
