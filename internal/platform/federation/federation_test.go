package federation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestFederation_JoinLeaveNode(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	config.NodeName = "Local Node"
	config.NodeAddress = "http://localhost:8080"
	config.Region = "us-west-1"

	fed := NewFederation(config)

	// Join a node
	node := &Node{
		ID:      "remote-1",
		Name:    "Remote Node 1",
		Address: "http://remote1:8080",
		Region:  "us-east-1",
		Permissions: &NodePermissions{
			CanRead:      true,
			CanWrite:     true,
			CanReplicate: true,
		},
	}

	err := fed.JoinNode(node)
	if err != nil {
		t.Fatalf("JoinNode failed: %v", err)
	}

	// Verify node exists
	retrieved, err := fed.GetNode("remote-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved.Name != "Remote Node 1" {
		t.Errorf("expected name 'Remote Node 1', got %s", retrieved.Name)
	}

	// List nodes should include local + remote
	nodes := fed.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	// Leave the node
	err = fed.LeaveNode("remote-1")
	if err != nil {
		t.Fatalf("LeaveNode failed: %v", err)
	}

	// Verify node is gone
	_, err = fed.GetNode("remote-1")
	if err == nil {
		t.Error("expected error after leaving")
	}
}

func TestFederation_JoinDuplicateNode(t *testing.T) {
	fed := NewFederation(DefaultConfig())

	node := &Node{
		ID:      "node-1",
		Name:    "Node 1",
		Address: "http://node1:8080",
	}

	fed.JoinNode(node)

	// Try to join again
	err := fed.JoinNode(node)
	if err != nil && !errors.Is(err, ErrNodeAlreadyExists) {
		t.Errorf("expected ErrNodeAlreadyExists, got %v", err)
	}
}

func TestFederation_ShareFeature(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	feature := &FederatedFeature{
		ID:          "user_purchase_count",
		Name:        "User Purchase Count",
		Description: "Total number of purchases by user",
		OwnerTeam:   "data-science",
		DataType:    "int64",
		Tags:        []string{"user", "purchases", "ecommerce"},
		Visibility:  VisibilityFederation,
	}

	err := fed.ShareFeature(feature)
	if err != nil {
		t.Fatalf("ShareFeature failed: %v", err)
	}

	// Retrieve the feature
	retrieved, err := fed.GetFeature(context.Background(), "user_purchase_count")
	if err != nil {
		t.Fatalf("GetFeature failed: %v", err)
	}

	if retrieved.Name != "User Purchase Count" {
		t.Errorf("expected name 'User Purchase Count', got %s", retrieved.Name)
	}

	if retrieved.OwnerNode != "local-1" {
		t.Errorf("expected owner node 'local-1', got %s", retrieved.OwnerNode)
	}

	if retrieved.Version != 1 {
		t.Errorf("expected version 1, got %d", retrieved.Version)
	}
}

func TestFederation_UpdateFeature(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share a feature
	feature := &FederatedFeature{
		ID:          "feature-1",
		Name:        "Feature One",
		Description: "Original description",
		Visibility:  VisibilityFederation,
	}
	fed.ShareFeature(feature)

	// Update it
	updated := &FederatedFeature{
		ID:          "feature-1",
		Name:        "Feature One Updated",
		Description: "Updated description",
		Visibility:  VisibilityFederation,
	}

	err := fed.UpdateFeature(updated)
	if err != nil {
		t.Fatalf("UpdateFeature failed: %v", err)
	}

	// Verify update
	retrieved, _ := fed.GetFeature(context.Background(), "feature-1")
	if retrieved.Name != "Feature One Updated" {
		t.Errorf("expected updated name, got %s", retrieved.Name)
	}

	if retrieved.Version != 2 {
		t.Errorf("expected version 2, got %d", retrieved.Version)
	}
}

