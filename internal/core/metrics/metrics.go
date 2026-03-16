package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Metrics provides Prometheus metrics for Feather.
type Metrics struct {
	// Serving metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	grpcRequestsTotal   *prometheus.CounterVec
	grpcRequestDuration *prometheus.HistogramVec

	// Storage metrics
	cacheHits     prometheus.Counter
	cacheMisses   prometheus.Counter
	hotTierSize   prometheus.Gauge
	warmTierSize  prometheus.Gauge
	entityCount   prometheus.Gauge
	warmTierOps   *prometheus.HistogramVec // Warm tier operation latency
	evictionTotal *prometheus.CounterVec   // Eviction events
	shardWaitTime *prometheus.HistogramVec // Shard lock contention

	// Ingestion metrics
	messagesReceived  *prometheus.CounterVec
	messagesProcessed *prometheus.CounterVec
	ingestionLag      prometheus.Gauge

	// Feature metrics
	featureFreshness   *prometheus.GaugeVec
	featureRequests    *prometheus.CounterVec
	aggregationCompute *prometheus.HistogramVec

	// Error metrics
	errorsTotal *prometheus.CounterVec

	// Storage backpressure metrics
	warmWriteDrops  prometheus.Gauge
	warmWriteErrors prometheus.Gauge

	// Runtime metrics
	goroutineCount    prometheus.Gauge
	memoryAlloc       prometheus.Gauge
	memoryTotalAlloc  prometheus.Gauge
	memoryHeapObjects prometheus.Gauge
	gcPauseTotal      prometheus.Gauge
}

// NewMetrics creates a new Metrics instance using the default Prometheus registry.
func NewMetrics(namespace string) *Metrics {
	return newMetrics(namespace, promauto.With(prometheus.DefaultRegisterer))
}

// NewMetricsWithRegistry creates a new Metrics instance using a custom Prometheus registry.
// This is useful for testing where metrics need to be gathered and asserted.
func NewMetricsWithRegistry(namespace string, reg prometheus.Registerer) *Metrics {
	return newMetrics(namespace, promauto.With(reg))
}

func newMetrics(namespace string, factory promauto.Factory) *Metrics {
	return &Metrics{
		// HTTP metrics
		httpRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"method", "path"},
		),

		// gRPC metrics
		grpcRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "grpc_requests_total",
				Help:      "Total gRPC requests",
			},
			[]string{"method", "status"},
		),
		grpcRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "grpc_request_duration_seconds",
				Help:      "gRPC request duration in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"method"},
		),

		// Cache metrics
		cacheHits: factory.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_hits_total",
				Help:      "Total cache hits",
			},
		),
		cacheMisses: factory.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_misses_total",
				Help:      "Total cache misses",
			},
		),
		hotTierSize: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "hot_tier_bytes",
				Help:      "Hot tier size in bytes",
			},
		),
		warmTierSize: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "warm_tier_bytes",
				Help:      "Warm tier size in bytes",
			},
		),
		entityCount: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "entity_count",
				Help:      "Number of entities in store",
			},
		),

		// Ingestion metrics
		messagesReceived: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ingestion_messages_received_total",
				Help:      "Total messages received",
			},
			[]string{"source"},
		),
		messagesProcessed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ingestion_messages_processed_total",
				Help:      "Total messages processed",
			},
			[]string{"source", "status"},
		),
		ingestionLag: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "ingestion_lag_seconds",
				Help:      "Ingestion lag in seconds",
			},
		),

		// Feature metrics
		featureFreshness: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "feature_freshness_seconds",
				Help:      "Feature freshness (age) in seconds",
			},
			[]string{"feature_group"},
		),
		featureRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "feature_requests_total",
				Help:      "Total feature requests",
			},
			[]string{"feature"},
		),
		aggregationCompute: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "aggregation_compute_seconds",
				Help:      "Time to compute aggregations",
				Buckets:   []float64{.00001, .00005, .0001, .0005, .001, .005, .01},
			},
			[]string{"feature"},
		),

		// Warm tier operation latency
		warmTierOps: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "warm_tier_operation_seconds",
				Help:      "Warm tier operation latency in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
			},
			[]string{"operation"},
		),

		// Eviction events
		evictionTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "evictions_total",
				Help:      "Total eviction events",
			},
			[]string{"tier", "reason"},
		),

		// Shard contention
		shardWaitTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "shard_wait_seconds",
				Help:      "Time spent waiting for shard locks",
				Buckets:   []float64{.00001, .00005, .0001, .0005, .001, .005, .01},
			},
			[]string{"shard"},
		),

		// Error categorization
		errorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total errors by category and code",
			},
			[]string{"component", "code"},
		),

		// Warm tier backpressure
		warmWriteDrops: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "warm_write_drops_total",
				Help:      "Total warm tier writes dropped due to buffer full or shutdown",
			},
		),
		warmWriteErrors: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "warm_write_errors_total",
				Help:      "Total warm tier write errors",
			},
		),

		// Runtime metrics
		goroutineCount: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "goroutines",
				Help:      "Current number of goroutines",
			},
		),
		memoryAlloc: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_alloc_bytes",
				Help:      "Bytes of allocated heap objects",
			},
		),
		memoryTotalAlloc: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_total_alloc_bytes",
				Help:      "Cumulative bytes allocated for heap objects",
			},
		),
		memoryHeapObjects: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_heap_objects",
				Help:      "Number of allocated heap objects",
			},
		),
		gcPauseTotal: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "gc_pause_total_seconds",
				Help:      "Total GC pause time in seconds",
			},
		),
	}
}

