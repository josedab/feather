package ingestion

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// BatchImporter imports features from files.
type BatchImporter struct {
	store   *storage.Store
	agg     *aggregation.Engine
	schema  *storage.Registry
	metrics *BatchMetrics
}

// BatchMetrics tracks batch import performance.
type BatchMetrics struct {
	FilesProcessed   int64
	RowsProcessed    int64
	RowsSuccess      int64
	RowsError        int64
	FeaturesImported int64
}

// ImportConfig configures the batch import.
type ImportConfig struct {
	// EntityKeyColumn is the name/index of the entity key column
	EntityKeyColumn string

	// TimestampColumn is the name/index of the timestamp column (optional)
	TimestampColumn string

	// TimestampFormat for parsing timestamps (default: RFC3339)
	TimestampFormat string

	// FeatureColumns maps column names to feature names (if different)
	FeatureColumns map[string]string

	// BatchSize for writing to storage
	BatchSize int

	// SkipErrors continues import on row errors
	SkipErrors bool

	// HasHeader indicates if CSV has header row
	HasHeader bool
}

// NewBatchImporter creates a new batch importer.
func NewBatchImporter(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
) *BatchImporter {
	return &BatchImporter{
		store:   store,
		agg:     agg,
		schema:  schema,
		metrics: &BatchMetrics{},
	}
}

// ImportResult contains import statistics.
type ImportResult struct {
	RowsProcessed    int64
	RowsSuccess      int64
	RowsError        int64
	FeaturesImported int64
	Duration         time.Duration
	Errors           []string
}

// ImportCSV imports features from a CSV file.
func (b *BatchImporter) ImportCSV(ctx context.Context, path string, config ImportConfig) (*ImportResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	return b.ImportCSVReader(ctx, file, config)
}

