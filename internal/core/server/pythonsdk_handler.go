package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/pythonsdk"
)

// PythonSDKHandler handles Python transform SDK API requests.
type PythonSDKHandler struct {
	registry *pythonsdk.Registry
}

// NewPythonSDKHandler creates a new Python SDK handler.
func NewPythonSDKHandler(registry *pythonsdk.Registry) *PythonSDKHandler {
	return &PythonSDKHandler{registry: registry}
}

// RegisterRoutes registers Python SDK API routes.
func (h *PythonSDKHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/transforms", h.handleList)
	mux.HandleFunc("POST /v1/transforms", h.handleRegister)
	mux.HandleFunc("GET /v1/transforms/{id}", h.handleGet)
	mux.HandleFunc("PUT /v1/transforms/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /v1/transforms/{id}", h.handleDelete)
	mux.HandleFunc("POST /v1/transforms/{id}/execute", h.handleExecute)
	mux.HandleFunc("POST /v1/transforms/{id}/deploy", h.handleDeploy)
	mux.HandleFunc("POST /v1/transforms/validate", h.handleValidate)
	mux.HandleFunc("GET /v1/transforms/stats", h.handleStats)
}

func (h *PythonSDKHandler) handleList(w http.ResponseWriter, r *http.Request) {
	transforms := h.registry.List()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"transforms": transforms,
		"count":      len(transforms),
	})
}

func (h *PythonSDKHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var def pythonsdk.TransformDef
	if err := strictDecode(r.Body, &def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.registry.Register(def)
	if err != nil {
		if errors.Is(err, pythonsdk.ErrTransformExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "transform already exists")
			return
		}
		if errors.Is(err, pythonsdk.ErrInvalidTransform) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, created)
}

func (h *PythonSDKHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	def, err := h.registry.Get(id)
	if err != nil {
		if errors.Is(err, pythonsdk.ErrTransformNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, def)
}

func (h *PythonSDKHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var def pythonsdk.TransformDef
	if err := strictDecode(r.Body, &def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	def.ID = id

	updated, err := h.registry.Update(def)
	if err != nil {
		if errors.Is(err, pythonsdk.ErrTransformNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, updated)
}

func (h *PythonSDKHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.registry.Delete(id); err != nil {
		if errors.Is(err, pythonsdk.ErrTransformNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "transform deleted"})
}

func (h *PythonSDKHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var inputs map[string]interface{}
	if err := strictDecode(r.Body, &inputs); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.registry.Execute(id, inputs)
	if err != nil {
		if errors.Is(err, pythonsdk.ErrTransformNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *PythonSDKHandler) handleDeploy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.registry.Deploy(id); err != nil {
		if errors.Is(err, pythonsdk.ErrTransformNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "transform not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "transform deployed"})
}

func (h *PythonSDKHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var def pythonsdk.TransformDef
	if err := strictDecode(r.Body, &def); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.registry.Validate(def); err != nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"message": err.Error(),
		})
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"message": "transform definition is valid",
	})
}

func (h *PythonSDKHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *PythonSDKHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *PythonSDKHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
