package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/export"
	"github.com/feather-store/feather/internal/ingestion"
	"github.com/feather-store/feather/internal/server"
	"github.com/feather-store/feather/internal/storage"
)

// TestIntegration_EndToEnd tests the full feature store workflow.
func TestIntegration_EndToEnd(t *testing.T) {
	// Create components
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	// Register a feature group
	group := &domain.FeatureGroup{
		Name:       "user_features",
		EntityType: "user",
		TTL:        24 * time.Hour,
		Features: []domain.FeatureSpec{
			{Name: "click_count", DataType: domain.DataTypeInt64},
			{Name: "purchase_total", DataType: domain.DataTypeFloat64},
			{Name: "click_count_1h", DataType: domain.DataTypeFloat64,
				Aggregation: &domain.AggregationSpec{
					Function: domain.AggCount,
					Window:   time.Hour,
				}},
		},
	}
	if err := schema.RegisterGroup(group); err != nil {
		t.Fatalf("Failed to register group: %v", err)
	}

	// Register aggregations
	for _, feature := range group.Features {
		if feature.Aggregation != nil {
			agg.RegisterAggregation(feature.Name, feature.Aggregation)
		}
	}

	// Create store
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 100, // 100MB
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create HTTP server
	httpServer := server.NewHTTPServer(
		store, agg, schema, nil,
		server.HTTPServerConfig{
			Port:         0, // Use any available port
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	)

	// Test 1: Store features via HTTP
	t.Run("StoreFeatures", func(t *testing.T) {
		update := domain.FeatureUpdate{
			EntityKey: "user:123",
			Features: map[string]interface{}{
				"click_count":    15,
				"purchase_total": 245.50,
			},
		}

		body, _ := json.Marshal(update)
		req := httptest.NewRequest("POST", "/v1/features", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		httpServer.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test 2: Retrieve features
	t.Run("GetFeatures", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/features?entity=user:123&feature=click_count&feature=purchase_total", nil)
		w := httptest.NewRecorder()

		httpServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Unwrap APIResponse
		var apiResp domain.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &apiResp); err != nil {
			t.Fatalf("Failed to unmarshal API response: %v", err)
		}

		if !apiResp.Success {
			t.Fatalf("Expected success=true, got false")
		}

		// Parse the data field as GetFeaturesResponse
		dataBytes, err := json.Marshal(apiResp.Data)
		if err != nil {
			t.Fatalf("Failed to marshal data: %v", err)
		}
		var resp domain.GetFeaturesResponse
		if err := json.Unmarshal(dataBytes, &resp); err != nil {
			t.Fatalf("Failed to unmarshal response data: %v", err)
		}

		if len(resp.Entities) != 1 {
			t.Errorf("Expected 1 entity, got %d", len(resp.Entities))
		}

		entityFeatures := resp.Entities["user:123"]
		if entityFeatures == nil {
			t.Fatal("Expected user:123 in response")
		}

		if len(entityFeatures.Features) != 2 {
			t.Errorf("Expected 2 features, got %d", len(entityFeatures.Features))
		}
	})

	// Test 3: Batch request
	t.Run("BatchGetFeatures", func(t *testing.T) {
		// Add another entity
		update := domain.FeatureUpdate{
			EntityKey: "user:456",
			Features: map[string]interface{}{
				"click_count":    30,
				"purchase_total": 100.00,
			},
		}
		body, _ := json.Marshal(update)
		req := httptest.NewRequest("POST", "/v1/features", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		httpServer.ServeHTTP(w, req)

		// Batch request
		batchReq := domain.GetFeaturesRequest{
			Entities: []string{"user:123", "user:456"},
			Features: []string{"click_count", "purchase_total"},
		}
		body, _ = json.Marshal(batchReq)
		req = httptest.NewRequest("POST", "/v1/features/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()

		httpServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Unwrap APIResponse
		var apiResp domain.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &apiResp); err != nil {
			t.Fatalf("Failed to unmarshal API response: %v", err)
		}

		if !apiResp.Success {
			t.Fatalf("Expected success=true, got false")
		}

		// Parse the data field as GetFeaturesResponse
		dataBytes, err := json.Marshal(apiResp.Data)
		if err != nil {
			t.Fatalf("Failed to marshal data: %v", err)
		}
		var resp domain.GetFeaturesResponse
		if err := json.Unmarshal(dataBytes, &resp); err != nil {
			t.Fatalf("Failed to unmarshal response data: %v", err)
		}

		if len(resp.Entities) != 2 {
			t.Errorf("Expected 2 entities, got %d", len(resp.Entities))
		}
	})

	// Test 4: Aggregations
	t.Run("Aggregations", func(t *testing.T) {
		// Add some click events
		now := time.Now()
		for i := 0; i < 10; i++ {
			agg.Update("user:789", "click_count_1h", 1.0, now.Add(-time.Duration(i)*time.Minute))
		}

		// Get the aggregated count
		count, err := agg.Compute("user:789", "click_count_1h", domain.AggCount)
		if err != nil {
			t.Fatalf("Failed to compute aggregation: %v", err)
		}

		if count != 10 {
			t.Errorf("Expected click_count_1h=10, got %f", count)
		}
	})

	// Test 5: Health endpoint
	t.Run("HealthCheck", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		httpServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

// TestIntegration_Export tests the export functionality.
func TestIntegration_Export(t *testing.T) {
	// Create components
	schema := storage.NewRegistry()

	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 10,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add some data
	for i := 0; i < 10; i++ {
		entityKey := "user:" + string(rune('A'+i))
		features := map[string]*domain.FeatureValue{
			"score": {
				Value:     float64(i * 10),
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			},
			"level": {
				Value:     int64(i + 1),
				Timestamp: time.Now().UnixNano(),
				Version:   1,
			},
		}
		if err := store.Put(entityKey, features); err != nil {
			t.Fatalf("Failed to put features: %v", err)
		}
	}

	// Test CSV export
	exporter := export.NewExporter(store, schema)

	t.Run("ExportCSV", func(t *testing.T) {
		entities := make([]string, 10)
		for i := 0; i < 10; i++ {
			entities[i] = "user:" + string(rune('A'+i))
		}

		result, err := exporter.Export(context.Background(), export.ExportRequest{
			Entities:   entities,
			Features:   []string{"score", "level"},
			Format:     export.FormatCSV,
			OutputPath: "/tmp/feather_test_export.csv",
		})

		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		if result.EntitiesExported != 10 {
			t.Errorf("Expected 10 entities exported, got %d", result.EntitiesExported)
		}

		if result.RowsWritten != 10 {
			t.Errorf("Expected 10 rows written, got %d", result.RowsWritten)
		}
	})

	t.Run("ExportJSON", func(t *testing.T) {
		entities := make([]string, 5)
		for i := 0; i < 5; i++ {
			entities[i] = "user:" + string(rune('A'+i))
		}

		result, err := exporter.Export(context.Background(), export.ExportRequest{
			Entities:   entities,
			Features:   []string{"score"},
			Format:     export.FormatJSON,
			OutputPath: "/tmp/feather_test_export.json",
		})

		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		if result.EntitiesExported != 5 {
			t.Errorf("Expected 5 entities exported, got %d", result.EntitiesExported)
		}
	})
}

// TestIntegration_BatchImport tests the batch import functionality.
func TestIntegration_BatchImport(t *testing.T) {
	schema := storage.NewRegistry()
	agg := aggregation.NewEngine()

	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1024 * 1024 * 10,
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	importer := ingestion.NewBatchImporter(store, agg, schema)

	t.Run("ImportCSV", func(t *testing.T) {
		csvData := `entity_key,click_count,purchase_total
user:A,10,100.50
user:B,20,200.75
user:C,30,300.00`

		result, err := importer.ImportCSVReader(context.Background(),
			strings.NewReader(csvData),
			ingestion.ImportConfig{
				EntityKeyColumn: "entity_key",
				HasHeader:       true,
			})

		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if result.RowsSuccess != 3 {
			t.Errorf("Expected 3 rows success, got %d", result.RowsSuccess)
		}

		// Verify data was imported
		features, err := store.Get("user:A", []string{"click_count", "purchase_total"})
		if err != nil {
			t.Fatalf("Failed to get features: %v", err)
		}

		if len(features) != 2 {
			t.Errorf("Expected 2 features, got %d", len(features))
		}
	})

	t.Run("ImportJSONL", func(t *testing.T) {
		jsonlData := `{"entity_key":"user:X","score":100,"level":5}
{"entity_key":"user:Y","score":200,"level":10}
{"entity_key":"user:Z","score":300,"level":15}`

		result, err := importer.ImportJSONLReader(context.Background(),
			strings.NewReader(jsonlData),
			ingestion.ImportConfig{
				EntityKeyColumn: "entity_key",
			})

		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		if result.RowsSuccess != 3 {
			t.Errorf("Expected 3 rows success, got %d", result.RowsSuccess)
		}

		// Verify data was imported
		features, err := store.Get("user:X", []string{"score", "level"})
		if err != nil {
			t.Fatalf("Failed to get features: %v", err)
		}

		if len(features) != 2 {
			t.Errorf("Expected 2 features, got %d", len(features))
		}
	})
}
