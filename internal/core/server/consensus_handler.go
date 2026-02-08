package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/consensus"
)

// ConsensusHandler handles distributed consensus API requests.
type ConsensusHandler struct {
	raft         *consensus.RaftNode
	shardManager *consensus.ShardManager
}

// NewConsensusHandler creates a new consensus handler.
func NewConsensusHandler(raft *consensus.RaftNode, shardManager *consensus.ShardManager) *ConsensusHandler {
	return &ConsensusHandler{raft: raft, shardManager: shardManager}
}

// RegisterRoutes registers consensus API routes.
func (h *ConsensusHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/consensus/status", h.handleGetStatus)
	mux.HandleFunc("GET /v1/consensus/leader", h.handleGetLeader)
	mux.HandleFunc("GET /v1/consensus/log", h.handleGetLog)
	mux.HandleFunc("POST /v1/consensus/propose", h.handlePropose)
	mux.HandleFunc("GET /v1/consensus/shards", h.handleListShards)
	mux.HandleFunc("GET /v1/consensus/shards/{id}", h.handleGetShard)
	mux.HandleFunc("POST /v1/consensus/shards/rebalance", h.handleRebalance)
	mux.HandleFunc("GET /v1/consensus/peers", h.handleListPeers)
	mux.HandleFunc("POST /v1/consensus/peers", h.handleAddPeer)
	mux.HandleFunc("DELETE /v1/consensus/peers/{id}", h.handleRemovePeer)
	mux.HandleFunc("GET /v1/consensus/health", h.handleClusterHealth)
}

func (h *ConsensusHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.raft == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "raft node not configured")
		return
	}
	state, term, isLeader := h.raft.GetState(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"state":     string(state),
		"term":      term,
		"is_leader": isLeader,
	})
}

func (h *ConsensusHandler) handleGetLeader(w http.ResponseWriter, r *http.Request) {
	if h.raft == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "raft node not configured")
		return
	}
	leader := h.raft.GetLeader(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"leader": leader})
}

func (h *ConsensusHandler) handleGetLog(w http.ResponseWriter, r *http.Request) {
	if h.raft == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "raft node not configured")
		return
	}
	entries := h.raft.GetLog(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"entries": entries})
}

type proposeRequest struct {
	Command []byte `json:"command"`
}

func (h *ConsensusHandler) handlePropose(w http.ResponseWriter, r *http.Request) {
	if h.raft == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "raft node not configured")
		return
	}
	var req proposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	entry, err := h.raft.Propose(r.Context(), req.Command)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, entry)
}

func (h *ConsensusHandler) handleListShards(w http.ResponseWriter, r *http.Request) {
	if h.shardManager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "shard manager not configured")
		return
	}
	shards := h.shardManager.ListShards(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"shards": shards})
}

func (h *ConsensusHandler) handleGetShard(w http.ResponseWriter, r *http.Request) {
	if h.shardManager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "shard manager not configured")
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid shard id")
		return
	}
	info, err := h.shardManager.GetShardInfo(r.Context(), id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, info)
}

type rebalanceRequest struct {
	NodeIDs []string `json:"node_ids"`
}

func (h *ConsensusHandler) handleRebalance(w http.ResponseWriter, r *http.Request) {
	if h.shardManager == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "shard manager not configured")
		return
	}
	var req rebalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.shardManager.RebalanceShards(r.Context(), req.NodeIDs); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "message": "rebalance initiated"})
}

func (h *ConsensusHandler) handleListPeers(w http.ResponseWriter, r *http.Request) {
	peers := h.raft.ListPeers(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"peers": peers})
}

func (h *ConsensusHandler) handleAddPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.raft.AddPeer(r.Context(), req.PeerID); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"added": req.PeerID})
}

func (h *ConsensusHandler) handleRemovePeer(w http.ResponseWriter, r *http.Request) {
	peerID := r.PathValue("id")
	if err := h.raft.RemovePeer(r.Context(), peerID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"removed": peerID})
}

func (h *ConsensusHandler) handleClusterHealth(w http.ResponseWriter, r *http.Request) {
	health := h.raft.GetClusterHealth(r.Context())
	h.writeJSON(r.Context(), w, http.StatusOK, health)
}

func (h *ConsensusHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ConsensusHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
