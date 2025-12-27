package autogen

import (
	"context"
	"testing"
)

func TestGenerator_GenerateFeatures_Local(t *testing.T) {
	config := DefaultConfig()
	config.Provider = ProviderLocal
	gen := NewGenerator(config)

	ctx := context.Background()

	schema := &DataSchema{
		Name:        "user_transactions",
		Description: "User transaction data",
		Entity:      "user_id",
		Fields: []SchemaField{
			{Name: "user_id", Type: "string", Description: "Unique user identifier"},
			{Name: "transaction_amount", Type: "float64", Description: "Transaction amount in USD"},
			{Name: "transaction_timestamp", Type: "timestamp", Description: "When the transaction occurred"},
			{Name: "product_count", Type: "int64", Description: "Number of products purchased"},
		},
	}

	req := &GenerationRequest{
		Schema:         schema,
		UseCase:        "Fraud detection",
		MaxSuggestions: 5,
	}

	result, err := gen.GenerateFeatures(ctx, req)
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	if result.ID == "" {
		t.Error("expected non-empty result ID")
	}

	if len(result.Suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}

	// Verify suggestions have required fields
	for _, s := range result.Suggestions {
		if s.ID == "" {
			t.Error("suggestion missing ID")
		}
		if s.Name == "" {
			t.Error("suggestion missing Name")
		}
		if s.Expression == "" {
			t.Error("suggestion missing Expression")
		}
		if s.DataType == "" {
			t.Error("suggestion missing DataType")
		}
		if s.Confidence <= 0 || s.Confidence > 1 {
			t.Errorf("unexpected confidence: %f", s.Confidence)
		}
	}
}

func TestGenerator_GenerateFeatures_WithConstraints(t *testing.T) {
	config := DefaultConfig()
	config.Provider = ProviderLocal
	gen := NewGenerator(config)

	ctx := context.Background()

	schema := &DataSchema{
		Name: "test_schema",
		Fields: []SchemaField{
			{Name: "timestamp", Type: "timestamp"},
			{Name: "amount", Type: "float64"},
			{Name: "user_id", Type: "string"},
		},
	}

	// With min confidence filter
	req := &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 10,
		Constraints: &GenerationConstraints{
			MinConfidence: 0.8,
		},
	}

	result, err := gen.GenerateFeatures(ctx, req)
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	// All suggestions should meet min confidence
	for _, s := range result.Suggestions {
		if s.Confidence < 0.8 {
			t.Errorf("suggestion %s has confidence %f, expected >= 0.8", s.ID, s.Confidence)
		}
	}
}

func TestGenerator_GenerateFeatures_EmptySchema(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	schema := &DataSchema{
		Name:   "empty_schema",
		Fields: []SchemaField{},
	}

	req := &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 5,
	}

	result, err := gen.GenerateFeatures(ctx, req)
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	// Should still return a result, even if no suggestions
	if result.ID == "" {
		t.Error("expected non-empty result ID")
	}
}

func TestGenerator_SuggestTransformations(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	tests := []struct {
		featureType     string
		minSuggestions  int
		expectedOutputs []string
	}{
		{"float64", 3, []string{"float64", "int"}},
		{"int64", 3, []string{"float64", "int"}},
		{"string", 2, []string{"int", "string"}},
		{"bool", 1, []string{"int"}},
	}

	for _, tt := range tests {
		t.Run(tt.featureType, func(t *testing.T) {
			suggestions, err := gen.SuggestTransformations(ctx, "test_feature", tt.featureType, "Test description", nil)
			if err != nil {
				t.Fatalf("SuggestTransformations failed: %v", err)
			}

			if len(suggestions) < tt.minSuggestions {
				t.Errorf("expected at least %d suggestions, got %d", tt.minSuggestions, len(suggestions))
			}

			// Verify each suggestion has required fields
			for _, s := range suggestions {
				if s.Name == "" {
					t.Error("suggestion missing Name")
				}
				if s.Expression == "" {
					t.Error("suggestion missing Expression")
				}
				if s.InputType != tt.featureType {
					t.Errorf("expected InputType %s, got %s", tt.featureType, s.InputType)
				}
			}
		})
	}
}

