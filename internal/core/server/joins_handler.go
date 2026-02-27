package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/joins"
)

// JoinsHandler handles streaming feature join API requests.
type JoinsHandler struct {
	engine *joins.Engine
}

// NewJoinsHandler creates a new joins handler.
func NewJoinsHandler(engine *joins.Engine) *JoinsHandler {
	return &JoinsHandler{engine: engine}
}

// RegisterRoutes registers join API routes.
func (h *JoinsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/joins/plans", h.handleCreatePlan)
	mux.HandleFunc("GET /v1/joins/plans", h.handleListPlans)
	mux.HandleFunc("GET /v1/joins/plans/{id}", h.handleGetPlan)
	mux.HandleFunc("DELETE /v1/joins/plans/{id}", h.handleDeletePlan)
	mux.HandleFunc("POST /v1/joins/execute/{id}", h.handleExecuteJoin)
}

func (h *JoinsHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var cfg joins.JoinConfig
	if err := strictDecode(r.Body, &cfg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	plan, err := h.engine.CreatePlan(cfg)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, plan)
}

func (h *JoinsHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.engine.ListPlans()
	writeJSONResponse(r.Context(), w, http.StatusOK, plans)
}

func (h *JoinsHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	plan, err := h.engine.GetPlan(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *JoinsHandler) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.DeletePlan(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "deleted"})
}

type executeJoinRequest struct {
	LeftData  map[string]map[string]*joins.FeatureValue `json:"left_data"`
	RightData map[string]map[string]*joins.FeatureValue `json:"right_data"`
}

func (h *JoinsHandler) handleExecuteJoin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req executeJoinRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	output, err := h.engine.ExecuteJoin(r.Context(), id, req.LeftData, req.RightData)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, output)
}
