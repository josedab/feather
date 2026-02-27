package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/semantic"
)

// NLDiscoveryHandler handles natural language feature discovery API requests.
type NLDiscoveryHandler struct {
	engine *semantic.NLDiscoveryEngine
}

// NewNLDiscoveryHandler creates a new NL discovery handler.
func NewNLDiscoveryHandler(engine *semantic.NLDiscoveryEngine) *NLDiscoveryHandler {
	return &NLDiscoveryHandler{engine: engine}
}

// RegisterRoutes registers NL discovery API routes.
func (h *NLDiscoveryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/discover/query", h.handleQuery)
	mux.HandleFunc("POST /v1/discover/chat", h.handleChat)
	mux.HandleFunc("GET /v1/discover/conversations/{id}", h.handleGetConversation)
	mux.HandleFunc("GET /v1/discover/history", h.handleHistory)
	mux.HandleFunc("POST /v1/discover/catalog", h.handleRegisterFeature)
}

func (h *NLDiscoveryHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	result := h.engine.Query(req.Query)
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *NLDiscoveryHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "message is required")
		return
	}
	if req.ConversationID == "" {
		req.ConversationID = "default"
	}

	result, conv := h.engine.Chat(req.ConversationID, req.Message)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"result":       result,
		"conversation": conv,
	})
}

func (h *NLDiscoveryHandler) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conv, err := h.engine.GetConversation(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, conv)
}

func (h *NLDiscoveryHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	history := h.engine.GetQueryHistory(limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

func (h *NLDiscoveryHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		EntityType  string   `json:"entity_type"`
		Tags        []string `json:"tags"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	h.engine.RegisterFeature(req.Name, req.Description, req.EntityType, req.Tags)
	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "feature registered for discovery"})
}

func (h *NLDiscoveryHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *NLDiscoveryHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
