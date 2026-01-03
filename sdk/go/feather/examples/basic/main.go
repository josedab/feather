// Example: Basic feature store operations
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/feather-store/feather/sdk/go/feather"
)

func main() {
	// Create client with default configuration
	client := feather.NewClient("http://localhost:8080", "", nil)

	ctx := context.Background()

	// Store some features
	fmt.Println("Storing features...")
	err := client.Features.Put(ctx, &feather.PutRequest{
		EntityID: "user:123",
		Features: map[string]interface{}{
			"purchase_count":  42,
			"avg_order_value": 55.99,
			"last_login":      time.Now().Unix(),
		},
	})
	if err != nil {
		log.Fatalf("Failed to store features: %v", err)
	}
	fmt.Println("Features stored successfully")

	// Retrieve features
	fmt.Println("\nRetrieving features...")
	resp, err := client.Features.Get(ctx, "user:123", []string{
		"purchase_count",
		"avg_order_value",
		"last_login",
	})
	if err != nil {
		log.Fatalf("Failed to get features: %v", err)
	}

	fmt.Printf("Entity: %s\n", resp.EntityID)
	for name, val := range resp.Features {
		fmt.Printf("  %s: %v (version: %d)\n", name, val.Value, val.Version)
	}

	// Batch retrieval
	fmt.Println("\nBatch retrieving features...")
	results, err := client.Features.GetBatch(ctx,
		[]string{"user:123", "user:456", "user:789"},
		[]string{"purchase_count", "avg_order_value"},
	)
	if err != nil {
		log.Fatalf("Failed to batch get features: %v", err)
	}

	for entityID, data := range results {
		fmt.Printf("Entity %s:\n", entityID)
		for name, val := range data.Features {
			fmt.Printf("  %s: %v\n", name, val.Value)
		}
	}

	// Point-in-time retrieval
	fmt.Println("\nPoint-in-time retrieval...")
	asOf := time.Now().Add(-24 * time.Hour) // 24 hours ago
	histResp, err := client.Features.GetAsOf(ctx, "user:123", []string{"purchase_count"}, asOf)
	if err != nil {
		log.Printf("Historical lookup failed (may not have data): %v", err)
	} else {
		fmt.Printf("Features as of %s:\n", asOf.Format(time.RFC3339))
		for name, val := range histResp.Features {
			fmt.Printf("  %s: %v\n", name, val.Value)
		}
	}

	fmt.Println("\nDone!")
}
