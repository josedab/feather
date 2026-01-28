package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func TestCircuitBreaker_ClosedAllows(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	if !cb.Allow() {
		t.Error("closed circuit breaker should allow requests")
	}
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %d", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	for i := int64(0); i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen after %d failures, got %d", 3, cb.State())
	}
	if cb.Allow() {
		t.Error("open circuit breaker should deny requests")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Fatal("expected CircuitOpen")
	}

	time.Sleep(20 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should allow after timeout (half-open)")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen, got %d", cb.State())
	}
}

func TestCircuitBreaker_ClosesOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	cb.Allow() // transitions to half-open
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after success in half-open, got %d", cb.State())
	}
}

func TestCircuitBreaker_ReopensOnHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	cb.Allow() // transitions to half-open
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen after failure in half-open, got %d", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Second)

	cb.RecordFailure()
	cb.RecordFailure()

	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after reset, got %d", cb.State())
	}
	if !cb.Allow() {
		t.Error("should allow after reset")
	}
}

func TestJSONDecoder_Valid(t *testing.T) {
	dec := &JSONDecoder{}
	data := []byte(`{"entity_key":"user:1","features":{"score":0.95}}`)

	update, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update.EntityKey != "user:1" {
		t.Errorf("entity_key = %q, want %q", update.EntityKey, "user:1")
	}
}

