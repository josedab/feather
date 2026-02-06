package registry

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewCatalog(t *testing.T) {
	c := NewCatalog()
	if c == nil {
		t.Fatal("NewCatalog returned nil")
	}

	if c.features == nil {
		t.Error("features map not initialized")
	}
	if c.byTag == nil {
		t.Error("byTag map not initialized")
	}
	if c.byOwner == nil {
		t.Error("byOwner map not initialized")
	}
	if c.byTeam == nil {
		t.Error("byTeam map not initialized")
	}
	if c.byCategory == nil {
		t.Error("byCategory map not initialized")
	}
	if c.byEntity == nil {
		t.Error("byEntity map not initialized")
	}
	if c.versions == nil {
		t.Error("versions map not initialized")
	}
}

func TestCatalog_Register_Validation(t *testing.T) {
	c := NewCatalog()

	// Test empty name
	err := c.Register(&FeatureDefinition{}, "test-user")
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestCatalog_Register_NewFeature(t *testing.T) {
	c := NewCatalog()

	def := &FeatureDefinition{
		Name:        "test_feature",
		Description: "A test feature",
		DataType:    "float",
		EntityType:  "user",
		Owner:       "owner1",
		Team:        "team1",
		Category:    "behavioral",
		Tags:        []string{"tag1", "tag2"},
	}

	err := c.Register(def, "test-user")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify the feature was registered
	got := c.Get("test_feature")
	if got == nil {
		t.Fatal("feature not found after registration")
	}

	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if got.Status != StatusDraft {
		t.Errorf("expected status draft, got %s", got.Status)
	}
	if got.CreatedBy != "test-user" {
		t.Errorf("expected CreatedBy 'test-user', got '%s'", got.CreatedBy)
	}
	if got.UpdatedBy != "test-user" {
		t.Errorf("expected UpdatedBy 'test-user', got '%s'", got.UpdatedBy)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestCatalog_Register_UpdateExisting(t *testing.T) {
	c := NewCatalog()

	// Register initial feature
	def := &FeatureDefinition{
		Name:        "test_feature",
		Description: "Original description",
		DataType:    "float",
		EntityType:  "user",
		Owner:       "owner1",
		Team:        "team1",
	}
	err := c.Register(def, "creator")
	if err != nil {
		t.Fatalf("initial Register failed: %v", err)
	}

	originalCreatedAt := c.Get("test_feature").CreatedAt

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Update the feature
	updated := &FeatureDefinition{
		Name:        "test_feature",
		Description: "Updated description",
		DataType:    "int",
		EntityType:  "user",
		Owner:       "owner2",
		Team:        "team2",
	}
	err = c.Register(updated, "updater")
	if err != nil {
		t.Fatalf("update Register failed: %v", err)
	}

	got := c.Get("test_feature")
	if got.Version != 2 {
		t.Errorf("expected version 2, got %d", got.Version)
	}
	if got.Description != "Updated description" {
		t.Errorf("description not updated")
	}
	if got.CreatedBy != "creator" {
		t.Errorf("CreatedBy should be preserved, got '%s'", got.CreatedBy)
	}
	if got.UpdatedBy != "updater" {
		t.Errorf("UpdatedBy should be 'updater', got '%s'", got.UpdatedBy)
	}
	if !got.CreatedAt.Equal(originalCreatedAt) {
		t.Error("CreatedAt should be preserved")
	}

	// Check version history
	history := c.GetVersionHistory("test_feature")
	if len(history) != 2 {
		t.Errorf("expected 2 versions in history, got %d", len(history))
	}
}

func TestCatalog_Get(t *testing.T) {
	c := NewCatalog()

	// Test getting non-existent feature
	got := c.Get("non_existent")
	if got != nil {
		t.Error("expected nil for non-existent feature")
	}

	// Register and get
	def := &FeatureDefinition{Name: "test_feature", DataType: "float"}
	c.Register(def, "user")

	got = c.Get("test_feature")
	if got == nil {
		t.Error("expected to find registered feature")
		return
	}
	if got.Name != "test_feature" {
		t.Errorf("expected name 'test_feature', got '%s'", got.Name)
	}
}

func TestCatalog_GetVersion(t *testing.T) {
	c := NewCatalog()

	// Register initial version
	def := &FeatureDefinition{Name: "versioned_feature", Description: "V1"}
	c.Register(def, "user")

	// Update to create version 2
	def2 := &FeatureDefinition{Name: "versioned_feature", Description: "V2"}
	c.Register(def2, "user")

	// Update to create version 3
	def3 := &FeatureDefinition{Name: "versioned_feature", Description: "V3"}
	c.Register(def3, "user")

	// Get version 1
	v1 := c.GetVersion("versioned_feature", 1)
	if v1 == nil {
		t.Fatal("version 1 not found")
	}
	if v1.Description != "V1" {
		t.Errorf("expected 'V1', got '%s'", v1.Description)
	}

	// Get version 2
	v2 := c.GetVersion("versioned_feature", 2)
	if v2 == nil {
		t.Fatal("version 2 not found")
	}
	if v2.Description != "V2" {
		t.Errorf("expected 'V2', got '%s'", v2.Description)
	}

	// Get current version (3)
	v3 := c.GetVersion("versioned_feature", 3)
	if v3 == nil {
		t.Fatal("version 3 not found")
	}
	if v3.Description != "V3" {
		t.Errorf("expected 'V3', got '%s'", v3.Description)
	}

	// Get version 0 should return current
	current := c.GetVersion("versioned_feature", 0)
	if current == nil {
		t.Fatal("current version not found for version 0")
	}
	if current.Description != "V3" {
		t.Errorf("expected current version 'V3', got '%s'", current.Description)
	}

	// Get non-existent version
	nonExistent := c.GetVersion("versioned_feature", 999)
	if nonExistent != nil {
		t.Error("expected nil for non-existent version")
	}
}

func TestCatalog_GetVersionHistory(t *testing.T) {
	c := NewCatalog()

	// Empty history for non-existent feature
	history := c.GetVersionHistory("non_existent")
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d items", len(history))
	}

	// Create multiple versions
	for i := 1; i <= 3; i++ {
		def := &FeatureDefinition{Name: "feature", Description: "Version"}
		c.Register(def, "user")
	}

	history = c.GetVersionHistory("feature")
	if len(history) != 3 {
		t.Errorf("expected 3 versions, got %d", len(history))
	}
}

func TestCatalog_Delete(t *testing.T) {
	c := NewCatalog()

	// Delete non-existent feature
	err := c.Delete("non_existent")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}

	// Register and delete
	def := &FeatureDefinition{
		Name:       "to_delete",
		DataType:   "float",
		EntityType: "user",
		Owner:      "owner1",
		Team:       "team1",
		Category:   "test",
		Tags:       []string{"tag1"},
	}
	c.Register(def, "user")

	err = c.Delete("to_delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	if c.Get("to_delete") != nil {
		t.Error("feature should be deleted")
	}

	// Verify indexes are cleaned up
	if len(c.GetByTag("tag1")) != 0 {
		t.Error("feature should be removed from tag index")
	}
	if len(c.GetByOwner("owner1")) != 0 {
		t.Error("feature should be removed from owner index")
	}
	if len(c.GetByTeam("team1")) != 0 {
		t.Error("feature should be removed from team index")
	}
	if len(c.GetByCategory("test")) != 0 {
		t.Error("feature should be removed from category index")
	}
	if len(c.GetByEntityType("user")) != 0 {
		t.Error("feature should be removed from entity index")
	}
}

func TestCatalog_SetStatus(t *testing.T) {
	c := NewCatalog()

	// Set status on non-existent feature
	err := c.SetStatus("non_existent", StatusActive, "user")
	if !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}

	// Register and set status
	def := &FeatureDefinition{Name: "feature", DataType: "float"}
	c.Register(def, "user")

	err = c.SetStatus("feature", StatusActive, "approver")
	if err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	got := c.Get("feature")
	if got.Status != StatusActive {
		t.Errorf("expected status active, got %s", got.Status)
	}
	if got.UpdatedBy != "approver" {
		t.Errorf("expected UpdatedBy 'approver', got '%s'", got.UpdatedBy)
	}

	// Test deprecation sets DeprecatedAt
	err = c.SetStatus("feature", StatusDeprecated, "admin")
	if err != nil {
		t.Fatalf("SetStatus to deprecated failed: %v", err)
	}

	got = c.Get("feature")
	if got.DeprecatedAt == nil {
		t.Error("DeprecatedAt should be set when status is deprecated")
	}
}

