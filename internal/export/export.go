// Package export provides training data export functionality.
package export

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// Exporter exports features for training.
type Exporter struct {
	store  *storage.Store
	schema *storage.Registry
}

// NewExporter creates a new exporter.
func NewExporter(store *storage.Store, schema *storage.Registry) *Exporter {
	return &Exporter{
		store:  store,
		schema: schema,
	}
}

// ExportRequest defines what to export.
type ExportRequest struct {
	// Entities to export (if empty, exports all)
	Entities []string

	// Features to export
	Features []string

	// Time range for point-in-time exports
	StartTime *time.Time
	EndTime   *time.Time

	// Output format
	Format ExportFormat

	// Output path
	OutputPath string
}

// ExportFormat defines the output format.
type ExportFormat string

const (
	FormatCSV     ExportFormat = "csv"
	FormatJSON    ExportFormat = "json"
	FormatJSONL   ExportFormat = "jsonl"
	FormatParquet ExportFormat = "parquet"
)

// ExportResult contains export statistics.
type ExportResult struct {
	EntitiesExported int
	FeaturesExported int
	RowsWritten      int
	BytesWritten     int64
	Duration         time.Duration
	OutputPath       string
}

// Export exports features to the specified format.
func (e *Exporter) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	start := time.Now()

	// Validate request
	if len(req.Features) == 0 {
		return nil, fmt.Errorf("at least one feature required")
	}

	if req.OutputPath == "" {
		return nil, fmt.Errorf("output path required")
	}

	// Create output directory if needed
	dir := filepath.Dir(req.OutputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Open output file
	file, err := os.Create(req.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	var result *ExportResult

	switch req.Format {
	case FormatCSV:
		result, err = e.exportCSV(ctx, req, file)
	case FormatJSON:
		result, err = e.exportJSON(ctx, req, file)
	case FormatJSONL:
		result, err = e.exportJSONL(ctx, req, file)
	case FormatParquet:
		result, err = e.exportParquet(ctx, req, file)
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}

	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	result.OutputPath = req.OutputPath

	// Get file size
	if stat, err := file.Stat(); err == nil {
		result.BytesWritten = stat.Size()
	}

	return result, nil
}

// TrainingRow represents a single row for training.
type TrainingRow struct {
	EntityKey string                 `json:"entity_key"`
	Timestamp int64                  `json:"timestamp"`
	Features  map[string]interface{} `json:"features"`
}

func (e *Exporter) exportCSV(ctx context.Context, req ExportRequest, w io.Writer) (*ExportResult, error) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	result := &ExportResult{}

	// Write header
	header := append([]string{"entity_key", "timestamp"}, req.Features...)
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}

	// Export data
	entities := req.Entities
	if len(entities) == 0 {
		// If no entities specified, we'd need to scan all - for now return error
		return nil, fmt.Errorf("entity list required for export")
	}

	for _, entityKey := range entities {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var features map[string]*domain.FeatureValue
		var err error

		if req.EndTime != nil {
			features, err = e.store.GetAsOf(entityKey, req.Features, *req.EndTime)
		} else {
			features, err = e.store.Get(entityKey, req.Features)
		}

		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("getting features for %s: %w", entityKey, err)
		}

		// Find the latest timestamp among features
		var maxTimestamp int64
		for _, v := range features {
			if v.Timestamp > maxTimestamp {
				maxTimestamp = v.Timestamp
			}
		}

		// Build row
		row := make([]string, len(header))
		row[0] = entityKey
		row[1] = fmt.Sprintf("%d", maxTimestamp)

		for i, featureName := range req.Features {
			if val, ok := features[featureName]; ok {
				row[i+2] = formatValue(val.Value)
			} else {
				row[i+2] = ""
			}
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("writing row: %w", err)
		}

		result.EntitiesExported++
		result.RowsWritten++
	}

	result.FeaturesExported = len(req.Features)
	return result, nil
}

func (e *Exporter) exportJSON(ctx context.Context, req ExportRequest, w io.Writer) (*ExportResult, error) {
	result := &ExportResult{}
	rows := make([]TrainingRow, 0)

	entities := req.Entities
	if len(entities) == 0 {
		return nil, fmt.Errorf("entity list required for export")
	}

	for _, entityKey := range entities {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var features map[string]*domain.FeatureValue
		var err error

		if req.EndTime != nil {
			features, err = e.store.GetAsOf(entityKey, req.Features, *req.EndTime)
		} else {
			features, err = e.store.Get(entityKey, req.Features)
		}

		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("getting features for %s: %w", entityKey, err)
		}

		var maxTimestamp int64
		featureMap := make(map[string]interface{})
		for name, val := range features {
			featureMap[name] = val.Value
			if val.Timestamp > maxTimestamp {
				maxTimestamp = val.Timestamp
			}
		}

		rows = append(rows, TrainingRow{
			EntityKey: entityKey,
			Timestamp: maxTimestamp,
			Features:  featureMap,
		})

		result.EntitiesExported++
		result.RowsWritten++
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rows); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}

	result.FeaturesExported = len(req.Features)
	return result, nil
}