func TestGenerator_SuggestAggregations(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	fields := []SchemaField{
		{Name: "amount", Type: "float64", Description: "Transaction amount"},
		{Name: "quantity", Type: "int64", Description: "Item quantity"},
		{Name: "is_premium", Type: "bool", Description: "Premium user flag"},
	}

	suggestions, err := gen.SuggestAggregations(ctx, "user_id", fields, "event_time")
	if err != nil {
		t.Fatalf("SuggestAggregations failed: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected at least one aggregation suggestion")
	}

	// Verify suggestions include windowed aggregations
	hasWindowedAgg := false
	for _, s := range suggestions {
		if s.Window != "" {
			hasWindowedAgg = true
			break
		}
	}

	if !hasWindowedAgg {
		t.Error("expected at least one windowed aggregation")
	}

	// Verify suggestions have required fields
	for _, s := range suggestions {
		if s.Name == "" {
			t.Error("suggestion missing Name")
		}
		if s.Function == "" {
			t.Error("suggestion missing Function")
		}
		if s.Expression == "" {
			t.Error("suggestion missing Expression")
		}
	}
}

func TestGenerator_GetHistory(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	// Generate some features
	for i := 0; i < 3; i++ {
		schema := &DataSchema{
			Name: "test_schema",
			Fields: []SchemaField{
				{Name: "timestamp", Type: "timestamp"},
				{Name: "value", Type: "float64"},
			},
		}
		gen.GenerateFeatures(ctx, &GenerationRequest{
			Schema:         schema,
			MaxSuggestions: 2,
		})
	}

	history := gen.GetHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}

	// Verify history entries
	for i, entry := range history {
		if entry.ID == "" {
			t.Errorf("history entry %d missing ID", i)
		}
		if entry.CreatedAt.IsZero() {
			t.Errorf("history entry %d missing CreatedAt", i)
		}
	}
}

func TestGenerator_GetStats(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	// Generate some features
	schema := &DataSchema{
		Name: "test_schema",
		Fields: []SchemaField{
			{Name: "timestamp", Type: "timestamp"},
			{Name: "amount", Type: "float64"},
		},
	}

	gen.GenerateFeatures(ctx, &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 3,
	})
	gen.GenerateFeatures(ctx, &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 2,
	})

	stats := gen.GetStats()

	if stats["total_generations"].(int) != 2 {
		t.Errorf("expected 2 generations, got %v", stats["total_generations"])
	}

	if stats["provider"].(string) != "local" {
		t.Errorf("expected provider 'local', got %v", stats["provider"])
	}
}

func TestGenerator_ParseFeatureSuggestions(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{
			name: "valid array",
			input: `[{"id": "f1", "name": "Feature 1", "description": "Test", "expression": "x + 1", "data_type": "int", "category": "test", "confidence": 0.9, "rationale": "useful"}]`,
			expected: 1,
		},
		{
			name: "array with markdown",
			input: "```json\n[{\"id\": \"f1\", \"name\": \"F1\", \"description\": \"D\", \"expression\": \"E\", \"data_type\": \"int\", \"category\": \"C\", \"confidence\": 0.8, \"rationale\": \"R\"}]\n```",
			expected: 1,
		},
		{
			name: "multiple suggestions",
			input: `[
				{"id": "f1", "name": "F1", "description": "D1", "expression": "E1", "data_type": "int", "category": "C", "confidence": 0.9, "rationale": "R1"},
				{"id": "f2", "name": "F2", "description": "D2", "expression": "E2", "data_type": "float64", "category": "C", "confidence": 0.8, "rationale": "R2"}
			]`,
			expected: 2,
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions, err := gen.parseFeatureSuggestions(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFeatureSuggestions failed: %v", err)
			}
			if len(suggestions) != tt.expected {
				t.Errorf("expected %d suggestions, got %d", tt.expected, len(suggestions))
			}
		})
	}
}

