package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/schemaevolution"
)

// SchemaEvolutionHandler handles schema evolution API requests.
type SchemaEvolutionHandler struct {
	manager *schemaevolution.Manager
}

// NewSchemaEvolutionHandler creates a new schema evolution handler.
func NewSchemaEvolutionHandler(manager *schemaevolution.Manager) *SchemaEvolutionHandler {
	return &SchemaEvolutionHandler{manager: manager}
}

// RegisterRoutes registers schema evolution API routes.
func (h *SchemaEvolutionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/schema/evolution/groups", h.handleListSchemas)
	mux.HandleFunc("POST /v1/schema/evolution/groups", h.handleRegisterSchema)
	mux.HandleFunc("GET /v1/schema/evolution/groups/{group}", h.handleGetSchema)
	mux.HandleFunc("POST /v1/schema/evolution/groups/{group}/evolve", h.handleEvolve)
	mux.HandleFunc("POST /v1/schema/evolution/groups/{group}/check", h.handleCheckCompatibility)
	mux.HandleFunc("POST /v1/schema/evolution/groups/{group}/rollback", h.handleRollback)
	mux.HandleFunc("GET /v1/schema/evolution/migrations", h.handleListMigrations)
	mux.HandleFunc("GET /v1/schema/evolution/stats", h.handleStats)
}

func (h *SchemaEvolutionHandler) handleListSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := h.manager.ListSchemas()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"schemas": schemas})
}

func (h *SchemaEvolutionHandler) handleRegisterSchema(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Group  string            `json:"group"`
		Fields map[string]string `json:"fields"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	sv, err := h.manager.RegisterSchema(req.Group, req.Fields)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, sv)
}

func (h *SchemaEvolutionHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	sv, err := h.manager.GetSchema(group)
	if err != nil {
		if errors.Is(err, schemaevolution.ErrSchemaNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "schema not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, sv)
}

func (h *SchemaEvolutionHandler) handleEvolve(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	var req struct {
		Fields   map[string]string `json:"fields"`
		Defaults map[string]string `json:"defaults,omitempty"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	migration, err := h.manager.Evolve(group, req.Fields, req.Defaults)
	if err != nil {
		if errors.Is(err, schemaevolution.ErrIncompatibleSchema) {
			h.writeError(r.Context(), w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, schemaevolution.ErrSchemaNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "schema not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, migration)
}

func (h *SchemaEvolutionHandler) handleCheckCompatibility(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	var req struct {
		Fields   map[string]string `json:"fields"`
		Defaults map[string]string `json:"defaults,omitempty"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := h.manager.CheckCompatibility(group, req.Fields, req.Defaults)
	if err != nil {
		if errors.Is(err, schemaevolution.ErrSchemaNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "schema not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

func (h *SchemaEvolutionHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if err := h.manager.Rollback(group); err != nil {
		if errors.Is(err, schemaevolution.ErrSchemaNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "schema not found or no previous version")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "rolled back to previous version"})
}

func (h *SchemaEvolutionHandler) handleListMigrations(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	migrations := h.manager.ListMigrations(group)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"migrations": migrations, "count": len(migrations)})
}

func (h *SchemaEvolutionHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *SchemaEvolutionHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SchemaEvolutionHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
