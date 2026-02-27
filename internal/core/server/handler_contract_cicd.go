package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/contractcicd"
)

// ---------------------------------------------------------------------------
// ContractCICDHandler
// ---------------------------------------------------------------------------

// ContractCICDHandler exposes feature contract CI/CD endpoints.
type ContractCICDHandler struct {
	engine *contractcicd.Engine
}

// NewContractCICDHandler creates a new ContractCICDHandler.
func NewContractCICDHandler(engine *contractcicd.Engine) *ContractCICDHandler {
	return &ContractCICDHandler{engine: engine}
}

// RegisterRoutes registers contract CI/CD API routes.
func (h *ContractCICDHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/contracts", h.handleList)
	mux.HandleFunc("POST /v1/contracts", h.handleRegister)
	mux.HandleFunc("GET /v1/contracts/{name}", h.handleGet)
	mux.HandleFunc("PUT /v1/contracts/{name}", h.handleUpdate)
	mux.HandleFunc("POST /v1/contracts/validate", h.handleValidate)
	mux.HandleFunc("POST /v1/contracts/plan", h.handlePlan)
	mux.HandleFunc("POST /v1/contracts/apply/{planID}", h.handleApply)
	mux.HandleFunc("GET /v1/contracts/plan/{planID}", h.handleGetPlan)
	mux.HandleFunc("GET /v1/contracts/ci-template/{provider}", h.handleCITemplate)
	mux.HandleFunc("GET /v1/contracts/stats", h.handleStats)
}

func (h *ContractCICDHandler) handleList(w http.ResponseWriter, r *http.Request) {
	contracts := h.engine.ListContracts()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"contracts": contracts,
		"total":     len(contracts),
	})
}

func (h *ContractCICDHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var contract contractcicd.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if errs := h.engine.Validate(&contract); len(errs) > 0 {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "validation errors: "+errs[0])
		return
	}
	if err := h.engine.RegisterContract(&contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, contract)
}

func (h *ContractCICDHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	contract, err := h.engine.GetContract(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, contract)
}

func (h *ContractCICDHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var contract contractcicd.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	contract.Metadata.Name = name
	plan, err := h.engine.UpdateContract(&contract)
	if err != nil {
		status := http.StatusBadRequest
		if err == contractcicd.ErrBreakingChange {
			status = http.StatusConflict
		}
		writeJSONResponse(r.Context(), w, status, map[string]interface{}{
			"error": err.Error(),
			"plan":  plan,
		})
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"contract": contract,
		"plan":     plan,
	})
}

func (h *ContractCICDHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var contract contractcicd.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	errs := h.engine.Validate(&contract)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":  len(errs) == 0,
		"errors": errs,
	})
}

func (h *ContractCICDHandler) handlePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contracts []*contractcicd.Contract `json:"contracts"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	plan, err := h.engine.PlanFromContracts(req.Contracts)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *ContractCICDHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planID")
	result, err := h.engine.Apply(planID)
	if err != nil {
		status := http.StatusBadRequest
		if err == contractcicd.ErrPlanNotFound {
			status = http.StatusNotFound
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ContractCICDHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planID")
	plan, err := h.engine.GetPlan(planID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *ContractCICDHandler) handleCITemplate(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	template := h.engine.GenerateCITemplate(provider)
	writeJSONResponse(r.Context(), w, http.StatusOK, template)
}

func (h *ContractCICDHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.engine.Stats())
}
