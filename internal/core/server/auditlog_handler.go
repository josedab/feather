package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/auditlog"
)

// AuditLogHandler handles audit log API requests.
type AuditLogHandler struct {
	logger *auditlog.Logger
}

// NewAuditLogHandler creates a new audit log handler.
func NewAuditLogHandler(logger *auditlog.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		logger: logger,
	}
}

// RegisterRoutes registers audit log API routes.
func (h *AuditLogHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/audit/log", h.handleLogEntry)
	mux.HandleFunc("GET /v1/audit/logs", h.handleQueryLogs)
	mux.HandleFunc("GET /v1/audit/logs/{id}", h.handleGetEntry)
	mux.HandleFunc("POST /v1/audit/export", h.handleExport)
	mux.HandleFunc("POST /v1/audit/purge", h.handlePurge)
	mux.HandleFunc("GET /v1/audit/stats", h.handleGetStats)
}

// handleLogEntry handles POST /v1/audit/log
func (h *AuditLogHandler) handleLogEntry(w http.ResponseWriter, r *http.Request) {
	var entry auditlog.AuditEntry
	if err := strictDecode(r.Body, &entry); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.logger.Log(entry); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "entry logged"})
}

// handleQueryLogs handles GET /v1/audit/logs
func (h *AuditLogHandler) handleQueryLogs(w http.ResponseWriter, r *http.Request) {
	filter := auditlog.QueryFilter{
		Action:   auditlog.ActionType(r.URL.Query().Get("action")),
		Actor:    r.URL.Query().Get("actor"),
		Resource: r.URL.Query().Get("resource"),
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filter.StartTime = parsed
		}
	}

	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if parsed, err := time.Parse(time.RFC3339, untilStr); err == nil {
			filter.EndTime = parsed
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			filter.Limit = parsed
		}
	}

	entries := h.logger.Query(filter)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// handleGetEntry handles GET /v1/audit/logs/{id}
func (h *AuditLogHandler) handleGetEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entry id required")
		return
	}

	entry, err := h.logger.GetEntry(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, entry)
}

// handleExport handles POST /v1/audit/export
func (h *AuditLogHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filter auditlog.QueryFilter `json:"filter"`
		Format string               `json:"format"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	format := auditlog.ExportJSON
	if req.Format == "csv" {
		format = auditlog.ExportCSV
	}

	exported, err := h.logger.Export(req.Filter, format)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"data":   exported,
		"format": req.Format,
	})
}

// handlePurge handles POST /v1/audit/purge
func (h *AuditLogHandler) handlePurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Before string `json:"before"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	before, err := time.Parse(time.RFC3339, req.Before)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid 'before' timestamp, use RFC3339 format")
		return
	}

	purged := h.logger.Purge(before)
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: fmt.Sprintf("purged %d entries", purged)})
}

// handleGetStats handles GET /v1/audit/stats
func (h *AuditLogHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.logger.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *AuditLogHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *AuditLogHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