func TestFederation_DeleteFeature(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share a feature
	feature := &FederatedFeature{
		ID:         "feature-to-delete",
		Name:       "Delete Me",
		Visibility: VisibilityFederation,
	}
	fed.ShareFeature(feature)

	// Delete it
	err := fed.DeleteFeature("feature-to-delete")
	if err != nil {
		t.Fatalf("DeleteFeature failed: %v", err)
	}

	// Verify it's gone
	_, err = fed.GetFeature(context.Background(), "feature-to-delete")
	if err != nil && !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestFederation_SearchFeatures(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share multiple features
	features := []*FederatedFeature{
		{
			ID:         "f1",
			Name:       "User Feature",
			OwnerTeam:  "team-a",
			Tags:       []string{"user", "core"},
			DataType:   "int64",
			Visibility: VisibilityFederation,
		},
		{
			ID:         "f2",
			Name:       "Product Feature",
			OwnerTeam:  "team-b",
			Tags:       []string{"product"},
			DataType:   "float64",
			Visibility: VisibilityFederation,
		},
		{
			ID:         "f3",
			Name:       "Another User Feature",
			OwnerTeam:  "team-a",
			Tags:       []string{"user", "experimental"},
			DataType:   "int64",
			Visibility: VisibilityFederation,
		},
	}

	for _, f := range features {
		fed.ShareFeature(f)
	}

	// Search by team
	results, err := fed.SearchFeatures(SearchQuery{
		Teams: []string{"team-a"},
	})
	if err != nil {
		t.Fatalf("SearchFeatures failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for team-a, got %d", len(results))
	}

	// Search by tag
	results, err = fed.SearchFeatures(SearchQuery{
		Tags: []string{"user"},
	})
	if err != nil {
		t.Fatalf("SearchFeatures failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for tag 'user', got %d", len(results))
	}

	// Search by data type
	results, err = fed.SearchFeatures(SearchQuery{
		DataTypes: []string{"float64"},
	})
	if err != nil {
		t.Fatalf("SearchFeatures failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for float64, got %d", len(results))
	}
}

func TestFederation_AccessControl(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share a private feature
	privateFeature := &FederatedFeature{
		ID:         "private-feature",
		Name:       "Private Feature",
		Visibility: VisibilityPrivate,
	}
	fed.ShareFeature(privateFeature)

	// Should be accessible (we own it)
	_, err := fed.GetFeature(context.Background(), "private-feature")
	if err != nil {
		t.Errorf("expected access to own private feature, got %v", err)
	}

	// Share a feature with access control
	restrictedFeature := &FederatedFeature{
		ID:         "restricted-feature",
		Name:       "Restricted Feature",
		Visibility: VisibilityFederation,
		AccessControl: &AccessControl{
			AllowedNodes: []string{"local-1", "allowed-node"},
		},
	}
	fed.ShareFeature(restrictedFeature)

	// Should be accessible (we're in allowed list)
	_, err = fed.GetFeature(context.Background(), "restricted-feature")
	if err != nil {
		t.Errorf("expected access to restricted feature, got %v", err)
	}
}

func TestFederation_ReplicationPolicy(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share a feature
	fed.ShareFeature(&FederatedFeature{
		ID:         "replicated-feature",
		Name:       "Replicated Feature",
		Visibility: VisibilityFederation,
	})

	// Set replication policy
	policy := &ReplicationPolicy{
		Mode:               ReplicationModeAsync,
		TargetRegions:      []string{"us-east-1", "eu-west-1"},
		MinReplicas:        2,
		MaxReplicas:        5,
		SyncInterval:       time.Minute,
		ConflictResolution: "last_write_wins",
	}

	err := fed.SetReplicationPolicy("replicated-feature", policy)
	if err != nil {
		t.Fatalf("SetReplicationPolicy failed: %v", err)
	}
}

func TestFederation_ReplicateFeature(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Add a target node
	fed.JoinNode(&Node{
		ID:      "remote-1",
		Address: "http://remote1:8080",
		State:   NodeStateHealthy,
		Permissions: &NodePermissions{
			CanReplicate: true,
		},
	})

	// Share a feature
	fed.ShareFeature(&FederatedFeature{
		ID:         "to-replicate",
		Name:       "To Replicate",
		Visibility: VisibilityFederation,
	})

	// Replicate to node
	err := fed.ReplicateFeature(context.Background(), "to-replicate", []string{"remote-1"})
	if err != nil {
		t.Fatalf("ReplicateFeature failed: %v", err)
	}

	// Verify replicated_to list updated
	feature, _ := fed.GetFeature(context.Background(), "to-replicate")
	if len(feature.ReplicatedTo) != 1 || feature.ReplicatedTo[0] != "remote-1" {
		t.Errorf("expected replicated_to to contain 'remote-1'")
	}
}

func TestFederation_Events(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	var events []Event
	var eventsMu sync.Mutex
	fed.OnEvent(func(e Event) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	})

	// Trigger events
	fed.JoinNode(&Node{ID: "node-1", Address: "http://node1:8080"})
	fed.ShareFeature(&FederatedFeature{ID: "f1", Visibility: VisibilityFederation})
	fed.LeaveNode("node-1")

	// Give time for async event handlers
	time.Sleep(50 * time.Millisecond)

	eventsMu.Lock()
	eventCount := len(events)
	eventsCopy := make([]Event, len(events))
	copy(eventsCopy, events)
	eventsMu.Unlock()

	if eventCount < 3 {
		t.Errorf("expected at least 3 events, got %d", eventCount)
	}

	// Check event types
	hasJoin := false
	hasShare := false
	hasLeave := false
	for _, e := range eventsCopy {
		switch e.Type {
		case EventNodeJoined:
			hasJoin = true
		case EventFeatureShared:
			hasShare = true
		case EventNodeLeft:
			hasLeave = true
		case EventNodeUnhealthy:
		case EventFeatureUpdated:
		case EventFeatureDeleted:
		case EventReplicationDone:
		case EventSyncCompleted:
		}
	}

	if !hasJoin || !hasShare || !hasLeave {
		t.Error("missing expected event types")
	}
}

