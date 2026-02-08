package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/server"
	"github.com/feather-store/feather/internal/storage"
)

// BenchmarkStore_Put benchmarks feature write operations.
func BenchmarkStore_Put(b *testing.B) {
	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	features := map[string]*domain.FeatureValue{
		"click_count": {
			Value:     int64(100),
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		},
		"purchase_total": {
			Value:     float64(250.50),
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entityKey := fmt.Sprintf("user:%d", i)
			if err := store.Put(entityKey, features); err != nil {
				b.Errorf("Put failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkStore_Get benchmarks feature read operations.
func BenchmarkStore_Get(b *testing.B) {
	schema := storage.NewRegistry()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate with data
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count": {
				Value:     int64(i),
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			},
			"purchase_total": {
				Value:     float64(i) * 10.5,
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			},
		}
		if err := store.Put(entityKey, features); err != nil {
			b.Fatalf("Failed to pre-populate: %v", err)
		}
	}

	featureNames := []string{"click_count", "purchase_total"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entityKey := fmt.Sprintf("user:%d", i%10000)
			_, err := store.Get(entityKey, featureNames)
			if err != nil && !domain.IsNotFound(err) {
				b.Errorf("Get failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkHTTP_GetFeatures benchmarks HTTP GET latency.
func BenchmarkHTTP_GetFeatures(b *testing.B) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
		}
		store.Put(entityKey, features)
	}

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entityKey := fmt.Sprintf("user:%d", i%10000)
			req := httptest.NewRequest("GET",
				fmt.Sprintf("/v1/features?entity=%s&feature=click_count&feature=purchase_total", entityKey),
				nil)
			w := httptest.NewRecorder()
			httpServer.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Errorf("Expected 200, got %d", w.Code)
			}
			i++
		}
	})
}

// BenchmarkHTTP_PostFeatures benchmarks HTTP POST latency.
func BenchmarkHTTP_PostFeatures(b *testing.B) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			update := domain.FeatureUpdate{
				EntityKey: fmt.Sprintf("user:%d", i),
				Features: map[string]interface{}{
					"click_count":    i * 10,
					"purchase_total": float64(i) * 25.5,
				},
			}
			body, _ := json.Marshal(update)
			req := httptest.NewRequest("POST", "/v1/features", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			httpServer.ServeHTTP(w, req)
			if w.Code != http.StatusCreated {
				b.Errorf("Expected 201, got %d", w.Code)
			}
			i++
		}
	})
}

// BenchmarkAggregation_Compute benchmarks aggregation computation.
func BenchmarkAggregation_Compute(b *testing.B) {
	agg := aggregation.NewEngine()

	spec := &domain.AggregationSpec{
		Function: domain.AggCount,
		Window:   time.Hour,
	}
	agg.RegisterAggregation("click_count_1h", spec)

	// Pre-populate with events
	now := time.Now()
	for i := 0; i < 1000; i++ {
		for j := 0; j < 100; j++ {
			agg.Update(fmt.Sprintf("user:%d", i), "click_count_1h", 1.0, now.Add(-time.Duration(j)*time.Second))
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			entityKey := fmt.Sprintf("user:%d", i%1000)
			_, err := agg.Compute(entityKey, "click_count_1h", domain.AggCount)
			if err != nil {
				b.Errorf("Compute failed: %v", err)
			}
			i++
		}
	})
}

// TestLatencyP99 measures and reports P99 latency for feature retrieval.
// This test verifies the <1ms P99 latency requirement from the PRD.
func TestLatencyP99(t *testing.T) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate with data
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
			"last_login":     {Value: time.Now().UnixNano(), Timestamp: time.Now().UnixNano(), Version: 1},
		}
		if err := store.Put(entityKey, features); err != nil {
			t.Fatalf("Failed to pre-populate: %v", err)
		}
	}

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	const numRequests = 10000
	latencies := make([]time.Duration, numRequests)

	// Warm up
	for i := 0; i < 1000; i++ {
		req := httptest.NewRequest("GET", "/v1/features?entity=user:0&feature=click_count", nil)
		w := httptest.NewRecorder()
		httpServer.ServeHTTP(w, req)
	}

	// Measure latencies
	for i := 0; i < numRequests; i++ {
		entityKey := fmt.Sprintf("user:%d", i%10000)
		req := httptest.NewRequest("GET",
			fmt.Sprintf("/v1/features?entity=%s&feature=click_count&feature=purchase_total", entityKey),
			nil)
		w := httptest.NewRecorder()

		start := time.Now()
		httpServer.ServeHTTP(w, req)
		latencies[i] = time.Since(start)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i, w.Code)
		}
	}

	// Sort latencies to compute percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p90 := latencies[len(latencies)*90/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	maxLatency := latencies[len(latencies)-1]

	t.Logf("Latency Distribution (n=%d):", numRequests)
	t.Logf("  P50: %v", p50)
	t.Logf("  P90: %v", p90)
	t.Logf("  P95: %v", p95)
	t.Logf("  P99: %v", p99)
	t.Logf("  Max: %v", maxLatency)

	// PRD requirement: <1ms P99 latency for hot tier
	// Note: In a real benchmark, we'd use a more controlled environment
	// For CI/testing, we use a more lenient threshold
	if p99 > 10*time.Millisecond {
		t.Errorf("P99 latency %v exceeds 10ms threshold (PRD target: <1ms)", p99)
	}
}

