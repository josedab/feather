package aggregation

import (
	"errors"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

func TestEngine_RegisterAndCompute(t *testing.T) {
	engine := NewEngine()

	// Register an aggregation
	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("click_count", spec)

	// Verify spec is registered
	if engine.GetSpec("click_count") == nil {
		t.Error("Expected spec to be registered")
	}

	if engine.GetSpec("nonexistent") != nil {
		t.Error("Expected nil for nonexistent spec")
	}
}

func TestEngine_Count(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("clicks", spec)

	// Add some values
	now := time.Now()
	for i := 0; i < 10; i++ {
		engine.Update("user:1", "clicks", 1.0, now.Add(-time.Duration(i)*time.Minute))
	}

	// Compute count
	count, err := engine.Compute("user:1", "clicks", domain.AggCount)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if count != 10 {
		t.Errorf("Expected count=10, got %f", count)
	}
}

func TestEngine_Sum(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggSum,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("purchases", spec)

	// Add some values
	now := time.Now()
	engine.Update("user:1", "purchases", 10.0, now)
	engine.Update("user:1", "purchases", 20.0, now)
	engine.Update("user:1", "purchases", 30.0, now)

	sum, err := engine.Compute("user:1", "purchases", domain.AggSum)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if sum != 60 {
		t.Errorf("Expected sum=60, got %f", sum)
	}
}

func TestEngine_Avg(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggAvg,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("rating", spec)

	// Add some values
	now := time.Now()
	engine.Update("user:1", "rating", 4.0, now)
	engine.Update("user:1", "rating", 5.0, now)
	engine.Update("user:1", "rating", 3.0, now)

	avg, err := engine.Compute("user:1", "rating", domain.AggAvg)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if avg != 4 {
		t.Errorf("Expected avg=4, got %f", avg)
	}
}

func TestEngine_MinMax(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggMin,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("price", spec)

	now := time.Now()
	engine.Update("user:1", "price", 100.0, now)
	engine.Update("user:1", "price", 50.0, now)
	engine.Update("user:1", "price", 200.0, now)

	minValue, err := engine.Compute("user:1", "price", domain.AggMin)
	if err != nil {
		t.Fatalf("Compute min failed: %v", err)
	}

	if minValue != 50 {
		t.Errorf("Expected min=50, got %f", minValue)
	}

	maxValue, err := engine.Compute("user:1", "price", domain.AggMax)
	if err != nil {
		t.Fatalf("Compute max failed: %v", err)
	}

	if maxValue != 200 {
		t.Errorf("Expected max=200, got %f", maxValue)
	}
}

func TestEngine_Last(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggLast,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("position", spec)

	now := time.Now()
	engine.Update("user:1", "position", 1.0, now.Add(-2*time.Minute))
	engine.Update("user:1", "position", 2.0, now.Add(-1*time.Minute))
	engine.Update("user:1", "position", 3.0, now)

	last, err := engine.Compute("user:1", "position", domain.AggLast)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if last != 3 {
		t.Errorf("Expected last=3, got %f", last)
	}
}

func TestEngine_WindowExpiry(t *testing.T) {
	engine := NewEngine()

	// Use a 5-minute window to avoid bucket boundary issues.
	// With 1-minute buckets, this gives us enough margin for timing variations.
	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   5 * time.Minute,
	}
	engine.RegisterAggregation("events", spec)

	now := time.Now()

	// Add old events (well outside the 5-minute window)
	engine.Update("user:1", "events", 1.0, now.Add(-10*time.Minute))

	// Add recent events (safely inside the 5-minute window)
	// Use -1 minute to ensure they're in a bucket that won't expire during the test
	engine.Update("user:1", "events", 1.0, now.Add(-1*time.Minute))
	engine.Update("user:1", "events", 1.0, now)

	count, err := engine.Compute("user:1", "events", domain.AggCount)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	// Only the 2 recent events should be counted
	if count != 2 {
		t.Errorf("Expected count=2, got %f (old events should have expired)", count)
	}
}

