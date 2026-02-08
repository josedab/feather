package composition

import (
	"errors"
	"testing"
	"time"
)

func TestNewDAG(t *testing.T) {
	dag := NewDAG("test-dag", "Test DAG")

	if dag.ID != "test-dag" {
		t.Errorf("Expected ID 'test-dag', got '%s'", dag.ID)
	}
	if dag.Name != "Test DAG" {
		t.Errorf("Expected Name 'Test DAG', got '%s'", dag.Name)
	}
	if len(dag.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(dag.Nodes))
	}
}

func TestDAG_AddNode(t *testing.T) {
	dag := NewDAG("test", "Test")

	node := &Node{
		ID:         "node1",
		Name:       "Node 1",
		Type:       NodeTypeSource,
		Expression: "feature1",
	}

	err := dag.AddNode(node)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	if len(dag.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(dag.Nodes))
	}

	// Try to add duplicate
	err = dag.AddNode(node)
	if err == nil {
		t.Error("Expected error when adding duplicate node")
	}
}

func TestDAG_AddNode_EmptyID(t *testing.T) {
	dag := NewDAG("test", "Test")

	node := &Node{
		Name: "Node without ID",
		Type: NodeTypeSource,
	}

	err := dag.AddNode(node)
	if err == nil {
		t.Error("Expected error when adding node without ID")
	}
}

func TestDAG_RemoveNode(t *testing.T) {
	dag := NewDAG("test", "Test")

	node := &Node{
		ID:   "node1",
		Name: "Node 1",
		Type: NodeTypeSource,
	}
	_ = dag.AddNode(node)

	err := dag.RemoveNode("node1")
	if err != nil {
		t.Fatalf("Failed to remove node: %v", err)
	}

	if len(dag.Nodes) != 0 {
		t.Errorf("Expected 0 nodes after removal, got %d", len(dag.Nodes))
	}
}

func TestDAG_RemoveNode_NotFound(t *testing.T) {
	dag := NewDAG("test", "Test")

	err := dag.RemoveNode("nonexistent")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestDAG_RemoveNode_HasDependents(t *testing.T) {
	dag := NewDAG("test", "Test")

	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "node2", Type: NodeTypeTransform, Inputs: []string{"node1"}})

	err := dag.RemoveNode("node1")
	if err == nil {
		t.Error("Expected error when removing node with dependents")
	}
}

func TestDAG_SetOutputs(t *testing.T) {
	dag := NewDAG("test", "Test")
	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeSource})

	err := dag.SetOutputs([]string{"node1"})
	if err != nil {
		t.Fatalf("Failed to set outputs: %v", err)
	}

	if len(dag.Outputs) != 1 || dag.Outputs[0] != "node1" {
		t.Errorf("Outputs not set correctly")
	}
}

func TestDAG_SetOutputs_NotFound(t *testing.T) {
	dag := NewDAG("test", "Test")

	err := dag.SetOutputs([]string{"nonexistent"})
	if err == nil {
		t.Error("Expected error when setting nonexistent output")
	}
}

func TestDAG_Validate_MissingDependency(t *testing.T) {
	dag := NewDAG("test", "Test")
	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeTransform, Inputs: []string{"nonexistent"}})

	err := dag.Validate()
	if err == nil {
		t.Error("Expected error for missing dependency")
	}
}

func TestDAG_Validate_Cycle(t *testing.T) {
	dag := NewDAG("test", "Test")

	// Create a cycle: node1 -> node2 -> node3 -> node1
	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeSource, Inputs: []string{}})
	_ = dag.AddNode(&Node{ID: "node2", Type: NodeTypeTransform, Inputs: []string{"node1"}})
	_ = dag.AddNode(&Node{ID: "node3", Type: NodeTypeTransform, Inputs: []string{"node2"}})

	// Manually add cycle
	dag.Nodes["node1"].Inputs = []string{"node3"}

	err := dag.Validate()
	if err == nil {
		t.Error("Expected error for cycle")
	}
}

