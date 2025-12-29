package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/semantic"
)

// SemanticHandler handles semantic search API requests.
type SemanticHandler struct {
	search *semantic.Search
}

// NewSemanticHandler creates a new semantic handler.
func NewSemanticHandler(search *semantic.Search) *SemanticHandler {
	return &SemanticHandler{
		search: search,
	}
}

// RegisterRoutes registers semantic search API routes.
func (h *SemanticHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feature indexing
	mux.HandleFunc("GET /v1/semantic/features", h.handleListFeatures)
	mux.HandleFunc("GET /v1/semantic/features/{id}", h.handleGetFeature)
	mux.HandleFunc("POST /v1/semantic/features", h.handleIndexFeature)
	mux.HandleFunc("POST /v1/semantic/features/batch", h.handleIndexBatch)
	mux.HandleFunc("DELETE /v1/semantic/features/{id}", h.handleDeleteFeature)

	// Search
	mux.HandleFunc("POST /v1/semantic/search", h.handleSearch)
	mux.HandleFunc("GET /v1/semantic/suggest/{id}", h.handleSuggest)

	// Stats
	mux.HandleFunc("GET /v1/semantic/stats", h.handleGetStats)
}

// FeatureDocJSON represents a feature document in JSON.
type FeatureDocJSON struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags,omitempty"`
	Category    string            `json:"category,omitempty"`
	DataType    string            `json:"data_type,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SearchResultJSON represents a search result in JSON.
type SearchResultJSON struct {
	Feature    *FeatureDocJSON `json:"feature"`
	Score      float32         `json:"score"`
	Similarity float32         `json:"similarity"`
}

// handleListFeatures handles GET /v1/semantic/features
func (h *SemanticHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	features := h.search.ListFeatures()
	response := make([]FeatureDocJSON, len(features))

	for i, f := range features {
		response[i] = h.featureToJSON(f)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"features": response,
		"count":    len(response),
	})
}

// handleGetFeature handles GET /v1/semantic/features/{id}
func (h *SemanticHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	feature, err := h.search.GetFeature(featureID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.featureToJSON(feature))
}

// handleIndexFeature handles POST /v1/semantic/features
func (h *SemanticHandler) handleIndexFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var req FeatureDocJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	feature := &semantic.FeatureDocument{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		Category:    req.Category,
		DataType:    req.DataType,
		Owner:       req.Owner,
		Metadata:    req.Metadata,
	}

	if err := h.search.IndexFeature(r.Context(), feature); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": req.ID,
	})
}

// handleIndexBatch handles POST /v1/semantic/features/batch
func (h *SemanticHandler) handleIndexBatch(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var req struct {
		Features []FeatureDocJSON `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	features := make([]*semantic.FeatureDocument, len(req.Features))
	for i, f := range req.Features {
		features[i] = &semantic.FeatureDocument{
			ID:          f.ID,
			Name:        f.Name,
			Description: f.Description,
			Tags:        f.Tags,
			Category:    f.Category,
			DataType:    f.DataType,
			Owner:       f.Owner,
			Metadata:    f.Metadata,
		}
	}

	if err := h.search.IndexBatch(r.Context(), features); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"count":   len(features),
	})
}

// handleDeleteFeature handles DELETE /v1/semantic/features/{id}
func (h *SemanticHandler) handleDeleteFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	if err := h.search.DeleteFeature(featureID); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	MinScore   float32  `json:"min_score,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Owner      string   `json:"owner,omitempty"`
}

// handleSearch handles POST /v1/semantic/search
func (h *SemanticHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		h.writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	opts := semantic.SearchOptions{
		Limit:      req.Limit,
		MinScore:   req.MinScore,
		Categories: req.Categories,
		Tags:       req.Tags,
		Owner:      req.Owner,
	}

	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	if opts.MinScore <= 0 {
		opts.MinScore = 0.3
	}

	results, err := h.search.Search(r.Context(), req.Query, opts)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]SearchResultJSON, len(results))
	for i, r := range results {
		response[i] = SearchResultJSON{
			Feature:    h.featureToJSONPtr(r.Feature),
			Score:      r.Score,
			Similarity: r.Similarity,
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": response,
		"count":   len(response),
		"query":   req.Query,
	})
}

// handleSuggest handles GET /v1/semantic/suggest/{id}
func (h *SemanticHandler) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	limit := 5
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var parsedLimit int
		if err := json.Unmarshal([]byte(limitStr), &parsedLimit); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	results, err := h.search.Suggest(r.Context(), featureID, limit)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	response := make([]SearchResultJSON, len(results))
	for i, r := range results {
		response[i] = SearchResultJSON{
			Feature:    h.featureToJSONPtr(r.Feature),
			Score:      r.Score,
			Similarity: r.Similarity,
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": response,
		"count":       len(response),
		"source":      featureID,
	})
}

// handleGetStats handles GET /v1/semantic/stats
func (h *SemanticHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	stats := h.search.GetStats()
	h.writeJSON(w, http.StatusOK, stats)
}

func (h *SemanticHandler) featureToJSON(f *semantic.FeatureDocument) FeatureDocJSON {
	return FeatureDocJSON{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Tags:        f.Tags,
		Category:    f.Category,
		DataType:    f.DataType,
		Owner:       f.Owner,
		Metadata:    f.Metadata,
	}
}

func (h *SemanticHandler) featureToJSONPtr(f *semantic.FeatureDocument) *FeatureDocJSON {
	if f == nil {
		return nil
	}
	j := h.featureToJSON(f)
	return &j
}

func (h *SemanticHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(w, status, data)
}

func (h *SemanticHandler) writeError(w http.ResponseWriter, status int, message string) {
	writeJSONError(w, status, message)
}