func TestJSONDecoder_Invalid(t *testing.T) {
	dec := &JSONDecoder{}
	_, err := dec.Decode([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestKafkaConsumer_NoCGO(t *testing.T) {
	consumer, err := NewKafkaConsumer(
		KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			Topic:         "features",
			ConsumerGroup: "test",
		},
		nil, nil, slog.Default(),
	)

	// In non-CGO builds, should return an error
	if consumer == nil && err != nil {
		// Expected: Kafka not available without CGO
		return
	}

	// If consumer was created (CGO build), test basic methods
	if consumer != nil {
		_ = consumer.Metrics()
		consumer.Stop()
		_ = consumer.Close()
		_ = consumer.CircuitBreakerStatus()
	}
}

func TestHTTPIngestion_SchemaValidation(t *testing.T) {
	schema := storage.NewRegistry()

	// Register a feature group with a typed feature
	group := &domain.FeatureGroup{
		Name:       "user_features",
		EntityType: "user",
		Features: []domain.FeatureSpec{
			{Name: "score", DataType: domain.DataTypeFloat64},
		},
	}
	if err := schema.RegisterGroup(group); err != nil {
		t.Fatalf("failed to register group: %v", err)
	}

	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   1 << 20,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	agg := aggregation.NewEngine()

	h := NewHTTPIngestionWithConfig(store, agg, schema, HTTPIngestionConfig{
		ValidateSchema: true,
	})

	// Valid feature that passes schema validation
	body := map[string]interface{}{
		"entity_key": "user:1",
		"features":   map[string]interface{}{"score": 0.95},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.HandlePush(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("valid schema: status = %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestHTTPIngestion_RegisterRoutes(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{})

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Test POST /ingest is registered
	body := `{"entity_key":"user:1","features":{"score":0.5}}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Error("expected route /ingest to be registered")
	}

	// Test POST /ingest/bulk is registered
	bulkBody := `[{"entity_key":"user:1","features":{"score":0.5}}]`
	req2 := httptest.NewRequest(http.MethodPost, "/ingest/bulk", strings.NewReader(bulkBody))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code == http.StatusNotFound {
		t.Error("expected route /ingest/bulk to be registered")
	}
}

func TestHTTPIngestion_BulkPush_InvalidJSON(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{})

	req := httptest.NewRequest(http.MethodPost, "/ingest/bulk", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.HandleBulkPush(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHTTPIngestion_DefaultMaxRequestSize(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{})

	got := h.maxRequestSize()
	if got != DefaultMaxRequestSize {
		t.Errorf("maxRequestSize() = %d, want %d", got, DefaultMaxRequestSize)
	}
}

func TestHTTPIngestion_CustomMaxRequestSize(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		MaxRequestSize: 2048,
	})

	got := h.maxRequestSize()
	if got != 2048 {
		t.Errorf("maxRequestSize() = %d, want 2048", got)
	}
}

func TestHTTPIngestion_RateLimitDefaults(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		RateLimitEnabled: true,
		// Leave RateLimitPerSecond and RateLimitBurst at 0 to test defaults
	})

	if h.rateLimiter == nil {
		t.Fatal("expected rate limiter to be created")
	}
	if h.rateLimiter.rps != 100 {
		t.Errorf("default rps = %d, want 100", h.rateLimiter.rps)
	}
	if h.rateLimiter.burst != 200 {
		t.Errorf("default burst = %d, want 200", h.rateLimiter.burst)
	}
}

func TestHTTPIngestion_RateLimitDifferentClients(t *testing.T) {
	h := newTestHTTPIngestion(t, HTTPIngestionConfig{
		RateLimitEnabled:   true,
		RateLimitPerSecond: 1,
		RateLimitBurst:     1,
		ValidateSchema:     false,
	})

	body := `{"entity_key":"user:1","features":{"score":0.5}}`

	// First client uses up their burst
	req1 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req1.RemoteAddr = "10.0.0.1:1234"
	rr1 := httptest.NewRecorder()
	h.HandlePush(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Errorf("first client first request: status = %d, want 201", rr1.Code)
	}

	// Second client should still be allowed (separate bucket)
	req2 := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req2.RemoteAddr = "10.0.0.2:1234"
	rr2 := httptest.NewRecorder()
	h.HandlePush(rr2, req2)
	if rr2.Code != http.StatusCreated {
		t.Errorf("second client first request: status = %d, want 201", rr2.Code)
	}
}

func TestBatchImporter_ImportJSONLReader_NoEntityKey(t *testing.T) {
	b := newTestBatchImporter(t)

	jsonl := `{"score": 0.95}
{"score": 0.85}`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		SkipErrors:      false,
	}

	_, err := b.ImportJSONLReader(context.Background(), strings.NewReader(jsonl), config)
	if err == nil {
		t.Error("expected error for missing entity key without SkipErrors")
	}
}

func TestBatchImporter_ImportJSONReader_NoEntityKey_NoSkip(t *testing.T) {
	b := newTestBatchImporter(t)

	jsonData := `[{"score": 0.95}]`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		SkipErrors:      false,
	}

	_, err := b.ImportJSONReader(context.Background(), strings.NewReader(jsonData), config)
	if err == nil {
		t.Error("expected error for missing entity key without SkipErrors")
	}
}

func TestBatchImporter_ImportCSVReader_EmptyEntityKey_NoSkip(t *testing.T) {
	b := newTestBatchImporter(t)

	csv := `entity_id,score
,0.95`

	config := ImportConfig{
		EntityKeyColumn: "entity_id",
		HasHeader:       true,
		SkipErrors:      false,
	}

	_, err := b.ImportCSVReader(context.Background(), strings.NewReader(csv), config)
	if err == nil {
		t.Error("expected error for empty entity key without SkipErrors")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := newRateLimiter(10, 5)

	// Add a client
	rl.allow("old-client")

	// Set cleanup time far in the past
	rl.mu.Lock()
	rl.cleanupT = time.Now().Add(-2 * time.Minute)
	// Set the old client's lastRefill far in the past
	rl.clients["old-client"].lastRefill = time.Now().Add(-10 * time.Minute)
	rl.mu.Unlock()

	// Next call should trigger cleanup
	rl.allow("new-client")

	rl.mu.Lock()
	_, exists := rl.clients["old-client"]
	rl.mu.Unlock()

	if exists {
		t.Error("expected stale client to be cleaned up")
	}
}
