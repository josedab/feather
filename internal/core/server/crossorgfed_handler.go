package server

import (
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/federation"
)

// CrossOrgFedHandler provides HTTP endpoints for enhanced cross-org federation.
type CrossOrgFedHandler struct {
	fed *federation.CrossOrgFederation
}

// NewCrossOrgFedHandler creates a new handler.
func NewCrossOrgFedHandler(fed *federation.CrossOrgFederation) *CrossOrgFedHandler {
	return &CrossOrgFedHandler{fed: fed}
}

// RegisterRoutes registers cross-org federation API routes.
func (h *CrossOrgFedHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/crossorg/orgs", h.handleListOrgs)
	mux.HandleFunc("POST /v1/crossorg/orgs", h.handleRegisterOrg)
	mux.HandleFunc("GET /v1/crossorg/agreements", h.handleListAgreements)
	mux.HandleFunc("POST /v1/crossorg/agreements", h.handleCreateAgreement)
	mux.HandleFunc("POST /v1/crossorg/request", h.handleRequest)
	mux.HandleFunc("GET /v1/crossorg/privacy/{org}", h.handlePrivacyBudget)
	mux.HandleFunc("GET /v1/crossorg/audit", h.handleAudit)
	mux.HandleFunc("GET /v1/crossorg/stats", h.handleStats)
}

func (h *CrossOrgFedHandler) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	orgs := h.fed.ListOrgs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
		"total":         len(orgs),
	})
}

func (h *CrossOrgFedHandler) handleRegisterOrg(w http.ResponseWriter, r *http.Request) {
	var org federation.Organization
	if err := strictDecode(r.Body, &org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.fed.RegisterOrg(org); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "organization registered"})
}

func (h *CrossOrgFedHandler) handleListAgreements(w http.ResponseWriter, r *http.Request) {
	agreements := h.fed.ListAgreements()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"agreements": agreements,
		"total":      len(agreements),
	})
}

func (h *CrossOrgFedHandler) handleCreateAgreement(w http.ResponseWriter, r *http.Request) {
	var agreement federation.SharingAgreement
	if err := strictDecode(r.Body, &agreement); err != nil {
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

func (h *CrossOrgFedHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Request  federation.CrossOrgRequest `json:"request"`
		Features map[string]interface{}     `json:"features"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
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

func (h *CrossOrgFedHandler) handlePrivacyBudget(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("org")
	budget, err := h.fed.GetCrossOrgBudget(org)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, budget)
}

func (h *CrossOrgFedHandler) handleAudit(w http.ResponseWriter, r *http.Request) {
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

func (h *CrossOrgFedHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.fed.Stats())
}
