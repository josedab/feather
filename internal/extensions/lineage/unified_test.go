package lineage

import (
	"strings"
	"testing"
)

func TestUnifiedLineage_AddGetRemoveNode(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())

	// Add node
	err := ul.AddNode(LineageNode{ID: "n1", Name: "source1", Kind: UnifiedNodeSource})
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	// Get node
	node, err := ul.GetNode("n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Name != "source1" || node.Kind != UnifiedNodeSource {
		t.Fatalf("unexpected node: %+v", node)
	}
	if node.Tags == nil || node.Metadata == nil {
		t.Fatal("Tags and Metadata should be initialized")
	}

	// Get non-existent
	_, err = ul.GetNode("missing")
	if err == nil {
		t.Fatal("expected error for missing node")
	}

	// Remove node
	err = ul.RemoveNode("n1")
	if err != nil {
		t.Fatalf("RemoveNode: %v", err)
	}
	_, err = ul.GetNode("n1")
	if err == nil {
		t.Fatal("expected error after removal")
	}

	// Remove non-existent
	err = ul.RemoveNode("n1")
	if err == nil {
		t.Fatal("expected error removing non-existent node")
	}
}

func TestUnifiedLineage_AddNodeEmptyID(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	err := ul.AddNode(LineageNode{Name: "noID"})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestUnifiedLineage_MaxNodes(t *testing.T) {
	cfg := DefaultUnifiedConfig()
	cfg.MaxNodes = 2
	ul := NewUnifiedLineage(cfg)

	ul.AddNode(LineageNode{ID: "a", Name: "a"})
	ul.AddNode(LineageNode{ID: "b", Name: "b"})
	err := ul.AddNode(LineageNode{ID: "c", Name: "c"})
	if err == nil {
		t.Fatal("expected max nodes error")
	}

	// Updating existing should succeed
	err = ul.AddNode(LineageNode{ID: "a", Name: "a-updated"})
	if err != nil {
		t.Fatalf("updating existing node should succeed: %v", err)
	}
}

func TestUnifiedLineage_AddEdge(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "a", Name: "A", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "b", Name: "B", Kind: UnifiedNodeFeature})

	err := ul.AddEdge(LineageEdge{From: "a", To: "b", Label: "feeds"})
	if err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	// Missing source
	err = ul.AddEdge(LineageEdge{From: "missing", To: "b"})
	if err == nil {
		t.Fatal("expected error for missing source")
	}

	// Missing target
	err = ul.AddEdge(LineageEdge{From: "a", To: "missing"})
	if err == nil {
		t.Fatal("expected error for missing target")
	}

	// Empty from/to
	err = ul.AddEdge(LineageEdge{})
	if err == nil {
		t.Fatal("expected error for empty from/to")
	}
}

func TestUnifiedLineage_CycleDetection(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "a", Name: "A"})
	ul.AddNode(LineageNode{ID: "b", Name: "B"})
	ul.AddNode(LineageNode{ID: "c", Name: "C"})

	ul.AddEdge(LineageEdge{From: "a", To: "b"})
	ul.AddEdge(LineageEdge{From: "b", To: "c"})

	// c -> a would create a cycle
	err := ul.AddEdge(LineageEdge{From: "c", To: "a"})
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error message, got: %v", err)
	}
}

func TestUnifiedLineage_GetUpstream(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddNode(LineageNode{ID: "m1", Name: "Model1", Kind: UnifiedNodeModel})

	ul.AddEdge(LineageEdge{From: "s1", To: "f1"})
	ul.AddEdge(LineageEdge{From: "f1", To: "m1"})

	upstream := ul.GetUpstream("m1", 10)
	if len(upstream) != 2 {
		t.Fatalf("expected 2 upstream nodes, got %d", len(upstream))
	}

	// Limited depth
	upstream = ul.GetUpstream("m1", 1)
	if len(upstream) != 1 {
		t.Fatalf("expected 1 upstream node at depth 1, got %d", len(upstream))
	}
}

