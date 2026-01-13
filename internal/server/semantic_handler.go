package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/semantic"
)

// SemanticHandler handles semantic feature discovery API requests.
type SemanticHandler struct {
	search    *semantic.Search
	indexer   *semantic.EnhancedIndexer
	ranker    *semantic.HybridRanker
	explainer *semantic.Explainer
	logger    *slog.Logger
}

// NewSemanticHandler creates a new semantic handler.
func NewSemanticHandler(search *semantic.Search) *SemanticHandler {
	logger := slog.Default()
	var indexer *semantic.EnhancedIndexer
	var ranker *semantic.HybridRanker
	var explainer *semantic.Explainer

	if search != nil {
		indexer = semantic.NewEnhancedIndexer(search)
		ranker = semantic.NewHybridRanker(indexer, semantic.DefaultRankerConfig())
		explainer = semantic.NewExplainer(indexer, nil, semantic.DefaultExplainerConfig())
	}

	return &SemanticHandler{
		search:    search,
		indexer:   indexer,
		ranker:    ranker,
		explainer: explainer,
		logger:    logger,
	}
}

// NewSemanticHandlerWithLogger creates a handler with a custom logger.
func NewSemanticHandlerWithLogger(search *semantic.Search, logger *slog.Logger) *SemanticHandler {
	if logger == nil {
		logger = slog.Default()
	}

	var indexer *semantic.EnhancedIndexer
	var ranker *semantic.HybridRanker
	var explainer *semantic.Explainer

	if search != nil {
		indexer = semantic.NewEnhancedIndexer(search)
		ranker = semantic.NewHybridRanker(indexer, semantic.DefaultRankerConfig())
		explainer = semantic.NewExplainer(indexer, nil, semantic.DefaultExplainerConfig())
	}

	return &SemanticHandler{
		search:    search,
		indexer:   indexer,
		ranker:    ranker,
		explainer: explainer,
		logger:    logger,
	}
}

// RegisterRoutes registers semantic search API routes.
func (h *SemanticHandler) RegisterRoutes(mux *http.ServeMux) {
	// Basic search routes
	mux.HandleFunc("POST /v1/semantic/search", h.handleSearch)
	mux.HandleFunc("GET /v1/semantic/search", h.handleSearchGet)

	// Feature indexing routes
	mux.HandleFunc("GET /v1/semantic/features", h.handleListFeatures)
	mux.HandleFunc("POST /v1/semantic/features", h.handleIndexFeature)
	mux.HandleFunc("POST /v1/semantic/features/batch", h.handleIndexBatch)
	mux.HandleFunc("GET /v1/semantic/features/{id}", h.handleGetFeature)
	mux.HandleFunc("DELETE /v1/semantic/features/{id}", h.handleDeleteFeature)

	// Enhanced metadata routes
	mux.HandleFunc("GET /v1/semantic/features/{id}/enriched", h.handleGetEnrichedFeature)
	mux.HandleFunc("GET /v1/semantic/features/{id}/metadata", h.handleGetMetadata)
	mux.HandleFunc("PUT /v1/semantic/features/{id}/metadata", h.handleUpdateMetadata)
	mux.HandleFunc("PUT /v1/semantic/features/{id}/statistics", h.handleSetStatistics)
	mux.HandleFunc("PUT /v1/semantic/features/{id}/lineage", h.handleSetLineage)
	mux.HandleFunc("PUT /v1/semantic/features/{id}/usage", h.handleSetUsage)

	// Advanced search routes
	mux.HandleFunc("POST /v1/semantic/rank", h.handleRank)
	mux.HandleFunc("GET /v1/semantic/suggest/{id}", h.handleSuggest)
	mux.HandleFunc("POST /v1/semantic/suggest", h.handleSuggestPost)
	mux.HandleFunc("POST /v1/semantic/recommend", h.handleRecommend)

	// Explanation routes
	mux.HandleFunc("POST /v1/semantic/explain", h.handleExplain)
	mux.HandleFunc("POST /v1/semantic/explain/batch", h.handleExplainBatch)

	// Discovery routes
	mux.HandleFunc("GET /v1/semantic/discover/popular", h.handleDiscoverPopular)
	mux.HandleFunc("GET /v1/semantic/discover/quality", h.handleDiscoverHighQuality)
	mux.HandleFunc("GET /v1/semantic/discover/domain/{domain}", h.handleDiscoverByDomain)
	mux.HandleFunc("GET /v1/semantic/discover/entity/{entityType}", h.handleDiscoverByEntity)
	mux.HandleFunc("GET /v1/semantic/discover/usecase/{usecase}", h.handleDiscoverByUseCase)

	// Stats route
	mux.HandleFunc("GET /v1/semantic/stats", h.handleGetStats)
}