func TestCatalog_List(t *testing.T) {
	c := NewCatalog()

	// Empty list
	list := c.List(nil)
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Add some features
	features := []FeatureDefinition{
		{Name: "feature_a", EntityType: "user", Owner: "owner1", Team: "team1", Category: "behavioral", Tags: []string{"ml"}},
		{Name: "feature_b", EntityType: "user", Owner: "owner2", Team: "team1", Category: "demographic", Tags: []string{"core"}},
		{Name: "feature_c", EntityType: "item", Owner: "owner1", Team: "team2", Category: "behavioral", Tags: []string{"ml"}},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Set statuses for filtering tests
	c.SetStatus("feature_a", StatusActive, "user")
	c.SetStatus("feature_c", StatusActive, "user")

	// List all
	list = c.List(nil)
	if len(list) != 3 {
		t.Errorf("expected 3 features, got %d", len(list))
	}

	// Verify sorted by name
	if list[0].Name != "feature_a" || list[1].Name != "feature_b" || list[2].Name != "feature_c" {
		t.Error("features should be sorted by name")
	}
}

func TestCatalog_ListFilter(t *testing.T) {
	c := NewCatalog()

	// Add features
	features := []FeatureDefinition{
		{Name: "feature_a", Description: "User age feature", EntityType: "user", Owner: "owner1", Team: "team1", Category: "behavioral", Tags: []string{"ml"}},
		{Name: "feature_b", Description: "User name feature", EntityType: "user", Owner: "owner2", Team: "team1", Category: "demographic", Tags: []string{"core"}},
		{Name: "feature_c", Description: "Item price feature", EntityType: "item", Owner: "owner1", Team: "team2", Category: "behavioral", Tags: []string{"ml"}},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Set statuses for filtering
	c.SetStatus("feature_a", StatusActive, "user")
	c.SetStatus("feature_c", StatusActive, "user")

	tests := []struct {
		name     string
		filter   *ListFilter
		expected int
	}{
		{"filter by owner", &ListFilter{Owner: "owner1"}, 2},
		{"filter by team", &ListFilter{Team: "team1"}, 2},
		{"filter by category", &ListFilter{Category: "behavioral"}, 2},
		{"filter by entity type", &ListFilter{EntityType: "user"}, 2},
		{"filter by status", &ListFilter{Status: StatusActive}, 2},
		{"filter by tags", &ListFilter{Tags: []string{"ml"}}, 2},
		{"filter by search", &ListFilter{Search: "user"}, 2},
		{"multiple filters", &ListFilter{Owner: "owner1", EntityType: "user"}, 1},
		{"no matches", &ListFilter{Owner: "nonexistent"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.List(tt.filter)
			if len(result) != tt.expected {
				t.Errorf("expected %d results, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestCatalog_Search(t *testing.T) {
	c := NewCatalog()

	// Add features
	features := []FeatureDefinition{
		{Name: "user_age", Description: "Age of the user"},
		{Name: "user_name", Description: "Name of the user"},
		{Name: "item_price", Description: "Price of the item", Tags: []string{"pricing"}},
		{Name: "pricing_tier", Description: "Tier of pricing"},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	tests := []struct {
		name          string
		query         string
		limit         int
		expectedFirst string
		minCount      int // Use minimum count since scoring can vary
	}{
		{"exact match", "user_age", 10, "user_age", 1},
		{"prefix match", "user", 10, "user_age", 2},
		{"contains match", "price", 10, "item_price", 1},
		{"description match", "tier", 10, "pricing_tier", 1},
		{"tag match", "pricing", 10, "", 1}, // Either could be first
		{"with limit", "user", 1, "user_age", 1},
		{"no match", "nonexistent", 10, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := c.Search(tt.query, tt.limit)
			if len(results) < tt.minCount {
				t.Errorf("expected at least %d results, got %d", tt.minCount, len(results))
			}
			if tt.expectedFirst != "" && len(results) > 0 && results[0].Name != tt.expectedFirst {
				t.Errorf("expected first result '%s', got '%s'", tt.expectedFirst, results[0].Name)
			}
		})
	}
}

func TestCatalog_GetByTag(t *testing.T) {
	c := NewCatalog()

	def1 := &FeatureDefinition{Name: "feature1", Tags: []string{"ml", "core"}}
	def2 := &FeatureDefinition{Name: "feature2", Tags: []string{"ml"}}
	def3 := &FeatureDefinition{Name: "feature3", Tags: []string{"core"}}

	c.Register(def1, "user")
	c.Register(def2, "user")
	c.Register(def3, "user")

	mlFeatures := c.GetByTag("ml")
	if len(mlFeatures) != 2 {
		t.Errorf("expected 2 ml features, got %d", len(mlFeatures))
	}

	coreFeatures := c.GetByTag("core")
	if len(coreFeatures) != 2 {
		t.Errorf("expected 2 core features, got %d", len(coreFeatures))
	}

	nonExistent := c.GetByTag("nonexistent")
	if len(nonExistent) != 0 {
		t.Errorf("expected 0 features for nonexistent tag, got %d", len(nonExistent))
	}
}

func TestCatalog_GetByOwner(t *testing.T) {
	c := NewCatalog()

	def1 := &FeatureDefinition{Name: "feature1", Owner: "alice"}
	def2 := &FeatureDefinition{Name: "feature2", Owner: "alice"}
	def3 := &FeatureDefinition{Name: "feature3", Owner: "bob"}

	c.Register(def1, "user")
	c.Register(def2, "user")
	c.Register(def3, "user")

	aliceFeatures := c.GetByOwner("alice")
	if len(aliceFeatures) != 2 {
		t.Errorf("expected 2 alice features, got %d", len(aliceFeatures))
	}

	bobFeatures := c.GetByOwner("bob")
	if len(bobFeatures) != 1 {
		t.Errorf("expected 1 bob feature, got %d", len(bobFeatures))
	}
}

func TestCatalog_GetByTeam(t *testing.T) {
	c := NewCatalog()

	def1 := &FeatureDefinition{Name: "feature1", Team: "ml-team"}
	def2 := &FeatureDefinition{Name: "feature2", Team: "ml-team"}
	def3 := &FeatureDefinition{Name: "feature3", Team: "data-team"}

	c.Register(def1, "user")
	c.Register(def2, "user")
	c.Register(def3, "user")

	mlFeatures := c.GetByTeam("ml-team")
	if len(mlFeatures) != 2 {
		t.Errorf("expected 2 ml-team features, got %d", len(mlFeatures))
	}
}

func TestCatalog_GetByCategory(t *testing.T) {
	c := NewCatalog()

	def1 := &FeatureDefinition{Name: "feature1", Category: "behavioral"}
	def2 := &FeatureDefinition{Name: "feature2", Category: "behavioral"}
	def3 := &FeatureDefinition{Name: "feature3", Category: "demographic"}

	c.Register(def1, "user")
	c.Register(def2, "user")
	c.Register(def3, "user")

	behavioralFeatures := c.GetByCategory("behavioral")
	if len(behavioralFeatures) != 2 {
		t.Errorf("expected 2 behavioral features, got %d", len(behavioralFeatures))
	}
}

func TestCatalog_GetByEntityType(t *testing.T) {
	c := NewCatalog()

	def1 := &FeatureDefinition{Name: "feature1", EntityType: "user"}
	def2 := &FeatureDefinition{Name: "feature2", EntityType: "user"}
	def3 := &FeatureDefinition{Name: "feature3", EntityType: "item"}

	c.Register(def1, "user")
	c.Register(def2, "user")
	c.Register(def3, "user")

	userFeatures := c.GetByEntityType("user")
	if len(userFeatures) != 2 {
		t.Errorf("expected 2 user features, got %d", len(userFeatures))
	}

	itemFeatures := c.GetByEntityType("item")
	if len(itemFeatures) != 1 {
		t.Errorf("expected 1 item feature, got %d", len(itemFeatures))
	}
}

func TestCatalog_GetLineage(t *testing.T) {
	c := NewCatalog()

	// Create a lineage: raw_clicks -> click_count -> user_engagement -> churn_score
	features := []FeatureDefinition{
		{Name: "raw_clicks", Downstream: []string{"click_count"}},
		{Name: "click_count", Upstream: []string{"raw_clicks"}, Downstream: []string{"user_engagement"}},
		{Name: "user_engagement", Upstream: []string{"click_count"}, Downstream: []string{"churn_score"}},
		{Name: "churn_score", Upstream: []string{"user_engagement"}},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Get lineage for user_engagement
	lineage := c.GetLineage("user_engagement")
	if lineage == nil {
		t.Fatal("lineage should not be nil")
	}

	if lineage.Feature.Name != "user_engagement" {
		t.Errorf("expected feature name 'user_engagement', got '%s'", lineage.Feature.Name)
	}

	// Should have click_count and raw_clicks as upstream
	if len(lineage.Upstream) != 2 {
		t.Errorf("expected 2 upstream features, got %d", len(lineage.Upstream))
	}

	// Should have churn_score as downstream
	if len(lineage.Downstream) != 1 {
		t.Errorf("expected 1 downstream feature, got %d", len(lineage.Downstream))
	}

	// Non-existent feature
	lineage = c.GetLineage("nonexistent")
	if lineage != nil {
		t.Error("expected nil lineage for non-existent feature")
	}
}

func TestCatalog_GetStats(t *testing.T) {
	c := NewCatalog()

	// Empty stats
	stats := c.GetStats()
	if stats.TotalFeatures != 0 {
		t.Errorf("expected 0 total features, got %d", stats.TotalFeatures)
	}

	// Add features
	features := []FeatureDefinition{
		{Name: "f1", Category: "behavioral", EntityType: "user", Team: "team1", Tags: []string{"ml"}},
		{Name: "f2", Category: "behavioral", EntityType: "user", Team: "team1", Tags: []string{"ml", "core"}},
		{Name: "f3", Category: "demographic", EntityType: "item", Team: "team2", Tags: []string{"core"}},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Set statuses
	c.SetStatus("f1", StatusActive, "user")
	c.SetStatus("f2", StatusActive, "user")

	stats = c.GetStats()

	if stats.TotalFeatures != 3 {
		t.Errorf("expected 3 total features, got %d", stats.TotalFeatures)
	}

	if stats.ByStatus[StatusActive] != 2 {
		t.Errorf("expected 2 active features, got %d", stats.ByStatus[StatusActive])
	}
	if stats.ByStatus[StatusDraft] != 1 {
		t.Errorf("expected 1 draft feature, got %d", stats.ByStatus[StatusDraft])
	}

	if stats.ByCategory["behavioral"] != 2 {
		t.Errorf("expected 2 behavioral features, got %d", stats.ByCategory["behavioral"])
	}

	if stats.ByEntityType["user"] != 2 {
		t.Errorf("expected 2 user features, got %d", stats.ByEntityType["user"])
	}

	if stats.ByTeam["team1"] != 2 {
		t.Errorf("expected 2 team1 features, got %d", stats.ByTeam["team1"])
	}

	if stats.TagCounts["ml"] != 2 {
		t.Errorf("expected 2 ml tags, got %d", stats.TagCounts["ml"])
	}
	if stats.TagCounts["core"] != 2 {
		t.Errorf("expected 2 core tags, got %d", stats.TagCounts["core"])
	}
}

func TestCatalog_ExportImport(t *testing.T) {
	c := NewCatalog()

	// Add features
	features := []FeatureDefinition{
		{Name: "feature1", Description: "First feature", DataType: "float"},
		{Name: "feature2", Description: "Second feature", DataType: "int"},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Export
	data, err := c.Export()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import into new catalog
	c2 := NewCatalog()
	err = c2.Import(data, "importer")
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify import
	list := c2.List(nil)
	if len(list) != 2 {
		t.Errorf("expected 2 imported features, got %d", len(list))
	}

	// Verify feature data
	f1 := c2.Get("feature1")
	if f1 == nil {
		t.Fatal("feature1 not found after import")
	}
	if f1.Description != "First feature" {
		t.Errorf("expected description 'First feature', got '%s'", f1.Description)
	}
}

func TestCatalog_Import_InvalidJSON(t *testing.T) {
	c := NewCatalog()

	err := c.Import([]byte("invalid json"), "user")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCatalog_Concurrency(t *testing.T) {
	c := NewCatalog()

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 20

	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				def := &FeatureDefinition{
					Name:       "feature",
					EntityType: "user",
					Tags:       []string{"tag1"},
				}
				c.Register(def, "user")
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				c.Get("feature")
				c.List(nil)
				c.Search("feature", 10)
				c.GetByTag("tag1")
				c.GetStats()
			}
		}()
	}

	wg.Wait()

	// Verify final state
	if c.Get("feature") == nil {
		t.Error("feature should exist after concurrent operations")
	}
}

func TestListFilter_Matches(t *testing.T) {
	def := &FeatureDefinition{
		Name:        "test_feature",
		Description: "A test feature description",
		Owner:       "alice",
		Team:        "ml-team",
		Category:    "behavioral",
		EntityType:  "user",
		Status:      StatusActive,
		Tags:        []string{"ml", "core"},
	}

	tests := []struct {
		name    string
		filter  *ListFilter
		matches bool
	}{
		{"empty filter matches all", &ListFilter{}, true},
		{"matching owner", &ListFilter{Owner: "alice"}, true},
		{"non-matching owner", &ListFilter{Owner: "bob"}, false},
		{"matching team", &ListFilter{Team: "ml-team"}, true},
		{"non-matching team", &ListFilter{Team: "other"}, false},
		{"matching category", &ListFilter{Category: "behavioral"}, true},
		{"non-matching category", &ListFilter{Category: "other"}, false},
		{"matching entity type", &ListFilter{EntityType: "user"}, true},
		{"non-matching entity type", &ListFilter{EntityType: "item"}, false},
		{"matching status", &ListFilter{Status: StatusActive}, true},
		{"non-matching status", &ListFilter{Status: StatusDraft}, false},
		{"matching tag", &ListFilter{Tags: []string{"ml"}}, true},
		{"non-matching tag", &ListFilter{Tags: []string{"other"}}, false},
		{"search in name", &ListFilter{Search: "test"}, true},
		{"search in description", &ListFilter{Search: "description"}, true},
		{"search no match", &ListFilter{Search: "xyz"}, false},
		{"multiple matching filters", &ListFilter{Owner: "alice", Team: "ml-team"}, true},
		{"one non-matching filter", &ListFilter{Owner: "alice", Team: "other"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Matches(def)
			if result != tt.matches {
				t.Errorf("expected %v, got %v", tt.matches, result)
			}
		})
	}
}

func TestCatalog_IndexUpdatesOnUpdate(t *testing.T) {
	c := NewCatalog()

	// Register with initial values
	def := &FeatureDefinition{
		Name:       "feature",
		Owner:      "owner1",
		Team:       "team1",
		Category:   "cat1",
		EntityType: "entity1",
		Tags:       []string{"tag1"},
	}
	c.Register(def, "user")

	// Verify initial indexes
	if len(c.GetByOwner("owner1")) != 1 {
		t.Error("owner1 index should have 1 feature")
	}

	// Update with different values
	updated := &FeatureDefinition{
		Name:       "feature",
		Owner:      "owner2",
		Team:       "team2",
		Category:   "cat2",
		EntityType: "entity2",
		Tags:       []string{"tag2"},
	}
	c.Register(updated, "user")

	// Old indexes should be empty
	if len(c.GetByOwner("owner1")) != 0 {
		t.Error("owner1 index should be empty after update")
	}
	if len(c.GetByTeam("team1")) != 0 {
		t.Error("team1 index should be empty after update")
	}
	if len(c.GetByCategory("cat1")) != 0 {
		t.Error("cat1 index should be empty after update")
	}
	if len(c.GetByEntityType("entity1")) != 0 {
		t.Error("entity1 index should be empty after update")
	}
	if len(c.GetByTag("tag1")) != 0 {
		t.Error("tag1 index should be empty after update")
	}

	// New indexes should have the feature
	if len(c.GetByOwner("owner2")) != 1 {
		t.Error("owner2 index should have 1 feature")
	}
	if len(c.GetByTeam("team2")) != 1 {
		t.Error("team2 index should have 1 feature")
	}
	if len(c.GetByCategory("cat2")) != 1 {
		t.Error("cat2 index should have 1 feature")
	}
	if len(c.GetByEntityType("entity2")) != 1 {
		t.Error("entity2 index should have 1 feature")
	}
	if len(c.GetByTag("tag2")) != 1 {
		t.Error("tag2 index should have 1 feature")
	}
}

func TestCatalog_SearchScoring(t *testing.T) {
	c := NewCatalog()

	// Add features with similar names
	features := []FeatureDefinition{
		{Name: "user_age", Description: "The user age"},
		{Name: "age_bucket", Description: "Age bucket for user"},
		{Name: "user", Description: "User feature"},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// Exact match should come first
	results := c.Search("user", 10)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Name != "user" {
		t.Errorf("expected first result 'user' (exact match), got '%s'", results[0].Name)
	}
}

func TestCatalog_LineageCyclicDependency(t *testing.T) {
	c := NewCatalog()

	// Create features with cyclic dependencies
	features := []FeatureDefinition{
		{Name: "feature_a", Upstream: []string{"feature_c"}, Downstream: []string{"feature_b"}},
		{Name: "feature_b", Upstream: []string{"feature_a"}, Downstream: []string{"feature_c"}},
		{Name: "feature_c", Upstream: []string{"feature_b"}, Downstream: []string{"feature_a"}},
	}
	for _, f := range features {
		def := f
		c.Register(&def, "user")
	}

	// GetLineage should handle cycles without infinite loop
	lineage := c.GetLineage("feature_a")
	if lineage == nil {
		t.Fatal("lineage should not be nil")
	}

	// Should have upstream and downstream (but not infinite due to visited tracking)
	if lineage.Upstream == nil {
		t.Error("Upstream should not be nil")
	}
	if lineage.Downstream == nil {
		t.Error("Downstream should not be nil")
	}
}

func TestRemoveFromSlice(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected []string
	}{
		{"remove existing", []string{"a", "b", "c"}, "b", []string{"a", "c"}},
		{"remove first", []string{"a", "b", "c"}, "a", []string{"b", "c"}},
		{"remove last", []string{"a", "b", "c"}, "c", []string{"a", "b"}},
		{"remove non-existing", []string{"a", "b", "c"}, "d", []string{"a", "b", "c"}},
		{"empty slice", []string{}, "a", []string{}},
		{"single element remove", []string{"a"}, "a", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeFromSlice(tt.slice, tt.item)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %v at index %d, got %v", tt.expected[i], i, v)
				}
			}
		})
	}
}