func TestUnifiedLineage_GetDownstream(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddNode(LineageNode{ID: "c1", Name: "Consumer1", Kind: UnifiedNodeConsumer})

	ul.AddEdge(LineageEdge{From: "s1", To: "f1"})
	ul.AddEdge(LineageEdge{From: "f1", To: "c1"})

	downstream := ul.GetDownstream("s1", 10)
	if len(downstream) != 2 {
		t.Fatalf("expected 2 downstream nodes, got %d", len(downstream))
	}

	downstream = ul.GetDownstream("s1", 1)
	if len(downstream) != 1 {
		t.Fatalf("expected 1 downstream node at depth 1, got %d", len(downstream))
	}
}

func TestUnifiedLineage_AnalyzeImpact(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddNode(LineageNode{ID: "f2", Name: "Feature2", Kind: UnifiedNodeFeature})
	ul.AddNode(LineageNode{ID: "m1", Name: "Model1", Kind: UnifiedNodeModel})

	ul.AddEdge(LineageEdge{From: "s1", To: "f1"})
	ul.AddEdge(LineageEdge{From: "s1", To: "f2"})
	ul.AddEdge(LineageEdge{From: "f1", To: "m1"})

	impact, err := ul.AnalyzeImpact("s1")
	if err != nil {
		t.Fatalf("AnalyzeImpact: %v", err)
	}
	if impact.BlastRadius != 3 {
		t.Fatalf("expected blast radius 3, got %d", impact.BlastRadius)
	}
	if impact.MaxDepth != 2 {
		t.Fatalf("expected max depth 2, got %d", impact.MaxDepth)
	}
	if impact.SourceNode != "s1" {
		t.Fatalf("expected source node s1, got %s", impact.SourceNode)
	}

	// Non-existent node
	_, err = ul.AnalyzeImpact("missing")
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestUnifiedLineage_SetFreshness(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})

	// Within SLA
	err := ul.SetFreshness("f1", 500, 1000)
	if err != nil {
		t.Fatalf("SetFreshness: %v", err)
	}
	node, _ := ul.GetNode("f1")
	if node.FreshnessMs != 500 || node.FreshnessSLA != 1000 || node.SLAViolation {
		t.Fatalf("unexpected freshness state: %+v", node)
	}

	// SLA violation
	err = ul.SetFreshness("f1", 2000, 1000)
	if err != nil {
		t.Fatalf("SetFreshness: %v", err)
	}
	node, _ = ul.GetNode("f1")
	if !node.SLAViolation {
		t.Fatal("expected SLA violation")
	}

	// Missing node
	err = ul.SetFreshness("missing", 100, 200)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestUnifiedLineage_SetQuality(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})

	err := ul.SetQuality("f1", 0.95, 0.02)
	if err != nil {
		t.Fatalf("SetQuality: %v", err)
	}
	node, _ := ul.GetNode("f1")
	if node.QualityScore != 0.95 || node.DriftScore != 0.02 {
		t.Fatalf("unexpected quality state: quality=%f drift=%f", node.QualityScore, node.DriftScore)
	}

	// Missing node
	err = ul.SetQuality("missing", 0.5, 0.1)
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestUnifiedLineage_ExportDOT(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddEdge(LineageEdge{From: "s1", To: "f1", Label: "raw"})

	dot := ul.ExportDOT()
	if !strings.Contains(dot, "digraph lineage") {
		t.Fatal("DOT output should contain 'digraph lineage'")
	}
	if !strings.Contains(dot, "rankdir=LR") {
		t.Fatal("DOT output should contain 'rankdir=LR'")
	}
	if !strings.Contains(dot, "Source1") {
		t.Fatal("DOT output should contain node name")
	}
}