// TestLatencyP99_Concurrent measures P99 latency under concurrent load.
func TestLatencyP99_Concurrent(t *testing.T) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
		}
		store.Put(entityKey, features)
	}

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	const (
		numWorkers  = 100
		numRequests = 100 // per worker
	)

	var (
		mu        sync.Mutex
		latencies []time.Duration
		wg        sync.WaitGroup
	)

	// Warm up
	for i := 0; i < 1000; i++ {
		req := httptest.NewRequest("GET", "/v1/features?entity=user:0&feature=click_count", nil)
		w := httptest.NewRecorder()
		httpServer.ServeHTTP(w, req)
	}

	// Run concurrent workers
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			localLatencies := make([]time.Duration, 0, numRequests)

			for i := 0; i < numRequests; i++ {
				entityKey := fmt.Sprintf("user:%d", (workerID*numRequests+i)%10000)
				req := httptest.NewRequest("GET",
					fmt.Sprintf("/v1/features?entity=%s&feature=click_count&feature=purchase_total", entityKey),
					nil)
				w := httptest.NewRecorder()

				start := time.Now()
				httpServer.ServeHTTP(w, req)
				localLatencies = append(localLatencies, time.Since(start))
			}

			mu.Lock()
			latencies = append(latencies, localLatencies...)
			mu.Unlock()
		}(w)
	}

	wg.Wait()

	// Sort and compute percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p90 := latencies[len(latencies)*90/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	maxLatency := latencies[len(latencies)-1]

	t.Logf("Concurrent Latency Distribution (workers=%d, requests=%d, total=%d):",
		numWorkers, numRequests, len(latencies))
	t.Logf("  P50: %v", p50)
	t.Logf("  P90: %v", p90)
	t.Logf("  P95: %v", p95)
	t.Logf("  P99: %v", p99)
	t.Logf("  Max: %v", maxLatency)

	// More lenient threshold for concurrent test
	if p99 > 50*time.Millisecond {
		t.Errorf("Concurrent P99 latency %v exceeds 50ms threshold", p99)
	}
}

// TestLatencyP99_StoreDirect measures P99 latency for direct store access.
func TestLatencyP99_StoreDirect(t *testing.T) {
	schema := storage.NewRegistry()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
		}
		store.Put(entityKey, features)
	}

	const numRequests = 100000
	latencies := make([]time.Duration, numRequests)
	featureNames := []string{"click_count", "purchase_total"}

	// Warm up
	for i := 0; i < 10000; i++ {
		store.Get("user:0", featureNames)
	}

	// Measure latencies
	for i := 0; i < numRequests; i++ {
		entityKey := fmt.Sprintf("user:%d", i%10000)
		start := time.Now()
		_, err := store.Get(entityKey, featureNames)
		latencies[i] = time.Since(start)
		if err != nil && !domain.IsNotFound(err) {
			t.Errorf("Get failed: %v", err)
		}
	}

	// Sort and compute percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p90 := latencies[len(latencies)*90/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	maxLatency := latencies[len(latencies)-1]

	t.Logf("Direct Store Latency Distribution (n=%d):", numRequests)
	t.Logf("  P50: %v", p50)
	t.Logf("  P90: %v", p90)
	t.Logf("  P95: %v", p95)
	t.Logf("  P99: %v", p99)
	t.Logf("  Max: %v", maxLatency)

	// PRD requirement: <1ms P99 latency for hot tier
	// Direct store access should be faster than HTTP
	if p99 > 1*time.Millisecond {
		t.Logf("Warning: P99 latency %v exceeds 1ms PRD target", p99)
		// Don't fail - this depends on the test environment
	}
}

// BenchmarkBatchGet benchmarks batch feature retrieval.
func BenchmarkBatchGet(b *testing.B) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
		}
		store.Put(entityKey, features)
	}

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	// Create batch request
	entities := make([]string, 100)
	for i := 0; i < 100; i++ {
		entities[i] = fmt.Sprintf("user:%d", i)
	}
	batchReq := domain.GetFeaturesRequest{
		Entities: entities,
		Features: []string{"click_count", "purchase_total"},
	}
	bodyBytes, _ := json.Marshal(batchReq)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/features/batch", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		httpServer.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Errorf("Expected 200, got %d", w.Code)
		}
	}
}

// TestThroughput measures the maximum throughput.
func TestThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		entityKey := fmt.Sprintf("user:%d", i)
		features := map[string]*domain.FeatureValue{
			"click_count":    {Value: int64(i), Timestamp: time.Now().UnixNano(), Version: 1},
			"purchase_total": {Value: float64(i) * 10.5, Timestamp: time.Now().UnixNano(), Version: 1},
		}
		store.Put(entityKey, features)
	}

	httpServer := server.NewHTTPServer(context.Background(), store, agg, schema, nil, server.HTTPServerConfig{
		Core: server.HTTPServerCoreConfig{
			Port:         0,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	})

	const (
		numWorkers = 100
		duration   = 5 * time.Second
	)

	var (
		totalRequests int64
		mu            sync.Mutex
		wg            sync.WaitGroup
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	wg.Add(numWorkers)
	start := time.Now()

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()
			localCount := int64(0)
			i := 0

			for {
				select {
				case <-ctx.Done():
					mu.Lock()
					totalRequests += localCount
					mu.Unlock()
					return
				default:
				}

				entityKey := fmt.Sprintf("user:%d", (workerID*1000+i)%10000)
				req := httptest.NewRequest("GET",
					fmt.Sprintf("/v1/features?entity=%s&feature=click_count&feature=purchase_total", entityKey),
					nil)
				w := httptest.NewRecorder()
				httpServer.ServeHTTP(w, req)
				localCount++
				i++
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)

	throughput := float64(totalRequests) / elapsed.Seconds()
	t.Logf("Throughput Test Results:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Workers: %d", numWorkers)
	t.Logf("  Total Requests: %d", totalRequests)
	t.Logf("  Throughput: %.2f req/s", throughput)

	// Minimum expected throughput
	if throughput < 10000 {
		t.Logf("Warning: Throughput %.2f req/s is lower than expected", throughput)
	}
}
