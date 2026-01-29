package replication

import (
	"testing"
)

func TestLWWRegister_Merge(t *testing.T) {
	r1 := &LWWRegister{Value: "old", Timestamp: 100, RegionID: "us-east"}
	r2 := &LWWRegister{Value: "new", Timestamp: 200, RegionID: "eu-west"}

	result := r1.Merge(r2)
	if result.Value != "new" {
		t.Errorf("expected 'new', got %v", result.Value)
	}

	// Same timestamp: higher region ID wins.
	r3 := &LWWRegister{Value: "a", Timestamp: 100, RegionID: "a-region"}
	r4 := &LWWRegister{Value: "b", Timestamp: 100, RegionID: "b-region"}
	result = r3.Merge(r4)
	if result.Value != "b" {
		t.Errorf("expected 'b' (higher region), got %v", result.Value)
	}
}

func TestGCounter(t *testing.T) {
	c1 := NewGCounter()
	c1.Increment("us-east", 5)
	c1.Increment("eu-west", 3)

	if c1.Value() != 8 {
		t.Errorf("expected 8, got %d", c1.Value())
	}

	c2 := NewGCounter()
	c2.Increment("us-east", 7)
	c2.Increment("ap-south", 2)

	merged := c1.Merge(c2)
	// us-east: max(5,7)=7, eu-west: 3, ap-south: 2 => total 12
	if merged.Value() != 12 {
		t.Errorf("expected 12, got %d", merged.Value())
	}
}

func TestMerkleTree(t *testing.T) {
	tree := NewMerkleTree()
	tree.Insert("key1", "hash1")
	tree.Insert("key2", "hash2")

	hash1 := tree.RootHash()
	if hash1 == "" {
		t.Error("root hash should not be empty")
	}

	// Same data = same hash.
	tree2 := NewMerkleTree()
	tree2.Insert("key1", "hash1")
	tree2.Insert("key2", "hash2")
	if tree.RootHash() != tree2.RootHash() {
		t.Error("identical trees should have same root hash")
	}

	// Different data = different hash.
	tree2.Insert("key3", "hash3")
	if tree.RootHash() == tree2.RootHash() {
		t.Error("different trees should have different hashes")
	}
}

func TestMerkleTree_Diff(t *testing.T) {
	tree := NewMerkleTree()
	tree.Insert("key1", "hash1")
	tree.Insert("key2", "hash2")
	tree.Insert("key3", "hash3")

	otherData := map[string]string{
		"key1": "hash1",        // same
		"key2": "hash_changed", // different
		"key4": "hash4",        // new in other
	}

	diffs := tree.Diff(otherData)
	// key2 (changed), key3 (missing from other), key4 (missing from local)
	if len(diffs) != 3 {
		t.Errorf("expected 3 diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestMerkleTree_DataSnapshot(t *testing.T) {
	tree := NewMerkleTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")

	snap := tree.DataSnapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 entries, got %d", len(snap))
	}
}

func TestConflictResolver_LWW(t *testing.T) {
	resolver := NewConflictResolver(PolicyLastWriterWins)

	values := []*ReplicatedValue{
		{Value: "old", Clock: VectorClock{"r1": 1}, Origin: "r1"},
		{Value: "new", Clock: VectorClock{"r2": 5}, Origin: "r2"},
	}

	resolution, err := resolver.Resolve("key1", values)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.WinnerValue != "new" {
		t.Errorf("expected 'new', got %v", resolution.WinnerValue)
	}
	if resolution.WinnerRegion != "r2" {
		t.Errorf("expected r2, got %q", resolution.WinnerRegion)
	}
	if len(resolution.LoserRegions) != 1 {
		t.Errorf("expected 1 loser, got %d", len(resolution.LoserRegions))
	}
}

func TestConflictResolver_SingleValue(t *testing.T) {
	resolver := NewConflictResolver(PolicyLastWriterWins)

	values := []*ReplicatedValue{
		{Value: "only", Clock: VectorClock{"r1": 1}, Origin: "r1"},
	}

	resolution, err := resolver.Resolve("key1", values)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolution.Strategy != "single_value" {
		t.Errorf("expected single_value strategy, got %q", resolution.Strategy)
	}
}

func TestConflictResolver_NoValues(t *testing.T) {
	resolver := NewConflictResolver(PolicyLastWriterWins)
	_, err := resolver.Resolve("key", nil)
	if err == nil {
		t.Error("expected error for empty values")
	}
}

func TestConflictResolver_History(t *testing.T) {
	resolver := NewConflictResolver(PolicyLastWriterWins)

	values := []*ReplicatedValue{
		{Value: "a", Clock: VectorClock{"r1": 1}, Origin: "r1"},
		{Value: "b", Clock: VectorClock{"r2": 2}, Origin: "r2"},
	}
	resolver.Resolve("k1", values)
	resolver.Resolve("k2", values)

	history := resolver.History()
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestDataResidencyChecker(t *testing.T) {
	checker := NewDataResidencyChecker()
	checker.AddPolicy(DataResidencyPolicy{
		Name:           "eu-data",
		AllowedRegions: []string{"eu-west-1", "eu-central-1"},
	})

	ok, _ := checker.CheckCompliance("eu-west-1")
	if !ok {
		t.Error("eu-west-1 should be compliant")
	}

	ok, msg := checker.CheckCompliance("us-east-1")
	if ok {
		t.Error("us-east-1 should not be compliant")
	}
	if msg == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDataResidencyChecker_DeniedRegion(t *testing.T) {
	checker := NewDataResidencyChecker()
	checker.AddPolicy(DataResidencyPolicy{
		Name:          "no-china",
		DeniedRegions: []string{"cn-north-1"},
	})

	ok, _ := checker.CheckCompliance("us-east-1")
	if !ok {
		t.Error("us-east-1 should be allowed")
	}

	ok, _ = checker.CheckCompliance("cn-north-1")
	if ok {
		t.Error("cn-north-1 should be denied")
	}
}

func TestDataResidencyChecker_ListPolicies(t *testing.T) {
	checker := NewDataResidencyChecker()
	checker.AddPolicy(DataResidencyPolicy{Name: "p1"})
	checker.AddPolicy(DataResidencyPolicy{Name: "p2"})

	policies := checker.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(policies))
	}
}