func TestUnifiedLineage_ExportDOT_SLAViolation(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.SetFreshness("f1", 2000, 1000)

	dot := ul.ExportDOT()
	if !strings.Contains(dot, "red") {
		t.Fatal("SLA violation node should be colored red in DOT")
	}
}

func TestUnifiedLineage_ExportMermaid(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddEdge(LineageEdge{From: "s1", To: "f1", Label: "raw"})

	mermaid := ul.ExportMermaid()
	if !strings.Contains(mermaid, "graph LR") {
		t.Fatal("Mermaid output should contain 'graph LR'")
	}
	if !strings.Contains(mermaid, "-->") {
		t.Fatal("Mermaid output should contain edge arrows")
	}
}

func TestUnifiedLineage_Stats(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddNode(LineageNode{ID: "f2", Name: "Feature2", Kind: UnifiedNodeFeature})
	ul.AddEdge(LineageEdge{From: "s1", To: "f1"})
	ul.AddEdge(LineageEdge{From: "s1", To: "f2"})

	ul.SetQuality("f1", 0.9, 0.1)
	ul.SetQuality("f2", 0.8, 0.2)
	ul.SetFreshness("f2", 2000, 1000)

	stats := ul.Stats()
	if stats.TotalNodes != 3 {
		t.Fatalf("expected 3 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 2 {
		t.Fatalf("expected 2 edges, got %d", stats.TotalEdges)
	}
	if stats.NodesByKind["source"] != 1 {
		t.Fatalf("expected 1 source, got %d", stats.NodesByKind["source"])
	}
	if stats.NodesByKind["feature"] != 2 {
		t.Fatalf("expected 2 features, got %d", stats.NodesByKind["feature"])
	}
	if stats.SLAViolations != 1 {
		t.Fatalf("expected 1 SLA violation, got %d", stats.SLAViolations)
	}
	if stats.AvgQuality < 0.84 || stats.AvgQuality > 0.86 {
		t.Fatalf("expected avg quality ~0.85, got %f", stats.AvgQuality)
	}
}

func TestUnifiedLineage_GetGraph(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "b", Name: "B"})
	ul.AddNode(LineageNode{ID: "a", Name: "A"})
	ul.AddEdge(LineageEdge{From: "a", To: "b"})

	graph := ul.GetGraph()
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	// Nodes should be sorted by ID
	if graph.Nodes[0].ID != "a" || graph.Nodes[1].ID != "b" {
		t.Fatal("nodes should be sorted by ID")
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestUnifiedLineage_RemoveNodeCleansEdges(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "a", Name: "A"})
	ul.AddNode(LineageNode{ID: "b", Name: "B"})
	ul.AddNode(LineageNode{ID: "c", Name: "C"})
	ul.AddEdge(LineageEdge{From: "a", To: "b"})
	ul.AddEdge(LineageEdge{From: "b", To: "c"})

	ul.RemoveNode("b")
	graph := ul.GetGraph()
	if len(graph.Edges) != 0 {
		t.Fatalf("expected 0 edges after removing middle node, got %d", len(graph.Edges))
	}
}

func TestGenerateVisualizationHTML(t *testing.T) {
	ul := NewUnifiedLineage(DefaultUnifiedConfig())
	ul.AddNode(LineageNode{ID: "s1", Name: "Source1", Kind: UnifiedNodeSource})
	ul.AddNode(LineageNode{ID: "f1", Name: "Feature1", Kind: UnifiedNodeFeature})
	ul.AddEdge(LineageEdge{From: "s1", To: "f1"})
	ul.SetFreshness("f1", 2000, 1000) // SLA violation

	html := ul.GenerateVisualizationHTML()

	checks := []string{
		"Feather Lineage Explorer",
		"d3.v7.min.js",
		"Source1",
		"Feature1",
		"forceSimulation",
		"SLA VIOLATION",
		"resetView",
		"highlightSLA",
	}
	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("HTML should contain %q", check)
		}
	}
}
