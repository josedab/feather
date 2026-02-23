package arrowflight

import (
	"context"
	"testing"
)

func TestNewBatchServer(t *testing.T) {
	srv := NewServer(DefaultConfig())
	bs := NewBatchServer(srv, DefaultBatchConfig())
	if bs == nil {
		t.Fatal("NewBatchServer returned nil")
	}
	if bs.config.MaxBatchSize != 10000 {
		t.Errorf("MaxBatchSize = %d, want 10000", bs.config.MaxBatchSize)
	}
}

func TestNewBatchServer_DefaultsForInvalid(t *testing.T) {
	srv := NewServer(DefaultConfig())
	bs := NewBatchServer(srv, BatchConfig{MaxBatchSize: -1, MaxConcurrency: 0})
	if bs.config.MaxBatchSize != DefaultBatchConfig().MaxBatchSize {
		t.Errorf("MaxBatchSize = %d, want default", bs.config.MaxBatchSize)
	}
	if bs.config.MaxConcurrency != DefaultBatchConfig().MaxConcurrency {
		t.Errorf("MaxConcurrency = %d, want default", bs.config.MaxConcurrency)
	}
}

func TestBatchServer_ServeBatch(t *testing.T) {
	srv := NewServer(DefaultConfig())
	srv.SetReader(&mockReader{})

	bs := NewBatchServer(srv, DefaultBatchConfig())
	batch, err := bs.ServeBatch(context.Background(), []string{"user:1", "user:2"}, []string{"clicks", "views"})
	if err != nil {
		t.Fatalf("ServeBatch: %v", err)
	}
	if batch == nil {
		t.Fatal("expected non-nil batch")
	}
	if batch.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", batch.RowCount)
	}
}

func TestBatchServer_ServeBatch_NoEntities(t *testing.T) {
	srv := NewServer(DefaultConfig())
	bs := NewBatchServer(srv, DefaultBatchConfig())
	_, err := bs.ServeBatch(context.Background(), nil, []string{"f1"})
	if err == nil {
		t.Fatal("expected error for empty entities")
	}
}

func TestBatchServer_ServeBatch_NoFeatures(t *testing.T) {
	srv := NewServer(DefaultConfig())
	bs := NewBatchServer(srv, DefaultBatchConfig())
	_, err := bs.ServeBatch(context.Background(), []string{"u1"}, nil)
	if err == nil {
		t.Fatal("expected error for empty features")
	}
}

func TestBatchServer_ServeBatch_WithoutReader(t *testing.T) {
	srv := NewServer(DefaultConfig())
	bs := NewBatchServer(srv, DefaultBatchConfig())

	batch, err := bs.ServeBatch(context.Background(), []string{"u1"}, []string{"f1"})
	if err != nil {
		t.Fatalf("ServeBatch: %v", err)
	}
	if batch == nil {
		t.Fatal("expected non-nil batch")
	}
}

func TestBatchServer_ServeBatch_MultiChunk(t *testing.T) {
	srv := NewServer(DefaultConfig())
	srv.SetReader(&mockReader{})

	cfg := DefaultBatchConfig()
	cfg.MaxBatchSize = 2
	bs := NewBatchServer(srv, cfg)

	entities := []string{"u1", "u2", "u3", "u4", "u5"}
	batch, err := bs.ServeBatch(context.Background(), entities, []string{"score"})
	if err != nil {
		t.Fatalf("ServeBatch: %v", err)
	}
	if batch.RowCount != 5 {
		t.Errorf("RowCount = %d, want 5", batch.RowCount)
	}
}

func TestBatchServer_Stats(t *testing.T) {
	srv := NewServer(DefaultConfig())
	srv.SetReader(&mockReader{})

	cfg := DefaultBatchConfig()
	cfg.EnableStats = true
	bs := NewBatchServer(srv, cfg)

	_, err := bs.ServeBatch(context.Background(), []string{"u1"}, []string{"f1"})
	if err != nil {
		t.Fatalf("ServeBatch: %v", err)
	}

	stats := bs.Stats()
	if stats.TotalBatches != 1 {
		t.Errorf("TotalBatches = %d, want 1", stats.TotalBatches)
	}
	if stats.TotalRows != 1 {
		t.Errorf("TotalRows = %d, want 1", stats.TotalRows)
	}
	if stats.AvgBatchSize != 1.0 {
		t.Errorf("AvgBatchSize = %f, want 1.0", stats.AvgBatchSize)
	}
}

func TestBatchServer_Stats_Disabled(t *testing.T) {
	srv := NewServer(DefaultConfig())
	srv.SetReader(&mockReader{})

	cfg := DefaultBatchConfig()
	cfg.EnableStats = false
	bs := NewBatchServer(srv, cfg)

	_, err := bs.ServeBatch(context.Background(), []string{"u1"}, []string{"f1"})
	if err != nil {
		t.Fatalf("ServeBatch: %v", err)
	}

	stats := bs.Stats()
	if stats.TotalBatches != 0 {
		t.Errorf("TotalBatches = %d, want 0 (stats disabled)", stats.TotalBatches)
	}
}

func TestBatchServer_Stats_MultipleRequests(t *testing.T) {
	srv := NewServer(DefaultConfig())
	srv.SetReader(&mockReader{})

	bs := NewBatchServer(srv, DefaultBatchConfig())
	for i := 0; i < 3; i++ {
		_, err := bs.ServeBatch(context.Background(), []string{"u1", "u2"}, []string{"f1"})
		if err != nil {
			t.Fatalf("ServeBatch %d: %v", i, err)
		}
	}

	stats := bs.Stats()
	if stats.TotalBatches != 3 {
		t.Errorf("TotalBatches = %d, want 3", stats.TotalBatches)
	}
	if stats.TotalRows != 6 {
		t.Errorf("TotalRows = %d, want 6", stats.TotalRows)
	}
}

func TestSplitEntities(t *testing.T) {
	tests := []struct {
		name     string
		entities []string
		maxSize  int
		want     int // number of chunks
	}{
		{"no split needed", []string{"a", "b"}, 10, 1},
		{"exact split", []string{"a", "b", "c", "d"}, 2, 2},
		{"uneven split", []string{"a", "b", "c"}, 2, 2},
		{"single per chunk", []string{"a", "b", "c"}, 1, 3},
		{"zero max returns single", []string{"a"}, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitEntities(tt.entities, tt.maxSize)
			if len(chunks) != tt.want {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.want)
			}
			// Verify all entities are present
			var total int
			for _, c := range chunks {
				total += len(c)
			}
			if total != len(tt.entities) {
				t.Errorf("total entities = %d, want %d", total, len(tt.entities))
			}
		})
	}
}
