package export

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

var (
	// privateTempDir is a process-private temp directory with restricted permissions
	privateTempDir     string
	privateTempDirOnce sync.Once
	privateTempDirErr  error
)

// getPrivateTempDir returns a private temp directory for this process.
// The directory is created with 0700 permissions (owner read/write/execute only).
func getPrivateTempDir() (string, error) {
	privateTempDirOnce.Do(func() {
		// Create a private subdirectory in the system temp directory
		baseTemp := os.TempDir()
		privateTempDir = filepath.Join(baseTemp, fmt.Sprintf("feather-export-%d", os.Getpid()))

		// Create with restrictive permissions (owner only)
		if err := os.MkdirAll(privateTempDir, 0700); err != nil {
			privateTempDirErr = fmt.Errorf("creating private temp dir: %w", err)
			return
		}

		// Double-check permissions in case directory already existed
		if err := os.Chmod(privateTempDir, 0700); err != nil {
			privateTempDirErr = fmt.Errorf("setting temp dir permissions: %w", err)
		}
	})
	return privateTempDir, privateTempDirErr
}

// FeatureRecord is the base struct for Parquet export.
// Features are stored as JSON-encoded strings for flexibility.
type FeatureRecord struct {
	EntityKey string `parquet:"name=entity_key, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN"`
	Timestamp int64  `parquet:"name=timestamp, type=INT64, encoding=PLAIN"`
	Features  string `parquet:"name=features, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN"`
}

// writeParquet writes rows in Parquet format.
func writeParquet(w io.Writer, rows []ParquetRow, featureNames []string) error {
	if len(rows) == 0 {
		return writeEmptyParquet(w)
	}

	// Get private temp directory with restricted permissions
	tempDir, err := getPrivateTempDir()
	if err != nil {
		return err
	}

	// For parquet-go, we need to write to a file first, then copy to the writer
	// This is because parquet-go requires seeking which io.Writer doesn't support
	tmpFile, err := os.CreateTemp(tempDir, "feather-export-*.parquet")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create parquet file writer
	fw, err := local.NewLocalFileWriter(tmpPath)
	if err != nil {
		return fmt.Errorf("creating file writer: %w", err)
	}

	pw, err := writer.NewParquetWriter(fw, new(FeatureRecord), 4)
	if err != nil {
		fw.Close()
		return fmt.Errorf("creating parquet writer: %w", err)
	}

	pw.CompressionType = 0 // UNCOMPRESSED

	// Write rows
	for _, row := range rows {
		// Convert features map to JSON
		featuresJSON, err := json.Marshal(row.Features)
		if err != nil {
			pw.WriteStop()
			fw.Close()
			return fmt.Errorf("marshaling features: %w", err)
		}

		record := FeatureRecord{
			EntityKey: row.EntityKey,
			Timestamp: row.Timestamp,
			Features:  string(featuresJSON),
		}

		if err := pw.Write(record); err != nil {
			pw.WriteStop()
			fw.Close()
			return fmt.Errorf("writing record: %w", err)
		}
	}

	if err := pw.WriteStop(); err != nil {
		fw.Close()
		return fmt.Errorf("finalizing parquet: %w", err)
	}
	fw.Close()

	// Copy temp file to writer
	tmpFile, err = os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(w, tmpFile); err != nil {
		return fmt.Errorf("copying to output: %w", err)
	}

	return nil
}

func writeEmptyParquet(w io.Writer) error {
	// Get private temp directory with restricted permissions
	tempDir, err := getPrivateTempDir()
	if err != nil {
		return err
	}

	// Create an empty parquet file with just schema
	tmpFile, err := os.CreateTemp(tempDir, "feather-export-empty-*.parquet")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	fw, err := local.NewLocalFileWriter(tmpPath)
	if err != nil {
		return fmt.Errorf("creating file writer: %w", err)
	}

	pw, err := writer.NewParquetWriter(fw, new(FeatureRecord), 4)
	if err != nil {
		fw.Close()
		return fmt.Errorf("creating parquet writer: %w", err)
	}

	if err := pw.WriteStop(); err != nil {
		fw.Close()
		return fmt.Errorf("finalizing parquet: %w", err)
	}
	fw.Close()

	// Copy to writer
	tmpFile, err = os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(w, tmpFile); err != nil {
		return fmt.Errorf("copying to output: %w", err)
	}

	return nil
}

// ParquetSchemaBuilder builds dynamic Parquet schemas.
type ParquetSchemaBuilder struct {
	fields []reflect.StructField
}

// NewParquetSchemaBuilder creates a new schema builder.
func NewParquetSchemaBuilder() *ParquetSchemaBuilder {
	return &ParquetSchemaBuilder{
		fields: make([]reflect.StructField, 0),
	}
}

// AddStringField adds a string field to the schema.
func (b *ParquetSchemaBuilder) AddStringField(name string) {
	b.fields = append(b.fields, reflect.StructField{
		Name: name,
		Type: reflect.TypeOf(""),
		Tag:  reflect.StructTag(fmt.Sprintf(`parquet:"name=%s, type=BYTE_ARRAY, convertedtype=UTF8"`, name)),
	})
}

// AddInt64Field adds an int64 field to the schema.
func (b *ParquetSchemaBuilder) AddInt64Field(name string) {
	b.fields = append(b.fields, reflect.StructField{
		Name: name,
		Type: reflect.TypeOf(int64(0)),
		Tag:  reflect.StructTag(fmt.Sprintf(`parquet:"name=%s, type=INT64"`, name)),
	})
}

// AddFloat64Field adds a float64 field to the schema.
func (b *ParquetSchemaBuilder) AddFloat64Field(name string) {
	b.fields = append(b.fields, reflect.StructField{
		Name: name,
		Type: reflect.TypeOf(float64(0)),
		Tag:  reflect.StructTag(fmt.Sprintf(`parquet:"name=%s, type=DOUBLE"`, name)),
	})
}