// Indexer returns the enhanced indexer for external use.
func (h *SemanticHandler) Indexer() *semantic.EnhancedIndexer {
	return h.indexer
}

// Search returns the underlying search instance.
func (h *SemanticHandler) Search() *semantic.Search {
	return h.search
}

// Request/Response types

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

// EnhancedFeatureRequest represents an enhanced feature indexing request.
type EnhancedFeatureRequest struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Category      string            `json:"category,omitempty"`
	Subcategory   string            `json:"subcategory,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	DataType      string            `json:"data_type,omitempty"`
	ValueType     string            `json:"value_type,omitempty"`
	EntityType    string            `json:"entity_type,omitempty"`
	Domain        string            `json:"domain,omitempty"`
	BusinessUnit  string            `json:"business_unit,omitempty"`
	UseCases      []string          `json:"use_cases,omitempty"`
	Owner         string            `json:"owner,omitempty"`
	Team          string            `json:"team,omitempty"`
	QualityScore  float32           `json:"quality_score,omitempty"`
	DataQuality   string            `json:"data_quality,omitempty"`
	Freshness     string            `json:"freshness,omitempty"`
	Documentation string            `json:"documentation,omitempty"`
	Examples      []string          `json:"examples,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
}

// SearchResultJSON represents a search result in JSON.
type SearchResultJSON struct {
	Feature    *FeatureDocJSON `json:"feature"`
	Score      float32         `json:"score"`
	Similarity float32         `json:"similarity"`
}

// SemanticSearchRequest represents a search request.
type SemanticSearchRequest struct {
	Query      string   `json:"query"`
	Limit      int      `json:"limit,omitempty"`
	MinScore   float32  `json:"min_score,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Owner      string   `json:"owner,omitempty"`
}

// SemanticRankRequest represents a hybrid ranking request.
type SemanticRankRequest struct {
	Query           string   `json:"query"`
	Categories      []string `json:"categories,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	EntityTypes     []string `json:"entity_types,omitempty"`
	Domains         []string `json:"domains,omitempty"`
	UseCases        []string `json:"use_cases,omitempty"`
	Owner           string   `json:"owner,omitempty"`
	Team            string   `json:"team,omitempty"`
	MinQuality      float32  `json:"min_quality,omitempty"`
	OnlyFresh       bool     `json:"only_fresh,omitempty"`
	ExcludeFeatures []string `json:"exclude_features,omitempty"`
	Limit           int      `json:"limit,omitempty"`
}

// SuggestRequestBody represents a similarity suggestion request.
type SuggestRequestBody struct {
	FeatureID string `json:"feature_id"`
	Limit     int    `json:"limit,omitempty"`
}

// RecommendRequestBody represents a model recommendation request.
type RecommendRequestBody struct {
	ExistingFeatures []string `json:"existing_features"`
	ModelUseCase     string   `json:"model_use_case"`
	Limit            int      `json:"limit,omitempty"`
}

// ExplainRequestBody represents an explanation request.
type ExplainRequestBody struct {
	FeatureID string  `json:"feature_id"`
	Query     string  `json:"query"`
	Score     float64 `json:"score,omitempty"`
}

// handleListFeatures handles GET /v1/semantic/features
func (h *SemanticHandler) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	features := h.search.ListFeatures()
	response := make([]FeatureDocJSON, len(features))

	for i, f := range features {
		response[i] = h.featureToJSON(f)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": response,
		"count":    len(response),
	})
}

// handleGetFeature handles GET /v1/semantic/features/{id}
func (h *SemanticHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	feature, err := h.search.GetFeature(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.featureToJSON(feature))
}

