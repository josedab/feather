package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/timetravel"
)

// TimeTravelHandler provides HTTP endpoints for time-travel debugging.
type TimeTravelHandler struct {
	debugger *timetravel.Debugger
}

// NewTimeTravelHandler creates a new time-travel handler.
func NewTimeTravelHandler(debugger *timetravel.Debugger) *TimeTravelHandler {
	return &TimeTravelHandler{debugger: debugger}
}

// RegisterRoutes registers time-travel API routes.
func (h *TimeTravelHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/timetravel/sessions", h.handleListSessions)
	mux.HandleFunc("POST /v1/timetravel/sessions", h.handleCreateSession)
	mux.HandleFunc("GET /v1/timetravel/sessions/{id}", h.handleGetSession)
	mux.HandleFunc("POST /v1/timetravel/sessions/{id}/close", h.handleCloseSession)
	mux.HandleFunc("POST /v1/timetravel/sessions/{id}/snapshots", h.handleAddSnapshot)
	mux.HandleFunc("GET /v1/timetravel/sessions/{id}/replay", h.handleReplay)
	mux.HandleFunc("POST /v1/timetravel/compare", h.handleCompare)
}

func (h *TimeTravelHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	sessions := h.debugger.ListSessions()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true, "sessions": sessions, "count": len(sessions),
	})
}

func (h *TimeTravelHandler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	var req struct {
		ID        string   `json:"id"`
		EntityKey string   `json:"entity_key"`
		Features  []string `json:"features"`
		StartTime string   `json:"start_time"`
		EndTime   string   `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	start, _ := time.Parse(time.RFC3339, req.StartTime)
	end, _ := time.Parse(time.RFC3339, req.EndTime)
	if start.IsZero() {
		start = time.Now().Add(-24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}

	session, err := h.debugger.CreateSession(req.ID, req.EntityKey, req.Features, start, end)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "session": session})
}

func (h *TimeTravelHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	session, err := h.debugger.GetSession(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "session": session})
}

func (h *TimeTravelHandler) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	if err := h.debugger.CloseSession(r.PathValue("id")); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "session closed"})
}

func (h *TimeTravelHandler) handleAddSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	var snapshot timetravel.Snapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.debugger.AddSnapshot(r.PathValue("id"), &snapshot); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "snapshot added"})
}

func (h *TimeTravelHandler) handleReplay(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	result, err := h.debugger.Replay(r.PathValue("id"))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "replay": result})
}

func (h *TimeTravelHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	if h.debugger == nil {
		writeJSONError(r.Context(), w, http.StatusServiceUnavailable, "time-travel not configured")
		return
	}
	var req struct {
		EntityKey string               `json:"entity_key"`
		WindowA   timetravel.TimeWindow `json:"window_a"`
		WindowB   timetravel.TimeWindow `json:"window_b"`
		Snapshots []*timetravel.Snapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	comparison, err := h.debugger.Compare(req.EntityKey, req.WindowA, req.WindowB, req.Snapshots)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "comparison": comparison})
}
