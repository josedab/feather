package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/mesh"
)

// MeshHandler provides HTTP endpoints for the feature store mesh.
type MeshHandler struct {
	manager *mesh.MeshManager
}

// NewMeshHandler creates a new mesh handler.
func NewMeshHandler(manager *mesh.MeshManager) *MeshHandler {
	return &MeshHandler{manager: manager}
}

// RegisterRoutes registers mesh API routes.
func (h *MeshHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/mesh/nodes", h.handleListNodes)
	mux.HandleFunc("POST /v1/mesh/nodes", h.handleRegisterNode)
	mux.HandleFunc("DELETE /v1/mesh/nodes/{id}", h.handleDeregisterNode)
	mux.HandleFunc("GET /v1/mesh/nodes/{id}", h.handleGetNode)
	mux.HandleFunc("GET /v1/mesh/route", h.handleRoute)
	mux.HandleFunc("GET /v1/mesh/circuits", h.handleListCircuits)
	mux.HandleFunc("POST /v1/mesh/circuits/{id}/reset", h.handleResetCircuit)
	mux.HandleFunc("GET /v1/mesh/health", h.handleHealth)
	mux.HandleFunc("GET /v1/mesh/stats", h.handleStats)

	// Mesh protocol (cross-org feature sharing)
	mux.HandleFunc("GET /v1/mesh/orgs", h.handleListOrgs)
	mux.HandleFunc("POST /v1/mesh/orgs", h.handleRegisterOrg)
	mux.HandleFunc("GET /v1/mesh/grants", h.handleListGrants)
	mux.HandleFunc("POST /v1/mesh/grants", h.handleGrantAccess)
	mux.HandleFunc("DELETE /v1/mesh/grants/{id}", h.handleRevokeAccess)
	mux.HandleFunc("POST /v1/mesh/transfer", h.handleTransferFeatures)
	mux.HandleFunc("GET /v1/mesh/protocol/stats", h.handleProtocolStats)
}

func (h *MeshHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.manager.Registry().ListNodes()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
	})
}

func (h *MeshHandler) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var node mesh.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := h.manager.Registry().Register(&node); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"node":    node,
	})
}

func (h *MeshHandler) handleDeregisterNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.Registry().Deregister(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MeshHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.manager.Registry().GetNode(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, node)
}

func (h *MeshHandler) handleRoute(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "key query parameter is required")
		return
	}

	nodes := h.manager.Registry().ListNodes()
	node, err := h.manager.Router().RouteWithFallback(key, nodes)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"key":  key,
		"node": node,
	})
}

func (h *MeshHandler) handleListCircuits(w http.ResponseWriter, r *http.Request) {
	circuits := h.manager.CircuitBreakers()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"circuit_breakers": circuits,
	})
}

func (h *MeshHandler) handleResetCircuit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.manager.ResetCircuit(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "circuit breaker for " + id + " reset",
	})
}

func (h *MeshHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := h.manager.Registry().HealthCheck()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"health": health,
	})
}

func (h *MeshHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}

func (h *MeshHandler) ensureProtocol() *mesh.MeshProtocol {
	proto := h.manager.GetProtocol()
	if proto == nil {
		proto = mesh.NewMeshProtocol("local")
		h.manager.AddProtocol(proto)
	}
	return proto
}

func (h *MeshHandler) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	orgs := proto.ListOrganizations()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
		"count":         len(orgs),
	})
}

func (h *MeshHandler) handleRegisterOrg(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	var org mesh.Organization
	if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := proto.RegisterOrganization(org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "organization registered"})
}

func (h *MeshHandler) handleListGrants(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	orgID := r.URL.Query().Get("org_id")
	grants := proto.ListGrants(orgID)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"grants": grants,
		"count":  len(grants),
	})
}

func (h *MeshHandler) handleGrantAccess(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	var grant mesh.AccessGrant
	if err := json.NewDecoder(r.Body).Decode(&grant); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := proto.GrantAccess(grant); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "access granted"})
}

func (h *MeshHandler) handleRevokeAccess(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	id := r.PathValue("id")
	if err := proto.RevokeAccess(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "access revoked"})
}

func (h *MeshHandler) handleTransferFeatures(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	var req mesh.FeatureTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := proto.RequestFeatures(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusForbidden, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *MeshHandler) handleProtocolStats(w http.ResponseWriter, r *http.Request) {
	proto := h.ensureProtocol()
	stats := proto.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
