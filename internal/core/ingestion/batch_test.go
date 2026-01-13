package ingestion

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/storage"
)

// newTestBatchImporter creates a BatchImporter for testing.
func newTestBatchImporter(t *testing.T) *BatchImporter {
	t.Helper()

	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20, // 1MB
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	agg := aggregation.NewEngine()

	t.Cleanup(func() {
		store.Close()
	})

	return NewBatchImporter(store, agg, schema)
}

func TestBatchImporter_ImportCSVReader(t *testing.T) {
	b := newTestBatchImporter(t)

	tests := []struct {
		name        string
		csv         string
		config      ImportConfig
		wantRows    int64
		wantSuccess int64
		wantErr     bool
	}{
		{
			name: "basic csv with header",
			csv: `entity_id,score,rank
user:1,0.95,1
user:2,0.85,2
user:3,0.75,3`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				HasHeader:       true,
			},
			wantRows:    3,
			wantSuccess: 3,
		},
		{
			name: "csv without header using indices",
			csv: `user:1,0.95,1
user:2,0.85,2`,
			config: ImportConfig{
				EntityKeyColumn: "0", // First column
				HasHeader:       false,
			},
			wantRows:    2,
			wantSuccess: 2,
		},
		{
			name: "csv with timestamp column",
			csv: `entity_id,score,timestamp
user:1,0.95,2024-01-01T00:00:00Z
user:2,0.85,2024-01-02T00:00:00Z`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				TimestampColumn: "timestamp",
				HasHeader:       true,
			},
			wantRows:    2,
			wantSuccess: 2,
		},
		{
			name: "csv with column mapping",
			csv: `id,value
user:1,100`,
			config: ImportConfig{
				EntityKeyColumn: "id",
				HasHeader:       true,
				FeatureColumns: map[string]string{
					"value": "score",
				},
			},
			wantRows:    1,
			wantSuccess: 1,
		},
		{
			name: "empty csv with header",
			csv:  `entity_id,score`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				HasHeader:       true,
			},
			wantRows:    0,
			wantSuccess: 0,
		},
		{
			name: "csv with empty entity key and skip errors",
			csv: `entity_id,score
user:1,0.95
,0.85
user:3,0.75`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				HasHeader:       true,
				SkipErrors:      true,
			},
			wantRows:    3,
			wantSuccess: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.csv)
			result, err := b.ImportCSVReader(context.Background(), reader, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.RowsProcessed != tt.wantRows {
				t.Errorf("RowsProcessed = %d, want %d", result.RowsProcessed, tt.wantRows)
			}
			if result.RowsSuccess != tt.wantSuccess {
				t.Errorf("RowsSuccess = %d, want %d", result.RowsSuccess, tt.wantSuccess)
			}
		})
	}
}

func TestBatchImporter_ImportCSVReader_MissingEntityColumn(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `score,rank
0.95,1
0.85,2`

	config := ImportConfig{
		EntityKeyColumn: "entity_id", // Doesn't exist in header
		HasHeader:       true,
	}

	_, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err == nil {
		t.Error("expected error for missing entity key column")
	}
}

func TestBatchImporter_ImportCSVReader_ContextCancellation(t *testing.T) {
	b := newTestBatchImporter(t)

	// Large CSV that will take time to process
	var lines []string
	lines = append(lines, "entity_id,score")
	for i := 0; i < 1000; i++ {
		lines = append(lines, "user:"+string(rune('0'+i%10))+",0.5")
	}
	csv := strings.Join(lines, "\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	_, err := b.ImportCSVReader(ctx, strings.NewReader(csv), config)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestBatchImporter_ImportJSONReader(t *testing.T) {
	b := newTestBatchImporter(t)

	tests := []struct {
		name        string
		json        string
		config      ImportConfig
		wantRows    int64
		wantSuccess int64
		wantErr     bool
	}{
		{
			name: "basic json array",
			json: `[
				{"entity_id": "user:1", "score": 0.95, "rank": 1},
				{"entity_id": "user:2", "score": 0.85, "rank": 2}
			]`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
			},
			wantRows:    2,
			wantSuccess: 2,
		},
		{
			name: "json with timestamp",
			json: `[
				{"entity_id": "user:1", "score": 0.95, "timestamp": "2024-01-01T00:00:00Z"},
				{"entity_id": "user:2", "score": 0.85, "timestamp": 1704067200000000000}
			]`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				TimestampColumn: "timestamp",
			},
			wantRows:    2,
			wantSuccess: 2,
		},
		{
			name: "empty array",
			json: `[]`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
			},
			wantRows:    0,
			wantSuccess: 0,
		},
		{
			name: "json with missing entity key and skip errors",
			json: `[
				{"entity_id": "user:1", "score": 0.95},
				{"score": 0.85},
				{"entity_id": "user:3", "score": 0.75}
			]`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				SkipErrors:      true,
			},
			wantRows:    3,
			wantSuccess: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.json)
			result, err := b.ImportJSONReader(context.Background(), reader, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.RowsProcessed != tt.wantRows {
				t.Errorf("RowsProcessed = %d, want %d", result.RowsProcessed, tt.wantRows)
			}
			if result.RowsSuccess != tt.wantSuccess {
				t.Errorf("RowsSuccess = %d, want %d", result.RowsSuccess, tt.wantSuccess)
			}
		})
	}
}

