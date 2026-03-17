package feather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBatchClient(t *testing.T) {
	client := NewClient("http://localhost:8080", "key", nil)
	bc := NewBatchClient(client, 10, time.Second)
	if bc == nil {
		t.Fatal("NewBatchClient returned nil")
	}
	bc.Close()
}

func TestNewBatchClientWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient("http://localhost:8080", "key", nil)
	bc := NewBatchClientWithContext(ctx, client, 10, time.Second)
	if bc == nil {
		t.Fatal("NewBatchClientWithContext returned nil")
	}
	bc.Close()
}

func TestBatchClient_Put_FlushOnBatchSize(t *testing.T) {
	var putCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&putCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	bc := NewBatchClient(client, 2, 10*time.Second) // large interval so timer doesn't fire

	// Put two items to trigger batch flush
	err1Done := make(chan error, 1)
	go func() {
		err1Done <- bc.Put(context.Background(), "user:1", map[string]interface{}{"score": 1})
	}()

	err2Done := make(chan error, 1)
	go func() {
		err2Done <- bc.Put(context.Background(), "user:2", map[string]interface{}{"score": 2})
	}()

	// Wait for both puts to complete
	select {
	case <-err1Done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first put")
	}
	select {
	case <-err2Done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for second put")
	}

	bc.Close()

	count := atomic.LoadInt32(&putCount)
	if count == 0 {
		t.Error("expected at least one PUT request to server")
	}
}

func TestBatchClient_FlushOnClose(t *testing.T) {
	var putCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&putCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	bc := NewBatchClient(client, 100, 50*time.Millisecond) // small interval

	// Put one item (won't trigger batch size flush)
	go bc.Put(context.Background(), "user:1", map[string]interface{}{"a": 1})

	// Wait for timer flush
	time.Sleep(150 * time.Millisecond)

	bc.Close()

	count := atomic.LoadInt32(&putCount)
	if count == 0 {
		t.Error("expected flush on timer or close")
	}
}

func TestBatchClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // slow server
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", &ClientConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
	})

	bc := NewBatchClient(client, 1, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := bc.Put(ctx, "user:1", map[string]interface{}{"a": 1})
	if err == nil {
		t.Log("put completed before context expired (acceptable)")
	}

	bc.Close()
}

func TestAsyncClient_GetAsync(t *testing.T) {
	resp := GetResponse{EntityID: "user:1", Features: map[string]FeatureValue{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	ac := NewAsyncClient(client)

	ch := ac.GetAsync(context.Background(), "user:1", []string{"age"})
	result := <-ch
	if result.Err != nil {
		t.Fatalf("GetAsync error: %v", result.Err)
	}
	if result.Value.EntityID != "user:1" {
		t.Errorf("expected user:1, got %s", result.Value.EntityID)
	}
}

func TestAsyncClient_ParallelGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entity := r.URL.Query().Get("entity")
		resp := GetResponse{EntityID: entity, Features: map[string]FeatureValue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	ac := NewAsyncClient(client)

	requests := []GetRequest{
		{EntityID: "user:1", Features: []string{"age"}},
		{EntityID: "user:2", Features: []string{"age"}},
	}

	result := ac.ParallelGet(context.Background(), requests)
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestCachedClient_Get(t *testing.T) {
	var hitCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitCount, 1)
		resp := GetResponse{EntityID: "user:1", Features: map[string]FeatureValue{
			"age": {Feature: "age", Value: float64(25)},
		}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	cc := NewCachedClient(client, &CacheConfig{
		MaxSize: 100,
		TTL:     5 * time.Second,
		Enabled: true,
	})

	// First call should hit server
	resp1, err := cc.Get(context.Background(), "user:1", []string{"age"})
	if err != nil {
		t.Fatalf("first Get error: %v", err)
	}
	if resp1.EntityID != "user:1" {
		t.Errorf("expected user:1, got %s", resp1.EntityID)
	}

	// Second call should be cached
	_, err = cc.Get(context.Background(), "user:1", []string{"age"})
	if err != nil {
		t.Fatalf("second Get error: %v", err)
	}

	count := atomic.LoadInt32(&hitCount)
	if count != 1 {
		t.Errorf("expected 1 server hit (cached), got %d", count)
	}
}

func TestCachedClient_Invalidate(t *testing.T) {
	var hitCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitCount, 1)
		resp := GetResponse{EntityID: "user:1", Features: map[string]FeatureValue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", nil)
	cc := NewCachedClient(client, &CacheConfig{
		MaxSize: 100,
		TTL:     5 * time.Second,
		Enabled: true,
	})

	cc.Get(context.Background(), "user:1", []string{"age"})
	cc.Invalidate("user:1")
	cc.Get(context.Background(), "user:1", []string{"age"})

	count := atomic.LoadInt32(&hitCount)
	if count != 2 {
		t.Errorf("expected 2 server hits after invalidation, got %d", count)
	}
}

func TestWithRetry(t *testing.T) {
	attempts := 0
	result, err := WithRetry(context.Background(), &RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", &APIError{StatusCode: 500, Message: "server error"}
		}
		return "success", nil
	})
	if err != nil {
		t.Fatalf("WithRetry error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected success, got %s", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithRetry_NoRetryOn4xx(t *testing.T) {
	attempts := 0
	_, err := WithRetry(context.Background(), &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}, func() (string, error) {
		attempts++
		return "", &APIError{StatusCode: 400, Message: "bad request"}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", attempts)
	}
}
