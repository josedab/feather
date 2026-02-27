package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/llmgateway"
)

// LLMGatewayHandler provides HTTP endpoints for the LLM gateway.
type LLMGatewayHandler struct {
	gw *llmgateway.Gateway
}

// NewLLMGatewayHandler creates a new LLM gateway handler.
func NewLLMGatewayHandler(gw *llmgateway.Gateway) *LLMGatewayHandler {
	return &LLMGatewayHandler{gw: gw}
}

// RegisterRoutes registers LLM gateway API routes.
func (h *LLMGatewayHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/llm/gateway/lookup", h.handleLookup)
	mux.HandleFunc("POST /v1/llm/gateway/store", h.handleStore)
	mux.HandleFunc("POST /v1/llm/gateway/templates", h.handleRegisterTemplate)
	mux.HandleFunc("GET /v1/llm/gateway/templates", h.handleListTemplates)
	mux.HandleFunc("POST /v1/llm/gateway/render", h.handleRender)
	mux.HandleFunc("POST /v1/llm/gateway/abtests", h.handleCreateABTest)
	mux.HandleFunc("GET /v1/llm/gateway/abtests", h.handleListABTests)
	mux.HandleFunc("GET /v1/llm/gateway/abtests/{id}", h.handleGetABTest)
	mux.HandleFunc("POST /v1/llm/gateway/abtests/{id}/resolve", h.handleResolveABTest)
	mux.HandleFunc("GET /v1/llm/gateway/costs", h.handleGetCosts)
	mux.HandleFunc("GET /v1/llm/gateway/stats", h.handleGetStats)
}

func (h *LLMGatewayHandler) handleLookup(w http.ResponseWriter, r *http.Request) {
	var req llmgateway.LookupRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp := h.gw.Lookup(req)
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *LLMGatewayHandler) handleStore(w http.ResponseWriter, r *http.Request) {
	var req llmgateway.StoreRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	entry, err := h.gw.Store(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, entry)
}

func (h *LLMGatewayHandler) handleRegisterTemplate(w http.ResponseWriter, r *http.Request) {
	var tmpl llmgateway.PromptTemplate
	if err := strictDecode(r.Body, &tmpl); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.gw.RegisterTemplate(tmpl); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, tmpl)
}

func (h *LLMGatewayHandler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.gw.ListTemplates()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (h *LLMGatewayHandler) handleRender(w http.ResponseWriter, r *http.Request) {
	var req llmgateway.RenderRequest
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp, err := h.gw.RenderTemplate(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, resp)
}

func (h *LLMGatewayHandler) handleCreateABTest(w http.ResponseWriter, r *http.Request) {
	var test llmgateway.ABTest
	if err := strictDecode(r.Body, &test); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	created, err := h.gw.CreateABTest(test)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, created)
}

func (h *LLMGatewayHandler) handleListABTests(w http.ResponseWriter, r *http.Request) {
	tests := h.gw.ListABTests()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tests": tests,
		"total": len(tests),
	})
}

func (h *LLMGatewayHandler) handleGetABTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	test, err := h.gw.GetABTestResults(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, test)
}

func (h *LLMGatewayHandler) handleResolveABTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		EntityID string `json:"entity_id"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	variant, prompt, err := h.gw.ResolveABTest(id, req.EntityID)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"variant": variant,
		"prompt":  prompt,
	})
}

func (h *LLMGatewayHandler) handleGetCosts(w http.ResponseWriter, r *http.Request) {
	costs := h.gw.GetCosts()
	writeJSONResponse(r.Context(), w, http.StatusOK, costs)
}

func (h *LLMGatewayHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.gw.GetStats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
