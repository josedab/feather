// Package compression provides intelligent tiered compression with ML-based
// strategy selection based on per-feature data characteristics.
//
// # Usage
//
//	selector := compression.NewSelector(compression.DefaultConfig())
//	strategy := selector.SelectStrategy(stats)
//	compressed, _ := selector.Compress(data, strategy)
package compression
