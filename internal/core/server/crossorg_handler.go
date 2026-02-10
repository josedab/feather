package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/federation"
)

// CrossOrgFederationHandler provides HTTP endpoints for cross-org federation.
type CrossOrgFederationHandler struct {
	fed *federation.CrossOrgFederation
}

// NewCrossOrgFederationHandler creates a new handler.
func NewCrossOrgFederationHandler(fed *federation.CrossOrgFederation) *CrossOrgFederationHandler {
	return &CrossOrgFederationHandler{fed: fed}
}

// RegisterRoutes registers federation API routes.
func (h *CrossOrgFederationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/federation/orgs", h.handleListOrgs)
	mux.HandleFunc("POST /v1/federation/orgs", h.handleRegisterOrg)
	mux.HandleFunc("GET /v1/federation/agreements", h.handleListAgreements)
	mux.HandleFunc("POST /v1/federation/agreements", h.handleCreateAgreement)
	mux.HandleFunc("POST /v1/federation/request", h.handleProcessRequest)
	mux.HandleFunc("GET /v1/federation/privacy/{org}", h.handleGetPrivacyBudget)
	mux.HandleFunc("GET /v1/federation/audit", h.handleGetAuditLog)
	mux.HandleFunc("GET /v1/federation/crossorg/stats", h.handleStats)
}

func (h *CrossOrgFederationHandler) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	orgs := h.fed.ListOrgs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
		"total":         len(orgs),
	})
}

func (h *CrossOrgFederationHandler) handleRegisterOrg(w http.ResponseWriter, r *http.Request) {
	var org federation.Organization
	if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.fed.RegisterOrg(org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"org_id":  org.ID,
	})
}

func (h *CrossOrgFederationHandler) handleListAgreements(w http.ResponseWriter, r *http.Request) {
	agreements := h.fed.ListAgreements()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"agreements": agreements,
		"total":      len(agreements),
	})
}

func (h *CrossOrgFederationHandler) handleCreateAgreement(w http.ResponseWriter, r *http.Request) {
	var agreement federation.SharingAgreement
	if err := json.NewDecoder(r.Body).Decode(&agreement); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.fed.CreateAgreement(agreement)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, result)
}

func (h *CrossOrgFederationHandler) handleProcessRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Request  federation.CrossOrgRequest `json:"request"`
		Features map[string]interface{}      `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp, err := h.fed.ProcessRequest(req.Request, req.Features)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusForbidden, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *CrossOrgFederationHandler) handleGetPrivacyBudget(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("org")
	budget, err := h.fed.GetCrossOrgBudget(org)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, budget)
}

func (h *CrossOrgFederationHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	log := h.fed.GetAuditLog(limit)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": log,
		"total":   len(log),
	})
}

func (h *CrossOrgFederationHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.fed.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