func TestBatchImporter_ImportJSONReader_InvalidJSON(t *testing.T) {
	b := newTestBatchImporter(t)

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
	}

	_, err := b.ImportJSONReader(context.Background(), strings.NewReader("not json"), config)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBatchImporter_ImportJSONLReader(t *testing.T) {
	b := newTestBatchImporter(t)

	tests := []struct {
		name        string
		jsonl       string
		config      ImportConfig
		wantRows    int64
		wantSuccess int64
		wantErr     bool
	}{
		{
			name: "basic jsonl",
			jsonl: `{"entity_id": "user:1", "score": 0.95}
{"entity_id": "user:2", "score": 0.85}
{"entity_id": "user:3", "score": 0.75}`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
			},
			wantRows:    3,
			wantSuccess: 3,
		},
		{
			name: "jsonl with empty lines",
			jsonl: `{"entity_id": "user:1", "score": 0.95}

{"entity_id": "user:2", "score": 0.85}`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
			},
			wantRows:    2,
			wantSuccess: 2,
		},
		{
			name: "jsonl with parse error and skip errors",
			jsonl: `{"entity_id": "user:1", "score": 0.95}
invalid json line
{"entity_id": "user:2", "score": 0.85}`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				SkipErrors:      true,
			},
			wantRows:    3,
			wantSuccess: 2,
		},
		{
			name:  "jsonl with timestamp",
			jsonl: `{"entity_id": "user:1", "score": 0.95, "ts": "2024-01-01T00:00:00Z"}`,
			config: ImportConfig{
				EntityKeyColumn: "entity_id",
				TimestampColumn: "ts",
			},
			wantRows:    1,
			wantSuccess: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.jsonl)
			result, err := b.ImportJSONLReader(context.Background(), reader, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.RowsProcessed != tt.wantRows {
				t.Errorf("RowsProcessed = %d, want %d", result.RowsProcessed, tt.wantRows)
			}
			if result.RowsSuccess != tt.wantSuccess {
				t.Errorf("RowsSuccess = %d, want %d", result.RowsSuccess, tt.wantSuccess)
			}
		})
	}
}

func TestBatchImporter_Metrics(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `entity_id,score
user:1,0.95
user:2,0.85`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	_, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	metrics := b.Metrics()

	if metrics.FilesProcessed != 1 {
		t.Errorf("FilesProcessed = %d, want 1", metrics.FilesProcessed)
	}
	if metrics.RowsProcessed != 2 {
		t.Errorf("RowsProcessed = %d, want 2", metrics.RowsProcessed)
	}
	if metrics.RowsSuccess != 2 {
		t.Errorf("RowsSuccess = %d, want 2", metrics.RowsSuccess)
	}
	if metrics.FeaturesImported != 2 {
		t.Errorf("FeaturesImported = %d, want 2", metrics.FeaturesImported)
	}
}

func TestBatchImporter_ImportResult_Duration(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `entity_id,score
user:1,0.95`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
	}

	result, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Duration <= 0 {
		t.Error("expected Duration to be positive")
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		{"integer", "42", int64(42)},
		{"negative integer", "-10", int64(-10)},
		{"float", "3.14", float64(3.14)},
		{"bool true", "true", true},
		{"bool false", "false", false},
		{"string", "hello", "hello"},
		{"vector", "[1.0, 2.0, 3.0]", []float32{1.0, 2.0, 3.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseValue(tt.input)

			// Handle special case for slices
			switch want := tt.want.(type) {
			case []float32:
				gotSlice, ok := got.([]float32)
				if !ok {
					t.Errorf("parseValue(%q) = %T, want []float32", tt.input, got)
					return
				}
				if len(gotSlice) != len(want) {
					t.Errorf("parseValue(%q) = %v, want %v", tt.input, got, tt.want)
				}
			default:
				if got != tt.want {
					t.Errorf("parseValue(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
				}
			}
		})
	}
}

func TestImportConfig_Defaults(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `entity_id,score
user:1,0.95`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
		// BatchSize and TimestampFormat should use defaults
	}

	result, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RowsSuccess != 1 {
		t.Errorf("RowsSuccess = %d, want 1", result.RowsSuccess)
	}
}

func TestBatchImporter_FeatureColumns_Mapping(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `id,val,count
user:1,100,5`

	config := ImportConfig{
		EntityKeyColumn: "id",
		HasHeader:       true,
		FeatureColumns: map[string]string{
			"val":   "price",
			"count": "purchase_count",
		},
	}

	result, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 features mapped: price and purchase_count
	if result.FeaturesImported != 2 {
		t.Errorf("FeaturesImported = %d, want 2", result.FeaturesImported)
	}
}

func TestBatchImporter_TimestampParsing(t *testing.T) {
	b := newTestBatchImporter(t)

	tests := []struct {
		name   string
		csv    string
		format string
	}{
		{
			name: "RFC3339 format",
			csv: `entity_id,score,ts
user:1,0.95,2024-01-15T10:30:00Z`,
			format: time.RFC3339,
		},
		{
			name: "unix nano format",
			csv: `entity_id,score,ts
user:1,0.95,1704067200000000000`,
			format: "", // Will try parsing as int64
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ImportConfig{
				EntityKeyColumn: "entity_id",
				TimestampColumn: "ts",
				TimestampFormat: tt.format,
				HasHeader:       true,
			}

			result, err := b.ImportCSVReader(context.Background(), strings.NewReader(tt.csv), config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.RowsSuccess != 1 {
				t.Errorf("RowsSuccess = %d, want 1", result.RowsSuccess)
			}
		})
	}
}
