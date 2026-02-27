package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/starlarkudf"
)

// StarlarkUDFHandler provides HTTP endpoints for the Starlark UDF registry.
type StarlarkUDFHandler struct {
	registry *starlarkudf.Registry
}

// NewStarlarkUDFHandler creates a new Starlark UDF handler.
func NewStarlarkUDFHandler(registry *starlarkudf.Registry) *StarlarkUDFHandler {
	return &StarlarkUDFHandler{registry: registry}
}

// RegisterRoutes registers UDF API routes.
func (h *StarlarkUDFHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/udfs", h.handleList)
	mux.HandleFunc("POST /v1/udfs", h.handleRegister)
	mux.HandleFunc("GET /v1/udfs/{name}", h.handleGet)
	mux.HandleFunc("DELETE /v1/udfs/{name}", h.handleRemove)
	mux.HandleFunc("POST /v1/udfs/{name}/execute", h.handleExecute)
	mux.HandleFunc("GET /v1/udfs/stats", h.handleStats)
}

func (h *StarlarkUDFHandler) handleList(w http.ResponseWriter, r *http.Request) {
	udfs := h.registry.List()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"udfs":  udfs,
		"total": len(udfs),
	})
}

func (h *StarlarkUDFHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var udf starlarkudf.UDF
	if err := strictDecode(r.Body, &udf); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	registered, err := h.registry.Register(udf)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, registered)
}

func (h *StarlarkUDFHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	udf, err := h.registry.Get(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, udf)
}

func (h *StarlarkUDFHandler) handleRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.registry.Remove(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *StarlarkUDFHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.registry.Execute(name, req.Inputs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *StarlarkUDFHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
