package feather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		config  *ClientConfig
	}{
		{
			name:    "with default config",
			baseURL: "http://localhost:8080",
			apiKey:  "test-key",
			config:  nil,
		},
		{
			name:    "with custom config",
			baseURL: "http://localhost:9090",
			apiKey:  "custom-key",
			config: &ClientConfig{
				Timeout:         5 * time.Second,
				MaxRetries:      1,
				RetryBackoff:    50 * time.Millisecond,
				MaxRetryBackoff: 1 * time.Second,
				RetryJitter:     0.1,
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.apiKey, tt.config)
			if client == nil {
				t.Fatal("NewClient returned nil")
			}
			if client.Features == nil {
				t.Error("Features sub-client is nil")
			}
			if client.Catalog == nil {
				t.Error("Catalog sub-client is nil")
			}
			if client.Transform == nil {
				t.Error("Transform sub-client is nil")
			}
			if client.Vectors == nil {
				t.Error("Vectors sub-client is nil")
			}
			if client.Streaming == nil {
				t.Error("Streaming sub-client is nil")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryJitter <= 0 || cfg.RetryJitter > 1.0 {
		t.Errorf("expected RetryJitter in (0, 1.0], got %f", cfg.RetryJitter)
	}
}

func TestFeatureClient_Get(t *testing.T) {
	resp := GetResponse{
		EntityID: "user:123",
		Features: map[string]FeatureValue{
			"age": {Feature: "age", Value: float64(25), Timestamp: time.Now()},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong Authorization header")
		}
		if r.URL.Query().Get("entity") != "user:123" {
			t.Errorf("expected entity=user:123, got %s", r.URL.Query().Get("entity"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	got, err := client.Features.Get(context.Background(), "user:123", []string{"age"})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.EntityID != "user:123" {
		t.Errorf("expected entity user:123, got %s", got.EntityID)
	}
	if _, ok := got.Features["age"]; !ok {
		t.Error("expected age feature in response")
	}
}

func TestFeatureClient_Put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req PutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.EntityID != "user:456" {
			t.Errorf("expected entity user:456, got %s", req.EntityID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	err := client.Features.Put(context.Background(), &PutRequest{
		EntityID: "user:456",
		Features: map[string]interface{}{"score": 0.95},
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}
}

func TestFeatureClient_GetBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Entities map[string]*GetResponse `json:"entities"`
		}{
			Entities: map[string]*GetResponse{
				"user:1": {EntityID: "user:1", Features: map[string]FeatureValue{}},
				"user:2": {EntityID: "user:2", Features: map[string]FeatureValue{}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	results, err := client.Features.GetBatch(context.Background(), []string{"user:1", "user:2"}, []string{"age"})
	if err != nil {
		t.Fatalf("GetBatch() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFeatureClient_GetAsOf(t *testing.T) {
	asOf := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("as_of") == "" {
			t.Error("expected as_of query parameter")
		}
		resp := GetResponse{EntityID: "user:1", Features: map[string]FeatureValue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	_, err := client.Features.GetAsOf(context.Background(), "user:1", []string{"age"}, asOf)
	if err != nil {
		t.Fatalf("GetAsOf() error: %v", err)
	}
}

func TestClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "entity not found"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})
	_, err := client.Features.Get(context.Background(), "missing", []string{"age"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestClient_RetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
			return
		}
		resp := GetResponse{EntityID: "user:1", Features: map[string]FeatureValue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:         5 * time.Second,
		MaxRetries:      3,
		RetryBackoff:    1 * time.Millisecond,
		MaxRetryBackoff: 10 * time.Millisecond,
		RetryJitter:     0,
	})
	_, err := client.Features.Get(context.Background(), "user:1", []string{"age"})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    10 * time.Second,
		MaxRetries: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Features.Get(ctx, "user:1", []string{"age"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestConnectionPool(t *testing.T) {
	pool := NewConnectionPool("http://localhost:8080", "key", 3, nil)
	if pool == nil {
		t.Fatal("NewConnectionPool returned nil")
	}

	seen := make(map[*Client]bool)
	for i := 0; i < 6; i++ {
		c := pool.Get()
		seen[c] = true
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 unique clients in round-robin, got %d", len(seen))
	}

	pool.Close()
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 400, Message: "bad request"}
	expected := "API error 400: bad request"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	errWithCode := &APIError{StatusCode: 400, Code: "VALIDATION_FAILED", Message: "entity required"}
	if errWithCode.Code != "VALIDATION_FAILED" {
		t.Errorf("expected code VALIDATION_FAILED, got %q", errWithCode.Code)
	}
}

func TestClient_PostNoRetryOn5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:         5 * time.Second,
		MaxRetries:      3,
		RetryBackoff:    1 * time.Millisecond,
		MaxRetryBackoff: 10 * time.Millisecond,
	})

	// POST should NOT retry on 5xx (non-idempotent)
	err := client.Features.Put(context.Background(), &PutRequest{
		EntityID: "user:1",
		Features: map[string]interface{}{"age": 25},
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if attempts != 1 {
		t.Errorf("POST should not retry: expected 1 attempt, got %d", attempts)
	}
}

func TestClient_ErrorEnvelopeParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		// API envelope format
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error": map[string]string{
				"code":    "VALIDATION_FAILED",
				"message": "entity_key required",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})
	_, err := client.Features.Get(context.Background(), "", []string{"age"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "VALIDATION_FAILED" {
		t.Errorf("expected code VALIDATION_FAILED, got %q", apiErr.Code)
	}
	if apiErr.Message != "entity_key required" {
		t.Errorf("expected message 'entity_key required', got %q", apiErr.Message)
	}
}
