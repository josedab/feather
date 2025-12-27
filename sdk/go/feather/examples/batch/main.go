// Example: Batch operations with BatchClient
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
	// Create base client
	client := feather.NewClient("http://localhost:8080", "", nil)

	// Create batch client for efficient bulk writes
	// Batch size: 100 items, flush interval: 1 second
	batch := feather.NewBatchClient(client, 100, time.Second)
	defer batch.Close()

	ctx := context.Background()

	fmt.Println("Queueing batch writes...")

	// Simulate high-throughput writes
	var wg sync.WaitGroup
	errChan := make(chan error, 1000)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			err := batch.Put(ctx, fmt.Sprintf("user:%d", userID), map[string]interface{}{
				"activity_score": float64(userID%100) / 100.0,
				"last_action":    time.Now().Unix(),
				"session_count":  userID % 50,
			})
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errorCount := 0
	for err := range errChan {
		errorCount++
		if errorCount == 1 {
			log.Printf("First error: %v", err)
		}
	}

	if errorCount > 0 {
		log.Printf("Total errors: %d", errorCount)
	} else {
		fmt.Println("All 1000 writes completed successfully!")
	}

	fmt.Println("\nDone!")
}
