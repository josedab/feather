package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/feather-store/feather/internal/platform/cluster"
)

func TestClusterHandler_ListNodes(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/nodes", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := response["nodes"]; !ok {
		t.Error("expected nodes field in response")
	}
	if _, ok := response["count"]; !ok {
		t.Error("expected count field in response")
	}
}

func TestClusterHandler_ListNodes_AliveFilter(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/nodes?status=alive", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestClusterHandler_GetNode_NotFound(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/nodes/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestClusterHandler_GetLocalNode(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/local", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response ClusterNodeJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.ID == "" {
		t.Error("expected non-empty node ID")
	}
}

func TestClusterHandler_GetStats(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/stats", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response cluster.MembershipStats
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
}

func TestClusterHandler_GetRing(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HashRingJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.NodeCount != 1 {
		t.Errorf("expected node count 1, got %d", response.NodeCount)
	}
}

func TestClusterHandler_GetRing_WithDistribution(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring?distribution=true", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response HashRingJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Distribution == nil {
		t.Error("expected distribution in response")
	}
}

func TestClusterHandler_RingLookup(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring/lookup?key=test-key", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["key"] != "test-key" {
		t.Error("expected key in response")
	}
	if _, ok := response["nodes"]; !ok {
		t.Error("expected nodes in response")
	}
}

func TestClusterHandler_RingLookup_NoKey(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring/lookup", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestClusterHandler_RingLookup_WithCount(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})
	ring.AddNode(&cluster.Node{ID: "node-2", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring/lookup?key=test-key&count=2", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	nodes := response["nodes"].([]interface{})
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestClusterHandler_ListPartitions(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["total"] != float64(64) {
		t.Errorf("expected total 64, got %v", response["total"])
	}
	if response["replication_factor"] != float64(2) {
		t.Errorf("expected replication_factor 2, got %v", response["replication_factor"])
	}
}

func TestClusterHandler_ListPartitions_ByNode(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions?node=node-1", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestClusterHandler_GetPartition(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions/0", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response PartitionJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.ID != 0 {
		t.Errorf("expected partition ID 0, got %d", response.ID)
	}
}

func TestClusterHandler_GetPartition_InvalidID(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions/invalid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestClusterHandler_GetPartition_OutOfRange(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions/999", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestClusterHandler_GetPartitionForKey(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/partitions/key/my-key", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["key"] != "my-key" {
		t.Error("expected key in response")
	}
	if _, ok := response["partition"]; !ok {
		t.Error("expected partition in response")
	}
}

func TestClusterHandler_GetRebalanceStatus(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/rebalance", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := response["total_rebalances"]; !ok {
		t.Error("expected total_rebalances in response")
	}
	if _, ok := response["partition_distribution"]; !ok {
		t.Error("expected partition_distribution in response")
	}
}

func TestClusterHandler_TriggerRebalance_NoChangesNeeded(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/v1/cluster/rebalance", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Should return conflict when no rebalancing needed
	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestClusterHandler_CancelRebalance_NoPlanInProgress(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("DELETE", "/v1/cluster/rebalance", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestClusterHandler_GetRebalanceHistory(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/rebalance/history", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := response["history"]; !ok {
		t.Error("expected history in response")
	}
	if _, ok := response["count"]; !ok {
		t.Error("expected count in response")
	}
}

func TestClusterHandler_GetRebalanceHistory_WithLimit(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/rebalance/history?limit=5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestClusterHandler_GetRebalanceHistory_InvalidLimit(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/rebalance/history?limit=invalid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestClusterHandler_NilMembership(t *testing.T) {
	handler := NewClusterHandler(nil, nil, nil, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"ListNodes", "GET", "/v1/cluster/nodes"},
		{"GetNode", "GET", "/v1/cluster/nodes/test"},
		{"GetLocalNode", "GET", "/v1/cluster/local"},
		{"GetStats", "GET", "/v1/cluster/stats"},
		{"GetRing", "GET", "/v1/cluster/ring"},
		{"RingLookup", "GET", "/v1/cluster/ring/lookup?key=test"},
		{"ListPartitions", "GET", "/v1/cluster/partitions"},
		{"GetPartition", "GET", "/v1/cluster/partitions/0"},
		{"GetPartitionForKey", "GET", "/v1/cluster/partitions/key/test"},
		{"GetRebalanceStatus", "GET", "/v1/cluster/rebalance"},
		{"TriggerRebalance", "POST", "/v1/cluster/rebalance"},
		{"CancelRebalance", "DELETE", "/v1/cluster/rebalance"},
		{"GetRebalanceHistory", "GET", "/v1/cluster/rebalance/history"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusServiceUnavailable {
				t.Errorf("expected status 503, got %d", rr.Code)
			}
		})
	}
}

func TestClusterHandler_RingLookup_InvalidCount(t *testing.T) {
	config := cluster.DefaultMembershipConfig()
	membership := cluster.NewMembershipManager(config)

	ring := cluster.NewHashRing(100)
	ring.AddNode(&cluster.Node{ID: "node-1", Weight: 100, VirtualNodes: 100})

	pm := cluster.NewPartitionMap(ring, 64, 2)
	rebalancer := cluster.NewRebalancer(cluster.DefaultRebalancerConfig(), membership, ring, pm)

	handler := NewClusterHandler(membership, ring, pm, rebalancer)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/cluster/ring/lookup?key=test&count=invalid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestClusterHandler_NodeToJSON(t *testing.T) {
	handler := NewClusterHandler(nil, nil, nil, nil)

	node := &cluster.Node{
		ID:           "test-node",
		Name:         "Test Node",
		Address:      "127.0.0.1",
		GossipPort:   7946,
		DataPort:     7947,
		Status:       cluster.NodeStatusAlive,
		Role:         cluster.NodeRoleFollower,
		Zone:         "us-east-1a",
		Region:       "us-east-1",
		Weight:       100,
		VirtualNodes: 150,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
		Generation: 5,
	}

	json := handler.nodeToJSON(node)

	if json.ID != "test-node" {
		t.Errorf("expected ID 'test-node', got '%s'", json.ID)
	}
	if json.Name != "Test Node" {
		t.Errorf("expected Name 'Test Node', got '%s'", json.Name)
	}
	if json.Status != "alive" {
		t.Errorf("expected Status 'alive', got '%s'", json.Status)
	}
	if json.Role != "follower" {
		t.Errorf("expected Role 'follower', got '%s'", json.Role)
	}
	if json.Metadata["version"] != "1.0.0" {
		t.Error("expected metadata to be preserved")
	}
}
