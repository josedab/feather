package server

import (
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/finops"
)

// FinOpsHandler provides HTTP endpoints for cost attribution and FinOps.
type FinOpsHandler struct {
	manager *finops.Manager
}

// NewFinOpsHandler creates a new FinOps handler.
func NewFinOpsHandler(manager *finops.Manager) *FinOpsHandler {
	return &FinOpsHandler{manager: manager}
}

// RegisterRoutes registers FinOps API routes.
func (h *FinOpsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/finops/teams", h.handleListTeams)
	mux.HandleFunc("POST /v1/finops/teams", h.handleRegisterTeam)
	mux.HandleFunc("GET /v1/finops/teams/{id}", h.handleGetTeam)
	mux.HandleFunc("GET /v1/finops/teams/{id}/cost", h.handleGetTeamCost)
	mux.HandleFunc("GET /v1/finops/teams/{id}/predict", h.handlePredictCost)
	mux.HandleFunc("POST /v1/finops/usage", h.handleRecordUsage)
	mux.HandleFunc("GET /v1/finops/groups/{name}/cost", h.handleGetGroupCost)
	mux.HandleFunc("GET /v1/finops/recommendations", h.handleGetRecommendations)
	mux.HandleFunc("GET /v1/finops/summary", h.handleSummary)
	mux.HandleFunc("GET /v1/finops/rates", h.handleGetRates)
	mux.HandleFunc("POST /v1/finops/rates", h.handleSetRate)
}

func (h *FinOpsHandler) handleListTeams(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	teams := h.manager.ListTeams()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "teams": teams, "count": len(teams),
	})
}

func (h *FinOpsHandler) handleRegisterTeam(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	var team finops.Team
	if err := strictDecode(r.Body, &team); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.RegisterTeam(&team); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "team": team})
}

func (h *FinOpsHandler) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	team, err := h.manager.GetTeam(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "team": team})
}

func (h *FinOpsHandler) handleGetTeamCost(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	until := time.Now()
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = t
		}
	}
	cost, err := h.manager.GetTeamCost(r.PathValue("id"), since, until)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "cost": cost})
}

func (h *FinOpsHandler) handlePredictCost(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	pred, err := h.manager.PredictCost(r.PathValue("id"), 30)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "prediction": pred})
}

func (h *FinOpsHandler) handleRecordUsage(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	var record finops.UsageRecord
	if err := strictDecode(r.Body, &record); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.manager.RecordUsage(record)
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "usage recorded"})
}

func (h *FinOpsHandler) handleGetGroupCost(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	until := time.Now()
	cost := h.manager.GetFeatureGroupCost(r.PathValue("name"), since, until)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "cost": cost})
}

func (h *FinOpsHandler) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	recs := h.manager.GetRecommendations()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "recommendations": recs, "count": len(recs),
	})
}

func (h *FinOpsHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "summary": h.manager.Summary(since),
	})
}

func (h *FinOpsHandler) handleGetRates(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "rates": h.manager.GetRates(),
	})
}

func (h *FinOpsHandler) handleSetRate(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "finops not configured")
		return
	}
	var rate finops.CostRate
	if err := strictDecode(r.Body, &rate); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.manager.SetRate(&rate)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "rate": rate})
}
