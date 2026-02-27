package server

import (
	"context"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/lineage"
)

// LineageHandler handles lineage API requests.
type LineageHandler struct {
	tracker *lineage.Tracker
}

// NewLineageHandler creates a new lineage handler.
func NewLineageHandler(tracker *lineage.Tracker) *LineageHandler {
	return &LineageHandler{
		tracker: tracker,
	}
}

// RegisterRoutes registers lineage API routes.
func (h *LineageHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feature lineage
	mux.HandleFunc("GET /v1/lineage/features", h.handleListFeatures)
	mux.HandleFunc("GET /v1/lineage/features/{id}", h.handleGetFeature)
	mux.HandleFunc("POST /v1/lineage/features", h.handleRegisterFeature)

	// Sources
	mux.HandleFunc("GET /v1/lineage/sources", h.handleListSources)
	mux.HandleFunc("POST /v1/lineage/sources", h.handleRegisterSource)

	// Consumers
	mux.HandleFunc("GET /v1/lineage/consumers", h.handleListConsumers)
	mux.HandleFunc("POST /v1/lineage/consumers", h.handleRegisterConsumer)

	// Links
	mux.HandleFunc("POST /v1/lineage/link/source", h.handleLinkSource)
	mux.HandleFunc("POST /v1/lineage/link/consumer", h.handleLinkConsumer)

	// Analysis
	mux.HandleFunc("GET /v1/lineage/impact/{id}", h.handleImpactAnalysis)
	mux.HandleFunc("GET /v1/lineage/graph", h.handleGetGraph)
	mux.HandleFunc("GET /v1/lineage/graph/dot", h.handleGetGraphDOT)
	mux.HandleFunc("GET /v1/lineage/graph/mermaid", h.handleGetGraphMermaid)

	// Compliance
	mux.HandleFunc("GET /v1/lineage/pii", h.handleGetPIIFeatures)
	mux.HandleFunc("POST /v1/lineage/pii/{id}", h.handleSetPIIMetadata)
	mux.HandleFunc("GET /v1/lineage/audit", h.handleGetAuditLog)
}

// handleListFeatures handles GET /v1/lineage/features
func (h *LineageHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	features := h.tracker.GetAllFeatures()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

// handleGetFeature handles GET /v1/lineage/features/{id}
func (h *LineageHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	feature, err := h.tracker.GetFeatureLineage(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, feature)
}

// RegisterLineageRequest represents a request to register a feature for lineage.
type RegisterLineageRequest struct {
	FeatureID    string   `json:"feature_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	CreatedBy    string   `json:"created_by,omitempty"`
}

// handleRegisterFeature handles POST /v1/lineage/features
func (h *LineageHandler) handleRegisterFeature(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	var req RegisterLineageRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_id is required")
		return
	}

	lineageData := &lineage.FeatureLineage{
		FeatureID:    req.FeatureID,
		Name:         req.Name,
		Description:  req.Description,
		Dependencies: req.Dependencies,
		Tags:         req.Tags,
		CreatedBy:    req.CreatedBy,
		Metadata:     make(map[string]string),
	}

	if err := h.tracker.RegisterFeature(lineageData); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": req.FeatureID,
	})
}

// handleListSources handles GET /v1/lineage/sources
func (h *LineageHandler) handleListSources(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	// Get sources from graph nodes of type source
	graph := h.tracker.GetDependencyGraph()
	nodes := graph.GetNodes()

	sources := make([]*lineage.GraphNode, 0)
	for _, n := range nodes {
		if n.Type == lineage.NodeTypeSource {
			sources = append(sources, n)
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	})
}

// RegisterSourceRequest represents a request to register a source.
type RegisterSourceRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Connection  string `json:"connection,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Description string `json:"description,omitempty"`
}

// handleRegisterSource handles POST /v1/lineage/sources
func (h *LineageHandler) handleRegisterSource(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	var req RegisterSourceRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}

	source := &lineage.DataSource{
		ID:          req.ID,
		Name:        req.Name,
		Type:        lineage.SourceType(req.Type),
		Connection:  req.Connection,
		Owner:       req.Owner,
		Description: req.Description,
		Metadata:    make(map[string]string),
	}

	if err := h.tracker.RegisterSource(source); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"source_id": req.ID,
	})
}

// handleListConsumers handles GET /v1/lineage/consumers
func (h *LineageHandler) handleListConsumers(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	graph := h.tracker.GetDependencyGraph()
	nodes := graph.GetNodes()

	consumers := make([]*lineage.GraphNode, 0)
	for _, n := range nodes {
		if n.Type == lineage.NodeTypeConsumer {
			consumers = append(consumers, n)
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"consumers": consumers,
		"count":     len(consumers),
	})
}

