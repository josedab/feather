package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/platform/federation"
)

// SMPCHandler handles SMPC API requests.
type SMPCHandler struct {
	engine *federation.SMPCEngine
}

// NewSMPCHandler creates a new SMPC handler.
func NewSMPCHandler(engine *federation.SMPCEngine) *SMPCHandler {
	return &SMPCHandler{engine: engine}
}

// RegisterRoutes registers SMPC API routes.
func (h *SMPCHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/smpc/parties", h.handleRegisterParty)
	mux.HandleFunc("GET /v1/smpc/parties", h.handleListParties)
	mux.HandleFunc("GET /v1/smpc/parties/{id}", h.handleGetParty)
	mux.HandleFunc("DELETE /v1/smpc/parties/{id}", h.handleRemoveParty)
	mux.HandleFunc("POST /v1/smpc/shares", h.handleCreateShares)
	mux.HandleFunc("POST /v1/smpc/reconstruct", h.handleReconstructSecret)
	mux.HandleFunc("POST /v1/smpc/compute", h.handleSubmitCompute)
	mux.HandleFunc("POST /v1/smpc/compute/{id}/execute", h.handleExecuteCompute)
	mux.HandleFunc("GET /v1/smpc/results", h.handleListResults)
	mux.HandleFunc("GET /v1/smpc/results/{id}", h.handleGetResult)
	mux.HandleFunc("GET /v1/smpc/stats", h.handleStats)
}

// handleRegisterParty handles POST /v1/smpc/parties.
func (h *SMPCHandler) handleRegisterParty(w http.ResponseWriter, r *http.Request) {
	var party federation.Party
	if err := json.NewDecoder(r.Body).Decode(&party); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.engine.RegisterParty(&party); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, &party)
}

// handleListParties handles GET /v1/smpc/parties.
func (h *SMPCHandler) handleListParties(w http.ResponseWriter, r *http.Request) {
	parties := h.engine.ListParties()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"parties": parties,
		"count":   len(parties),
	})
}

// handleGetParty handles GET /v1/smpc/parties/{id}.
func (h *SMPCHandler) handleGetParty(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "party ID required")
		return
	}

	party, err := h.engine.GetParty(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, party)
}

// handleRemoveParty handles DELETE /v1/smpc/parties/{id}.
func (h *SMPCHandler) handleRemoveParty(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "party ID required")
		return
	}

	if err := h.engine.RemoveParty(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "removed"})
}

// createSharesRequest represents the body for creating secret shares.
type createSharesRequest struct {
	Value       float64 `json:"value"`
	Threshold   int     `json:"threshold"`
	TotalShares int     `json:"total_shares"`
}

// handleCreateShares handles POST /v1/smpc/shares.
func (h *SMPCHandler) handleCreateShares(w http.ResponseWriter, r *http.Request) {
	var req createSharesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	shares, err := h.engine.CreateShares(req.Value, req.Threshold, req.TotalShares)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"shares": shares,
		"count":  len(shares),
	})
}

// reconstructRequest represents the body for reconstructing a secret.
type reconstructRequest struct {
	Shares []*federation.SecretShare `json:"shares"`
}

// handleReconstructSecret handles POST /v1/smpc/reconstruct.
func (h *SMPCHandler) handleReconstructSecret(w http.ResponseWriter, r *http.Request) {
	var req reconstructRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	value, err := h.engine.ReconstructSecret(req.Shares)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"value": value,
	})
}

// handleSubmitCompute handles POST /v1/smpc/compute.
func (h *SMPCHandler) handleSubmitCompute(w http.ResponseWriter, r *http.Request) {
	var req federation.ComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.engine.SubmitCompute(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, &req)
}

// handleExecuteCompute handles POST /v1/smpc/compute/{id}/execute.
func (h *SMPCHandler) handleExecuteCompute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "compute request ID required")
		return
	}

	result, err := h.engine.ExecuteCompute(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleListResults handles GET /v1/smpc/results.
func (h *SMPCHandler) handleListResults(w http.ResponseWriter, r *http.Request) {
	results := h.engine.ListResults()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// handleGetResult handles GET /v1/smpc/results/{id}.
func (h *SMPCHandler) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "result ID required")
		return
	}

	result, err := h.engine.GetResult(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleStats handles GET /v1/smpc/stats.
func (h *SMPCHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(r.Context(), w, http.StatusOK, h.engine.Stats())
}

func (h *SMPCHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SMPCHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
