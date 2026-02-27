package server

import (
	"context"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/audittrail"
)

// AuditTrailHandler handles audit trail API requests.
type AuditTrailHandler struct {
	trail *audittrail.Trail
}

// NewAuditTrailHandler creates a new audit trail handler.
func NewAuditTrailHandler(trail *audittrail.Trail) *AuditTrailHandler {
	return &AuditTrailHandler{trail: trail}
}

// RegisterRoutes registers audit trail API routes.
func (h *AuditTrailHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/audit/trail/record", h.handleRecord)
	mux.HandleFunc("GET /v1/audit/trail/entity/{entity}", h.handleQueryByEntity)
	mux.HandleFunc("GET /v1/audit/trail/feature/{feature}", h.handleQueryByFeature)
	mux.HandleFunc("GET /v1/audit/trail/verify", h.handleVerifyChain)
	mux.HandleFunc("GET /v1/audit/trail/proof/{id}", h.handleGetProof)
	mux.HandleFunc("POST /v1/audit/trail/compliance", h.handleComplianceReport)
	mux.HandleFunc("GET /v1/audit/trail/stats", h.handleGetStats)
}

// handleRecord handles POST /v1/audit/trail/record
func (h *AuditTrailHandler) handleRecord(w http.ResponseWriter, r *http.Request) {
	var event audittrail.Event
	if err := strictDecode(r.Body, &event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.trail.Record(event); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "event recorded"})
}

// handleQueryByEntity handles GET /v1/audit/trail/entity/{entity}
func (h *AuditTrailHandler) handleQueryByEntity(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity required")
		return
	}

	start, end := h.parseTimeRange(r)
	events := h.trail.QueryByEntity(entity, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity": entity,
		"events": events,
	})
}

// handleQueryByFeature handles GET /v1/audit/trail/feature/{feature}
func (h *AuditTrailHandler) handleQueryByFeature(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	if feature == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature required")
		return
	}

	start, end := h.parseTimeRange(r)
	events := h.trail.QueryByFeature(feature, start, end)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature": feature,
		"events":  events,
	})
}

// handleVerifyChain handles GET /v1/audit/trail/verify
func (h *AuditTrailHandler) handleVerifyChain(w http.ResponseWriter, r *http.Request) {
	valid, err := h.trail.VerifyChain()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":       valid,
		"merkle_root": h.trail.GetMerkleRoot(),
	})
}

// handleGetProof handles GET /v1/audit/trail/proof/{id}
func (h *AuditTrailHandler) handleGetProof(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "event id required")
		return
	}

	proof, err := h.trail.GetProof(id)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, proof)
}

// handleComplianceReport handles POST /v1/audit/trail/compliance
func (h *AuditTrailHandler) handleComplianceReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportType string `json:"report_type"`
		Start      string `json:"start"`
		End        string `json:"end"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	start, _ := time.Parse(time.RFC3339, req.Start)
	end, _ := time.Parse(time.RFC3339, req.End)
	if end.IsZero() {
		end = time.Now()
	}

	report, err := h.trail.GenerateComplianceReport(req.ReportType, start, end)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// handleGetStats handles GET /v1/audit/trail/stats
func (h *AuditTrailHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.trail.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *AuditTrailHandler) parseTimeRange(r *http.Request) (time.Time, time.Time) {
	var start, end time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			start = parsed
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if parsed, err := time.Parse(time.RFC3339, untilStr); err == nil {
			end = parsed
		}
	}
	if end.IsZero() {
		end = time.Now()
	}
	return start, end
}

func (h *AuditTrailHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *AuditTrailHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
