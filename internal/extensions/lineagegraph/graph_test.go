package lineagegraph

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	stats := g.Stats()
	if stats.TotalNodes != 0 {
		t.Errorf("expected 0 nodes, got %d", stats.TotalNodes)
	}
}

func TestAddNodeAndEdge(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "src", Name: "Kafka", Type: NodeSource})
	_ = g.AddNode(Node{ID: "feat", Name: "user_age", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "model", Name: "fraud_model", Type: NodeModel})

	if err := g.AddEdge("src", "feat", "ingests"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("feat", "model", "serves"); err != nil {
		t.Fatal(err)
	}

	view := g.GetView()
	if view.TotalNodes != 3 {
		t.Errorf("expected 3 nodes, got %d", view.TotalNodes)
	}
	if view.TotalEdges != 2 {
		t.Errorf("expected 2 edges, got %d", view.TotalEdges)
	}
}

func TestCycleDetection(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "a", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "b", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "c", Type: NodeFeature})
	_ = g.AddEdge("a", "b", "")
	_ = g.AddEdge("b", "c", "")

	err := g.AddEdge("c", "a", "")
	if err != ErrCyclicDependency {
		t.Fatalf("expected ErrCyclicDependency, got %v", err)
	}
}

func TestUpstreamDownstream(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "s1", Type: NodeSource})
	_ = g.AddNode(Node{ID: "f1", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "f2", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "m1", Type: NodeModel})
	_ = g.AddEdge("s1", "f1", "")
	_ = g.AddEdge("f1", "f2", "")
	_ = g.AddEdge("f2", "m1", "")

	upstream, _ := g.GetUpstream("m1")
	if len(upstream) != 3 {
		t.Errorf("expected 3 upstream nodes, got %d", len(upstream))
	}

	downstream, _ := g.GetDownstream("s1")
	if len(downstream) != 3 {
		t.Errorf("expected 3 downstream nodes, got %d", len(downstream))
	}
}

func TestImpactAnalysis(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "s", Type: NodeSource})
	_ = g.AddNode(Node{ID: "f1", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "f2", Type: NodeFeature})
	_ = g.AddEdge("s", "f1", "")
	_ = g.AddEdge("s", "f2", "")

	impact, err := g.GetImpact("s")
	if err != nil {
		t.Fatal(err)
	}
	if impact.TotalImpact != 2 {
		t.Errorf("expected 2 impacted, got %d", impact.TotalImpact)
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "a", Type: NodeFeature})
	_ = g.AddNode(Node{ID: "b", Type: NodeFeature})
	_ = g.AddEdge("a", "b", "")

	if err := g.RemoveNode("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.GetNode("a"); err != ErrNodeNotFound {
		t.Fatal("expected node to be removed")
	}
}

func TestDuplicateNode(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())
	_ = g.AddNode(Node{ID: "a", Type: NodeFeature})
	err := g.AddNode(Node{ID: "a", Type: NodeFeature})
	if err != ErrNodeExists {
		t.Fatalf("expected ErrNodeExists, got %v", err)
	}
}
