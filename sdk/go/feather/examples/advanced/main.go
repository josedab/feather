// Example: Advanced usage with connection pooling, async, and caching
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/feather-store/feather/sdk/go/feather"
)

func main() {
	// Custom configuration with tuned settings
	config := &feather.ClientConfig{
		Timeout:         10 * time.Second,
		MaxRetries:      5,
		RetryBackoff:    50 * time.Millisecond,
		MaxRetryBackoff: 5 * time.Second,
		RetryJitter:     0.25, // 25% jitter
		MaxIdleConns:    200,
		IdleConnTimeout: 120 * time.Second,
	}

	// Create a connection pool for high-throughput scenarios
	pool := feather.NewConnectionPool(
		"http://localhost:8080",
		"",
		10, // 10 clients in the pool
		config,
	)
	defer pool.Close()

	ctx := context.Background()

	// Example 1: Parallel reads using connection pool
	fmt.Println("=== Connection Pool Example ===")
	demonstrateConnectionPool(ctx, pool)

	// Example 2: Async client for non-blocking operations
	fmt.Println("\n=== Async Client Example ===")
	client := pool.Get()
	demonstrateAsyncClient(ctx, client)

	// Example 3: Cached client for read-heavy workloads
	fmt.Println("\n=== Cached Client Example ===")
	demonstrateCachedClient(ctx, client)

	// Example 4: Custom retry logic
	fmt.Println("\n=== Retry Example ===")
	demonstrateRetry(ctx, client)

	fmt.Println("\nAll examples completed!")
}

func demonstrateConnectionPool(ctx context.Context, pool *feather.ConnectionPool) {
	var wg sync.WaitGroup
	results := make(chan string, 100)

	// Parallel reads using different clients from the pool
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			client := pool.Get() // Round-robin client selection
			entityID := fmt.Sprintf("user:%d", idx%10)

			resp, err := client.Features.Get(ctx, entityID, []string{"feature1"})
			if err != nil {
				results <- fmt.Sprintf("user:%d - error: %v", idx%10, err)
			} else {
				results <- fmt.Sprintf("user:%d - got %d features", idx%10, len(resp.Features))
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	errorCount := 0
	for result := range results {
		if len(result) > 0 && result[len(result)-1] != ')' {
			successCount++
		} else {
			errorCount++
		}
	}
	fmt.Printf("Completed %d requests (%d success, %d errors)\n", 100, successCount, errorCount)
}

func demonstrateAsyncClient(ctx context.Context, client *feather.Client) {
	async := feather.NewAsyncClient(client)

	// Fire off multiple async requests
	requests := []feather.GetRequest{
		{EntityID: "user:1", Features: []string{"score", "rank"}},
		{EntityID: "user:2", Features: []string{"score", "rank"}},
		{EntityID: "user:3", Features: []string{"score", "rank"}},
	}

	// Start async operations
	fmt.Println("Starting parallel async requests...")
	start := time.Now()
	results := async.ParallelGet(ctx, requests)
	elapsed := time.Since(start)

	fmt.Printf("Completed %d parallel requests in %v\n", len(results), elapsed)
	for entityID, data := range results {
		if data != nil {
			fmt.Printf("  %s: %d features\n", entityID, len(data.Features))
		}
	}

	// Single async get with channel
	fmt.Println("\nSingle async get:")
	resultCh := async.GetAsync(ctx, "user:1", []string{"score"})

	// Do other work while waiting...
	fmt.Println("  Doing other work...")

	// Get result when ready
	result := <-resultCh
	if result.Err != nil {
		fmt.Printf("  Error: %v\n", result.Err)
	} else {
		fmt.Printf("  Got result: %d features\n", len(result.Value.Features))
	}
}

func demonstrateCachedClient(ctx context.Context, client *feather.Client) {
	cached := feather.NewCachedClient(client, &feather.CacheConfig{
		MaxSize: 1000,
		TTL:     30 * time.Second,
		Enabled: true,
	})

	entityID := "user:cache-test"
	features := []string{"score"}

	// First call - cache miss
	start := time.Now()
	_, err := cached.Get(ctx, entityID, features)
	firstCallTime := time.Since(start)
	if err != nil {
		fmt.Printf("First call (cache miss): %v, took %v\n", err, firstCallTime)
	} else {
		fmt.Printf("First call (cache miss): took %v\n", firstCallTime)
	}

	// Second call - cache hit (should be faster)
	start = time.Now()
	_, err = cached.Get(ctx, entityID, features)
	secondCallTime := time.Since(start)
	if err != nil {
		fmt.Printf("Second call (cache hit): %v, took %v\n", err, secondCallTime)
	} else {
		fmt.Printf("Second call (cache hit): took %v\n", secondCallTime)
	}

	// Invalidate and try again
	cached.Invalidate(entityID)
	fmt.Println("Cache invalidated")

	start = time.Now()
	_, err = cached.Get(ctx, entityID, features)
	thirdCallTime := time.Since(start)
	if err != nil {
		fmt.Printf("Third call (after invalidation): %v, took %v\n", err, thirdCallTime)
	} else {
		fmt.Printf("Third call (after invalidation): took %v\n", thirdCallTime)
	}
}

func demonstrateRetry(ctx context.Context, client *feather.Client) {
	// Custom retry configuration
	retryConfig := &feather.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		Multiplier:     2.0,
	}

	fmt.Println("Executing with custom retry logic...")
	start := time.Now()

	result, err := feather.WithRetry(ctx, retryConfig, func() (*feather.GetResponse, error) {
		return client.Features.Get(ctx, "user:retry-test", []string{"score"})
	})

	elapsed := time.Since(start)
	if err != nil {
		if apiErr, ok := err.(*feather.APIError); ok {
			fmt.Printf("API error after retries: status=%d, message=%s, took %v\n",
				apiErr.StatusCode, apiErr.Message, elapsed)
		} else {
			log.Printf("Network error after retries: %v, took %v\n", err, elapsed)
		}
	} else {
		fmt.Printf("Success after retries: got %d features, took %v\n",
			len(result.Features), elapsed)
	}
}
