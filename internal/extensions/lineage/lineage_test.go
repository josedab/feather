package lineage

import (
	"testing"
	"time"
)

func TestTracker_RegisterFeature(t *testing.T) {
	tracker := NewTracker()

	feature := &FeatureLineage{
		FeatureID:   "user_purchase_count",
		Name:        "User Purchase Count",
		Description: "Total number of purchases by user",
		Tags:        []string{"user", "purchases"},
	}

	err := tracker.RegisterFeature(feature)
	if err != nil {
		t.Fatalf("RegisterFeature failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := tracker.GetFeatureLineage("user_purchase_count")
	if err != nil {
		t.Fatalf("GetFeatureLineage failed: %v", err)
	}

	if retrieved.Name != "User Purchase Count" {
		t.Errorf("expected name 'User Purchase Count', got %s", retrieved.Name)
	}

	if retrieved.Version != 1 {
		t.Errorf("expected version 1, got %d", retrieved.Version)
	}

	// Update feature
	feature.Description = "Updated description"
	err = tracker.RegisterFeature(feature)
	if err != nil {
		t.Fatalf("RegisterFeature update failed: %v", err)
	}

	retrieved, _ = tracker.GetFeatureLineage("user_purchase_count")
	if retrieved.Version != 2 {
		t.Errorf("expected version 2 after update, got %d", retrieved.Version)
	}
}

func TestTracker_Dependencies(t *testing.T) {
	tracker := NewTracker()

	// Register base features
	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "raw_transactions",
		Name:      "Raw Transactions",
	})

	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "user_info",
		Name:      "User Info",
	})

	// Register derived feature
	tracker.RegisterFeature(&FeatureLineage{
		FeatureID:    "user_total_spend",
		Name:         "User Total Spend",
		Dependencies: []string{"raw_transactions", "user_info"},
	})

	// Check dependency graph
	graph := tracker.GetDependencyGraph()
	downstream := graph.GetDownstream("raw_transactions")

	found := false
	for _, n := range downstream {
		if n.ID == "user_total_spend" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected user_total_spend in downstream of raw_transactions")
	}
}

func TestTracker_ImpactAnalysis(t *testing.T) {
	tracker := NewTracker()

	// Create a chain: source -> feature1 -> feature2 -> consumer
	tracker.RegisterSource(&DataSource{
		ID:   "orders_db",
		Name: "Orders Database",
		Type: SourceTypeDatabase,
	})

	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "order_count",
		Name:      "Order Count",
	})
	tracker.LinkSourceToFeature("orders_db", "order_count", []string{"order_id"})

	tracker.RegisterFeature(&FeatureLineage{
		FeatureID:    "customer_lifetime_value",
		Name:         "Customer Lifetime Value",
		Dependencies: []string{"order_count"},
	})

	tracker.RegisterConsumer(&Consumer{
		ID:   "recommendation_model",
		Name: "Recommendation Model",
		Type: ConsumerTypeModel,
	})
	tracker.LinkFeatureToConsumer("customer_lifetime_value", "recommendation_model", "training")

	// Analyze impact of changing order_count
	analysis, err := tracker.GetImpactAnalysis("order_count")
	if err != nil {
		t.Fatalf("GetImpactAnalysis failed: %v", err)
	}

	if len(analysis.AffectedFeatures) != 1 {
		t.Errorf("expected 1 affected feature, got %d", len(analysis.AffectedFeatures))
	}

	if len(analysis.AffectedConsumers) != 1 {
		t.Errorf("expected 1 affected consumer, got %d", len(analysis.AffectedConsumers))
	}
}

