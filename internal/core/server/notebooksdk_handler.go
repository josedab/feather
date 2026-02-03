package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/notebooksdk"
)

// NotebookSDKHandler handles notebook SDK API requests.
type NotebookSDKHandler struct {
	service *notebooksdk.Service
}

// NewNotebookSDKHandler creates a new notebook SDK handler.
func NewNotebookSDKHandler(service *notebooksdk.Service) *NotebookSDKHandler {
	return &NotebookSDKHandler{service: service}
}

// RegisterRoutes registers notebook SDK API routes.
func (h *NotebookSDKHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/notebook/sessions", h.handleCreateSession)
	mux.HandleFunc("GET /v1/notebook/sessions", h.handleListSessions)
	mux.HandleFunc("GET /v1/notebook/sessions/{id}", h.handleGetSession)
	mux.HandleFunc("DELETE /v1/notebook/sessions/{id}", h.handleCloseSession)
	mux.HandleFunc("POST /v1/notebook/execute", h.handleExecute)
	mux.HandleFunc("POST /v1/notebook/visualize", h.handleVisualize)
	mux.HandleFunc("GET /v1/notebook/stats", h.handleGetStats)
}

// handleCreateSession handles POST /v1/notebook/sessions
func (h *NotebookSDKHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var cfg notebooksdk.SessionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.service.CreateSession(cfg)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, session)
}

// handleListSessions handles GET /v1/notebook/sessions
func (h *NotebookSDKHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.service.ListSessions()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// handleGetSession handles GET /v1/notebook/sessions/{id}
func (h *NotebookSDKHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "session id required")
		return
	}

	session, err := h.service.GetSession(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, session)
}

// handleCloseSession handles DELETE /v1/notebook/sessions/{id}
func (h *NotebookSDKHandler) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "session id required")
		return
	}

	if err := h.service.CloseSession(id); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "session closed"})
}

// handleExecute handles POST /v1/notebook/execute
func (h *NotebookSDKHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Command   string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" || req.Command == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "session_id and command required")
		return
	}

	result, err := h.service.Execute(req.SessionID, req.Command)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleVisualize handles POST /v1/notebook/visualize
func (h *NotebookSDKHandler) handleVisualize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Feature   string `json:"feature"`
		Type      string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "session_id required")
		return
	}

	var (
		viz *notebooksdk.Visualization
		err error
	)

	switch req.Type {
	case "histogram":
		viz, err = h.service.GenerateHistogram(req.SessionID, req.Feature)
	case "drift":
		viz, err = h.service.GenerateDriftChart(req.SessionID, req.Feature)
	case "freshness":
		viz, err = h.service.GenerateFreshnessIndicator(req.SessionID)
	default:
		h.writeError(r.Context(), w, http.StatusBadRequest, "unsupported visualization type: use histogram, drift, or freshness")
		return
	}

	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, viz)
}

// handleGetStats handles GET /v1/notebook/stats
func (h *NotebookSDKHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *NotebookSDKHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *NotebookSDKHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
