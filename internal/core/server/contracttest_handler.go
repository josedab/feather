package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/contracttest"
)

// ContractTestHandler handles contract test API requests.
type ContractTestHandler struct {
	runner *contracttest.Runner
}

// NewContractTestHandler creates a new contract test handler.
func NewContractTestHandler(runner *contracttest.Runner) *ContractTestHandler {
	return &ContractTestHandler{runner: runner}
}

// RegisterRoutes registers contract test API routes.
func (h *ContractTestHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/contracts", h.handleListContracts)
	mux.HandleFunc("POST /v1/contracts", h.handleRegisterContract)
	mux.HandleFunc("GET /v1/contracts/{id}", h.handleGetContract)
	mux.HandleFunc("DELETE /v1/contracts/{id}", h.handleDeleteContract)
	mux.HandleFunc("POST /v1/contracts/validate/schema", h.handleValidateSchema)
	mux.HandleFunc("POST /v1/contracts/validate/range", h.handleValidateRange)
	mux.HandleFunc("POST /v1/contracts/run-all", h.handleRunAll)
	mux.HandleFunc("GET /v1/contracts/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/contracts/stats", h.handleGetStats)
}

// handleListContracts handles GET /v1/contracts
func (h *ContractTestHandler) handleListContracts(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	contracts := h.runner.ListContracts()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"contracts": contracts,
	})
}

// handleRegisterContract handles POST /v1/contracts
func (h *ContractTestHandler) handleRegisterContract(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	var contract contracttest.Contract
	if err := strictDecode(r.Body, &contract); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.runner.RegisterContract(contract); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"id":      contract.ID,
	})
}

// handleGetContract handles GET /v1/contracts/{id}
func (h *ContractTestHandler) handleGetContract(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contract id is required")
		return
	}

	contract, err := h.runner.GetContract(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, contract)
}

// handleDeleteContract handles DELETE /v1/contracts/{id}
func (h *ContractTestHandler) handleDeleteContract(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "contract id is required")
		return
	}

	if err := h.runner.DeleteContract(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "contract deleted"})
}

// handleValidateSchema handles POST /v1/contracts/validate/schema
func (h *ContractTestHandler) handleValidateSchema(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	var req struct {
		ContractID string            `json:"contract_id"`
		Fields     map[string]string `json:"fields"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := h.runner.ValidateSchema(req.ContractID, req.Fields)

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleValidateRange handles POST /v1/contracts/validate/range
func (h *ContractTestHandler) handleValidateRange(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	var req struct {
		ContractID string             `json:"contract_id"`
		Values     map[string]float64 `json:"values"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result := h.runner.ValidateRange(req.ContractID, req.Values)

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleRunAll handles POST /v1/contracts/run-all
func (h *ContractTestHandler) handleRunAll(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	var req struct {
		SchemaData map[string]string  `json:"schema_data"`
		RangeData  map[string]float64 `json:"range_data"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.runner.RunAll(req.SchemaData, req.RangeData)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetResults handles GET /v1/contracts/results
func (h *ContractTestHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	contractID := r.URL.Query().Get("contract_id")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results := h.runner.GetResults(contractID, limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetStats handles GET /v1/contracts/stats
func (h *ContractTestHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "contract test runner not configured")
		return
	}

	stats := h.runner.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *ContractTestHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ContractTestHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
