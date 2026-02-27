// Feather Go Quickstart - Get started in 30 seconds!
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/feather-store/feather/sdk/go/feather"
)

func main() {
	// 1. Connect to Feather
	client := feather.NewClient("http://localhost:8080", "", nil)

	ctx := context.Background()

	// 2. Store features for an entity
	err := client.Features.Put(ctx, &feather.PutRequest{
		EntityID: "user:123",
		Features: map[string]interface{}{
			"score":     0.95,
			"purchases": 42,
			"premium":   true,
		},
	})
	if err != nil {
		slog.Error("failed to store features", "error", err)
		os.Exit(1)
	}
	fmt.Println("Stored features for user:123")

	// 3. Retrieve features
	resp, err := client.Features.Get(ctx, "user:123", []string{"score", "purchases"})
	if err != nil {
		slog.Error("failed to get features", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Retrieved features for %s:\n", resp.EntityID)
	for name, fv := range resp.Features {
		fmt.Printf("  %s: %v (updated: %s)\n", name, fv.Value, fv.Timestamp.Format(time.RFC3339))
	}

	// 4. Batch retrieval (multiple entities)
	results, err := client.Features.GetBatch(ctx, []string{"user:123", "user:456"}, nil)
	if err != nil {
		slog.Error("failed to batch get", "error", err)
		os.Exit(1)
	}
	fmt.Printf("\nBatch retrieved %d entities\n", len(results))

	fmt.Println("\nQuickstart complete!")
}
