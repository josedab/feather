package metrics

import (
	"context"
	"net/http"
	"runtime"
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

	// Runtime metrics
	goroutineCount    prometheus.Gauge
	memoryAlloc       prometheus.Gauge
	memoryTotalAlloc  prometheus.Gauge
	memoryHeapObjects prometheus.Gauge
	gcPauseTotal      prometheus.Gauge
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		// HTTP metrics
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"method", "path"},
		),

		// gRPC metrics
		grpcRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "grpc_requests_total",
				Help:      "Total gRPC requests",
			},
			[]string{"method", "status"},
		),
		grpcRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "grpc_request_duration_seconds",
				Help:      "gRPC request duration in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"method"},
		),

		// Cache metrics
		cacheHits: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_hits_total",
				Help:      "Total cache hits",
			},
		),
		cacheMisses: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_misses_total",
				Help:      "Total cache misses",
			},
		),
		hotTierSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "hot_tier_bytes",
				Help:      "Hot tier size in bytes",
			},
		),
		warmTierSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "warm_tier_bytes",
				Help:      "Warm tier size in bytes",
			},
		),
		entityCount: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "entity_count",
				Help:      "Number of entities in store",
			},
		),

		// Ingestion metrics
		messagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ingestion_messages_received_total",
				Help:      "Total messages received",
			},
			[]string{"source"},
		),
		messagesProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ingestion_messages_processed_total",
				Help:      "Total messages processed",
			},
			[]string{"source", "status"},
		),
		ingestionLag: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "ingestion_lag_seconds",
				Help:      "Ingestion lag in seconds",
			},
		),

		// Feature metrics
		featureFreshness: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "feature_freshness_seconds",
				Help:      "Feature freshness (age) in seconds",
			},
			[]string{"feature_group"},
		),
		featureRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "feature_requests_total",
				Help:      "Total feature requests",
			},
			[]string{"feature"},
		),
		aggregationCompute: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "aggregation_compute_seconds",
				Help:      "Time to compute aggregations",
				Buckets:   []float64{.00001, .00005, .0001, .0005, .001, .005, .01},
			},
			[]string{"feature"},
		),

		// Warm tier operation latency
		warmTierOps: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "warm_tier_operation_seconds",
				Help:      "Warm tier operation latency in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
			},
			[]string{"operation"},
		),

		// Eviction events
		evictionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "evictions_total",
				Help:      "Total eviction events",
			},
			[]string{"tier", "reason"},
		),

		// Shard contention
		shardWaitTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "shard_wait_seconds",
				Help:      "Time spent waiting for shard locks",
				Buckets:   []float64{.00001, .00005, .0001, .0005, .001, .005, .01},
			},
			[]string{"shard"},
		),

		// Error categorization
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total errors by category and code",
			},
			[]string{"component", "code"},
		),

		// Runtime metrics
		goroutineCount: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "goroutines",
				Help:      "Current number of goroutines",
			},
		),
		memoryAlloc: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_alloc_bytes",
				Help:      "Bytes of allocated heap objects",
			},
		),
		memoryTotalAlloc: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_total_alloc_bytes",
				Help:      "Cumulative bytes allocated for heap objects",
			},
		),
		memoryHeapObjects: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_heap_objects",
				Help:      "Number of allocated heap objects",
			},
		),
		gcPauseTotal: promauto.NewGauge(
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

// SetFeatureFreshness sets the feature freshness gauge.
func (m *Metrics) SetFeatureFreshness(group string, age time.Duration) {
	m.featureFreshness.WithLabelValues(group).Set(age.Seconds())
}

// RecordFeatureRequest records a feature request.
func (m *Metrics) RecordFeatureRequest(feature string) {
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
		for {
			select {
			case <-ticker.C:
				m.UpdateRuntimeMetrics()
			case <-done:
				ticker.Stop()
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
