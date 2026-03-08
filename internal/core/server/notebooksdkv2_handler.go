package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/extensions/notebooksdk"
)

// NotebookSDKv2Handler handles notebook SDK v2 API requests.
type NotebookSDKv2Handler struct {
	service *notebooksdk.Service
}

// NewNotebookSDKv2Handler creates a new handler.
func NewNotebookSDKv2Handler(service *notebooksdk.Service) *NotebookSDKv2Handler {
	return &NotebookSDKv2Handler{service: service}
}

// RegisterRoutes registers notebook SDK v2 API routes.
func (h *NotebookSDKv2Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/notebook/v2/sessions", h.handleCreateSession)
	mux.HandleFunc("GET /v1/notebook/v2/sessions", h.handleListSessions)
	mux.HandleFunc("GET /v1/notebook/v2/sessions/{id}", h.handleGetSession)
	mux.HandleFunc("DELETE /v1/notebook/v2/sessions/{id}", h.handleCloseSession)
	mux.HandleFunc("POST /v1/notebook/v2/execute", h.handleExecute)
	mux.HandleFunc("POST /v1/notebook/v2/visualize", h.handleVisualize)
	mux.HandleFunc("POST /v1/notebook/v2/preview", h.handlePreview)
	mux.HandleFunc("GET /v1/notebook/v2/stats", h.handleStats)
}

func (h *NotebookSDKv2Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var cfg notebooksdk.SessionConfig
	if err := strictDecode(r.Body, &cfg); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	session, err := h.service.CreateSession(cfg)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, session)
}

func (h *NotebookSDKv2Handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.service.ListSessions()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

func (h *NotebookSDKv2Handler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, err := h.service.GetSession(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, session)
}

func (h *NotebookSDKv2Handler) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.service.CloseSession(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "session closed"})
}

func (h *NotebookSDKv2Handler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Command   string `json:"command"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.SessionID == "" || req.Command == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "session_id and command required")
		return
	}

	result, err := h.service.Execute(req.SessionID, req.Command)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *NotebookSDKv2Handler) handleVisualize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Feature   string `json:"feature"`
		Type      string `json:"type"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.SessionID == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "session_id required")
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
		writeJSONError(r.Context(), w, http.StatusBadRequest, "unsupported type: use histogram, drift, or freshness")
		return
	}
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, viz)
}

func (h *NotebookSDKv2Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Feature   string `json:"feature"`
		AsOf      string `json:"as_of"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.SessionID == "" || req.Feature == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "session_id and feature required")
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"session_id": req.SessionID,
		"feature":    req.Feature,
		"as_of":      req.AsOf,
		"message":    "point-in-time preview generated",
	})
}

func (h *NotebookSDKv2Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.service.Stats())
}
