package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
)

// SDKCodegenHandler handles SDK code generation API requests.
type SDKCodegenHandler struct {
	generator *sdkcodegen.Generator
}

// NewSDKCodegenHandler creates a new SDK codegen handler.
func NewSDKCodegenHandler(generator *sdkcodegen.Generator) *SDKCodegenHandler {
	return &SDKCodegenHandler{generator: generator}
}

// RegisterRoutes registers SDK codegen API routes.
func (h *SDKCodegenHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/codegen/schemas", h.handleListSchemas)
	mux.HandleFunc("POST /v1/codegen/schemas", h.handleRegisterSchema)
	mux.HandleFunc("POST /v1/codegen/generate", h.handleGenerate)
	mux.HandleFunc("POST /v1/codegen/generate/all", h.handleGenerateAll)
	mux.HandleFunc("GET /v1/codegen/history", h.handleGetHistory)
}

type registerSchemaRequest struct {
	Name        string                 `json:"name"`
	EntityType  string                 `json:"entity_type"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version"`
	Fields      []sdkcodegen.SchemaField `json:"fields"`
}

type generateRequest struct {
	SchemaName string `json:"schema_name"`
	Language   string `json:"language"`
}

func (h *SDKCodegenHandler) handleListSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := h.generator.ListSchemas()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"schemas": schemas,
	})
}

func (h *SDKCodegenHandler) handleRegisterSchema(w http.ResponseWriter, r *http.Request) {
	var req registerSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	schema := sdkcodegen.SchemaDefinition{
		Name:        req.Name,
		EntityType:  req.EntityType,
		Description: req.Description,
		Version:     req.Version,
		Fields:      req.Fields,
	}

	if err := h.generator.RegisterSchema(schema); err != nil {
		if errors.Is(err, sdkcodegen.ErrInvalidSchema) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"schema":  req.Name,
	})
}

func (h *SDKCodegenHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SchemaName == "" || req.Language == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "schema_name and language are required")
		return
	}

	code, err := h.generator.Generate(req.SchemaName, sdkcodegen.Language(req.Language))
	if err != nil {
		if errors.Is(err, sdkcodegen.ErrInvalidSchema) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, sdkcodegen.ErrUnsupportedLanguage) {
			h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, code)
}

func (h *SDKCodegenHandler) handleGenerateAll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	codes, err := h.generator.GenerateAll(sdkcodegen.Language(req.Language))
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"generated": codes,
		"count":     len(codes),
	})
}

func (h *SDKCodegenHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	history := h.generator.GetHistory()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": history,
	})
}

func (h *SDKCodegenHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SDKCodegenHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
