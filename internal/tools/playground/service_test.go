package playground

import (
	"testing"
)

func TestService_ComputeSummary(t *testing.T) {
	svc := NewService(nil)

	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	summary := svc.ComputeSummary("clicks", "user_engagement", "float64", values)

	if summary.Count != 10 {
		t.Fatalf("expected 10, got %d", summary.Count)
	}
	if summary.Min != 1 {
		t.Fatalf("expected min 1, got %f", summary.Min)
	}
	if summary.Max != 10 {
		t.Fatalf("expected max 10, got %f", summary.Max)
	}
	if summary.Mean != 5.5 {
		t.Fatalf("expected mean 5.5, got %f", summary.Mean)
	}
	if summary.StdDev < 2.8 || summary.StdDev > 2.9 {
		t.Fatalf("expected stddev ~2.87, got %f", summary.StdDev)
	}
	if len(summary.Histogram) != 10 {
		t.Fatalf("expected 10 histogram bins, got %d", len(summary.Histogram))
	}
}

func TestService_ComputeSummaryEmpty(t *testing.T) {
	svc := NewService(nil)
	summary := svc.ComputeSummary("empty", "", "", nil)
	if summary.Count != 0 {
		t.Fatal("expected 0 count")
	}
}

func TestService_SaveAndGetQuery(t *testing.T) {
	svc := NewService(nil)

	q := &SavedQuery{
		Name:     "my query",
		Entities: []string{"user:1"},
		Features: []string{"clicks"},
	}

	if err := svc.SaveQuery(q); err != nil {
		t.Fatal(err)
	}
	if q.ID == "" {
		t.Fatal("expected ID to be set")
	}

	got, err := svc.GetQuery(q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "my query" {
		t.Fatalf("expected 'my query', got %s", got.Name)
	}

	list := svc.ListQueries()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	if err := svc.DeleteQuery(q.ID); err != nil {
		t.Fatal(err)
	}

	_, err = svc.GetQuery(q.ID)
	if err != ErrQueryNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestService_InvalidQuery(t *testing.T) {
	svc := NewService(nil)

	err := svc.SaveQuery(&SavedQuery{})
	if err != ErrInvalidQuery {
		t.Fatalf("expected ErrInvalidQuery, got %v", err)
	}
}

func TestService_CreateDataset(t *testing.T) {
	svc := NewService(nil)

	cfg := &DatasetConfig{
		ID:       "ds-1",
		Name:     "training",
		Entities: []string{"user:1", "user:2"},
		Features: []string{"clicks"},
		Format:   "csv",
	}

	ds, err := svc.CreateDataset(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Status != "pending" {
		t.Fatalf("expected pending, got %s", ds.Status)
	}

	// Check duplicate
	_, err = svc.CreateDataset(cfg)
	if err != ErrDatasetExists {
		t.Fatalf("expected ErrDatasetExists, got %v", err)
	}

	list := svc.ListDatasets()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
}

func TestHistogram(t *testing.T) {
	data := []float64{1, 1, 2, 3, 5, 8}
	h := computeHistogram(data, 4)
	if len(h) != 4 {
		t.Fatalf("expected 4 bins, got %d", len(h))
	}

	var total int64
	for _, b := range h {
		total += b.Count
	}
	if total != 6 {
		t.Fatalf("expected 6 total, got %d", total)
	}
}
