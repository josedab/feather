package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/graphqlfederation"
)

// GraphQLFederationHandler handles GraphQL federation gateway API requests.
type GraphQLFederationHandler struct {
	gateway *graphqlfederation.Gateway
}

// NewGraphQLFederationHandler creates a new GraphQL federation handler.
func NewGraphQLFederationHandler(gateway *graphqlfederation.Gateway) *GraphQLFederationHandler {
	return &GraphQLFederationHandler{gateway: gateway}
}

// RegisterRoutes registers GraphQL federation API routes.
func (h *GraphQLFederationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/graphql/federation/services", h.handleListServices)
	mux.HandleFunc("POST /v1/graphql/federation/services", h.handleRegisterService)
	mux.HandleFunc("DELETE /v1/graphql/federation/services/{name}", h.handleRemoveService)
	mux.HandleFunc("GET /v1/graphql/federation/fields", h.handleListFields)
	mux.HandleFunc("POST /v1/graphql/federation/fields", h.handleRegisterField)
	mux.HandleFunc("POST /v1/graphql/federation/query", h.handleExecuteQuery)
	mux.HandleFunc("POST /v1/graphql/federation/batch", h.handleBatchResolve)
	mux.HandleFunc("GET /v1/graphql/federation/schema", h.handleGetSchema)
	mux.HandleFunc("GET /v1/graphql/federation/stats", h.handleStats)
}

func (h *GraphQLFederationHandler) handleListServices(w http.ResponseWriter, r *http.Request) {
	services := h.gateway.ListServices()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"services": services,
		"total":    len(services),
	})
}

func (h *GraphQLFederationHandler) handleRegisterService(w http.ResponseWriter, r *http.Request) {
	var svc graphqlfederation.ServiceConfig
	if err := strictDecode(r.Body, &svc); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.gateway.RegisterService(svc); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "service registered"})
}

func (h *GraphQLFederationHandler) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.gateway.RemoveService(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "service removed"})
}

func (h *GraphQLFederationHandler) handleListFields(w http.ResponseWriter, r *http.Request) {
	fields := h.gateway.ListFields()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"fields": fields,
		"total":  len(fields),
	})
}

func (h *GraphQLFederationHandler) handleRegisterField(w http.ResponseWriter, r *http.Request) {
	var field graphqlfederation.FederatedField
	if err := strictDecode(r.Body, &field); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.gateway.RegisterField(field); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "field registered"})
}

func (h *GraphQLFederationHandler) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Query == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	result, err := h.gateway.ExecuteQuery(r.Context(), req.Query)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *GraphQLFederationHandler) handleBatchResolve(w http.ResponseWriter, r *http.Request) {
	var batch graphqlfederation.BatchRequest
	if err := strictDecode(r.Body, &batch); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	results, err := h.gateway.BatchResolve(r.Context(), batch)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entities": results,
		"total":    len(results),
	})
}

func (h *GraphQLFederationHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.gateway.GetSchema())
}

func (h *GraphQLFederationHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.gateway.Stats())
}
