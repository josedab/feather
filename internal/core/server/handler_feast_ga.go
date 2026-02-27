package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/feastcompat"
)

// ---------------------------------------------------------------------------
// FeastGAHandler
// ---------------------------------------------------------------------------

// FeastGAHandler exposes production-grade Feast-compatible gateway endpoints.
type FeastGAHandler struct {
	gateway *feastcompat.GAGateway
}

// NewFeastGAHandler creates a new FeastGAHandler.
func NewFeastGAHandler(gw *feastcompat.GAGateway) *FeastGAHandler {
	return &FeastGAHandler{gateway: gw}
}

// RegisterRoutes registers Feast GA gateway API routes.
func (h *FeastGAHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/feast-ga/get-online-features", h.handleGetOnlineFeatures)
	mux.HandleFunc("POST /v1/feast-ga/push", h.handlePush)
	mux.HandleFunc("POST /v1/feast-ga/migrate/plan", h.handlePlanMigration)
	mux.HandleFunc("POST /v1/feast-ga/migrate/execute", h.handleExecuteMigration)
	mux.HandleFunc("GET /v1/feast-ga/compat/tests", h.handleListTests)
	mux.HandleFunc("POST /v1/feast-ga/compat/run", h.handleRunTests)
	mux.HandleFunc("POST /v1/feast-ga/compat/run/{name}", h.handleRunSingleTest)
	mux.HandleFunc("GET /v1/feast-ga/stats", h.handleStats)
}

func (h *FeastGAHandler) handleGetOnlineFeatures(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityRows  []map[string]interface{} `json:"entity_rows"`
		FeatureRefs []string                 `json:"feature_refs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.gateway.GetOnlineFeatures(req.EntityRows, req.FeatureRefs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FeastGAHandler) handlePush(w http.ResponseWriter, r *http.Request) {
	var req feastcompat.PushRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	resp, err := h.gateway.Push(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *FeastGAHandler) handlePlanMigration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeastConfig string `json:"feast_config"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	plan, err := h.gateway.PlanMigration(req.FeastConfig)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *FeastGAHandler) handleExecuteMigration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeastConfig string `json:"feast_config"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.gateway.ExecuteMigration(req.FeastConfig)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FeastGAHandler) handleListTests(w http.ResponseWriter, r *http.Request) {
	tests := h.gateway.ListTests()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tests": tests,
		"total": len(tests),
	})
}

func (h *FeastGAHandler) handleRunTests(w http.ResponseWriter, r *http.Request) {
	report := h.gateway.RunCompatTests()
	writeJSONResponse(r.Context(), w, http.StatusOK, report)
}

func (h *FeastGAHandler) handleRunSingleTest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	result := h.gateway.RunTest(name)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FeastGAHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.gateway.Stats())
}
