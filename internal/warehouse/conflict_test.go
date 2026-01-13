package warehouse

import (
	"context"
	"testing"
	"time"
)

func TestConflictResolver_ResolveLatest(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)

	now := time.Now()
	earlier := now.Add(-time.Hour)

	tests := []struct {
		name           string
		sourceTime     time.Time
		targetTime     time.Time
		expectedWinner string
	}{
		{"source later", now, earlier, "source"},
		{"target later", earlier, now, "target"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := &Conflict{
				EntityID:    "entity-1",
				FeatureName: "feature-1",
				SourceValue: &FeatureConflictValue{
					Value:     "source-value",
					Timestamp: tt.sourceTime,
					Version:   1,
				},
				TargetValue: &FeatureConflictValue{
					Value:     "target-value",
					Timestamp: tt.targetTime,
					Version:   1,
				},
				ConflictType: ConflictTypeValueMismatch,
			}

			resolution, err := resolver.Resolve(context.Background(), conflict)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}

			if resolution.Winner != tt.expectedWinner {
				t.Errorf("Winner = %q, want %q", resolution.Winner, tt.expectedWinner)
			}
			if resolution.Strategy != ConflictResolutionLatest {
				t.Errorf("Strategy = %q, want %q", resolution.Strategy, ConflictResolutionLatest)
			}
		})
	}
}

func TestConflictResolver_ResolveSource(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionSource, nil)

	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{
			Value:     "source-value",
			Timestamp: time.Now().Add(-time.Hour), // Earlier
			Version:   1,
		},
		TargetValue: &FeatureConflictValue{
			Value:     "target-value",
			Timestamp: time.Now(), // Later
			Version:   2,
		},
		ConflictType: ConflictTypeValueMismatch,
	}

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	if resolution.Winner != "source" {
		t.Errorf("Winner = %q, want %q", resolution.Winner, "source")
	}
	if resolution.ResolvedValue != "source-value" {
		t.Errorf("ResolvedValue = %v, want %v", resolution.ResolvedValue, "source-value")
	}
}

func TestConflictResolver_ResolveTarget(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionTarget, nil)

	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{
			Value:     "source-value",
			Timestamp: time.Now(),
			Version:   2,
		},
		TargetValue: &FeatureConflictValue{
			Value:     "target-value",
			Timestamp: time.Now().Add(-time.Hour),
			Version:   1,
		},
		ConflictType: ConflictTypeValueMismatch,
	}

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	if resolution.Winner != "target" {
		t.Errorf("Winner = %q, want %q", resolution.Winner, "target")
	}
	if resolution.ResolvedValue != "target-value" {
		t.Errorf("ResolvedValue = %v, want %v", resolution.ResolvedValue, "target-value")
	}
}

func TestConflictResolver_ResolveHigherVersion(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionHigherVer, nil)

	tests := []struct {
		name           string
		sourceVersion  int64
		targetVersion  int64
		expectedWinner string
	}{
		{"source higher", 5, 3, "source"},
		{"target higher", 2, 4, "target"},
		{"equal versions", 3, 3, "target"}, // Falls back to latest
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := &Conflict{
				EntityID:    "entity-1",
				FeatureName: "feature-1",
				SourceValue: &FeatureConflictValue{
					Value:     "source-value",
					Timestamp: time.Now().Add(-time.Hour),
					Version:   tt.sourceVersion,
				},
				TargetValue: &FeatureConflictValue{
					Value:     "target-value",
					Timestamp: time.Now(),
					Version:   tt.targetVersion,
				},
				ConflictType: ConflictTypeVersionConflict,
			}

			resolution, err := resolver.Resolve(context.Background(), conflict)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}

			if resolution.Winner != tt.expectedWinner {
				t.Errorf("Winner = %q, want %q", resolution.Winner, tt.expectedWinner)
			}
		})
	}
}