func TestTracker_PIITracking(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "user_email_hash",
		Name:      "User Email Hash",
	})

	pii := &PIIMetadata{
		PIILevel:        PIIMedium,
		PIITypes:        []string{"email"},
		LegalBasis:      "consent",
		RetentionPolicy: "2 years",
		DataSubjects:    []string{"customer"},
		Encrypted:       true,
	}

	err := tracker.SetPIIMetadata("user_email_hash", pii)
	if err != nil {
		t.Fatalf("SetPIIMetadata failed: %v", err)
	}

	// Get PII features
	piiFeatures := tracker.GetPIIFeatures(PIIMedium)
	if len(piiFeatures) != 1 {
		t.Errorf("expected 1 PII feature, got %d", len(piiFeatures))
	}

	// Get features by data subject
	subjectFeatures := tracker.GetDataSubjectFeatures("customer")
	if len(subjectFeatures) != 1 {
		t.Errorf("expected 1 feature for customer subject, got %d", len(subjectFeatures))
	}
}

func TestTracker_AuditLog(t *testing.T) {
	tracker := NewTracker()

	// Perform some actions
	tracker.RegisterFeature(&FeatureLineage{
		FeatureID: "test_feature",
		Name:      "Test Feature",
	})

	tracker.RegisterSource(&DataSource{
		ID:   "test_source",
		Name: "Test Source",
	})

	// Check audit log
	events := tracker.GetAuditLog(time.Now().Add(-1 * time.Hour))
	if len(events) < 2 {
		t.Errorf("expected at least 2 audit events, got %d", len(events))
	}
}

func TestDependencyGraph_CycleDetection(t *testing.T) {
	graph := NewDependencyGraph()

	// Create nodes
	graph.AddNode("A", NodeTypeFeature)
	graph.AddNode("B", NodeTypeFeature)
	graph.AddNode("C", NodeTypeFeature)

	// Create a cycle: A -> B -> C -> A
	graph.AddEdge("A", "B", EdgeTypeDependsOn)
	graph.AddEdge("B", "C", EdgeTypeDependsOn)
	graph.AddEdge("C", "A", EdgeTypeDependsOn)

	cycles := graph.DetectCycles()
	if len(cycles) == 0 {
		t.Error("expected cycle to be detected")
	}
}

func TestDependencyGraph_TopologicalSort(t *testing.T) {
	graph := NewDependencyGraph()

	// Create DAG
	graph.AddNode("A", NodeTypeSource)
	graph.AddNode("B", NodeTypeFeature)
	graph.AddNode("C", NodeTypeFeature)
	graph.AddNode("D", NodeTypeConsumer)

	graph.AddEdge("A", "B", EdgeTypeSourceOf)
	graph.AddEdge("A", "C", EdgeTypeSourceOf)
	graph.AddEdge("B", "D", EdgeTypeConsumedBy)
	graph.AddEdge("C", "D", EdgeTypeConsumedBy)

	order, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	// A should come before B, C; B, C should come before D
	aIdx, dIdx := -1, -1
	for i, id := range order {
		if id == "A" {
			aIdx = i
		}
		if id == "D" {
			dIdx = i
		}
	}

	if aIdx >= dIdx {
		t.Error("expected A before D in topological order")
	}
}

func TestDependencyGraph_ExportFormats(t *testing.T) {
	graph := NewDependencyGraph()

	graph.AddNode("source1", NodeTypeSource)
	graph.AddNode("feature1", NodeTypeFeature)
	graph.AddNode("model1", NodeTypeConsumer)

	graph.AddEdge("source1", "feature1", EdgeTypeSourceOf)
	graph.AddEdge("feature1", "model1", EdgeTypeConsumedBy)

	// Test DOT export
	dot := graph.ExportDOT()
	if dot == "" {
		t.Error("DOT export should not be empty")
	}
	if !stringContains(dot, "digraph") {
		t.Error("DOT export should contain 'digraph'")
	}

	// Test Mermaid export
	mermaid := graph.ExportMermaid()
	if mermaid == "" {
		t.Error("Mermaid export should not be empty")
	}
	if !stringContains(mermaid, "graph LR") {
		t.Error("Mermaid export should contain 'graph LR'")
	}

	// Test JSON export
	jsonData, err := graph.ExportJSON()
	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON export should not be empty")
	}
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