func TestEngine_MultipleEntities(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggSum,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("score", spec)

	now := time.Now()
	engine.Update("user:1", "score", 10.0, now)
	engine.Update("user:2", "score", 20.0, now)
	engine.Update("user:1", "score", 5.0, now)

	sum1, _ := engine.Compute("user:1", "score", domain.AggSum)
	sum2, _ := engine.Compute("user:2", "score", domain.AggSum)

	if sum1 != 15 {
		t.Errorf("Expected user:1 sum=15, got %f", sum1)
	}

	if sum2 != 20 {
		t.Errorf("Expected user:2 sum=20, got %f", sum2)
	}
}

func TestEngine_ComputeWithSpec(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggSum,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("total", spec)

	now := time.Now()
	engine.Update("user:1", "total", 10.0, now)
	engine.Update("user:1", "total", 20.0, now)

	// ComputeWithSpec should use the spec's function
	result, err := engine.ComputeWithSpec("user:1", "total")
	if err != nil {
		t.Fatalf("ComputeWithSpec failed: %v", err)
	}

	if result != 30 {
		t.Errorf("Expected result=30, got %f", result)
	}
}

func TestEngine_EntityNotFound(t *testing.T) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("feature", spec)

	_, err := engine.Compute("nonexistent", "feature", domain.AggCount)
	if !errors.Is(err, domain.ErrEntityNotFound) {
		t.Errorf("Expected ErrEntityNotFound, got %v", err)
	}
}

func TestRingBuffer_Basic(t *testing.T) {
	rb := NewRingBuffer(5)

	// Push some buckets
	for i := 0; i < 3; i++ {
		rb.Push(AggregationBucket{
			StartTime: int64(i),
			Count:     int64(i + 1),
			Sum:       float64(i + 1),
		})
	}

	if rb.Size() != 3 {
		t.Errorf("Expected size=3, got %d", rb.Size())
	}

	// Get oldest
	oldest := rb.Get(0)
	if oldest.StartTime != 0 {
		t.Errorf("Expected oldest StartTime=0, got %d", oldest.StartTime)
	}

	// Get latest
	latest := rb.GetLatest()
	if latest.StartTime != 2 {
		t.Errorf("Expected latest StartTime=2, got %d", latest.StartTime)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push more than capacity
	for i := 0; i < 5; i++ {
		rb.Push(AggregationBucket{
			StartTime: int64(i),
		})
	}

	if rb.Size() != 3 {
		t.Errorf("Expected size=3, got %d", rb.Size())
	}

	// Oldest should be bucket 2 (0 and 1 were evicted)
	oldest := rb.Get(0)
	if oldest.StartTime != 2 {
		t.Errorf("Expected oldest StartTime=2, got %d", oldest.StartTime)
	}

	// Latest should be bucket 4
	latest := rb.GetLatest()
	if latest.StartTime != 4 {
		t.Errorf("Expected latest StartTime=4, got %d", latest.StartTime)
	}
}

func TestRingBuffer_Aggregate(t *testing.T) {
	rb := NewRingBuffer(5)

	rb.Push(AggregationBucket{Count: 10, Sum: 100, Min: 5, Max: 15})
	rb.Push(AggregationBucket{Count: 20, Sum: 200, Min: 3, Max: 25})
	rb.Push(AggregationBucket{Count: 30, Sum: 300, Min: 8, Max: 35})

	agg := rb.Aggregate()

	if agg.Count != 60 {
		t.Errorf("Expected count=60, got %d", agg.Count)
	}

	if agg.Sum != 600 {
		t.Errorf("Expected sum=600, got %f", agg.Sum)
	}

	if agg.Min != 3 {
		t.Errorf("Expected min=3, got %f", agg.Min)
	}

	if agg.Max != 35 {
		t.Errorf("Expected max=35, got %f", agg.Max)
	}
}

func BenchmarkEngine_Update(b *testing.B) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("bench", spec)

	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Update("user:bench", "bench", 1.0, now)
	}
}

func BenchmarkEngine_Compute(b *testing.B) {
	engine := NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggSum,
		Window:   time.Hour,
	}
	engine.RegisterAggregation("bench", spec)

	now := time.Now()
	for i := 0; i < 100; i++ {
		engine.Update("user:bench", "bench", float64(i), now)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Compute("user:bench", "bench", domain.AggSum)
	}
}
