package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/extensions/sharding"
	"github.com/feather-store/feather/internal/platform/cluster"
)

// testShardingServer wraps a ShardingHandler for testing.
type testShardingServer struct {
	handler *ShardingHandler
	mux     *http.ServeMux
	t       *testing.T
}

func newTestShardingServer(t *testing.T) *testShardingServer {
	t.Helper()

	ring := cluster.NewHashRing(50)
	ring.AddNode(&cluster.Node{
		ID:           "node-a",
		Name:         "node-a",
		Address:      "127.0.0.1",
		DataPort:     8080,
		Weight:       100,
		VirtualNodes: 50,
		Zone:         "us-east-1a",
		Region:       "us-east-1",
	})
	ring.AddNode(&cluster.Node{
		ID:           "node-b",
		Name:         "node-b",
		Address:      "127.0.0.1",
		DataPort:     8081,
		Weight:       100,
		VirtualNodes: 50,
		Zone:         "us-east-1b",
		Region:       "us-east-1",
	})

	cfg := sharding.DefaultRouterConfig()
	cfg.LocalNodeID = "node-a"
	router := sharding.NewRouter(cfg, ring, nil)

	handler := NewShardingHandler(router)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &testShardingServer{
		handler: handler,
		mux:     mux,
		t:       t,
	}
}

func (ts *testShardingServer) request(method, path string, body string) *httptest.ResponseRecorder {
	ts.t.Helper()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

func (ts *testShardingServer) get(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodGet, path, "")
}

func (ts *testShardingServer) post(path string) *httptest.ResponseRecorder {
	return ts.request(http.MethodPost, path, "")
}

func TestShardingHandler_GetStats(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.get("/v1/sharding/stats")
	assertStatus(t, rr, http.StatusOK)

	body := assertJSON(t, rr)
	if body["success"] != true {
		t.Error("expected success=true")
	}
	if body["stats"] == nil {
		t.Error("expected stats field")
	}
}

func TestShardingHandler_GetPartition(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.get("/v1/sharding/partition?key=user:123")
	assertStatus(t, rr, http.StatusOK)

	body := assertJSON(t, rr)
	if body["success"] != true {
		t.Error("expected success=true")
	}
	if body["key"] != "user:123" {
		t.Errorf("expected key=user:123, got %v", body["key"])
	}
	if body["partition"] == nil {
		t.Error("expected partition field")
	}
	if body["owners"] == nil {
		t.Error("expected owners field")
	}
	if body["is_local"] == nil {
		t.Error("expected is_local field")
	}
}

func TestShardingHandler_GetPartition_MissingKey(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.get("/v1/sharding/partition")
	assertStatus(t, rr, http.StatusBadRequest)

	var body map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &body)
	if body["error"] == nil {
		t.Error("expected error field")
	}
}

func TestShardingHandler_GetOwners(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.get("/v1/sharding/owners?key=user:456")
	assertStatus(t, rr, http.StatusOK)

	body := assertJSON(t, rr)
	if body["success"] != true {
		t.Error("expected success=true")
	}
	if body["owners"] == nil {
		t.Error("expected owners field")
	}
}

func TestShardingHandler_GetOwners_MissingKey(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.get("/v1/sharding/owners")
	assertStatus(t, rr, http.StatusBadRequest)
}

func TestShardingHandler_Recompute(t *testing.T) {
	ts := newTestShardingServer(t)

	rr := ts.post("/v1/sharding/recompute")
	assertStatus(t, rr, http.StatusOK)

	body := assertJSON(t, rr)
	if body["success"] != true {
		t.Error("expected success=true")
	}
	if body["message"] != "partition map recomputed" {
		t.Errorf("unexpected message: %v", body["message"])
	}
}

func TestShardingHandler_PartitionConsistency(t *testing.T) {
	ts := newTestShardingServer(t)

	// Same key should always return same partition
	rr1 := ts.get("/v1/sharding/partition?key=consistent-key")
	rr2 := ts.get("/v1/sharding/partition?key=consistent-key")

	body1 := assertJSON(t, rr1)
	body2 := assertJSON(t, rr2)

	if body1["partition"] != body2["partition"] {
		t.Errorf("partition mismatch for same key: %v != %v", body1["partition"], body2["partition"])
	}
}