// RecordHTTPLatency records HTTP request latency.
func (m *Metrics) RecordHTTPLatency(method, path string, duration time.Duration) {
	m.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordHTTPRequest records an HTTP request.
func (m *Metrics) RecordHTTPRequest(method, path string, status int) {
	m.httpRequestsTotal.WithLabelValues(method, path, http.StatusText(status)).Inc()
}

// RecordGRPCLatency records gRPC request latency.
func (m *Metrics) RecordGRPCLatency(method string, duration time.Duration) {
	m.grpcRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordGRPCRequest records a gRPC request.
func (m *Metrics) RecordGRPCRequest(method, status string) {
	m.grpcRequestsTotal.WithLabelValues(method, status).Inc()
}

// RecordCacheHit records a cache hit.
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Inc()
}

// RecordCacheMiss records a cache miss.
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Inc()
}

// SetHotTierSize sets the hot tier size gauge.
func (m *Metrics) SetHotTierSize(bytes int64) {
	m.hotTierSize.Set(float64(bytes))
}

// SetWarmTierSize sets the warm tier size gauge.
func (m *Metrics) SetWarmTierSize(bytes int64) {
	m.warmTierSize.Set(float64(bytes))
}

// SetEntityCount sets the entity count gauge.
func (m *Metrics) SetEntityCount(count int) {
	m.entityCount.Set(float64(count))
}

// RecordIngestion records an ingestion event.
func (m *Metrics) RecordIngestion(source string, success bool) {
	m.messagesReceived.WithLabelValues(source).Inc()
	status := "success"
	if !success {
		status = "error"
	}
	m.messagesProcessed.WithLabelValues(source, status).Inc()
}

// SetIngestionLag sets the ingestion lag gauge.
func (m *Metrics) SetIngestionLag(lag time.Duration) {
	m.ingestionLag.Set(lag.Seconds())
}

// SetFeatureFreshness sets the feature freshness gauge for a feature group.
func (m *Metrics) SetFeatureFreshness(featureGroup string, age time.Duration) {
	m.featureFreshness.WithLabelValues(featureGroup).Set(age.Seconds())
}

// RecordFeatureRequest records a feature request. Feature names exceeding 256
// bytes are replaced with "unknown" to prevent cardinality explosion.
func (m *Metrics) RecordFeatureRequest(feature string) {
	if len(feature) > 256 {
		feature = "unknown"
	}
	m.featureRequests.WithLabelValues(feature).Inc()
}

// RecordAggregationCompute records aggregation computation time.
func (m *Metrics) RecordAggregationCompute(feature string, duration time.Duration) {
	m.aggregationCompute.WithLabelValues(feature).Observe(duration.Seconds())
}

// RecordWarmTierOp records warm tier operation latency.
func (m *Metrics) RecordWarmTierOp(operation string, duration time.Duration) {
	m.warmTierOps.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordEviction records an eviction event.
func (m *Metrics) RecordEviction(tier, reason string) {
	m.evictionTotal.WithLabelValues(tier, reason).Inc()
}

// RecordShardWait records time spent waiting for a shard lock.
func (m *Metrics) RecordShardWait(shard string, duration time.Duration) {
	m.shardWaitTime.WithLabelValues(shard).Observe(duration.Seconds())
}

// RecordError records an error by component and code.
func (m *Metrics) RecordError(component, code string) {
	m.errorsTotal.WithLabelValues(component, code).Inc()
}

// SetWarmWriteDrops sets the warm write drops gauge.
func (m *Metrics) SetWarmWriteDrops(count int64) {
	m.warmWriteDrops.Set(float64(count))
}

// SetWarmWriteErrors sets the warm write errors gauge.
func (m *Metrics) SetWarmWriteErrors(count int64) {
	m.warmWriteErrors.Set(float64(count))
}

// UpdateRuntimeMetrics updates runtime-related metrics.
// This should be called periodically (e.g., every 15 seconds).
func (m *Metrics) UpdateRuntimeMetrics() {
	m.goroutineCount.Set(float64(runtime.NumGoroutine()))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.memoryAlloc.Set(float64(memStats.Alloc))
	m.memoryTotalAlloc.Set(float64(memStats.TotalAlloc))
	m.memoryHeapObjects.Set(float64(memStats.HeapObjects))
	m.gcPauseTotal.Set(float64(memStats.PauseTotalNs) / 1e9) // Convert to seconds
}

// StartRuntimeMetricsCollector starts a background goroutine to collect runtime metrics.
func (m *Metrics) StartRuntimeMetricsCollector(interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("metrics collection panic", "error", r, "stack", string(debug.Stack()))
						}
					}()
					m.UpdateRuntimeMetrics()
				}()
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

// Handler returns the Prometheus HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// UnaryServerInterceptor returns a gRPC unary interceptor for metrics.
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		status := "OK"
		if err != nil {
			status = "ERROR"
		}

		m.RecordGRPCLatency(info.FullMethod, duration)
		m.RecordGRPCRequest(info.FullMethod, status)

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream interceptor for metrics.
func (m *Metrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		status := "OK"
		if err != nil {
			status = "ERROR"
		}

		m.RecordGRPCLatency(info.FullMethod, duration)
		m.RecordGRPCRequest(info.FullMethod, status)

		return err
	}
}
