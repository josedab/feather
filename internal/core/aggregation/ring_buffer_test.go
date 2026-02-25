package aggregation

import (
	"math"
	"testing"
)

func TestRingBuffer_PushAndGet(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push(AggregationBucket{StartTime: 1, Count: 10, Sum: 100})
	rb.Push(AggregationBucket{StartTime: 2, Count: 20, Sum: 200})
	rb.Push(AggregationBucket{StartTime: 3, Count: 30, Sum: 300})

	b := rb.Get(0) // oldest
	if b == nil || b.StartTime != 1 {
		t.Errorf("expected StartTime=1, got %v", b)
	}

	b = rb.Get(2) // newest
	if b == nil || b.StartTime != 3 {
		t.Errorf("expected StartTime=3, got %v", b)
	}
}

func TestRingBuffer_WrapAround(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push(AggregationBucket{StartTime: 1})
	rb.Push(AggregationBucket{StartTime: 2})
	rb.Push(AggregationBucket{StartTime: 3})
	rb.Push(AggregationBucket{StartTime: 4}) // wraps, overwrites bucket 1

	if rb.Size() != 3 {
		t.Errorf("expected size=3, got %d", rb.Size())
	}

	oldest := rb.Get(0)
	if oldest == nil || oldest.StartTime != 2 {
		t.Errorf("expected oldest StartTime=2, got %v", oldest)
	}

	newest := rb.GetLatest()
	if newest == nil || newest.StartTime != 4 {
		t.Errorf("expected newest StartTime=4, got %v", newest)
	}
}

func TestRingBuffer_GetOutOfBounds(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(AggregationBucket{StartTime: 1})

	if rb.Get(-1) != nil {
		t.Error("expected nil for negative index")
	}
	if rb.Get(1) != nil {
		t.Error("expected nil for index >= size")
	}
}

func TestRingBuffer_GetLatest_Empty(t *testing.T) {
	rb := NewRingBuffer(3)
	if rb.GetLatest() != nil {
		t.Error("expected nil for empty buffer")
	}
}

func TestRingBuffer_Capacity(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Capacity() != 5 {
		t.Errorf("expected capacity=5, got %d", rb.Capacity())
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(AggregationBucket{StartTime: 1})
	rb.Push(AggregationBucket{StartTime: 2})

	rb.Clear()

	if rb.Size() != 0 {
		t.Errorf("expected size=0 after clear, got %d", rb.Size())
	}
	if rb.GetLatest() != nil {
		t.Error("expected nil GetLatest after clear")
	}
}

func TestRingBuffer_PopOldest_Empty(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.PopOldest() // should not panic
	if rb.Size() != 0 {
		t.Error("expected size=0")
	}
}

func TestRingBuffer_PopOldest(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(AggregationBucket{StartTime: 1})
	rb.Push(AggregationBucket{StartTime: 2})
	rb.Push(AggregationBucket{StartTime: 3})

	rb.PopOldest()

	if rb.Size() != 2 {
		t.Errorf("expected size=2, got %d", rb.Size())
	}

	oldest := rb.Get(0)
	if oldest == nil || oldest.StartTime != 2 {
		t.Errorf("expected oldest StartTime=2, got %v", oldest)
	}
}

func TestRingBuffer_Aggregate_Empty(t *testing.T) {
	rb := NewRingBuffer(3)
	agg := rb.Aggregate()

	if agg.Count != 0 || agg.Sum != 0 || agg.Min != 0 || agg.Max != 0 {
		t.Errorf("expected all zeros for empty aggregate, got count=%d sum=%.1f min=%.1f max=%.1f",
			agg.Count, agg.Sum, agg.Min, agg.Max)
	}
}

func TestRingBuffer_AggregateMultiple(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push(AggregationBucket{StartTime: 100, Count: 1, Sum: 10, Min: 5, Max: 15, LastValue: 10})
	rb.Push(AggregationBucket{StartTime: 200, Count: 2, Sum: 20, Min: 3, Max: 25, LastValue: 20})
	rb.Push(AggregationBucket{StartTime: 300, Count: 3, Sum: 30, Min: 7, Max: 10, LastValue: 30})

	agg := rb.Aggregate()

	if agg.Count != 6 {
		t.Errorf("expected count=6, got %d", agg.Count)
	}
	if agg.Sum != 60 {
		t.Errorf("expected sum=60, got %.1f", agg.Sum)
	}
	if agg.Min != 3 {
		t.Errorf("expected min=3, got %.1f", agg.Min)
	}
	if agg.Max != 25 {
		t.Errorf("expected max=25, got %.1f", agg.Max)
	}
	if agg.LastValue != 30 {
		t.Errorf("expected lastValue=30, got %.1f", agg.LastValue)
	}
	if agg.StartTime != 100 {
		t.Errorf("expected startTime=100, got %d", agg.StartTime)
	}
}

func TestRingBuffer_Aggregate_WrapAround(t *testing.T) {
	rb := NewRingBuffer(2)
	rb.Push(AggregationBucket{Count: 1, Sum: 10, Min: 10, Max: 10})
	rb.Push(AggregationBucket{Count: 2, Sum: 20, Min: 5, Max: 20})
	rb.Push(AggregationBucket{Count: 3, Sum: 30, Min: 1, Max: 30}) // wraps

	agg := rb.Aggregate()

	// Should aggregate only the last 2 buckets
	if agg.Count != 5 { // 2+3
		t.Errorf("expected count=5, got %d", agg.Count)
	}
	if agg.Sum != 50 { // 20+30
		t.Errorf("expected sum=50, got %.1f", agg.Sum)
	}
	if agg.Min != 1 {
		t.Errorf("expected min=1, got %.1f", agg.Min)
	}
	if agg.Max != 30 {
		t.Errorf("expected max=30, got %.1f", agg.Max)
	}
}

func TestRingBuffer_Range(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(AggregationBucket{StartTime: 1})
	rb.Push(AggregationBucket{StartTime: 2})
	rb.Push(AggregationBucket{StartTime: 3})

	var visited []int64
	rb.Range(func(b *AggregationBucket) bool {
		visited = append(visited, b.StartTime)
		return true
	})

	if len(visited) != 3 {
		t.Fatalf("expected 3, got %d", len(visited))
	}
	if visited[0] != 1 || visited[1] != 2 || visited[2] != 3 {
		t.Errorf("expected [1,2,3], got %v", visited)
	}
}

func TestRingBuffer_Range_EarlyTermination(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 5; i++ {
		rb.Push(AggregationBucket{StartTime: int64(i)})
	}

	count := 0
	rb.Range(func(b *AggregationBucket) bool {
		count++
		return count < 2 // stop after 2
	})

	if count != 2 {
		t.Errorf("expected 2 iterations, got %d", count)
	}
}

func TestRingBuffer_Aggregate_NegativeValues(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(AggregationBucket{Count: 1, Sum: -10, Min: -20, Max: -5})
	rb.Push(AggregationBucket{Count: 1, Sum: -5, Min: -15, Max: -1})

	agg := rb.Aggregate()

	if agg.Min != -20 {
		t.Errorf("expected min=-20, got %.1f", agg.Min)
	}
	if agg.Max != -1 {
		t.Errorf("expected max=-1, got %.1f", agg.Max)
	}
	if math.Abs(agg.Sum-(-15)) > 0.001 {
		t.Errorf("expected sum=-15, got %.1f", agg.Sum)
	}
}
