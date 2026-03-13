package aggregation

import "math"

//revive:disable:exported

// AggregationBucket stores pre-aggregated values for a time bucket.
type AggregationBucket struct {
	StartTime int64   // Unix nanoseconds
	Count     int64   // Number of values
	Sum       float64 // Sum of values
	Min       float64 // Minimum value
	Max       float64 // Maximum value
	LastValue float64 // Last value seen
}

// RingBuffer provides efficient sliding window storage.
type RingBuffer struct {
	buckets  []AggregationBucket
	head     int
	size     int
	capacity int
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buckets:  make([]AggregationBucket, capacity),
		head:     0,
		size:     0,
		capacity: capacity,
	}
}

// Push adds a new bucket to the ring buffer.
func (r *RingBuffer) Push(bucket AggregationBucket) {
	r.buckets[r.head] = bucket
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

// Get returns the bucket at the given index (0 = oldest).
// The oldest bucket starts at (head - size) in the circular array.
// Adding capacity before modulo ensures correct wrapping for negative values.
func (r *RingBuffer) Get(index int) *AggregationBucket {
	if index < 0 || index >= r.size {
		return nil
	}
	// Calculate actual index in circular buffer
	actualIdx := (r.head - r.size + index + r.capacity) % r.capacity
	return &r.buckets[actualIdx]
}

// GetLatest returns the most recent bucket.
// head points to the next write position, so the latest bucket is at head-1.
// Modular arithmetic with +capacity handles the wrap-around when head is 0.
func (r *RingBuffer) GetLatest() *AggregationBucket {
	if r.size == 0 {
		return nil
	}
	latestIdx := (r.head - 1 + r.capacity) % r.capacity
	return &r.buckets[latestIdx]
}

// Size returns the current number of buckets.
func (r *RingBuffer) Size() int {
	return r.size
}

// Capacity returns the maximum capacity.
func (r *RingBuffer) Capacity() int {
	return r.capacity
}

// Clear removes all buckets.
func (r *RingBuffer) Clear() {
	r.head = 0
	r.size = 0
}

// Range iterates over buckets from oldest to newest.
func (r *RingBuffer) Range(fn func(bucket *AggregationBucket) bool) {
	for i := 0; i < r.size; i++ {
		bucket := r.Get(i)
		if !fn(bucket) {
			return
		}
	}
}

// PopOldest removes the oldest bucket.
func (r *RingBuffer) PopOldest() {
	if r.size == 0 {
		return
	}
	// Zero out the oldest slot to prevent stale data from being readable
	oldestIdx := (r.head - r.size + r.capacity) % r.capacity
	r.buckets[oldestIdx] = AggregationBucket{}
	r.size--
}

// Aggregate computes aggregate statistics across all buckets.
// Iterates from oldest to newest using Range(), which resolves circular
// indices via Get(). Min/Max are initialized to ±MaxFloat64 sentinels
// and reset to 0 when the buffer is empty.
func (r *RingBuffer) Aggregate() AggregationBucket {
	result := AggregationBucket{
		Min: math.MaxFloat64,
		Max: -math.MaxFloat64,
	}

	if r.size == 0 {
		result.Min = 0
		result.Max = 0
		return result
	}

	r.Range(func(bucket *AggregationBucket) bool {
		result.Count += bucket.Count
		result.Sum += bucket.Sum
		if bucket.Min < result.Min {
			result.Min = bucket.Min
		}
		if bucket.Max > result.Max {
			result.Max = bucket.Max
		}
		result.LastValue = bucket.LastValue
		return true
	})

	// Get start time from oldest bucket
	oldest := r.Get(0)
	if oldest != nil {
		result.StartTime = oldest.StartTime
	}

	return result
}

//revive:enable:exported