func TestFederation_Stats(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	config.Region = "us-west-1"
	fed := NewFederation(config)

	// Add nodes in different regions
	fed.JoinNode(&Node{
		ID:      "node-1",
		Address: "http://node1:8080",
		Region:  "us-east-1",
		State:   NodeStateHealthy,
	})
	fed.JoinNode(&Node{
		ID:      "node-2",
		Address: "http://node2:8080",
		Region:  "eu-west-1",
		State:   NodeStateHealthy,
	})

	// Share features
	fed.ShareFeature(&FederatedFeature{ID: "f1", Visibility: VisibilityFederation})
	fed.ShareFeature(&FederatedFeature{ID: "f2", Visibility: VisibilityFederation})

	stats := fed.GetStats()

	if stats["total_nodes"].(int) != 3 {
		t.Errorf("expected 3 total nodes, got %v", stats["total_nodes"])
	}

	if stats["healthy_nodes"].(int) != 3 {
		t.Errorf("expected 3 healthy nodes, got %v", stats["healthy_nodes"])
	}

	if stats["total_features"].(int) != 2 {
		t.Errorf("expected 2 total features, got %v", stats["total_features"])
	}

	regions := stats["regions"].(map[string]int)
	if regions["us-east-1"] != 1 || regions["eu-west-1"] != 1 {
		t.Errorf("unexpected region distribution: %v", regions)
	}
}

func TestFederation_ListCatalog(t *testing.T) {
	config := DefaultConfig()
	config.NodeID = "local-1"
	fed := NewFederation(config)

	// Share features with different visibility
	fed.ShareFeature(&FederatedFeature{
		ID:         "public-feature",
		Visibility: VisibilityPublic,
	})
	fed.ShareFeature(&FederatedFeature{
		ID:         "federation-feature",
		Visibility: VisibilityFederation,
	})
	fed.ShareFeature(&FederatedFeature{
		ID:         "private-feature",
		Visibility: VisibilityPrivate,
	})

	catalog := fed.ListCatalog()

	// All should be accessible (we own them all)
	if len(catalog) != 3 {
		t.Errorf("expected 3 catalog entries, got %d", len(catalog))
	}

	// Verify local copy flag
	for _, entry := range catalog {
		if !entry.LocalCopy {
			t.Errorf("expected local copy for feature %s", entry.Feature.ID)
		}
	}
}

func TestFederation_InvalidOperations(t *testing.T) {
	fed := NewFederation(DefaultConfig())

	// Join with empty ID
	err := fed.JoinNode(&Node{Address: "http://test:8080"})
	if err != nil && !errors.Is(err, ErrInvalidNodeID) {
		t.Errorf("expected ErrInvalidNodeID, got %v", err)
	}

	// Leave non-existent node
	err = fed.LeaveNode("non-existent")
	if err != nil && !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}

	// Share with empty ID
	err = fed.ShareFeature(&FederatedFeature{Name: "Test"})
	if err != nil && !errors.Is(err, ErrInvalidFeatureID) {
		t.Errorf("expected ErrInvalidFeatureID, got %v", err)
	}

	// Update non-existent feature
	err = fed.UpdateFeature(&FederatedFeature{ID: "non-existent"})
	if err != nil && !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}

	// Delete non-existent feature
	err = fed.DeleteFeature("non-existent")
	if err != nil && !errors.Is(err, ErrFeatureNotFound) {
		t.Errorf("expected ErrFeatureNotFound, got %v", err)
	}
}

func TestFederation_StartStop(t *testing.T) {
	config := DefaultConfig()
	config.HealthCheckInterval = 100 * time.Millisecond
	config.SyncInterval = 100 * time.Millisecond
	fed := NewFederation(config)

	// Start should not panic
	fed.Start()

	// Give some time for background tasks
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	fed.Stop()
}

func TestVisibility_AllTypes(t *testing.T) {
	visibilities := []Visibility{
		VisibilityPrivate,
		VisibilityTeam,
		VisibilityOrg,
		VisibilityFederation,
		VisibilityPublic,
	}

	for _, v := range visibilities {
		if string(v) == "" {
			t.Errorf("visibility %v has empty string representation", v)
		}
	}
}

func TestNodeRole_AllTypes(t *testing.T) {
	roles := []NodeRole{
		NodeRoleLeader,
		NodeRoleFollower,
		NodeRolePeer,
	}

	for _, r := range roles {
		if string(r) == "" {
			t.Errorf("role %v has empty string representation", r)
		}
	}
}

func TestReplicationMode_AllTypes(t *testing.T) {
	modes := []ReplicationMode{
		ReplicationModeSync,
		ReplicationModeAsync,
		ReplicationModeOnDemand,
	}

	for _, m := range modes {
		if string(m) == "" {
			t.Errorf("mode %v has empty string representation", m)
		}
	}
}
