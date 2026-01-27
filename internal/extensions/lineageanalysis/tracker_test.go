package lineageanalysis

import (
	"errors"
	"testing"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	if tr == nil {
		t.Fatal("expected non-nil tracker")
	}
}

func TestAddAndGetNode(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	err := tr.AddNode(LineageNode{ID: "users_db", Name: "Users Database", Type: NodeSource})
	if err != nil {
		t.Fatal(err)
	}

	node, err := tr.GetNode("users_db")
	if err != nil {
		t.Fatal(err)
	}
	if node.Type != NodeSource {
		t.Errorf("expected source type, got %s", node.Type)
	}
}

func TestDuplicateNode(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "n1", Name: "N1", Type: NodeFeature})
	err := tr.AddNode(LineageNode{ID: "n1", Name: "N1 dup", Type: NodeFeature})
	if !errors.Is(err, ErrNodeExists) {
		t.Fatalf("expected ErrNodeExists, got %v", err)
	}
}

func TestAddEdge(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "src", Name: "Source", Type: NodeSource})
	_ = tr.AddNode(LineageNode{ID: "feat", Name: "Feature", Type: NodeFeature})

	err := tr.AddEdge(LineageEdge{FromID: "src", ToID: "feat", Type: "derives_from"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCycleDetection(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "a", Name: "A", Type: NodeFeature})
	_ = tr.AddNode(LineageNode{ID: "b", Name: "B", Type: NodeFeature})
	_ = tr.AddEdge(LineageEdge{FromID: "a", ToID: "b"})

	err := tr.AddEdge(LineageEdge{FromID: "b", ToID: "a"})
	if !errors.Is(err, ErrCyclicLineage) {
		t.Fatalf("expected ErrCyclicLineage, got %v", err)
	}
}

func TestUpstreamDownstream(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "db", Name: "Users DB", Type: NodeSource})
	_ = tr.AddNode(LineageNode{ID: "transform", Name: "Age Bucket", Type: NodeTransformation})
	_ = tr.AddNode(LineageNode{ID: "feature", Name: "User Age Bucket", Type: NodeFeature})
	_ = tr.AddNode(LineageNode{ID: "model", Name: "Recommendation Model", Type: NodeConsumer})

	_ = tr.AddEdge(LineageEdge{FromID: "db", ToID: "transform"})
	_ = tr.AddEdge(LineageEdge{FromID: "transform", ToID: "feature"})
	_ = tr.AddEdge(LineageEdge{FromID: "feature", ToID: "model"})

	upstream := tr.GetUpstream("feature")
	if len(upstream) != 2 {
		t.Errorf("expected 2 upstream nodes, got %d", len(upstream))
	}

	downstream := tr.GetDownstream("db")
	if len(downstream) != 3 {
		t.Errorf("expected 3 downstream nodes, got %d", len(downstream))
	}
}

func TestImpactAnalysis(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "db", Name: "Users DB", Type: NodeSource})
	_ = tr.AddNode(LineageNode{ID: "t1", Name: "Transform 1", Type: NodeTransformation})
	_ = tr.AddNode(LineageNode{ID: "f1", Name: "Feature 1", Type: NodeFeature})
	_ = tr.AddNode(LineageNode{ID: "f2", Name: "Feature 2", Type: NodeFeature})
	_ = tr.AddNode(LineageNode{ID: "m1", Name: "Model 1", Type: NodeConsumer})

	_ = tr.AddEdge(LineageEdge{FromID: "db", ToID: "t1"})
	_ = tr.AddEdge(LineageEdge{FromID: "t1", ToID: "f1"})
	_ = tr.AddEdge(LineageEdge{FromID: "t1", ToID: "f2"})
	_ = tr.AddEdge(LineageEdge{FromID: "f1", ToID: "m1"})

	report, err := tr.AnalyzeImpact("db")
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalAffected != 4 {
		t.Errorf("expected 4 affected nodes, got %d", report.TotalAffected)
	}
	if report.MaxDepth < 3 {
		t.Errorf("expected max depth >= 3, got %d", report.MaxDepth)
	}
}

func TestListNodes(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "s1", Name: "S1", Type: NodeSource})
	_ = tr.AddNode(LineageNode{ID: "f1", Name: "F1", Type: NodeFeature})

	all := tr.ListNodes("")
	if len(all) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(all))
	}

	sources := tr.ListNodes("source")
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestStats(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	_ = tr.AddNode(LineageNode{ID: "s1", Name: "S1", Type: NodeSource})
	_ = tr.AddNode(LineageNode{ID: "f1", Name: "F1", Type: NodeFeature})
	_ = tr.AddEdge(LineageEdge{FromID: "s1", ToID: "f1"})

	stats := tr.Stats()
	if stats["total_nodes"] != 2 {
		t.Errorf("expected 2 nodes, got %v", stats["total_nodes"])
	}
	if stats["total_edges"] != 1 {
		t.Errorf("expected 1 edge, got %v", stats["total_edges"])
	}
}
