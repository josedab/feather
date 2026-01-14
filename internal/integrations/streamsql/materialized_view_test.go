package streamsql

import (
	"context"
	"testing"
	"time"
)

func TestMaterializedView_Create(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("events", map[string]string{"user_id": "string", "value": "int"})

	err := engine.CreateMaterializedView("test_view", "SELECT * FROM events", RefreshOnDemand, 0)
	if err != nil {
		t.Fatalf("CreateMaterializedView failed: %v", err)
	}

	mv, err := engine.GetView("test_view")
	if err != nil {
		t.Fatalf("GetView failed: %v", err)
	}
	if mv.Name != "test_view" {
		t.Errorf("expected name 'test_view', got %q", mv.Name)
	}
	if mv.Status != ViewStatusActive {
		t.Errorf("expected status active, got %q", mv.Status)
	}
	if mv.RefreshMode != RefreshOnDemand {
		t.Errorf("expected refresh mode on_demand, got %q", mv.RefreshMode)
	}

	// Duplicate should fail
	err = engine.CreateMaterializedView("test_view", "SELECT * FROM events", RefreshOnDemand, 0)
	if err == nil {
		t.Error("expected error for duplicate view")
	}

	// Stats should reflect view count
	stats := engine.Stats()
	if stats.ViewCount != 1 {
		t.Errorf("expected ViewCount 1, got %d", stats.ViewCount)
	}

	// ListViews should include the view
	views := engine.ListViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].Name != "test_view" {
		t.Errorf("expected view name 'test_view', got %q", views[0].Name)
	}
}

func TestMaterializedView_Refresh(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("sales", map[string]string{"category": "string", "amount": "int"})

	now := time.Now()
	records := []struct {
		cat string
		amt int
	}{
		{"A", 10},
		{"B", 20},
		{"A", 30},
	}
	for i, r := range records {
		rec := &Record{
			Fields:    map[string]interface{}{"category": r.cat, "amount": r.amt},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := engine.Push("sales", rec); err != nil {
			t.Fatal(err)
		}
	}

	err := engine.CreateMaterializedView("sales_view", "SELECT * FROM sales", RefreshOnDemand, 0)
	if err != nil {
		t.Fatalf("CreateMaterializedView failed: %v", err)
	}

	// Refresh the view
	if err := engine.RefreshView("sales_view"); err != nil {
		t.Fatalf("RefreshView failed: %v", err)
	}

	mv, _ := engine.GetView("sales_view")
	results := mv.GetResults()
	if results == nil {
		t.Fatal("expected non-nil results after refresh")
	}
	if results.Count != 3 {
		t.Errorf("expected 3 rows, got %d", results.Count)
	}

	// Verify refresh count
	mv.mu.RLock()
	refreshCount := mv.RefreshCount
	mv.mu.RUnlock()
	if refreshCount < 1 {
		t.Errorf("expected refresh count >= 1, got %d", refreshCount)
	}

	// Refresh non-existent view should fail
	if err := engine.RefreshView("nonexistent"); err == nil {
		t.Error("expected error for refreshing non-existent view")
	}
}

func TestMaterializedView_PauseResume(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("events", map[string]string{"val": "int"})

	err := engine.CreateMaterializedView("pv", "SELECT * FROM events", RefreshOnDemand, 0)
	if err != nil {
		t.Fatal(err)
	}

	mv, _ := engine.GetView("pv")

	// Pause
	if err := mv.Pause(); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if mv.Status != ViewStatusPaused {
		t.Errorf("expected paused, got %s", mv.Status)
	}

	// Pause again should fail
	if err := mv.Pause(); err == nil {
		t.Error("expected error pausing already-paused view")
	}

	// Refresh while paused should fail
	records := []*Record{
		{Fields: map[string]interface{}{"val": 1}, Timestamp: time.Now()},
	}
	if err := mv.Refresh(records); err == nil {
		t.Error("expected error refreshing paused view")
	}

	// Resume
	if err := mv.Resume(); err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if mv.Status != ViewStatusActive {
		t.Errorf("expected active, got %s", mv.Status)
	}

	// Resume again should fail
	if err := mv.Resume(); err == nil {
		t.Error("expected error resuming already-active view")
	}
}

func TestMaterializedView_Drop(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("events", map[string]string{"val": "int"})

	err := engine.CreateMaterializedView("drop_me", "SELECT * FROM events", RefreshOnDemand, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.DropMaterializedView("drop_me"); err != nil {
		t.Fatalf("DropMaterializedView failed: %v", err)
	}

	_, err = engine.GetView("drop_me")
	if err == nil {
		t.Error("expected error for getting dropped view")
	}

	stats := engine.Stats()
	if stats.ViewCount != 0 {
		t.Errorf("expected ViewCount 0 after drop, got %d", stats.ViewCount)
	}

	// Drop non-existent should fail
	if err := engine.DropMaterializedView("nonexistent"); err == nil {
		t.Error("expected error for dropping non-existent view")
	}
}

func TestMaterializedView_OnInsertRefresh(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("clicks", map[string]string{"user_id": "string", "count": "int"})

	err := engine.CreateMaterializedView("click_view", "SELECT * FROM clicks", RefreshOnInsert, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Push data - should trigger auto-refresh
	now := time.Now()
	for i := 0; i < 3; i++ {
		rec := &Record{
			Fields:    map[string]interface{}{"user_id": "u1", "count": i + 1},
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := engine.Push("clicks", rec); err != nil {
			t.Fatal(err)
		}
	}

	mv, _ := engine.GetView("click_view")
	results := mv.GetResults()
	if results == nil {
		t.Fatal("expected non-nil results after on_insert refresh")
	}
	if results.Count != 3 {
		t.Errorf("expected 3 rows after push, got %d", results.Count)
	}

	// PushBatch should also trigger refresh
	batchRecords := []*Record{
		{Fields: map[string]interface{}{"user_id": "u2", "count": 10}, Timestamp: now.Add(10 * time.Second)},
		{Fields: map[string]interface{}{"user_id": "u3", "count": 20}, Timestamp: now.Add(11 * time.Second)},
	}
	if _, err := engine.PushBatch("clicks", batchRecords); err != nil {
		t.Fatal(err)
	}

	results = mv.GetResults()
	if results.Count != 5 {
		t.Errorf("expected 5 rows after batch push, got %d", results.Count)
	}
}

func TestMaterializedView_ListViews(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("events", map[string]string{"val": "int"})

	_ = engine.CreateMaterializedView("v1", "SELECT * FROM events", RefreshOnDemand, 0)
	_ = engine.CreateMaterializedView("v2", "SELECT * FROM events", RefreshPeriodic, 5*time.Minute)

	views := engine.ListViews()
	if len(views) != 2 {
		t.Fatalf("expected 2 views, got %d", len(views))
	}

	// Verify periodic view has interval set
	for _, v := range views {
		if v.Name == "v2" {
			if v.Interval == "" {
				t.Error("expected non-empty interval for periodic view")
			}
		}
	}
}

func TestMaterializedView_StddevParsing(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	defer engine.Close()

	_ = engine.CreateStream("metrics", map[string]string{"value": "float"})

	ctx := context.Background()
	// Verify STDDEV is recognized as an aggregate function by the parser
	_, err := engine.ExecuteQuery(ctx, "SELECT STDDEV(value) FROM metrics")
	if err != nil {
		t.Fatalf("STDDEV query failed to parse: %v", err)
	}
}
