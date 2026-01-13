package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/autogen"
)

// AutogenHandler handles auto-generation API requests.
type AutogenHandler struct {
	generator *autogen.Generator
}

// NewAutogenHandler creates a new autogen handler.
func NewAutogenHandler(gen *autogen.Generator) *AutogenHandler {
	return &AutogenHandler{
		generator: gen,
	}
}

// RegisterRoutes registers auto-generation API routes.
func (h *AutogenHandler) RegisterRoutes(mux *http.ServeMux) {
	// Feature generation
	mux.HandleFunc("POST /v1/autogen/features", h.handleGenerateFeatures)
	mux.HandleFunc("POST /v1/autogen/transformations", h.handleSuggestTransformations)
	mux.HandleFunc("POST /v1/autogen/aggregations", h.handleSuggestAggregations)

	// History and stats
	mux.HandleFunc("GET /v1/autogen/history", h.handleGetHistory)
	mux.HandleFunc("GET /v1/autogen/stats", h.handleGetStats)
}

// GenerateFeaturesRequest represents a feature generation request.
type GenerateFeaturesRequest struct {
	Schema struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Entity      string `json:"entity,omitempty"`
		Source      string `json:"source,omitempty"`
		Fields      []struct {
			Name        string            `json:"name"`
			Type        string            `json:"type"`
			Description string            `json:"description,omitempty"`
			Nullable    bool              `json:"nullable,omitempty"`
			Examples    []interface{}     `json:"examples,omitempty"`
			Constraints map[string]string `json:"constraints,omitempty"`
		} `json:"fields"`
	} `json:"schema"`
	ExistingFeatures []string `json:"existing_features,omitempty"`
	UseCase          string   `json:"use_case,omitempty"`
	MaxSuggestions   int      `json:"max_suggestions,omitempty"`
	Constraints      *struct {
		AllowedTypes    []string `json:"allowed_types,omitempty"`
		ForbiddenFields []string `json:"forbidden_fields,omitempty"`
		RequireTags     []string `json:"require_tags,omitempty"`
		MinConfidence   float32  `json:"min_confidence,omitempty"`
		MaxComplexity   int      `json:"max_complexity,omitempty"`
	} `json:"constraints,omitempty"`
}

