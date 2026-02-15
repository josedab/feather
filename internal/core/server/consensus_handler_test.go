package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/feather-store/feather/internal/platform/consensus"
)

func setupConsensusHandler(t *testing.T) *http.ServeMux {
	t.Helper()
	raft := consensus.NewRaftNode(consensus.DefaultRaftConfig(), nil)
	shardMgr := consensus.NewShardManager(16, raft)
	handler := NewConsensusHandler(raft, shardMgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestConsensus_GetStatus(t *testing.T) {
	mux := setupConsensusHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/consensus/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsensus_PostAddPeer(t *testing.T) {
	mux := setupConsensusHandler(t)
	body := `{"peer_id":"node-2"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/consensus/peers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("expected 200 or 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestConsensus_PostInvalidJSON(t *testing.T) {
	mux := setupConsensusHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/consensus/peers", io.NopCloser(strings.NewReader("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
