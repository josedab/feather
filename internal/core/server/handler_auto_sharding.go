package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/platform/cluster"
)

// ---------------------------------------------------------------------------
// AutoShardingHandler
// ---------------------------------------------------------------------------

// AutoShardingHandler exposes auto-sharding management endpoints.
type AutoShardingHandler struct {
	engine *cluster.AutoShardingEngine
}

// NewAutoShardingHandler creates a new AutoShardingHandler.
func NewAutoShardingHandler(engine *cluster.AutoShardingEngine) *AutoShardingHandler {
	return &AutoShardingHandler{engine: engine}
}

// RegisterRoutes registers auto-sharding API routes.
func (h *AutoShardingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/autosharding/nodes", h.handleAddNode)
	mux.HandleFunc("DELETE /v1/autosharding/nodes/{nodeID}", h.handleRemoveNode)
	mux.HandleFunc("GET /v1/autosharding/nodes", h.handleListNodes)
	mux.HandleFunc("GET /v1/autosharding/owner/{key}", h.handleGetOwner)
	mux.HandleFunc("GET /v1/autosharding/replicas/{key}", h.handleGetReplicas)
	mux.HandleFunc("POST /v1/autosharding/quorum/read", h.handleQuorumRead)
	mux.HandleFunc("POST /v1/autosharding/quorum/write", h.handleQuorumWrite)
	mux.HandleFunc("POST /v1/autosharding/rebalance", h.handleRebalance)
	mux.HandleFunc("GET /v1/autosharding/assignments", h.handleAssignments)
	mux.HandleFunc("GET /v1/autosharding/stats", h.handleStats)
}

func (h *AutoShardingHandler) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.engine.AddNode(req.NodeID, req.Address); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"added": req.NodeID})
}

func (h *AutoShardingHandler) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("nodeID")
	if err := h.engine.RemoveNode(nodeID); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"removed": nodeID})
}

func (h *AutoShardingHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.engine.ListNodes()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (h *AutoShardingHandler) handleGetOwner(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	owner, err := h.engine.GetOwner(key)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"key": key, "owner": owner})
}

func (h *AutoShardingHandler) handleGetReplicas(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	replicas, err := h.engine.GetReplicas(key, 3)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"key":      key,
		"replicas": replicas,
	})
}

func (h *AutoShardingHandler) handleQuorumRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.engine.QuorumRead(r.Context(), req.Key)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *AutoShardingHandler) handleQuorumWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.engine.QuorumWrite(r.Context(), req.Key, req.Value)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *AutoShardingHandler) handleRebalance(w http.ResponseWriter, r *http.Request) {
	moved, err := h.engine.TriggerRebalance()
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"shards_moved": moved,
		"imbalance":    h.engine.CheckImbalance(),
	})
}

func (h *AutoShardingHandler) handleAssignments(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.engine.GetAssignments())
}

func (h *AutoShardingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.engine.Stats())
}
