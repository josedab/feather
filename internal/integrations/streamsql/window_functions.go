package streamsql

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// WindowFunction represents a streaming window function.
type WindowFunction struct {
	Name     string
	Function string // count, sum, avg, min, max, stddev, percentile
	Field    string
	Window   WindowSpec
}

// WindowSpec defines window parameters for window functions.
type WindowSpec struct {
	Type     string        // tumbling, sliding, session
	Size     time.Duration
	Slide    time.Duration
	GapSize  time.Duration // for session windows
}

// WindowResult holds the output of a window function evaluation.
type WindowResult struct {
	WindowStart time.Time              `json:"window_start"`
	WindowEnd   time.Time              `json:"window_end"`
	GroupKey    string                  `json:"group_key,omitempty"`
	Values     map[string]interface{}  `json:"values"`
	RecordCount int                    `json:"record_count"`
}

// WindowAggregator computes windowed aggregations over streaming records.
type WindowAggregator struct {
	functions []WindowFunction
	groupBy   []string
}

// NewWindowAggregator creates a new window aggregator.
func NewWindowAggregator(functions []WindowFunction, groupBy []string) *WindowAggregator {
	return &WindowAggregator{
		functions: functions,
		groupBy:   groupBy,
	}
}

// Aggregate computes window functions over a set of records.
func (wa *WindowAggregator) Aggregate(records []*Record) ([]WindowResult, error) {
	if len(records) == 0 || len(wa.functions) == 0 {
		return nil, nil
	}

	// Use the first function's window spec for bucketing
	spec := wa.functions[0].Window

	// Assign records to windows
	windows := wa.assignWindows(records, spec)

	var results []WindowResult
	for _, w := range windows {
		// Group within window
		groups := wa.groupRecords(w.records)

		for groupKey, groupRecords := range groups {
			wr := WindowResult{
				WindowStart: w.start,
				WindowEnd:   w.end,
				GroupKey:     groupKey,
				Values:       make(map[string]interface{}),
				RecordCount:  len(groupRecords),
			}

			for _, fn := range wa.functions {
				val, err := wa.computeFunction(fn, groupRecords)
				if err != nil {
					return nil, fmt.Errorf("computing %s(%s): %w", fn.Function, fn.Field, err)
				}
				wr.Values[fn.Name] = val
			}

			results = append(results, wr)
		}
	}

	return results, nil
}

type windowBucketAgg struct {
	start   time.Time
	end     time.Time
	records []*Record
}

func (wa *WindowAggregator) assignWindows(records []*Record, spec WindowSpec) []windowBucketAgg {
	if len(records) == 0 {
		return nil
	}

	// Sort by timestamp
	sorted := make([]*Record, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	minTime := sorted[0].Timestamp
	maxTime := sorted[len(sorted)-1].Timestamp

	switch spec.Type {
	case "session":
		return wa.sessionWindows(sorted, spec.GapSize)
	case "sliding":
		return wa.slidingWindows(sorted, minTime, maxTime, spec.Size, spec.Slide)
	default: // tumbling
		return wa.tumblingWindows(sorted, minTime, maxTime, spec.Size)
	}
}

func (wa *WindowAggregator) tumblingWindows(records []*Record, minTime, maxTime time.Time, size time.Duration) []windowBucketAgg {
	var windows []windowBucketAgg
	start := minTime.Truncate(size)

	for start.Before(maxTime.Add(size)) {
		end := start.Add(size)
		bucket := windowBucketAgg{start: start, end: end}

		for _, r := range records {
			if !r.Timestamp.Before(start) && r.Timestamp.Before(end) {
				bucket.records = append(bucket.records, r)
			}
		}

		if len(bucket.records) > 0 {
			windows = append(windows, bucket)
		}
		start = end
	}
	return windows
}

func (wa *WindowAggregator) slidingWindows(records []*Record, minTime, maxTime time.Time, size, slide time.Duration) []windowBucketAgg {
	if slide == 0 {
		slide = size / 2
	}

	var windows []windowBucketAgg
	start := minTime.Truncate(slide)

	for start.Before(maxTime) {
		end := start.Add(size)
		bucket := windowBucketAgg{start: start, end: end}

		for _, r := range records {
			if !r.Timestamp.Before(start) && r.Timestamp.Before(end) {
				bucket.records = append(bucket.records, r)
			}
		}

		if len(bucket.records) > 0 {
			windows = append(windows, bucket)
		}
		start = start.Add(slide)
	}
	return windows
}

func (wa *WindowAggregator) sessionWindows(records []*Record, gap time.Duration) []windowBucketAgg {
	if len(records) == 0 {
		return nil
	}

	var windows []windowBucketAgg
	current := windowBucketAgg{
		start:   records[0].Timestamp,
		records: []*Record{records[0]},
	}

	for _, r := range records[1:] {
		if r.Timestamp.Sub(current.records[len(current.records)-1].Timestamp) > gap {
			// Gap exceeded, close current window
			current.end = current.records[len(current.records)-1].Timestamp.Add(gap)
			windows = append(windows, current)
			current = windowBucketAgg{
				start:   r.Timestamp,
				records: []*Record{r},
			}
		} else {
			current.records = append(current.records, r)
		}
	}

	current.end = current.records[len(current.records)-1].Timestamp.Add(gap)
	windows = append(windows, current)
	return windows
}

func (wa *WindowAggregator) groupRecords(records []*Record) map[string][]*Record {
	if len(wa.groupBy) == 0 {
		return map[string][]*Record{"": records}
	}

	groups := make(map[string][]*Record)
	for _, r := range records {
		key := ""
		for _, g := range wa.groupBy {
			if v, ok := r.Fields[g]; ok {
				key += fmt.Sprintf("%v|", v)
			}
		}
		groups[key] = append(groups[key], r)
	}
	return groups
}

func (wa *WindowAggregator) computeFunction(fn WindowFunction, records []*Record) (interface{}, error) {
	values := make([]float64, 0, len(records))
	for _, r := range records {
		if v, ok := r.Fields[fn.Field]; ok {
			f := toFloat64(v)
			if f != 0 || v == 0 || v == 0.0 {
				values = append(values, f)
			}
		}
	}

	switch fn.Function {
	case "count":
		return int64(len(records)), nil
	case "sum":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum, nil
	case "avg":
		if len(values) == 0 {
			return 0.0, nil
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil
	case "min":
		if len(values) == 0 {
			return nil, nil
		}
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min, nil
	case "max":
		if len(values) == 0 {
			return nil, nil
		}
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max, nil
	case "stddev":
		if len(values) < 2 {
			return 0.0, nil
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		mean := sum / float64(len(values))
		var variance float64
		for _, v := range values {
			d := v - mean
			variance += d * d
		}
		return math.Sqrt(variance / float64(len(values))), nil
	default:
		return nil, fmt.Errorf("unknown aggregate function: %s", fn.Function)
	}
}
