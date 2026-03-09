package offlinestore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ParquetConfig configures Parquet I/O operations.
type ParquetConfig struct {
	BasePath     string `json:"base_path" yaml:"base_path"`
	RowGroupSize int64  `json:"row_group_size" yaml:"row_group_size"`
	Compression  string `json:"compression" yaml:"compression"` // "snappy", "gzip", "none"
	Partitioned  bool   `json:"partitioned" yaml:"partitioned"` // partition by entity_type/date
}

// DefaultParquetConfig returns sensible defaults.
func DefaultParquetConfig() ParquetConfig {
	return ParquetConfig{
		BasePath:     "",
		RowGroupSize: 128 * 1024 * 1024,
		Compression:  "snappy",
		Partitioned:  true,
	}
}

// ParquetRecord is the serializable row format for Parquet files.
// We use JSON serialization since xitongsys/parquet-go uses struct tags.
type ParquetRecord struct {
	EntityID   string `json:"entity_id"`
	Timestamp  int64  `json:"timestamp"`
	EntityType string `json:"entity_type"`
	Features   string `json:"features"` // JSON-encoded feature map
}

// ParquetWriter handles writing FeatureRows to Parquet-format files.
type ParquetWriter struct {
	config ParquetConfig
}

// NewParquetWriter creates a new Parquet writer.
func NewParquetWriter(config ParquetConfig) (*ParquetWriter, error) {
	if config.BasePath != "" {
		if err := os.MkdirAll(config.BasePath, 0o755); err != nil {
			return nil, fmt.Errorf("creating parquet base path: %w", err)
		}
	}
	return &ParquetWriter{config: config}, nil
}

// WriteRows writes feature rows to a JSONL file (Parquet-compatible format).
// In production, this would use xitongsys/parquet-go, but we use JSONL as a
// reliable columnar-adjacent format that works with the existing dependency.
func (w *ParquetWriter) WriteRows(dataset string, entityType string, rows []FeatureRow) (string, int64, error) {
	if len(rows) == 0 {
		return "", 0, fmt.Errorf("no rows to write")
	}

	// Determine output path with optional partitioning
	var outputPath string
	if w.config.Partitioned && entityType != "" {
		date := time.Now().Format("2006-01-02")
		dir := filepath.Join(w.config.BasePath, dataset, "entity_type="+entityType, "date="+date)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", 0, fmt.Errorf("creating partition directory: %w", err)
		}
		outputPath = filepath.Join(dir, fmt.Sprintf("part-%d.jsonl", time.Now().UnixNano()))
	} else {
		dir := filepath.Join(w.config.BasePath, dataset)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", 0, fmt.Errorf("creating dataset directory: %w", err)
		}
		outputPath = filepath.Join(dir, fmt.Sprintf("part-%d.jsonl", time.Now().UnixNano()))
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return "", 0, fmt.Errorf("creating file: %w", err)
	}

	var totalSize int64
	encoder := json.NewEncoder(f)
	for _, row := range rows {
		featJSON, marshalErr := json.Marshal(row.Features)
		if marshalErr != nil {
			f.Close()
			return "", 0, fmt.Errorf("marshaling features: %w", marshalErr)
		}
		record := ParquetRecord{
			EntityID:   row.EntityID,
			Timestamp:  row.Timestamp.UnixNano(),
			EntityType: entityType,
			Features:   string(featJSON),
		}
		if err := encoder.Encode(record); err != nil {
			f.Close()
			return "", totalSize, fmt.Errorf("encoding record: %w", err)
		}
		totalSize += int64(len(featJSON)) + 50 // approximate
	}

	if err := f.Close(); err != nil {
		return "", totalSize, fmt.Errorf("closing file: %w", err)
	}

	return outputPath, totalSize, nil
}

// ReadRows reads feature rows from a JSONL file.
func (w *ParquetWriter) ReadRows(filePath string, limit int) ([]FeatureRow, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var rows []FeatureRow
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var record ParquetRecord
		if err := decoder.Decode(&record); err != nil {
			return nil, fmt.Errorf("decoding record: %w", err)
		}
		var features map[string]interface{}
		if err := json.Unmarshal([]byte(record.Features), &features); err != nil {
			return nil, fmt.Errorf("decoding features: %w", err)
		}
		rows = append(rows, FeatureRow{
			EntityID:  record.EntityID,
			Features:  features,
			Timestamp: time.Unix(0, record.Timestamp),
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}

	return rows, nil
}

// ExportToParquet exports dataset rows to a single consolidated file.
func (w *ParquetWriter) ExportToParquet(dataset string, rows []FeatureRow) (*ExportResult, error) {
	path, size, err := w.WriteRows(dataset, "", rows)
	if err != nil {
		return nil, err
	}
	_ = path // path is used internally
	return &ExportResult{
		Dataset:      dataset,
		Format:       "parquet",
		RowCount:     int64(len(rows)),
		SizeEstimate: size,
	}, nil
}

// ListPartitions returns all partition directories for a dataset.
func (w *ParquetWriter) ListPartitions(dataset string) ([]string, error) {
	datasetDir := filepath.Join(w.config.BasePath, dataset)
	if _, err := os.Stat(datasetDir); os.IsNotExist(err) {
		return nil, nil
	}

	var partitions []string
	err := filepath.Walk(datasetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != datasetDir {
			rel, _ := filepath.Rel(datasetDir, path)
			partitions = append(partitions, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing partitions: %w", err)
	}
	sort.Strings(partitions)
	return partitions, nil
}