// handleGetEnrichedFeature handles GET /v1/semantic/features/{id}/enriched
func (h *SemanticHandler) handleGetEnrichedFeature(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	enriched, err := h.indexer.GetEnrichedFeature(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, enriched)
}

// handleIndexFeature handles POST /v1/semantic/features
func (h *SemanticHandler) handleIndexFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var req EnhancedFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "id is required")
		return
	}

	// If enhanced indexer available, use it
	if h.indexer != nil {
		meta := &semantic.FeatureMetadata{
			FeatureID:     req.ID,
			Name:          req.Name,
			Description:   req.Description,
			Category:      req.Category,
			Subcategory:   req.Subcategory,
			Tags:          req.Tags,
			Labels:        req.Labels,
			DataType:      req.DataType,
			ValueType:     req.ValueType,
			EntityType:    req.EntityType,
			Domain:        req.Domain,
			BusinessUnit:  req.BusinessUnit,
			UseCase:       req.UseCases,
			Owner:         req.Owner,
			Team:          req.Team,
			QualityScore:  req.QualityScore,
			DataQuality:   req.DataQuality,
			Freshness:     req.Freshness,
			Documentation: req.Documentation,
			Examples:      req.Examples,
			CustomFields:  req.CustomFields,
		}

		if err := h.indexer.IndexFeatureWithMetadata(r.Context(), meta); err != nil {
			h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		// Fallback to basic indexing
		feature := &semantic.FeatureDocument{
			ID:          req.ID,
			Name:        req.Name,
			Description: req.Description,
			Tags:        req.Tags,
			Category:    req.Category,
			DataType:    req.DataType,
			Owner:       req.Owner,
			Metadata:    req.CustomFields,
		}

		if err := h.search.IndexFeature(r.Context(), feature); err != nil {
			h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": req.ID,
	})
}

// handleIndexBatch handles POST /v1/semantic/features/batch
func (h *SemanticHandler) handleIndexBatch(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var features []EnhancedFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&features); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	indexed := 0
	var errors []string

	for _, req := range features {
		if req.ID == "" || req.Name == "" {
			errors = append(errors, "skipped feature with missing id or name")
			continue
		}

		if h.indexer != nil {
			meta := &semantic.FeatureMetadata{
				FeatureID:    req.ID,
				Name:         req.Name,
				Description:  req.Description,
				Category:     req.Category,
				Tags:         req.Tags,
				DataType:     req.DataType,
				ValueType:    req.ValueType,
				EntityType:   req.EntityType,
				Domain:       req.Domain,
				UseCase:      req.UseCases,
				Owner:        req.Owner,
				Team:         req.Team,
				QualityScore: req.QualityScore,
				DataQuality:  req.DataQuality,
				Freshness:    req.Freshness,
			}

			if err := h.indexer.IndexFeatureWithMetadata(r.Context(), meta); err != nil {
				errors = append(errors, req.ID+": "+err.Error())
				continue
			}
		} else {
			feature := &semantic.FeatureDocument{
				ID:          req.ID,
				Name:        req.Name,
				Description: req.Description,
				Tags:        req.Tags,
				Category:    req.Category,
				DataType:    req.DataType,
				Owner:       req.Owner,
			}

			if err := h.search.IndexFeature(r.Context(), feature); err != nil {
				errors = append(errors, req.ID+": "+err.Error())
				continue
			}
		}
		indexed++
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": len(errors) == 0,
		"indexed": indexed,
		"total":   len(features),
		"errors":  errors,
	})
}