// ImportCSVReader imports features from a CSV reader.
func (b *BatchImporter) ImportCSVReader(ctx context.Context, r io.Reader, config ImportConfig) (*ImportResult, error) {
	start := time.Now()
	result := &ImportResult{}

	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable fields

	// Read header if present
	var header []string
	if config.HasHeader {
		var err error
		header, err = reader.Read()
		if err != nil {
			return nil, fmt.Errorf("reading header: %w", err)
		}
	}

	// Find column indices
	entityKeyIdx := -1
	timestampIdx := -1
	featureIndices := make(map[int]string) // column index -> feature name

	if config.HasHeader && len(header) > 0 {
		for i, col := range header {
			col = strings.TrimSpace(col)
			if col == config.EntityKeyColumn {
				entityKeyIdx = i
			} else if col == config.TimestampColumn {
				timestampIdx = i
			} else {
				// Map to feature name
				featureName := col
				if mapped, ok := config.FeatureColumns[col]; ok {
					featureName = mapped
				}
				featureIndices[i] = featureName
			}
		}
	} else {
		// Use numeric indices
		if idx, err := strconv.Atoi(config.EntityKeyColumn); err == nil {
			entityKeyIdx = idx
		}
		if config.TimestampColumn != "" {
			if idx, err := strconv.Atoi(config.TimestampColumn); err == nil {
				timestampIdx = idx
			}
		}
	}

	if entityKeyIdx == -1 {
		return nil, fmt.Errorf("entity key column not found: %s", config.EntityKeyColumn)
	}

	timestampFormat := config.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = time.RFC3339
	}

	// Process rows
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if config.SkipErrors {
				result.RowsError++
				result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", result.RowsProcessed, err))
				continue
			}
			return nil, fmt.Errorf("reading row %d: %w", result.RowsProcessed, err)
		}

		result.RowsProcessed++

		if entityKeyIdx >= len(record) {
			if config.SkipErrors {
				result.RowsError++
				continue
			}
			return nil, fmt.Errorf("row %d: entity key column out of bounds", result.RowsProcessed)
		}

		entityKey := strings.TrimSpace(record[entityKeyIdx])
		if entityKey == "" {
			if config.SkipErrors {
				result.RowsError++
				continue
			}
			return nil, fmt.Errorf("row %d: empty entity key", result.RowsProcessed)
		}

		// Parse timestamp
		var timestamp int64
		if timestampIdx >= 0 && timestampIdx < len(record) {
			tsStr := strings.TrimSpace(record[timestampIdx])
			if tsStr != "" {
				if ts, err := time.Parse(timestampFormat, tsStr); err == nil {
					timestamp = ts.UnixNano()
				} else if tsNano, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
					timestamp = tsNano
				}
			}
		}
		if timestamp == 0 {
			timestamp = time.Now().UnixNano()
		}

		// Build features
		features := make(map[string]*domain.FeatureValue)
		for idx, featureName := range featureIndices {
			if idx >= len(record) {
				continue
			}

			value := strings.TrimSpace(record[idx])
			if value == "" {
				continue
			}

			parsedValue := parseValue(value)
			features[featureName] = &domain.FeatureValue{
				Value:     parsedValue,
				Timestamp: timestamp,
				Version:   1,
			}

			// Update aggregations
			if b.agg.GetSpec(featureName) != nil {
				if floatVal, ok := domain.ToFloat64(parsedValue); ok {
					if err := b.agg.Update(entityKey, featureName, floatVal, time.Unix(0, timestamp)); err != nil {
						return nil, fmt.Errorf("updating aggregation: %w", err)
					}
				}
			}

			result.FeaturesImported++
		}

		if len(features) > 0 {
			if err := b.store.Put(entityKey, features); err != nil {
				if config.SkipErrors {
					result.RowsError++
					result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", result.RowsProcessed, err))
					continue
				}
				return nil, fmt.Errorf("storing features for row %d: %w", result.RowsProcessed, err)
			}
		}

		result.RowsSuccess++

		atomic.AddInt64(&b.metrics.RowsProcessed, 1)
		atomic.AddInt64(&b.metrics.RowsSuccess, 1)
		atomic.AddInt64(&b.metrics.FeaturesImported, int64(len(features)))
	}

	atomic.AddInt64(&b.metrics.FilesProcessed, 1)
	result.Duration = time.Since(start)
	return result, nil
}

// ImportJSON imports features from a JSON file.
func (b *BatchImporter) ImportJSON(ctx context.Context, path string, config ImportConfig) (*ImportResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	return b.ImportJSONReader(ctx, file, config)
}

// ImportJSONReader imports features from a JSON reader.
func (b *BatchImporter) ImportJSONReader(ctx context.Context, r io.Reader, config ImportConfig) (*ImportResult, error) {
	start := time.Now()
	result := &ImportResult{}

	// Try to decode as array first
	var records []map[string]interface{}
	decoder := json.NewDecoder(r)

	// Check if it's an array
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	for _, record := range records {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result.RowsProcessed++

		entityKey, ok := record[config.EntityKeyColumn].(string)
		if !ok {
			if config.SkipErrors {
				result.RowsError++
				continue
			}
			return nil, fmt.Errorf("row %d: missing or invalid entity key", result.RowsProcessed)
		}

		timestamp := time.Now().UnixNano()
		if tsVal, ok := record[config.TimestampColumn]; ok {
			switch ts := tsVal.(type) {
			case float64:
				timestamp = int64(ts)
			case string:
				if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
					timestamp = parsed.UnixNano()
				}
			}
		}

		features := make(map[string]*domain.FeatureValue)
		for key, value := range record {
			if key == config.EntityKeyColumn || key == config.TimestampColumn {
				continue
			}

			featureName := key
			if mapped, ok := config.FeatureColumns[key]; ok {
				featureName = mapped
			}

			features[featureName] = &domain.FeatureValue{
				Value:     value,
				Timestamp: timestamp,
				Version:   1,
			}

			if b.agg.GetSpec(featureName) != nil {
				if floatVal, ok := domain.ToFloat64(value); ok {
					if err := b.agg.Update(entityKey, featureName, floatVal, time.Unix(0, timestamp)); err != nil {
						return nil, fmt.Errorf("updating aggregation: %w", err)
					}
				}
			}

			result.FeaturesImported++
		}

		if len(features) > 0 {
			if err := b.store.Put(entityKey, features); err != nil {
				if config.SkipErrors {
					result.RowsError++
					continue
				}
				return nil, fmt.Errorf("storing features: %w", err)
			}
		}

		result.RowsSuccess++
	}

	atomic.AddInt64(&b.metrics.FilesProcessed, 1)
	result.Duration = time.Since(start)
	return result, nil
}

