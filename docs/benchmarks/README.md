# Feather Benchmarks

## Latest Results

| Operation | Latency | Ops/sec | Allocs/op |
|-----------|---------|---------|-----------|
| Hot Tier Get | **223 ns** | 16.1M | 2 |
| Hot Tier Put | **114 ns** | 30.1M | 0 |
| Store Get (tiered) | **207 ns** | 18.2M | 2 |
| Store Put (tiered) | **276 ns** | 11.4M | 2 |
| Warm Tier Get | 4,091 ns | 969K | 47 |
| Aggregation Update | **56 ns** | 44.8M | 0 |
| Aggregation Compute | **75 ns** | 33.5M | 0 |

*Measured with `go test -bench=. -benchtime=3s -benchmem` on Go 1.24.4, darwin/arm64.*

## Key Highlights

- **Sub-microsecond hot tier**: 207ns per feature lookup (0.2µs)
- **Zero-allocation writes**: Hot tier puts allocate 0 bytes, creating no GC pressure
- **18.2M ops/sec**: Hot tier throughput from a single process
- **18 MB binary**: Smaller than most Docker base images

## Reproduce

```bash
git clone https://github.com/feather-store/feather
cd feather
go test -bench=. -benchtime=3s -benchmem ./internal/core/storage/... ./internal/core/aggregation/...
```

## Full Report

See [index.html](index.html) for the interactive visual benchmark report with charts.