// RegisterConsumerRequest represents a request to register a consumer.
type RegisterConsumerRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Owner       string `json:"owner,omitempty"`
	Description string `json:"description,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
}

// handleRegisterConsumer handles POST /v1/lineage/consumers
func (h *LineageHandler) handleRegisterConsumer(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	var req RegisterConsumerRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}

	consumer := &lineage.Consumer{
		ID:          req.ID,
		Name:        req.Name,
		Type:        lineage.ConsumerType(req.Type),
		Owner:       req.Owner,
		Description: req.Description,
		Endpoint:    req.Endpoint,
	}

	if err := h.tracker.RegisterConsumer(consumer); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":     true,
		"consumer_id": req.ID,
	})
}

// LinkSourceRequest represents a request to link a source to a feature.
type LinkSourceRequest struct {
	SourceID  string   `json:"source_id"`
	FeatureID string   `json:"feature_id"`
	Fields    []string `json:"fields,omitempty"`
}

// handleLinkSource handles POST /v1/lineage/link/source
func (h *LineageHandler) handleLinkSource(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	var req LinkSourceRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.tracker.LinkSourceToFeature(req.SourceID, req.FeatureID, req.Fields); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// LinkConsumerRequest represents a request to link a feature to a consumer.
type LinkConsumerRequest struct {
	FeatureID  string `json:"feature_id"`
	ConsumerID string `json:"consumer_id"`
	Purpose    string `json:"purpose,omitempty"`
}

// handleLinkConsumer handles POST /v1/lineage/link/consumer
func (h *LineageHandler) handleLinkConsumer(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	var req LinkConsumerRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.tracker.LinkFeatureToConsumer(req.FeatureID, req.ConsumerID, req.Purpose); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleImpactAnalysis handles GET /v1/lineage/impact/{id}
func (h *LineageHandler) handleImpactAnalysis(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	analysis, err := h.tracker.GetImpactAnalysis(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, analysis)
}

// handleGetGraph handles GET /v1/lineage/graph
func (h *LineageHandler) handleGetGraph(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	graph := h.tracker.GetDependencyGraph()
	data, err := graph.ExportJSON()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
	}
}

// handleGetGraphDOT handles GET /v1/lineage/graph/dot
func (h *LineageHandler) handleGetGraphDOT(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	graph := h.tracker.GetDependencyGraph()
	dot := graph.ExportDOT()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(dot)); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
	}
}

// handleGetGraphMermaid handles GET /v1/lineage/graph/mermaid
func (h *LineageHandler) handleGetGraphMermaid(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	graph := h.tracker.GetDependencyGraph()
	mermaid := graph.ExportMermaid()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(mermaid)); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
	}
}

// handleGetPIIFeatures handles GET /v1/lineage/pii
func (h *LineageHandler) handleGetPIIFeatures(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	// Default to showing any PII
	minLevel := lineage.PIILow

	if levelStr := r.URL.Query().Get("min_level"); levelStr != "" {
		switch levelStr {
		case "none":
			minLevel = lineage.PIINone
		case "low":
			minLevel = lineage.PIILow
		case "medium":
			minLevel = lineage.PIIMedium
		case "high":
			minLevel = lineage.PIIHigh
		case "critical":
			minLevel = lineage.PIICritical
		}
	}

	features := h.tracker.GetPIIFeatures(minLevel)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":  features,
		"count":     len(features),
		"min_level": minLevel.String(),
	})
}

// SetPIIMetadataRequest represents a request to set PII metadata.
type SetPIIMetadataRequest struct {
	PIILevel        string   `json:"pii_level"`
	PIITypes        []string `json:"pii_types,omitempty"`
	LegalBasis      string   `json:"legal_basis,omitempty"`
	RetentionPolicy string   `json:"retention_policy,omitempty"`
	DataSubjects    []string `json:"data_subjects,omitempty"`
	CrossBorder     bool     `json:"cross_border"`
	Encrypted       bool     `json:"encrypted"`
	Anonymized      bool     `json:"anonymized"`
}

// handleSetPIIMetadata handles POST /v1/lineage/pii/{id}
func (h *LineageHandler) handleSetPIIMetadata(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	var req SetPIIMetadataRequest
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	piiLevel := lineage.PIINone
	switch req.PIILevel {
	case "low":
		piiLevel = lineage.PIILow
	case "medium":
		piiLevel = lineage.PIIMedium
	case "high":
		piiLevel = lineage.PIIHigh
	case "critical":
		piiLevel = lineage.PIICritical
	}

	pii := &lineage.PIIMetadata{
		FeatureID:       featureID,
		PIILevel:        piiLevel,
		PIITypes:        req.PIITypes,
		LegalBasis:      req.LegalBasis,
		RetentionPolicy: req.RetentionPolicy,
		DataSubjects:    req.DataSubjects,
		CrossBorder:     req.CrossBorder,
		Encrypted:       req.Encrypted,
		Anonymized:      req.Anonymized,
		LastAudit:       time.Now(),
	}

	if err := h.tracker.SetPIIMetadata(featureID, pii); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"feature_id": featureID,
	})
}

// handleGetAuditLog handles GET /v1/lineage/audit
func (h *LineageHandler) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "lineage tracker not configured")
		return
	}

	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	events := h.tracker.GetAuditLog(since)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
		"since":  since,
	})
}

func (h *LineageHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *LineageHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
