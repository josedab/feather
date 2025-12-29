// Package aggregation provides real-time feature aggregation with sliding windows.
//
// It supports common aggregation functions including count, sum, average, min,
// max, and percentiles over configurable time windows. Aggregations are computed
// incrementally as new data arrives, enabling low-latency feature serving.
//
// The Engine is the main entry point for registering and computing aggregations:
//
//	engine := aggregation.NewEngine()
//	engine.RegisterAggregation("purchase_count_1h", &domain.AggregationSpec{
//	    Function: domain.AggCount,
//	    Window:   time.Hour,
//	})
//	engine.Update("user:123", "purchase_count_1h", 1.0, time.Now())
//	result := engine.Compute("user:123", "purchase_count_1h")
package aggregation
