package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/integrations/streamsql"
)

// StreamSQLHandler handles streaming SQL API requests.
type StreamSQLHandler struct {
	engine *streamsql.Engine
}

// NewStreamSQLHandler creates a new streaming SQL handler.
func NewStreamSQLHandler(engine *streamsql.Engine) *StreamSQLHandler {
	return &StreamSQLHandler{engine: engine}
}

// RegisterRoutes registers streaming SQL API routes.
func (h *StreamSQLHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sql/streams", h.handleListStreams)
	mux.HandleFunc("POST /v1/sql/streams", h.handleCreateStream)
	mux.HandleFunc("GET /v1/sql/streams/{name}", h.handleGetStream)
	mux.HandleFunc("DELETE /v1/sql/streams/{name}", h.handleDropStream)
	mux.HandleFunc("POST /v1/sql/streams/{name}/push", h.handlePushRecords)
	mux.HandleFunc("POST /v1/sql/query", h.handleExecuteQuery)
	mux.HandleFunc("GET /v1/sql/queries", h.handleListQueries)
	mux.HandleFunc("POST /v1/sql/queries", h.handleRegisterQuery)
	mux.HandleFunc("GET /v1/sql/queries/{name}", h.handleGetQuery)
	mux.HandleFunc("DELETE /v1/sql/queries/{name}", h.handleUnregisterQuery)
	mux.HandleFunc("POST /v1/sql/queries/{name}/pause", h.handlePauseQuery)
	mux.HandleFunc("POST /v1/sql/queries/{name}/resume", h.handleResumeQuery)
	mux.HandleFunc("GET /v1/sql/stats", h.handleStats)
}

func (h *StreamSQLHandler) handleListStreams(w http.ResponseWriter, r *http.Request) {
	streams := h.engine.ListStreams()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"streams": streams})
}

type createStreamRequest struct {
	Name   string            `json:"name"`
	Schema map[string]string `json:"schema"`
}

func (h *StreamSQLHandler) handleCreateStream(w http.ResponseWriter, r *http.Request) {
	var req createStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "stream name required")
		return
	}
	if err := h.engine.CreateStream(req.Name, req.Schema); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{"success": true, "name": req.Name})
}

func (h *StreamSQLHandler) handleDropStream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "stream name required")
		return
	}
	if err := h.engine.DropStream(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

type executeQueryRequest struct {
	SQL string `json:"sql"`
}

func (h *StreamSQLHandler) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	var req executeQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SQL == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "sql query required")
		return
	}
	result, err := h.engine.ExecuteQuery(r.Context(), req.SQL)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

func (h *StreamSQLHandler) handleListQueries(w http.ResponseWriter, r *http.Request) {
	queries := h.engine.ListQueries()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"queries": queries})
}

type registerQueryRequest struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

func (h *StreamSQLHandler) handleRegisterQuery(w http.ResponseWriter, r *http.Request) {
	var req registerQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.SQL == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "name and sql required")
		return
	}
	query, err := h.engine.RegisterQuery(r.Context(), req.Name, req.SQL)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, query)
}

func (h *StreamSQLHandler) handleUnregisterQuery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query name required")
		return
	}
	if err := h.engine.UnregisterQuery(name); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *StreamSQLHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *StreamSQLHandler) handleGetStream(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "stream name required")
		return
	}
	info, err := h.engine.GetStream(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, info)
}

type pushRecordsRequest struct {
	Records []map[string]interface{} `json:"records"`
}

func (h *StreamSQLHandler) handlePushRecords(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "stream name required")
		return
	}
	var req pushRecordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}
	records := make([]*streamsql.Record, 0, len(req.Records))
	for _, fields := range req.Records {
		records = append(records, &streamsql.Record{Fields: fields})
	}
	pushed, err := h.engine.PushBatch(name, records)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "pushed": pushed})
}

func (h *StreamSQLHandler) handleGetQuery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query name required")
		return
	}
	query, err := h.engine.GetQuery(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, query)
}

func (h *StreamSQLHandler) handlePauseQuery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query name required")
		return
	}
	if err := h.engine.PauseQuery(name); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "status": "paused"})
}

func (h *StreamSQLHandler) handleResumeQuery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query name required")
		return
	}
	if err := h.engine.ResumeQuery(name); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{"success": true, "status": "active"})
}

func (h *StreamSQLHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *StreamSQLHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
