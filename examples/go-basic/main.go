// Feather Go Example: Basic Feature CRUD
//
// Demonstrates storing and retrieving features using Feather's HTTP API.
// Uses only the Go standard library — no SDK or external dependencies needed.
//
// Prerequisites:
//
//	Start Feather: make run-dev
//
// Usage:
//
//	cd examples/go-basic && go run main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

const baseURL = "http://localhost:8080"

func main() {
	// Preflight: check server health
	if err := checkHealth(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ Cannot connect to Feather at", baseURL)
		fmt.Fprintln(os.Stderr, "   Start the server first: make run-dev")
		os.Exit(1)
	}
	fmt.Println("✅ Connected to Feather")

	// Step 1: Store features
	fmt.Println("\n── Step 1: Storing features ──")
	storeFeatures("user:go-example", map[string]interface{}{
		"click_count":    42,
		"purchase_total": 199.99,
		"last_activity":  "2024-06-15T12:00:00Z",
	})
	fmt.Println("  ✓ Stored features for user:go-example")

	// Step 2: Retrieve features
	fmt.Println("\n── Step 2: Retrieving features ──")
	features := getFeatures("user:go-example", []string{"click_count", "purchase_total"})
	prettyPrint(features)

	// Step 3: Batch retrieval
	fmt.Println("\n── Step 3: Batch retrieval ──")
	batch := batchGetFeatures(
		[]string{"user:go-example"},
		[]string{"click_count", "purchase_total"},
	)
	prettyPrint(batch)

	fmt.Println("\n✅ Done! See examples/README.md for more examples.")
}

func checkHealth() error {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func storeFeatures(entityKey string, features map[string]interface{}) {
	body := map[string]interface{}{
		"entity_key": entityKey,
		"features":   features,
	}
	post("/v1/features", body)
}

func getFeatures(entity string, features []string) map[string]interface{} {
	params := url.Values{"entity": {entity}}
	for _, f := range features {
		params.Add("feature", f)
	}
	return get("/v1/features?" + params.Encode())
}

func batchGetFeatures(entities, features []string) map[string]interface{} {
	body := map[string]interface{}{
		"entities": entities,
		"features": features,
	}
	return post("/v1/features/batch", body)
}

func get(path string) map[string]interface{} {
	resp, err := http.Get(baseURL + path)
	if err != nil {
		log.Fatalf("GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	return decodeJSON(resp.Body)
}

func post(path string, body interface{}) map[string]interface{} {
	data, _ := json.Marshal(body)
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Fatalf("POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	return decodeJSON(resp.Body)
}

func decodeJSON(r io.Reader) map[string]interface{} {
	var result map[string]interface{}
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		log.Fatalf("JSON decode failed: %v", err)
	}
	return result
}

func prettyPrint(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