// FeatureSuggestionJSON represents a feature suggestion in JSON.
type FeatureSuggestionJSON struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Expression   string            `json:"expression"`
	DataType     string            `json:"data_type"`
	Category     string            `json:"category"`
	Tags         []string          `json:"tags,omitempty"`
	Confidence   float32           `json:"confidence"`
	Rationale    string            `json:"rationale"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// handleGenerateFeatures handles POST /v1/autogen/features
func (h *AutogenHandler) handleGenerateFeatures(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "auto-generation not configured")
		return
	}

	var req GenerateFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Schema.Name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "schema name is required")
		return
	}

	if len(req.Schema.Fields) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "schema fields are required")
		return
	}

	// Convert request to autogen types
	fields := make([]autogen.SchemaField, len(req.Schema.Fields))
	for i, f := range req.Schema.Fields {
		fields[i] = autogen.SchemaField{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Nullable:    f.Nullable,
			Examples:    f.Examples,
			Constraints: f.Constraints,
		}
	}

	schema := &autogen.DataSchema{
		Name:        req.Schema.Name,
		Description: req.Schema.Description,
		Fields:      fields,
		Entity:      req.Schema.Entity,
		Source:      req.Schema.Source,
	}

	genReq := &autogen.GenerationRequest{
		Schema:           schema,
		ExistingFeatures: req.ExistingFeatures,
		UseCase:          req.UseCase,
		MaxSuggestions:   req.MaxSuggestions,
	}

	if req.Constraints != nil {
		genReq.Constraints = &autogen.GenerationConstraints{
			AllowedTypes:    req.Constraints.AllowedTypes,
			ForbiddenFields: req.Constraints.ForbiddenFields,
			RequireTags:     req.Constraints.RequireTags,
			MinConfidence:   req.Constraints.MinConfidence,
			MaxComplexity:   req.Constraints.MaxComplexity,
		}
	}

	result, err := h.generator.GenerateFeatures(r.Context(), genReq)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert suggestions to JSON format
	suggestions := make([]FeatureSuggestionJSON, len(result.Suggestions))
	for i, s := range result.Suggestions {
		suggestions[i] = FeatureSuggestionJSON{
			ID:           s.ID,
			Name:         s.Name,
			Description:  s.Description,
			Expression:   s.Expression,
			DataType:     s.DataType,
			Category:     s.Category,
			Tags:         s.Tags,
			Confidence:   s.Confidence,
			Rationale:    s.Rationale,
			Dependencies: s.Dependencies,
			Metadata:     s.Metadata,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"generation_id": result.ID,
		"suggestions":   suggestions,
		"count":         len(suggestions),
		"duration_ms":   result.Duration.Milliseconds(),
		"tokens_used":   result.TokensUsed,
	})
}

// SuggestTransformationsRequest represents a transformation suggestion request.
type SuggestTransformationsRequest struct {
	FeatureName string        `json:"feature_name"`
	FeatureType string        `json:"feature_type"`
	Description string        `json:"description,omitempty"`
	Examples    []interface{} `json:"examples,omitempty"`
}

// TransformationSuggestionJSON represents a transformation suggestion in JSON.
type TransformationSuggestionJSON struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputType   string   `json:"input_type"`
	OutputType  string   `json:"output_type"`
	Expression  string   `json:"expression"`
	Examples    []string `json:"examples,omitempty"`
}

// handleSuggestTransformations handles POST /v1/autogen/transformations
func (h *AutogenHandler) handleSuggestTransformations(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "auto-generation not configured")
		return
	}

	var req SuggestTransformationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FeatureName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_name is required")
		return
	}

	if req.FeatureType == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "feature_type is required")
		return
	}

	suggestions, err := h.generator.SuggestTransformations(r.Context(), req.FeatureName, req.FeatureType, req.Description, req.Examples)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to JSON format
	response := make([]TransformationSuggestionJSON, len(suggestions))
	for i, s := range suggestions {
		response[i] = TransformationSuggestionJSON{
			Name:        s.Name,
			Description: s.Description,
			InputType:   s.InputType,
			OutputType:  s.OutputType,
			Expression:  s.Expression,
			Examples:    s.Examples,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"suggestions": response,
		"count":       len(response),
	})
}

// SuggestAggregationsRequest represents an aggregation suggestion request.
type SuggestAggregationsRequest struct {
	Entity    string `json:"entity"`
	TimeField string `json:"time_field"`
	Fields    []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
	} `json:"fields"`
}

// AggregationSuggestionJSON represents an aggregation suggestion in JSON.
type AggregationSuggestionJSON struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Function    string   `json:"function"`
	Window      string   `json:"window"`
	GroupBy     []string `json:"group_by,omitempty"`
	Expression  string   `json:"expression"`
}

// handleSuggestAggregations handles POST /v1/autogen/aggregations
func (h *AutogenHandler) handleSuggestAggregations(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "auto-generation not configured")
		return
	}

	var req SuggestAggregationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity is required")
		return
	}

	if len(req.Fields) == 0 {
		h.writeError(r.Context(), w, http.StatusBadRequest, "fields are required")
		return
	}

	// Convert fields
	fields := make([]autogen.SchemaField, len(req.Fields))
	for i, f := range req.Fields {
		fields[i] = autogen.SchemaField{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
		}
	}

	suggestions, err := h.generator.SuggestAggregations(r.Context(), req.Entity, fields, req.TimeField)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to JSON format
	response := make([]AggregationSuggestionJSON, len(suggestions))
	for i, s := range suggestions {
		response[i] = AggregationSuggestionJSON{
			Name:        s.Name,
			Description: s.Description,
			Function:    s.Function,
			Window:      s.Window,
			GroupBy:     s.GroupBy,
			Expression:  s.Expression,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"suggestions": response,
		"count":       len(response),
	})
}

// GenerationResultJSON represents a generation result in JSON.
type GenerationResultJSON struct {
	ID          string                  `json:"id"`
	CreatedAt   string                  `json:"created_at"`
	DurationMs  int64                   `json:"duration_ms"`
	TokensUsed  int                     `json:"tokens_used"`
	Suggestions []FeatureSuggestionJSON `json:"suggestions"`
	Error       string                  `json:"error,omitempty"`
}

// handleGetHistory handles GET /v1/autogen/history
func (h *AutogenHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "auto-generation not configured")
		return
	}

	history := h.generator.GetHistory()
	response := make([]GenerationResultJSON, len(history))

	for i, result := range history {
		suggestions := make([]FeatureSuggestionJSON, len(result.Suggestions))
		for j, s := range result.Suggestions {
			suggestions[j] = FeatureSuggestionJSON{
				ID:          s.ID,
				Name:        s.Name,
				Description: s.Description,
				Expression:  s.Expression,
				DataType:    s.DataType,
				Category:    s.Category,
				Tags:        s.Tags,
				Confidence:  s.Confidence,
				Rationale:   s.Rationale,
			}
		}

		response[i] = GenerationResultJSON{
			ID:          result.ID,
			CreatedAt:   result.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			DurationMs:  result.Duration.Milliseconds(),
			TokensUsed:  result.TokensUsed,
			Suggestions: suggestions,
			Error:       result.Error,
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": response,
		"count":   len(response),
	})
}

// handleGetStats handles GET /v1/autogen/stats
func (h *AutogenHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "auto-generation not configured")
		return
	}

	stats := h.generator.GetStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *AutogenHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *AutogenHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