// ImportJSONL imports features from a JSON Lines file.
func (b *BatchImporter) ImportJSONL(ctx context.Context, path string, config ImportConfig) (*ImportResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	return b.ImportJSONLReader(ctx, file, config)
}

// ImportJSONLReader imports features from a JSON Lines reader.
func (b *BatchImporter) ImportJSONLReader(ctx context.Context, r io.Reader, config ImportConfig) (*ImportResult, error) {
	start := time.Now()
	result := &ImportResult{}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		result.RowsProcessed++

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			if config.SkipErrors {
				result.RowsError++
				result.Errors = append(result.Errors, fmt.Sprintf("line %d: %v", result.RowsProcessed, err))
				continue
			}
			return nil, fmt.Errorf("parsing line %d: %w", result.RowsProcessed, err)
		}

		entityKey, ok := record[config.EntityKeyColumn].(string)
		if !ok {
			if config.SkipErrors {
				result.RowsError++
				continue
			}
			return nil, fmt.Errorf("line %d: missing entity key", result.RowsProcessed)
		}

		timestamp := time.Now().UnixNano()
		if tsVal, ok := record[config.TimestampColumn]; ok {
			switch ts := tsVal.(type) {
			case float64:
				timestamp = int64(ts)
			case string:
				if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
					timestamp = parsed.UnixNano()
				}
			}
		}

		features := make(map[string]*domain.FeatureValue)
		for key, value := range record {
			if key == config.EntityKeyColumn || key == config.TimestampColumn {
				continue
			}

			featureName := key
			if mapped, ok := config.FeatureColumns[key]; ok {
				featureName = mapped
			}

			features[featureName] = &domain.FeatureValue{
				Value:     value,
				Timestamp: timestamp,
				Version:   1,
			}

			if b.agg.GetSpec(featureName) != nil {
				if floatVal, ok := domain.ToFloat64(value); ok {
					if err := b.agg.Update(entityKey, featureName, floatVal, time.Unix(0, timestamp)); err != nil {
						return nil, fmt.Errorf("updating aggregation: %w", err)
					}
				}
			}

			result.FeaturesImported++
		}

		if len(features) > 0 {
			if err := b.store.Put(entityKey, features); err != nil {
				if config.SkipErrors {
					result.RowsError++
					continue
				}
				return nil, fmt.Errorf("storing features: %w", err)
			}
		}

		result.RowsSuccess++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	atomic.AddInt64(&b.metrics.FilesProcessed, 1)
	result.Duration = time.Since(start)
	return result, nil
}

// Metrics returns current metrics.
func (b *BatchImporter) Metrics() BatchMetrics {
	return BatchMetrics{
		FilesProcessed:   atomic.LoadInt64(&b.metrics.FilesProcessed),
		RowsProcessed:    atomic.LoadInt64(&b.metrics.RowsProcessed),
		RowsSuccess:      atomic.LoadInt64(&b.metrics.RowsSuccess),
		RowsError:        atomic.LoadInt64(&b.metrics.RowsError),
		FeaturesImported: atomic.LoadInt64(&b.metrics.FeaturesImported),
	}
}

func parseValue(s string) interface{} {
	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Try bool
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}

	// Try JSON array (for vectors)
	if strings.HasPrefix(s, "[") {
		var arr []float32
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
	}

	// Return as string
	return s
}