// handleDeleteFeature handles DELETE /v1/semantic/features/{id}
func (h *SemanticHandler) handleDeleteFeature(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	if err := h.search.DeleteFeature(featureID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetMetadata handles GET /v1/semantic/features/{id}/metadata
func (h *SemanticHandler) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	meta, err := h.indexer.GetMetadata(featureID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, meta)
}

// handleUpdateMetadata handles PUT /v1/semantic/features/{id}/metadata
func (h *SemanticHandler) handleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	var meta semantic.FeatureMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	meta.FeatureID = featureID

	if err := h.indexer.IndexFeatureWithMetadata(r.Context(), &meta); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleSetStatistics handles PUT /v1/semantic/features/{id}/statistics
func (h *SemanticHandler) handleSetStatistics(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	var stats semantic.FeatureStatistics
	if err := json.NewDecoder(r.Body).Decode(&stats); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.indexer.SetStatistics(featureID, &stats); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleSetLineage handles PUT /v1/semantic/features/{id}/lineage
func (h *SemanticHandler) handleSetLineage(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	var lineage semantic.FeatureLineage
	if err := json.NewDecoder(r.Body).Decode(&lineage); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.indexer.SetLineage(featureID, &lineage); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleSetUsage handles PUT /v1/semantic/features/{id}/usage
func (h *SemanticHandler) handleSetUsage(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	var usage semantic.FeatureUsage
	if err := json.NewDecoder(r.Body).Decode(&usage); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.indexer.SetUsage(featureID, &usage); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleSearch handles POST /v1/semantic/search
func (h *SemanticHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	var req SemanticSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
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
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]SearchResultJSON, len(results))
	for i, res := range results {
		response[i] = SearchResultJSON{
			Feature:    h.featureToJSONPtr(res.Feature),
			Score:      res.Score,
			Similarity: res.Similarity,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": response,
		"count":   len(response),
		"query":   req.Query,
	})
}

// handleSearchGet handles GET /v1/semantic/search?q=...
func (h *SemanticHandler) handleSearchGet(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "q parameter is required")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	minScore := float32(0.3)
	if ms := r.URL.Query().Get("min_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 32); err == nil {
			minScore = float32(parsed)
		}
	}

	opts := semantic.SearchOptions{
		Limit:      limit,
		MinScore:   minScore,
		Categories: r.URL.Query()["category"],
		Tags:       r.URL.Query()["tag"],
		Owner:      r.URL.Query().Get("owner"),
	}

	results, err := h.search.Search(r.Context(), query, opts)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]SearchResultJSON, len(results))
	for i, res := range results {
		response[i] = SearchResultJSON{
			Feature:    h.featureToJSONPtr(res.Feature),
			Score:      res.Score,
			Similarity: res.Similarity,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": response,
		"count":   len(response),
		"query":   query,
	})
}

// handleRank handles POST /v1/semantic/rank
func (h *SemanticHandler) handleRank(w http.ResponseWriter, r *http.Request) {
	if h.ranker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "hybrid ranker not configured")
		return
	}

	var req SemanticRankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	rankReq := semantic.RankRequest{
		Query:           req.Query,
		Categories:      req.Categories,
		Tags:            req.Tags,
		EntityTypes:     req.EntityTypes,
		Domains:         req.Domains,
		UseCases:        req.UseCases,
		Owner:           req.Owner,
		Team:            req.Team,
		MinQuality:      req.MinQuality,
		OnlyFresh:       req.OnlyFresh,
		ExcludeFeatures: req.ExcludeFeatures,
		Limit:           req.Limit,
	}

	results, err := h.ranker.Rank(r.Context(), rankReq)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"query":   req.Query,
	})
}

// handleSuggest handles GET /v1/semantic/suggest/{id}
func (h *SemanticHandler) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature ID required")
		return
	}

	limit := 5
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Use ranker for better suggestions if available
	if h.ranker != nil {
		results, err := h.ranker.SuggestSimilar(r.Context(), featureID, limit)
		if err != nil {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}

		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
			"suggestions": results,
			"count":       len(results),
			"source":      featureID,
		})
		return
	}

	// Fallback to basic suggest
	results, err := h.search.Suggest(r.Context(), featureID, limit)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	response := make([]SearchResultJSON, len(results))
	for i, res := range results {
		response[i] = SearchResultJSON{
			Feature:    h.featureToJSONPtr(res.Feature),
			Score:      res.Score,
			Similarity: res.Similarity,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"suggestions": response,
		"count":       len(response),
		"source":      featureID,
	})
}

// handleSuggestPost handles POST /v1/semantic/suggest
func (h *SemanticHandler) handleSuggestPost(w http.ResponseWriter, r *http.Request) {
	if h.ranker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "hybrid ranker not configured")
		return
	}

	var req SuggestRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_id is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}

	results, err := h.ranker.SuggestSimilar(r.Context(), req.FeatureID, req.Limit)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"suggestions": results,
		"count":       len(results),
		"source":      req.FeatureID,
	})
}