func TestConflictResolver_ResolveMerge_Maps(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionMerge, nil)

	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{
			Value: map[string]interface{}{
				"key1": "source-value1",
				"key2": "source-value2",
			},
			Timestamp: time.Now(),
			Version:   1,
		},
		TargetValue: &FeatureConflictValue{
			Value: map[string]interface{}{
				"key1": "target-value1",
				"key3": "target-value3",
			},
			Timestamp: time.Now().Add(-time.Hour),
			Version:   1,
		},
		ConflictType: ConflictTypeValueMismatch,
	}

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	if resolution.Winner != "merged" {
		t.Errorf("Winner = %q, want %q", resolution.Winner, "merged")
	}

	merged, ok := resolution.ResolvedValue.(map[string]interface{})
	if !ok {
		t.Fatalf("ResolvedValue is not a map")
	}

	// Source wins on key1
	if merged["key1"] != "source-value1" {
		t.Errorf("merged[key1] = %v, want source-value1", merged["key1"])
	}
	// Source key2 included
	if merged["key2"] != "source-value2" {
		t.Errorf("merged[key2] = %v, want source-value2", merged["key2"])
	}
	// Target key3 included
	if merged["key3"] != "target-value3" {
		t.Errorf("merged[key3] = %v, want target-value3", merged["key3"])
	}
}

func TestConflictResolver_ResolveMerge_Slices(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionMerge, nil)

	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{
			Value:     []interface{}{"a", "b"},
			Timestamp: time.Now(),
			Version:   1,
		},
		TargetValue: &FeatureConflictValue{
			Value:     []interface{}{"c", "d"},
			Timestamp: time.Now().Add(-time.Hour),
			Version:   1,
		},
		ConflictType: ConflictTypeValueMismatch,
	}

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	if resolution.Winner != "merged" {
		t.Errorf("Winner = %q, want %q", resolution.Winner, "merged")
	}

	merged, ok := resolution.ResolvedValue.([]interface{})
	if !ok {
		t.Fatalf("ResolvedValue is not a slice")
	}

	if len(merged) != 4 {
		t.Errorf("merged len = %d, want 4", len(merged))
	}
}

func TestConflictResolver_ResolveMerge_NonMergeable(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionMerge, nil)

	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{
			Value:     "string-value",
			Timestamp: time.Now().Add(-time.Hour),
			Version:   1,
		},
		TargetValue: &FeatureConflictValue{
			Value:     int64(42),
			Timestamp: time.Now(),
			Version:   1,
		},
		ConflictType: ConflictTypeValueMismatch,
	}

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	// Should fall back to latest
	if resolution.Winner != "target" {
		t.Errorf("Winner = %q, want %q (fallback to latest)", resolution.Winner, "target")
	}
}

func TestConflictResolver_ResolveCustom(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionCustom, nil)

	// Without handler should fail
	conflict := &Conflict{
		EntityID:    "entity-1",
		FeatureName: "feature-1",
		SourceValue: &FeatureConflictValue{Value: "source"},
		TargetValue: &FeatureConflictValue{Value: "target"},
	}

	_, err := resolver.Resolve(context.Background(), conflict)
	if err == nil {
		t.Error("expected error without custom handler")
	}

	// With handler should succeed
	resolver.SetCustomHandler(func(ctx context.Context, c *Conflict) (*Resolution, error) {
		return &Resolution{
			ResolvedValue: "custom-value",
			Winner:        "custom",
			Strategy:      ConflictResolutionCustom,
		}, nil
	})

	resolution, err := resolver.Resolve(context.Background(), conflict)
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}

	if resolution.Winner != "custom" {
		t.Errorf("Winner = %q, want %q", resolution.Winner, "custom")
	}
	if resolution.ResolvedValue != "custom-value" {
		t.Errorf("ResolvedValue = %v, want %v", resolution.ResolvedValue, "custom-value")
	}
}

func TestConflictResolver_SetStrategy(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)

	if resolver.GetStrategy() != ConflictResolutionLatest {
		t.Errorf("GetStrategy() = %q, want %q", resolver.GetStrategy(), ConflictResolutionLatest)
	}

	resolver.SetStrategy(ConflictResolutionSource)

	if resolver.GetStrategy() != ConflictResolutionSource {
		t.Errorf("GetStrategy() = %q, want %q", resolver.GetStrategy(), ConflictResolutionSource)
	}
}