func TestDAG_ComputeTopology(t *testing.T) {
	dag := NewDAG("test", "Test")

	// Linear DAG: source -> transform -> output
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource, Inputs: []string{}})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.AddNode(&Node{ID: "output", Type: NodeTypeAggregate, Inputs: []string{"transform"}})
	_ = dag.SetOutputs([]string{"output"})

	err := dag.ComputeTopology()
	if err != nil {
		t.Fatalf("Failed to compute topology: %v", err)
	}

	order := dag.GetTopologicalOrder()
	if len(order) != 3 {
		t.Errorf("Expected 3 nodes in topological order, got %d", len(order))
	}

	// Check levels
	if dag.GetLevel("source") != 0 {
		t.Errorf("Expected source at level 0, got %d", dag.GetLevel("source"))
	}
	if dag.GetLevel("output") != 2 {
		t.Errorf("Expected output at level 2, got %d", dag.GetLevel("output"))
	}
}

func TestDAG_GetNodesAtLevel(t *testing.T) {
	dag := NewDAG("test", "Test")

	// Diamond DAG: source -> (a, b) -> output
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource, Inputs: []string{}})
	_ = dag.AddNode(&Node{ID: "a", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.AddNode(&Node{ID: "b", Type: NodeTypeTransform, Inputs: []string{"source"}})
	_ = dag.AddNode(&Node{ID: "output", Type: NodeTypeJoin, Inputs: []string{"a", "b"}})
	_ = dag.ComputeTopology()

	level1Nodes := dag.GetNodesAtLevel(1)
	if len(level1Nodes) != 2 {
		t.Errorf("Expected 2 nodes at level 1, got %d", len(level1Nodes))
	}
}

func TestDAG_GetDependencies(t *testing.T) {
	dag := NewDAG("test", "Test")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})

	deps, err := dag.GetDependencies("transform")
	if err != nil {
		t.Fatalf("Failed to get dependencies: %v", err)
	}

	if len(deps) != 1 || deps[0] != "source" {
		t.Errorf("Expected dependencies ['source'], got %v", deps)
	}
}

func TestDAG_GetDependents(t *testing.T) {
	dag := NewDAG("test", "Test")
	_ = dag.AddNode(&Node{ID: "source", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source"}})

	deps := dag.GetDependents("source")
	if len(deps) != 1 || deps[0] != "transform" {
		t.Errorf("Expected dependents ['transform'], got %v", deps)
	}
}

func TestDAG_Clone(t *testing.T) {
	dag := NewDAG("test", "Test")
	dag.Description = "Original"
	_ = dag.AddNode(&Node{ID: "node1", Type: NodeTypeSource, Config: map[string]interface{}{"key": "value"}})
	_ = dag.SetOutputs([]string{"node1"})
	_ = dag.ComputeTopology()

	clone := dag.Clone()

	if clone.ID != dag.ID {
		t.Errorf("Clone ID mismatch")
	}
	if clone.Description != dag.Description {
		t.Errorf("Clone description mismatch")
	}

	// Modify clone and ensure original is unchanged
	clone.Description = "Modified"
	if dag.Description == "Modified" {
		t.Error("Original DAG was modified through clone")
	}

	clone.Nodes["node1"].Config["key"] = "modified"
	if dag.Nodes["node1"].Config["key"] == "modified" {
		t.Error("Original node config was modified through clone")
	}
}

func TestDAG_Stats(t *testing.T) {
	dag := NewDAG("test", "Test")
	_ = dag.AddNode(&Node{ID: "source1", Type: NodeTypeSource, CacheEnabled: true})
	_ = dag.AddNode(&Node{ID: "source2", Type: NodeTypeSource})
	_ = dag.AddNode(&Node{ID: "transform", Type: NodeTypeTransform, Inputs: []string{"source1", "source2"}})
	_ = dag.SetOutputs([]string{"transform"})
	_ = dag.ComputeTopology()

	stats := dag.Stats()

	if stats.NodeCount != 3 {
		t.Errorf("Expected 3 nodes, got %d", stats.NodeCount)
	}
	if stats.OutputCount != 1 {
		t.Errorf("Expected 1 output, got %d", stats.OutputCount)
	}
	if stats.CacheCount != 1 {
		t.Errorf("Expected 1 cache-enabled node, got %d", stats.CacheCount)
	}
	if stats.NodeTypes["source"] != 2 {
		t.Errorf("Expected 2 source nodes, got %d", stats.NodeTypes["source"])
	}
}

func TestNode_Defaults(t *testing.T) {
	dag := NewDAG("test", "Test")
	node := &Node{
		ID:   "node1",
		Type: NodeTypeSource,
	}

	_ = dag.AddNode(node)

	if node.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", node.Timeout)
	}
	if node.CacheTTL != 5*time.Minute {
		t.Errorf("Expected default cache TTL 5m, got %v", node.CacheTTL)
	}
}