// handleRecommend handles POST /v1/semantic/recommend
func (h *SemanticHandler) handleRecommend(w http.ResponseWriter, r *http.Request) {
	if h.ranker == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "hybrid ranker not configured")
		return
	}

	var req RecommendRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.ExistingFeatures) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "existing_features is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	results, err := h.ranker.RecommendForModel(r.Context(), req.ExistingFeatures, req.ModelUseCase, req.Limit)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results":  results,
		"count":    len(results),
		"use_case": req.ModelUseCase,
	})
}

// handleExplain handles POST /v1/semantic/explain
func (h *SemanticHandler) handleExplain(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "explainer not configured")
		return
	}

	var req ExplainRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_id is required")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	if req.Score == 0 {
		req.Score = 0.5
	}

	explanation, err := h.explainer.Explain(r.Context(), req.FeatureID, req.Query, req.Score)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, explanation)
}

// handleExplainBatch handles POST /v1/semantic/explain/batch
func (h *SemanticHandler) handleExplainBatch(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "explainer not configured")
		return
	}

	var req struct {
		Results []struct {
			FeatureID string  `json:"feature_id"`
			Score     float64 `json:"score"`
		} `json:"results"`
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Results) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "results is required")
		return
	}
	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	// Convert to RankedResult format
	rankedResults := make([]semantic.RankedResult, 0, len(req.Results))
	for _, res := range req.Results {
		enriched, err := h.indexer.GetEnrichedFeature(res.FeatureID)
		if err != nil {
			continue
		}
		rankedResults = append(rankedResults, semantic.RankedResult{
			Feature:    enriched,
			TotalScore: res.Score,
		})
	}

	explanations, err := h.explainer.ExplainResults(r.Context(), rankedResults, req.Query)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"explanations": explanations,
		"count":        len(explanations),
		"query":        req.Query,
	})
}

// handleDiscoverPopular handles GET /v1/semantic/discover/popular
func (h *SemanticHandler) handleDiscoverPopular(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	features := h.indexer.GetMostPopular(limit)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
	})
}

// handleDiscoverHighQuality handles GET /v1/semantic/discover/quality
func (h *SemanticHandler) handleDiscoverHighQuality(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	minScore := float32(0.8)
	if ms := r.URL.Query().Get("min_score"); ms != "" {
		if parsed, err := strconv.ParseFloat(ms, 32); err == nil {
			minScore = float32(parsed)
		}
	}

	features := h.indexer.GetHighQuality(minScore)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":  features,
		"count":     len(features),
		"min_score": minScore,
	})
}

// handleDiscoverByDomain handles GET /v1/semantic/discover/domain/{domain}
func (h *SemanticHandler) handleDiscoverByDomain(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	domain := r.PathValue("domain")
	if domain == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "domain required")
		return
	}

	features := h.indexer.FindByDomain(domain)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"domain":   domain,
	})
}

// handleDiscoverByEntity handles GET /v1/semantic/discover/entity/{entityType}
func (h *SemanticHandler) handleDiscoverByEntity(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	entityType := r.PathValue("entityType")
	if entityType == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity type required")
		return
	}

	features := h.indexer.FindByEntityType(entityType)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features":    features,
		"count":       len(features),
		"entity_type": entityType,
	})
}

// handleDiscoverByUseCase handles GET /v1/semantic/discover/usecase/{usecase}
func (h *SemanticHandler) handleDiscoverByUseCase(w http.ResponseWriter, r *http.Request) {
	if h.indexer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "enhanced indexer not configured")
		return
	}

	useCase := r.PathValue("usecase")
	if useCase == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "use case required")
		return
	}

	features := h.indexer.FindByUseCase(useCase)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"features": features,
		"count":    len(features),
		"use_case": useCase,
	})
}

// handleGetStats handles GET /v1/semantic/stats
func (h *SemanticHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "semantic search not configured")
		return
	}

	stats := h.search.GetStats()

	if h.indexer != nil {
		indexerStats := h.indexer.GetStats()
		stats["indexer"] = indexerStats
	}

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
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

func (h *SemanticHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *SemanticHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
