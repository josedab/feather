// Package feather provides a Go client SDK for the Feather Feature Store.
//
// It offers a type-safe, idiomatic Go interface for interacting with the
// feature store including feature retrieval, storage, batch operations,
// and real-time aggregations. The SDK includes connection pooling, automatic
// retries with exponential backoff, and optional client-side caching.
//
// Basic usage:
//
//	client, err := feather.NewClient("http://localhost:8080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Get features
//	resp, err := client.Features.Get(ctx, "user:123", []string{"purchase_count"})
//
//	// Store features
//	err = client.Features.Put(ctx, &feather.PutRequest{
//	    EntityID: "user:123",
//	    Features: map[string]interface{}{"purchase_count": 42},
//	})
//
// For high-throughput scenarios, use the BatchClient:
//
//	batch := feather.NewBatchClient(client, 100, time.Second)
//	defer batch.Close()
//	err := batch.Put(ctx, "user:123", features)
package feather