func TestConflictResolver_Stats(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)

	// Resolve some conflicts
	for i := 0; i < 5; i++ {
		conflict := &Conflict{
			EntityID:     "entity-1",
			FeatureName:  "feature-1",
			SourceValue:  &FeatureConflictValue{Value: "source", Timestamp: time.Now()},
			TargetValue:  &FeatureConflictValue{Value: "target", Timestamp: time.Now().Add(-time.Hour)},
			ConflictType: ConflictTypeValueMismatch,
		}
		_, _ = resolver.Resolve(context.Background(), conflict)
	}

	stats := resolver.Stats()
	if stats["conflicts_resolved"].(int64) != 5 {
		t.Errorf("conflicts_resolved = %v, want 5", stats["conflicts_resolved"])
	}

	byType := stats["conflicts_by_type"].(map[string]int64)
	if byType[string(ConflictTypeValueMismatch)] != 5 {
		t.Errorf("conflicts_by_type[value_mismatch] = %v, want 5", byType[string(ConflictTypeValueMismatch)])
	}

	// Reset stats
	resolver.ResetStats()
	stats = resolver.Stats()
	if stats["conflicts_resolved"].(int64) != 0 {
		t.Errorf("conflicts_resolved after reset = %v, want 0", stats["conflicts_resolved"])
	}
}

func TestConflictResolver_DetectConflict(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)

	now := time.Now()
	tolerance := time.Second

	tests := []struct {
		name           string
		sourceVal      interface{}
		targetVal      interface{}
		sourceTime     time.Time
		targetTime     time.Time
		sourceVersion  int64
		targetVersion  int64
		expectConflict bool
		expectedType   ConflictType
	}{
		{
			name:           "no conflict - same value",
			sourceVal:      "same",
			targetVal:      "same",
			sourceTime:     now,
			targetTime:     now.Add(-time.Hour),
			expectConflict: false,
		},
		{
			name:           "concurrent update",
			sourceVal:      "source",
			targetVal:      "target",
			sourceTime:     now,
			targetTime:     now.Add(500 * time.Millisecond),
			expectConflict: true,
			expectedType:   ConflictTypeConcurrentUpdate,
		},
		{
			name:           "version conflict",
			sourceVal:      "source",
			targetVal:      "target",
			sourceTime:     now,
			targetTime:     now.Add(-time.Hour),
			sourceVersion:  5,
			targetVersion:  5,
			expectConflict: true,
			expectedType:   ConflictTypeVersionConflict,
		},
		{
			name:           "value mismatch",
			sourceVal:      "source",
			targetVal:      "target",
			sourceTime:     now,
			targetTime:     now.Add(-time.Hour),
			sourceVersion:  2,
			targetVersion:  1,
			expectConflict: true,
			expectedType:   ConflictTypeValueMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceVal := &FeatureConflictValue{
				Value:     tt.sourceVal,
				Timestamp: tt.sourceTime,
				Version:   tt.sourceVersion,
			}
			targetVal := &FeatureConflictValue{
				Value:     tt.targetVal,
				Timestamp: tt.targetTime,
				Version:   tt.targetVersion,
			}

			conflict := resolver.DetectConflict("entity-1", "feature-1", sourceVal, targetVal, tolerance)

			if tt.expectConflict {
				if conflict == nil {
					t.Error("expected conflict but got nil")
					return
				}
				if conflict.ConflictType != tt.expectedType {
					t.Errorf("ConflictType = %q, want %q", conflict.ConflictType, tt.expectedType)
				}
			} else {
				if conflict != nil {
					t.Errorf("expected no conflict but got %v", conflict)
				}
			}
		})
	}
}

