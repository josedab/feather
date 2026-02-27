package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/platform/contract"
)

// ContractHandler provides HTTP endpoints for feature contracts.
type ContractHandler struct {
	manager *contract.Manager
}

// NewContractHandler creates a new contract handler.
func NewContractHandler(manager *contract.Manager) *ContractHandler {
	return &ContractHandler{manager: manager}
}

// RegisterRoutes registers contract API routes.
func (h *ContractHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/contracts", h.handleListContracts)
	mux.HandleFunc("POST /v1/contracts", h.handleCreateContract)
	mux.HandleFunc("GET /v1/contracts/{name}", h.handleGetContract)
	mux.HandleFunc("PUT /v1/contracts/{name}", h.handleUpdateContract)
	mux.HandleFunc("DELETE /v1/contracts/{name}", h.handleDeleteContract)
	mux.HandleFunc("GET /v1/contracts/{name}/status", h.handleGetContractStatus)
	mux.HandleFunc("GET /v1/contracts/statuses", h.handleListStatuses)
	mux.HandleFunc("GET /v1/contracts/violations", h.handleGetViolations)
	mux.HandleFunc("POST /v1/contracts/evaluate", h.handleEvaluateAll)
}

func (h *ContractHandler) handleListContracts(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}
	contracts := h.manager.ListContracts()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"contracts": contracts,
		"count":     len(contracts),
	})
}

func (h *ContractHandler) handleCreateContract(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	var spec contract.Spec
	if err := strictDecode(r.Body, &spec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.CreateContract(&spec); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, contract.ErrContractExists) {
			status = http.StatusConflict
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"contract": spec,
	})
}

func (h *ContractHandler) handleGetContract(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	name := r.PathValue("name")
	spec, err := h.manager.GetContract(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"contract": spec,
	})
}

func (h *ContractHandler) handleUpdateContract(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	name := r.PathValue("name")
	var spec contract.Spec
	if err := strictDecode(r.Body, &spec); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	spec.Name = name

	if err := h.manager.UpdateContract(&spec); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, contract.ErrContractNotFound) {
			status = http.StatusNotFound
		}
		writeJSONError(r.Context(), w, status, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"contract": spec,
	})
}

func (h *ContractHandler) handleDeleteContract(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	name := r.PathValue("name")
	if err := h.manager.DeleteContract(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "contract deleted",
	})
}

func (h *ContractHandler) handleGetContractStatus(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	name := r.PathValue("name")
	status, err := h.manager.GetStatus(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

func (h *ContractHandler) handleListStatuses(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	statuses := h.manager.ListStatuses()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"statuses": statuses,
		"count":    len(statuses),
	})
}

func (h *ContractHandler) handleGetViolations(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	violations := h.manager.GetViolations(since)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"violations": violations,
		"count":      len(violations),
	})
}

func (h *ContractHandler) handleEvaluateAll(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "contract manager not configured")
		return
	}

	h.manager.EvaluateAll(r.Context())

	statuses := h.manager.ListStatuses()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "evaluation complete",
		"statuses": statuses,
	})
}
