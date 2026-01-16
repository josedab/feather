package lineage

import (
	"testing"
)

func buildTestGraph() *DependencyGraph {
	g := NewDependencyGraph()
	g.AddNode("kafka", NodeTypeSource)
	g.SetNodeLabel("kafka", "Kafka Topic")
	g.AddNode("click_count", NodeTypeFeature)
	g.SetNodeLabel("click_count", "Click Count")
	g.AddNode("purchase_total", NodeTypeFeature)
	g.SetNodeLabel("purchase_total", "Purchase Total")
	g.AddNode("derived_ctr", NodeTypeFeature)
	g.SetNodeLabel("derived_ctr", "Derived CTR")
	g.AddNode("fraud_model", NodeTypeConsumer)
	g.SetNodeLabel("fraud_model", "Fraud Model")

	g.AddEdge("kafka", "click_count", EdgeTypeSourceOf)
	g.AddEdge("kafka", "purchase_total", EdgeTypeSourceOf)
	g.AddEdge("click_count", "derived_ctr", EdgeTypeDependsOn)
	g.AddEdge("purchase_total", "derived_ctr", EdgeTypeDependsOn)
	g.AddEdge("derived_ctr", "fraud_model", EdgeTypeConsumedBy)
	return g
}

func TestGraphStats(t *testing.T) {
	g := buildTestGraph()
	stats := g.Stats()

	if stats.TotalNodes != 5 {
		t.Errorf("expected 5 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 5 {
		t.Errorf("expected 5 edges, got %d", stats.TotalEdges)
	}
	if stats.HasCycles {
		t.Error("expected no cycles")
	}
	if stats.IsolatedNodes != 0 {
		t.Errorf("expected 0 isolated nodes, got %d", stats.IsolatedNodes)
	}
	if stats.ConnectedGroups != 1 {
		t.Errorf("expected 1 connected group, got %d", stats.ConnectedGroups)
	}
	if stats.MaxDepth < 3 {
		t.Errorf("expected max depth >= 3, got %d", stats.MaxDepth)
	}
}

func TestGraphStatsEmpty(t *testing.T) {
	g := NewDependencyGraph()
	stats := g.Stats()
	if stats.TotalNodes != 0 {
		t.Errorf("expected 0 nodes, got %d", stats.TotalNodes)
	}
	if stats.ConnectedGroups != 0 {
		t.Errorf("expected 0 groups, got %d", stats.ConnectedGroups)
	}
}

func TestComputeBlastRadius(t *testing.T) {
	g := buildTestGraph()
	br, err := g.ComputeBlastRadius("click_count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalAffected < 1 {
		t.Errorf("expected at least 1 affected node, got %d", br.TotalAffected)
	}
	if br.RiskScore < 0 || br.RiskScore > 10 {
		t.Errorf("risk score out of range: %f", br.RiskScore)
	}
	if len(br.AffectedConsumers) == 0 {
		t.Error("expected at least one affected consumer")
	}
}

func TestComputeBlastRadiusNotFound(t *testing.T) {
	g := buildTestGraph()
	_, err := g.ComputeBlastRadius("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestFindPaths(t *testing.T) {
	g := buildTestGraph()
	result, err := g.FindPaths("kafka", "fraud_model", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Error("expected at least one path")
	}
	if result.Shortest < 3 {
		t.Errorf("expected shortest path >= 3, got %d", result.Shortest)
	}
}

func TestFindPathsNoPath(t *testing.T) {
	g := buildTestGraph()
	result, err := g.FindPaths("fraud_model", "kafka", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Paths) != 0 {
		t.Errorf("expected no paths, got %d", len(result.Paths))
	}
}

func TestFindPathsNodeNotFound(t *testing.T) {
	g := buildTestGraph()
	_, err := g.FindPaths("nonexistent", "kafka", 10)
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}
}

func TestSearchNodes(t *testing.T) {
	g := buildTestGraph()

	results := g.SearchNodes("click", "")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'click', got %d", len(results))
	}

	results = g.SearchNodes("", NodeTypeFeature)
	if len(results) != 3 {
		t.Errorf("expected 3 feature nodes, got %d", len(results))
	}

	results = g.SearchNodes("KAFKA", "")
	if len(results) != 1 {
		t.Errorf("expected case-insensitive match, got %d", len(results))
	}
}

func TestGraphStatsMultipleGroups(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", NodeTypeFeature)
	g.AddNode("b", NodeTypeFeature)
	g.AddNode("c", NodeTypeFeature)
	g.AddEdge("a", "b", EdgeTypeDependsOn)
	// c is isolated

	stats := g.Stats()
	if stats.ConnectedGroups != 2 {
		t.Errorf("expected 2 connected groups, got %d", stats.ConnectedGroups)
	}
	if stats.IsolatedNodes != 1 {
		t.Errorf("expected 1 isolated node, got %d", stats.IsolatedNodes)
	}
}
