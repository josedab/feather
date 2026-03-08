package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/contractcicd"
)

// ContractCICDNativeHandler handles native feature contract CI/CD API requests.
type ContractCICDNativeHandler struct {
	engine *contractcicd.Engine
}

// NewContractCICDNativeHandler creates a new handler.
func NewContractCICDNativeHandler(engine *contractcicd.Engine) *ContractCICDNativeHandler {
	return &ContractCICDNativeHandler{engine: engine}
}

// RegisterRoutes registers contract CI/CD API routes.
func (h *ContractCICDNativeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/contracts/cicd/list", h.handleListContracts)
	mux.HandleFunc("POST /v1/contracts/cicd/register", h.handleRegister)
	mux.HandleFunc("POST /v1/contracts/cicd/validate", h.handleValidate)
	mux.HandleFunc("POST /v1/contracts/cicd/plan", h.handlePlan)
	mux.HandleFunc("POST /v1/contracts/cicd/apply", h.handleApply)
	mux.HandleFunc("GET /v1/contracts/cicd/history", h.handleHistory)
	mux.HandleFunc("GET /v1/contracts/cicd/stats", h.handleStats)
}

func (h *ContractCICDNativeHandler) handleListContracts(w http.ResponseWriter, r *http.Request) {
	contracts := h.engine.ListContracts()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"contracts": contracts,
		"total":     len(contracts),
	})
}

func (h *ContractCICDNativeHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var contract contractcicd.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.engine.RegisterContract(&contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "contract registered"})
}

func (h *ContractCICDNativeHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var contract contractcicd.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	errors := h.engine.Validate(&contract)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":  len(errors) == 0,
		"errors": errors,
	})
}

func (h *ContractCICDNativeHandler) handlePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContractName string `json:"contract_name"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	contract, err := h.engine.GetContract(req.ContractName)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	plan, err := h.engine.PlanFromContracts([]*contractcicd.Contract{contract})
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *ContractCICDNativeHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.PlanID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "plan_id is required")
		return
	}

	result, err := h.engine.Apply(req.PlanID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ContractCICDNativeHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	contracts := h.engine.ListContracts()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"contracts": contracts,
		"total":     len(contracts),
	})
}

func (h *ContractCICDNativeHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.engine.Stats())
}