func (e *Exporter) exportJSONL(ctx context.Context, req ExportRequest, w io.Writer) (*ExportResult, error) {
	result := &ExportResult{}
	encoder := json.NewEncoder(w)

	entities := req.Entities
	if len(entities) == 0 {
		return nil, fmt.Errorf("entity list required for export")
	}

	for _, entityKey := range entities {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var features map[string]*domain.FeatureValue
		var err error

		if req.EndTime != nil {
			features, err = e.store.GetAsOf(entityKey, req.Features, *req.EndTime)
		} else {
			features, err = e.store.Get(entityKey, req.Features)
		}

		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("getting features for %s: %w", entityKey, err)
		}

		var maxTimestamp int64
		featureMap := make(map[string]interface{})
		for name, val := range features {
			featureMap[name] = val.Value
			if val.Timestamp > maxTimestamp {
				maxTimestamp = val.Timestamp
			}
		}

		row := TrainingRow{
			EntityKey: entityKey,
			Timestamp: maxTimestamp,
			Features:  featureMap,
		}

		if err := encoder.Encode(row); err != nil {
			return nil, fmt.Errorf("encoding JSON line: %w", err)
		}

		result.EntitiesExported++
		result.RowsWritten++
	}

	result.FeaturesExported = len(req.Features)
	return result, nil
}

func (e *Exporter) exportParquet(ctx context.Context, req ExportRequest, w io.Writer) (*ExportResult, error) {
	result := &ExportResult{}

	entities := req.Entities
	if len(entities) == 0 {
		return nil, fmt.Errorf("entity list required for export")
	}

	// Build schema and collect data
	rows := make([]ParquetRow, 0)

	for _, entityKey := range entities {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var features map[string]*domain.FeatureValue
		var err error

		if req.EndTime != nil {
			features, err = e.store.GetAsOf(entityKey, req.Features, *req.EndTime)
		} else {
			features, err = e.store.Get(entityKey, req.Features)
		}

		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("getting features for %s: %w", entityKey, err)
		}

		var maxTimestamp int64
		featureMap := make(map[string]interface{})
		for name, val := range features {
			featureMap[name] = val.Value
			if val.Timestamp > maxTimestamp {
				maxTimestamp = val.Timestamp
			}
		}

		row := ParquetRow{
			EntityKey: entityKey,
			Timestamp: maxTimestamp,
			Features:  featureMap,
		}
		rows = append(rows, row)

		result.EntitiesExported++
		result.RowsWritten++
	}

	// Write Parquet file using our simple Parquet writer
	if err := writeParquet(w, rows, req.Features); err != nil {
		return nil, fmt.Errorf("writing parquet: %w", err)
	}

	result.FeaturesExported = len(req.Features)
	return result, nil
}

// ParquetRow represents a row in the Parquet output.
type ParquetRow struct {
	EntityKey string
	Timestamp int64
	Features  map[string]interface{}
}

// ExportPointInTime exports features at multiple timestamps for training.
func (e *Exporter) ExportPointInTime(ctx context.Context, req PointInTimeRequest) (*ExportResult, error) {
	start := time.Now()

	if len(req.Entities) == 0 {
		return nil, fmt.Errorf("entities required")
	}
	if len(req.Features) == 0 {
		return nil, fmt.Errorf("features required")
	}
	if len(req.Timestamps) == 0 {
		return nil, fmt.Errorf("timestamps required")
	}

	// Create output file
	file, err := os.Create(req.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := append([]string{"entity_key", "as_of_timestamp"}, req.Features...)
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("writing header: %w", err)
	}

	result := &ExportResult{}

	// Sort timestamps
	timestamps := make([]time.Time, len(req.Timestamps))
	copy(timestamps, req.Timestamps)
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	for _, entityKey := range req.Entities {
		for _, ts := range timestamps {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			features, err := e.store.GetAsOf(entityKey, req.Features, ts)
			if err != nil {
				if domain.IsNotFound(err) {
					continue
				}
				return nil, fmt.Errorf("getting features: %w", err)
			}

			row := make([]string, len(header))
			row[0] = entityKey
			row[1] = ts.Format(time.RFC3339)

			for i, featureName := range req.Features {
				if val, ok := features[featureName]; ok {
					row[i+2] = formatValue(val.Value)
				} else {
					row[i+2] = ""
				}
			}

			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("writing row: %w", err)
			}

			result.RowsWritten++
		}
		result.EntitiesExported++
	}

	result.FeaturesExported = len(req.Features)
	result.Duration = time.Since(start)
	result.OutputPath = req.OutputPath

	if stat, err := file.Stat(); err == nil {
		result.BytesWritten = stat.Size()
	}

	return result, nil
}

// PointInTimeRequest defines a point-in-time export request.
type PointInTimeRequest struct {
	Entities   []string
	Features   []string
	Timestamps []time.Time
	OutputPath string
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case float32:
		return fmt.Sprintf("%g", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []float32:
		// Vector - encode as JSON array
		data, _ := json.Marshal(val)
		return string(data)
	case []byte:
		return string(val)
	default:
		data, _ := json.Marshal(val)
		return string(data)
	}
}