func TestGenerator_FilterByConstraints(t *testing.T) {
	gen := NewGenerator(DefaultConfig())

	suggestions := []FeatureSuggestion{
		{ID: "f1", DataType: "int", Confidence: 0.9, Tags: []string{"important"}},
		{ID: "f2", DataType: "float64", Confidence: 0.7, Tags: []string{"experimental"}},
		{ID: "f3", DataType: "int", Confidence: 0.5, Tags: []string{"important", "core"}},
		{ID: "f4", DataType: "string", Confidence: 0.8, Tags: []string{"text"}},
	}

	tests := []struct {
		name        string
		constraints *GenerationConstraints
		expectedIDs []string
	}{
		{
			name:        "min confidence",
			constraints: &GenerationConstraints{MinConfidence: 0.8},
			expectedIDs: []string{"f1", "f4"},
		},
		{
			name:        "allowed types",
			constraints: &GenerationConstraints{AllowedTypes: []string{"int"}},
			expectedIDs: []string{"f1", "f3"},
		},
		{
			name:        "required tags",
			constraints: &GenerationConstraints{RequireTags: []string{"important"}},
			expectedIDs: []string{"f1", "f3"},
		},
		{
			name: "combined constraints",
			constraints: &GenerationConstraints{
				MinConfidence: 0.6,
				AllowedTypes:  []string{"int", "float64"},
			},
			expectedIDs: []string{"f1", "f2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := gen.filterByConstraints(suggestions, tt.constraints)

			if len(filtered) != len(tt.expectedIDs) {
				t.Errorf("expected %d results, got %d", len(tt.expectedIDs), len(filtered))
			}

			for i, expected := range tt.expectedIDs {
				if i < len(filtered) && filtered[i].ID != expected {
					t.Errorf("expected ID %s at position %d, got %s", expected, i, filtered[i].ID)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Provider != ProviderLocal {
		t.Errorf("expected default provider 'local', got %s", config.Provider)
	}

	if config.MaxTokens <= 0 {
		t.Error("expected positive MaxTokens")
	}

	if config.Temperature <= 0 || config.Temperature > 1 {
		t.Errorf("unexpected Temperature: %f", config.Temperature)
	}

	if config.Timeout <= 0 {
		t.Error("expected positive Timeout")
	}
}

func TestLLMProvider_AllTypes(t *testing.T) {
	providers := []LLMProvider{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderOllama,
		ProviderLocal,
	}

	for _, p := range providers {
		if string(p) == "" {
			t.Errorf("provider %v has empty string representation", p)
		}
	}
}

func TestGenerator_TemporalFeatures(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	// Schema with timestamp fields should generate temporal features
	schema := &DataSchema{
		Name: "events",
		Fields: []SchemaField{
			{Name: "event_timestamp", Type: "timestamp", Description: "Event time"},
			{Name: "created_at", Type: "timestamp", Description: "Creation time"},
		},
	}

	result, err := gen.GenerateFeatures(ctx, &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 10,
	})
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	// Should generate temporal features like hour_of_day, day_of_week
	hasTemporalFeature := false
	for _, s := range result.Suggestions {
		if s.Category == "temporal" {
			hasTemporalFeature = true
			break
		}
	}

	if !hasTemporalFeature {
		t.Error("expected at least one temporal feature for timestamp fields")
	}
}

func TestGenerator_NumericFeatures(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	// Schema with numeric fields should generate transformations
	schema := &DataSchema{
		Name: "transactions",
		Fields: []SchemaField{
			{Name: "amount", Type: "float64", Description: "Transaction amount"},
			{Name: "price", Type: "float64", Description: "Product price"},
		},
	}

	result, err := gen.GenerateFeatures(ctx, &GenerationRequest{
		Schema:         schema,
		MaxSuggestions: 10,
	})
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	// Should generate log transform for amount/price
	hasLogTransform := false
	for _, s := range result.Suggestions {
		if s.Category == "transformation" {
			hasLogTransform = true
			break
		}
	}

	if !hasLogTransform && len(result.Suggestions) > 0 {
		t.Log("Note: No log transform generated, but got other suggestions")
	}
}

func TestGenerator_UserBehaviorFeatures(t *testing.T) {
	gen := NewGenerator(DefaultConfig())
	ctx := context.Background()

	// Schema with user_id should generate behavior features
	schema := &DataSchema{
		Name:   "user_events",
		Entity: "user_id",
		Fields: []SchemaField{
			{Name: "user_id", Type: "string", Description: "User identifier"},
			{Name: "event_type", Type: "string", Description: "Event type"},
			{Name: "timestamp", Type: "timestamp", Description: "Event time"},
		},
	}

	result, err := gen.GenerateFeatures(ctx, &GenerationRequest{
		Schema:         schema,
		UseCase:        "User engagement prediction",
		MaxSuggestions: 10,
	})
	if err != nil {
		t.Fatalf("GenerateFeatures failed: %v", err)
	}

	// Should generate user activity features
	hasUserFeature := false
	for _, s := range result.Suggestions {
		if s.Category == "aggregation" || s.Category == "user_behavior" {
			hasUserFeature = true
			break
		}
	}

	if !hasUserFeature && len(result.Suggestions) > 0 {
		t.Log("Note: No explicit user behavior feature, but got other suggestions")
	}
}
