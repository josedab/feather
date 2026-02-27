package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/activeactive"
)

// ActiveActiveHandler provides HTTP endpoints for active-active replication.
type ActiveActiveHandler struct {
	replicator *activeactive.Replicator
}

// NewActiveActiveHandler creates a new active-active handler.
func NewActiveActiveHandler(replicator *activeactive.Replicator) *ActiveActiveHandler {
	return &ActiveActiveHandler{replicator: replicator}
}

// RegisterRoutes registers active-active replication API routes.
func (h *ActiveActiveHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/activeactive/peers", h.handleListPeers)
	mux.HandleFunc("POST /v1/activeactive/peers", h.handleAddPeer)
	mux.HandleFunc("GET /v1/activeactive/peers/{id}", h.handleGetPeer)
	mux.HandleFunc("DELETE /v1/activeactive/peers/{id}", h.handleRemovePeer)
	mux.HandleFunc("POST /v1/activeactive/replicate", h.handleReplicate)
	mux.HandleFunc("POST /v1/activeactive/receive", h.handleReceive)
	mux.HandleFunc("POST /v1/activeactive/anti-entropy/{peerId}", h.handleAntiEntropy)
	mux.HandleFunc("GET /v1/activeactive/gossip", h.handleGossipState)
	mux.HandleFunc("GET /v1/activeactive/stats", h.handleStats)
}

func (h *ActiveActiveHandler) handleListPeers(w http.ResponseWriter, r *http.Request) {
	peers := h.replicator.ListPeers()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"total": len(peers),
	})
}

func (h *ActiveActiveHandler) handleAddPeer(w http.ResponseWriter, r *http.Request) {
	var peer activeactive.Peer
	if err := strictDecode(r.Body, &peer); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.replicator.AddPeer(&peer); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	added, _ := h.replicator.GetPeer(peer.ID)
	writeJSONResponse(r.Context(), w, http.StatusCreated, added)
}

func (h *ActiveActiveHandler) handleGetPeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	peer, err := h.replicator.GetPeer(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, peer)
}

func (h *ActiveActiveHandler) handleRemovePeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.replicator.RemovePeer(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *ActiveActiveHandler) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var msg activeactive.ReplicationMessage
	if err := strictDecode(r.Body, &msg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.replicator.Replicate(&msg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "replicated"})
}

func (h *ActiveActiveHandler) handleReceive(w http.ResponseWriter, r *http.Request) {
	var msg activeactive.ReplicationMessage
	if err := strictDecode(r.Body, &msg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.replicator.Receive(&msg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "received"})
}

func (h *ActiveActiveHandler) handleAntiEntropy(w http.ResponseWriter, r *http.Request) {
	peerID := r.PathValue("peerId")
	result, err := h.replicator.AntiEntropy(peerID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ActiveActiveHandler) handleGossipState(w http.ResponseWriter, r *http.Request) {
	state := h.replicator.GetGossipState()
	writeJSONResponse(r.Context(), w, http.StatusOK, state)
}

func (h *ActiveActiveHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.replicator.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