func TestBatchConflictResolver(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)
	batch := NewBatchConflictResolver(resolver)

	// Add conflicts
	for i := 0; i < 10; i++ {
		conflict := &Conflict{
			EntityID:     "entity-1",
			FeatureName:  "feature-1",
			SourceValue:  &FeatureConflictValue{Value: "source", Timestamp: time.Now()},
			TargetValue:  &FeatureConflictValue{Value: "target", Timestamp: time.Now().Add(-time.Hour)},
			ConflictType: ConflictTypeValueMismatch,
		}
		batch.AddConflict(conflict)
	}

	conflicts := batch.GetConflicts()
	if len(conflicts) != 10 {
		t.Errorf("GetConflicts() len = %d, want 10", len(conflicts))
	}

	// Resolve all
	resolutions, err := batch.ResolveAll(context.Background())
	if err != nil {
		t.Fatalf("ResolveAll error = %v", err)
	}

	if len(resolutions) != 10 {
		t.Errorf("resolutions len = %d, want 10", len(resolutions))
	}

	// All should be resolved
	for i, res := range resolutions {
		if res == nil {
			t.Errorf("resolution[%d] is nil", i)
		}
	}

	// Clear
	batch.Clear()
	conflicts = batch.GetConflicts()
	if len(conflicts) != 0 {
		t.Errorf("GetConflicts() after clear len = %d, want 0", len(conflicts))
	}
}

func TestConflictLog(t *testing.T) {
	log := NewConflictLog(5)

	// Log some conflicts
	for i := 0; i < 10; i++ {
		conflict := &Conflict{
			EntityID:    "entity-1",
			FeatureName: "feature-1",
		}
		resolution := &Resolution{
			Winner: "source",
		}
		log.Log(conflict, resolution)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	// Should only keep last 5
	entries := log.GetEntries(time.Time{})
	if len(entries) != 5 {
		t.Errorf("GetEntries() len = %d, want 5", len(entries))
	}

	// Filter by time
	since := time.Now().Add(-time.Millisecond * 3)
	recentEntries := log.GetEntries(since)
	if len(recentEntries) > len(entries) {
		t.Errorf("filtered entries should not exceed total entries")
	}

	// Clear
	log.Clear()
	entries = log.GetEntries(time.Time{})
	if len(entries) != 0 {
		t.Errorf("GetEntries() after clear len = %d, want 0", len(entries))
	}
}

func TestConflictLog_DefaultSize(t *testing.T) {
	log := NewConflictLog(0) // Should default to 1000

	// Should not panic with many entries
	for i := 0; i < 100; i++ {
		log.Log(&Conflict{}, &Resolution{})
	}

	entries := log.GetEntries(time.Time{})
	if len(entries) != 100 {
		t.Errorf("GetEntries() len = %d, want 100", len(entries))
	}
}

func TestValuesEqual(t *testing.T) {
	resolver := NewConflictResolver(ConflictResolutionLatest, nil)

	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"one nil", nil, "value", false},
		{"same int64", int64(42), int64(42), true},
		{"diff int64", int64(42), int64(43), false},
		{"same float64", float64(3.14), float64(3.14), true},
		{"diff float64", float64(3.14), float64(2.71), false},
		{"same string", "hello", "hello", true},
		{"diff string", "hello", "world", false},
		{"same bool", true, true, true},
		{"diff bool", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.valuesEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestConflictTypes(t *testing.T) {
	types := []ConflictType{
		ConflictTypeValueMismatch,
		ConflictTypeVersionConflict,
		ConflictTypeConcurrentUpdate,
		ConflictTypeDeleteUpdate,
		ConflictTypeSchemaMismatch,
	}

	expected := []string{
		"value_mismatch",
		"version_conflict",
		"concurrent_update",
		"delete_update",
		"schema_mismatch",
	}

	for i, ct := range types {
		if string(ct) != expected[i] {
			t.Errorf("ConflictType = %q, want %q", ct, expected[i])
		}
	}
}

func TestConflictResolutionTypes(t *testing.T) {
	types := []ConflictResolution{
		ConflictResolutionLatest,
		ConflictResolutionSource,
		ConflictResolutionTarget,
		ConflictResolutionHigherVer,
		ConflictResolutionMerge,
		ConflictResolutionCustom,
	}

	expected := []string{
		"latest",
		"source",
		"target",
		"higher_ver",
		"merge",
		"custom",
	}

	for i, cr := range types {
		if string(cr) != expected[i] {
			t.Errorf("ConflictResolution = %q, want %q", cr, expected[i])
		}
	}
}
