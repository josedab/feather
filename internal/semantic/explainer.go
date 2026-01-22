package semantic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Explainer generates human-readable explanations for feature matches.
type Explainer struct {
	indexer   *EnhancedIndexer
	llmClient LLMClient
	config    ExplainerConfig
	cache     map[string]*CachedExplanation
}

// LLMClient interface for generating explanations.
type LLMClient interface {
	// Complete generates a completion for the given prompt.
	Complete(ctx context.Context, prompt string) (string, error)
	// IsAvailable returns whether the LLM service is available.
	IsAvailable() bool
}

// ExplainerConfig configures the explainer behavior.
type ExplainerConfig struct {
	MaxExplanationLength int           `json:"max_explanation_length"`
	CacheTTL             time.Duration `json:"cache_ttl"`
	IncludeStatistics    bool          `json:"include_statistics"`
	IncludeLineage       bool          `json:"include_lineage"`
	IncludeUsage         bool          `json:"include_usage"`
	FallbackToTemplate   bool          `json:"fallback_to_template"`
}

// DefaultExplainerConfig returns sensible defaults.
func DefaultExplainerConfig() ExplainerConfig {
	return ExplainerConfig{
		MaxExplanationLength: 500,
		CacheTTL:             15 * time.Minute,
		IncludeStatistics:    true,
		IncludeLineage:       true,
		IncludeUsage:         true,
		FallbackToTemplate:   true,
	}
}

// CachedExplanation stores a cached explanation.
type CachedExplanation struct {
	Explanation *FeatureExplanation
	CachedAt    time.Time
}

// NewExplainer creates a new explainer.
func NewExplainer(indexer *EnhancedIndexer, llmClient LLMClient, config ExplainerConfig) *Explainer {
	return &Explainer{
		indexer:   indexer,
		llmClient: llmClient,
		config:    config,
		cache:     make(map[string]*CachedExplanation),
	}
}

// FeatureExplanation contains a detailed explanation of why a feature matches a query.
type FeatureExplanation struct {
	FeatureID       string            `json:"feature_id"`
	Query           string            `json:"query"`
	Summary         string            `json:"summary"`
	MatchReasons    []MatchReason     `json:"match_reasons"`
	Relevance       RelevanceAnalysis `json:"relevance"`
	DataInsights    *DataInsights     `json:"data_insights,omitempty"`
	UsageContext    *UsageContext     `json:"usage_context,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
	Confidence      float64           `json:"confidence"`
	GeneratedAt     time.Time         `json:"generated_at"`
	GeneratedBy     string            `json:"generated_by"` // "llm" or "template"
}

// MatchReason explains a specific reason for the match.
type MatchReason struct {
	Type        string  `json:"type"` // "semantic", "exact_match", "tag", "category", "metadata"
	Description string  `json:"description"`
	Score       float64 `json:"score"`
}

// RelevanceAnalysis analyzes the relevance of the feature.
type RelevanceAnalysis struct {
	SemanticMatch    string `json:"semantic_match"`        // Why semantically similar
	ContextualFit    string `json:"contextual_fit"`        // How it fits the context
	PotentialUseCase string `json:"potential_use_case"`    // How it might be used
	Limitations      string `json:"limitations,omitempty"` // Any limitations
}

// DataInsights provides insights about the feature's data.
type DataInsights struct {
	Distribution     string `json:"distribution,omitempty"`
	ValueRange       string `json:"value_range,omitempty"`
	QualitySummary   string `json:"quality_summary"`
	FreshnessSummary string `json:"freshness_summary"`
}

// UsageContext provides context about how the feature is used.
type UsageContext struct {
	Popularity      string   `json:"popularity"`
	ModelsUsing     []string `json:"models_using,omitempty"`
	CommonUseCases  []string `json:"common_use_cases,omitempty"`
	RelatedFeatures []string `json:"related_features,omitempty"`
}

// Explain generates an explanation for why a feature matches a query.
func (e *Explainer) Explain(ctx context.Context, featureID, query string, score float64) (*FeatureExplanation, error) {
	// Check cache
	cacheKey := fmt.Sprintf("%s:%s", featureID, query)
	if cached, ok := e.cache[cacheKey]; ok {
		if time.Since(cached.CachedAt) < e.config.CacheTTL {
			return cached.Explanation, nil
		}
	}

	// Get enriched feature
	enriched, err := e.indexer.GetEnrichedFeature(featureID)
	if err != nil {
		return nil, err
	}

	var explanation *FeatureExplanation

	// Try LLM generation if available
	if e.llmClient != nil && e.llmClient.IsAvailable() {
		explanation, err = e.generateLLMExplanation(ctx, enriched, query, score)
		if err == nil {
			explanation.GeneratedBy = "llm"
		}
	}

	// Fall back to template-based explanation
	if explanation == nil && e.config.FallbackToTemplate {
		explanation = e.generateTemplateExplanation(enriched, query, score)
		explanation.GeneratedBy = "template"
	}

	if explanation == nil {
		return nil, fmt.Errorf("failed to generate explanation")
	}

	// Cache result
	e.cache[cacheKey] = &CachedExplanation{
		Explanation: explanation,
		CachedAt:    time.Now(),
	}

	return explanation, nil
}

// ExplainResults generates explanations for multiple ranked results.
func (e *Explainer) ExplainResults(ctx context.Context, results []RankedResult, query string) ([]*FeatureExplanation, error) {
	explanations := make([]*FeatureExplanation, 0, len(results))

	for _, result := range results {
		explanation, err := e.Explain(ctx, result.Feature.Metadata.FeatureID, query, result.TotalScore)
		if err != nil {
			continue // Skip failed explanations
		}
		explanations = append(explanations, explanation)
	}

	return explanations, nil
}

func (e *Explainer) generateLLMExplanation(ctx context.Context, feature *EnrichedFeature, query string, score float64) (*FeatureExplanation, error) {
	prompt := e.buildExplanationPrompt(feature, query, score)

	response, err := e.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse LLM response into explanation
	explanation := e.parseLLMResponse(response, feature, query, score)
	return explanation, nil
}

func (e *Explainer) buildExplanationPrompt(feature *EnrichedFeature, query string, score float64) string {
	meta := feature.Metadata

	var sb strings.Builder

	sb.WriteString("You are a feature store expert. Explain why the following feature matches a search query.\n\n")
	sb.WriteString(fmt.Sprintf("User Query: \"%s\"\n\n", query))
	sb.WriteString("Feature Information:\n")
	sb.WriteString(fmt.Sprintf("- Name: %s\n", meta.Name))
	sb.WriteString(fmt.Sprintf("- Description: %s\n", meta.Description))
	sb.WriteString(fmt.Sprintf("- Category: %s\n", meta.Category))
	sb.WriteString(fmt.Sprintf("- Entity Type: %s\n", meta.EntityType))
	sb.WriteString(fmt.Sprintf("- Data Type: %s\n", meta.DataType))

	if len(meta.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(meta.Tags, ", ")))
	}

	if e.config.IncludeStatistics && feature.Statistics != nil {
		stats := feature.Statistics
		sb.WriteString("\nStatistics:\n")
		if stats.Mean != 0 {
			sb.WriteString(fmt.Sprintf("- Mean: %.2f, StdDev: %.2f\n", stats.Mean, stats.StdDev))
		}
		if stats.UniqueCount > 0 {
			sb.WriteString(fmt.Sprintf("- Unique Values: %d\n", stats.UniqueCount))
		}
		sb.WriteString(fmt.Sprintf("- Null Percentage: %.1f%%\n", stats.NullPercentage))
	}

	if e.config.IncludeUsage && feature.Usage != nil {
		usage := feature.Usage
		sb.WriteString("\nUsage:\n")
		sb.WriteString(fmt.Sprintf("- Total Reads: %d\n", usage.TotalReads))
		sb.WriteString(fmt.Sprintf("- Models Using: %d\n", usage.ModelCount))
	}

	sb.WriteString(fmt.Sprintf("\nMatch Score: %.2f\n", score))

	sb.WriteString("\nProvide a concise explanation (2-3 sentences) of why this feature is relevant to the query. ")
	sb.WriteString("Focus on semantic relevance and practical usefulness.")

	return sb.String()
}

func (e *Explainer) parseLLMResponse(response string, feature *EnrichedFeature, query string, score float64) *FeatureExplanation {
	// Use LLM response as summary, augment with structured data
	meta := feature.Metadata

	explanation := &FeatureExplanation{
		FeatureID:    meta.FeatureID,
		Query:        query,
		Summary:      strings.TrimSpace(response),
		MatchReasons: e.buildMatchReasons(feature, query, score),
		Relevance: RelevanceAnalysis{
			SemanticMatch:    fmt.Sprintf("The feature '%s' matches the query based on semantic similarity.", meta.Name),
			ContextualFit:    e.analyzeContextualFit(meta, query),
			PotentialUseCase: e.suggestUseCase(meta, query),
		},
		Confidence:  score,
		GeneratedAt: time.Now(),
	}

	if e.config.IncludeStatistics && feature.Statistics != nil {
		explanation.DataInsights = e.buildDataInsights(feature.Statistics, meta)
	}

	if e.config.IncludeUsage && feature.Usage != nil {
		explanation.UsageContext = e.buildUsageContext(feature.Usage, meta)
	}

	explanation.Recommendations = e.generateRecommendations(feature, query)

	return explanation
}

func (e *Explainer) generateTemplateExplanation(feature *EnrichedFeature, query string, score float64) *FeatureExplanation {
	meta := feature.Metadata

	// Generate template-based summary
	summary := e.generateTemplateSummary(meta, query, score)

	explanation := &FeatureExplanation{
		FeatureID:    meta.FeatureID,
		Query:        query,
		Summary:      summary,
		MatchReasons: e.buildMatchReasons(feature, query, score),
		Relevance: RelevanceAnalysis{
			SemanticMatch:    e.analyzeSemanticMatch(meta, query),
			ContextualFit:    e.analyzeContextualFit(meta, query),
			PotentialUseCase: e.suggestUseCase(meta, query),
		},
		Confidence:  score,
		GeneratedAt: time.Now(),
	}

	if e.config.IncludeStatistics && feature.Statistics != nil {
		explanation.DataInsights = e.buildDataInsights(feature.Statistics, meta)
	}

	if e.config.IncludeUsage && feature.Usage != nil {
		explanation.UsageContext = e.buildUsageContext(feature.Usage, meta)
	}

	explanation.Recommendations = e.generateRecommendations(feature, query)

	return explanation
}

func (e *Explainer) generateTemplateSummary(meta *FeatureMetadata, query string, score float64) string {
	var parts []string

	// Base match statement
	parts = append(parts, fmt.Sprintf("'%s' matches your query '%s'", meta.Name, query))

	// Add description context
	if meta.Description != "" {
		parts = append(parts, fmt.Sprintf("This feature represents %s", strings.ToLower(meta.Description)))
	}

	// Add category context
	if meta.Category != "" {
		parts = append(parts, fmt.Sprintf("It belongs to the '%s' category", meta.Category))
	}

	// Add quality context
	if meta.QualityScore >= 0.8 {
		parts = append(parts, "It has high data quality")
	}

	// Add popularity context
	if meta.DataQuality == "high" {
		parts = append(parts, "and is well-documented")
	}

	return strings.Join(parts, ". ") + "."
}

func (e *Explainer) buildMatchReasons(feature *EnrichedFeature, query string, score float64) []MatchReason {
	meta := feature.Metadata
	var reasons []MatchReason

	// Semantic match
	reasons = append(reasons, MatchReason{
		Type:        "semantic",
		Description: fmt.Sprintf("Semantic similarity with query: %.1f%%", score*100),
		Score:       score,
	})

	// Check for exact matches
	queryLower := strings.ToLower(query)
	nameLower := strings.ToLower(meta.Name)

	if strings.Contains(nameLower, queryLower) || strings.Contains(queryLower, nameLower) {
		reasons = append(reasons, MatchReason{
			Type:        "exact_match",
			Description: "Feature name closely matches query terms",
			Score:       1.0,
		})
	}

	// Check for tag matches
	for _, tag := range meta.Tags {
		if strings.Contains(queryLower, strings.ToLower(tag)) {
			reasons = append(reasons, MatchReason{
				Type:        "tag",
				Description: fmt.Sprintf("Query matches tag: '%s'", tag),
				Score:       0.8,
			})
			break
		}
	}

	// Check category relevance
	if meta.Category != "" && strings.Contains(queryLower, strings.ToLower(meta.Category)) {
		reasons = append(reasons, MatchReason{
			Type:        "category",
			Description: fmt.Sprintf("Query relates to category: '%s'", meta.Category),
			Score:       0.7,
		})
	}

	// Check domain relevance
	if meta.Domain != "" && strings.Contains(queryLower, strings.ToLower(meta.Domain)) {
		reasons = append(reasons, MatchReason{
			Type:        "metadata",
			Description: fmt.Sprintf("Query relates to domain: '%s'", meta.Domain),
			Score:       0.7,
		})
	}

	// Sort by score
	sort.Slice(reasons, func(i, j int) bool {
		return reasons[i].Score > reasons[j].Score
	})

	return reasons
}

func (e *Explainer) analyzeSemanticMatch(meta *FeatureMetadata, query string) string {
	queryTerms := strings.Fields(strings.ToLower(query))

	var matchingConcepts []string

	// Check description for matching concepts
	descLower := strings.ToLower(meta.Description)
	for _, term := range queryTerms {
		if len(term) > 3 && strings.Contains(descLower, term) {
			matchingConcepts = append(matchingConcepts, term)
		}
	}

	if len(matchingConcepts) > 0 {
		return fmt.Sprintf("The feature description contains related concepts: %s", strings.Join(matchingConcepts, ", "))
	}

	return "The feature is semantically related to your query based on its description and metadata"
}

func (e *Explainer) analyzeContextualFit(meta *FeatureMetadata, query string) string {
	var contexts []string

	if meta.EntityType != "" {
		contexts = append(contexts, fmt.Sprintf("associated with %s entities", meta.EntityType))
	}

	if meta.Domain != "" {
		contexts = append(contexts, fmt.Sprintf("relevant to the %s domain", meta.Domain))
	}

	if len(meta.UseCase) > 0 {
		contexts = append(contexts, fmt.Sprintf("commonly used for %s", strings.Join(meta.UseCase, ", ")))
	}

	if len(contexts) > 0 {
		return "This feature is " + strings.Join(contexts, " and ")
	}

	return "This feature may fit various contexts based on its generic nature"
}

func (e *Explainer) suggestUseCase(meta *FeatureMetadata, query string) string {
	// Infer use case from metadata
	if len(meta.UseCase) > 0 {
		return fmt.Sprintf("This feature is designed for: %s", strings.Join(meta.UseCase, ", "))
	}

	// Infer from data type
	switch meta.ValueType {
	case "numeric":
		return "Can be used for aggregations, comparisons, and numerical analysis"
	case "categorical":
		return "Useful for grouping, filtering, and categorical analysis"
	case "vector":
		return "Suitable for similarity search and embedding-based operations"
	case "boolean":
		return "Ideal for filtering and binary classification tasks"
	}

	return "This feature can support various ML and analytics use cases"
}

func (e *Explainer) buildDataInsights(stats *FeatureStatistics, meta *FeatureMetadata) *DataInsights {
	insights := &DataInsights{}

	// Distribution insight
	if stats.Distribution != "" {
		insights.Distribution = fmt.Sprintf("Data follows a %s distribution", stats.Distribution)
	}

	// Value range
	if stats.Min != 0 || stats.Max != 0 {
		insights.ValueRange = fmt.Sprintf("Values range from %.2f to %.2f", stats.Min, stats.Max)
	}

	// Quality summary
	if stats.NullPercentage < 1 {
		insights.QualitySummary = "Very high completeness (>99% non-null values)"
	} else if stats.NullPercentage < 5 {
		insights.QualitySummary = "Good completeness (>95% non-null values)"
	} else if stats.NullPercentage < 20 {
		insights.QualitySummary = "Moderate completeness (some missing values)"
	} else {
		insights.QualitySummary = "Consider data quality - significant missing values"
	}

	// Freshness
	switch meta.Freshness {
	case "real-time":
		insights.FreshnessSummary = "Data is updated in real-time"
	case "hourly":
		insights.FreshnessSummary = "Data is refreshed hourly"
	case "daily":
		insights.FreshnessSummary = "Data is refreshed daily"
	default:
		insights.FreshnessSummary = "Check freshness requirements for your use case"
	}

	return insights
}

func (e *Explainer) buildUsageContext(usage *FeatureUsage, meta *FeatureMetadata) *UsageContext {
	ctx := &UsageContext{}

	// Popularity assessment
	if usage.TotalReads > 100000 {
		ctx.Popularity = "Highly popular - frequently accessed"
	} else if usage.TotalReads > 10000 {
		ctx.Popularity = "Popular - regularly accessed"
	} else if usage.TotalReads > 1000 {
		ctx.Popularity = "Moderately used"
	} else {
		ctx.Popularity = "Less commonly used - consider validating fit"
	}

	// Models using
	if len(usage.ModelsUsing) > 0 {
		ctx.ModelsUsing = usage.ModelsUsing
	}

	// Common use cases
	if len(meta.UseCase) > 0 {
		ctx.CommonUseCases = meta.UseCase
	}

	// Related features
	if len(meta.RelatedFeatures) > 0 {
		ctx.RelatedFeatures = meta.RelatedFeatures
	}

	return ctx
}

func (e *Explainer) generateRecommendations(feature *EnrichedFeature, query string) []string {
	var recommendations []string
	meta := feature.Metadata

	// Quality-based recommendations
	if meta.QualityScore < 0.5 {
		recommendations = append(recommendations, "Consider data quality - verify this feature meets your requirements")
	}

	// Freshness recommendations
	if meta.Freshness == "weekly" || meta.Freshness == "" {
		recommendations = append(recommendations, "Check if data freshness meets your latency requirements")
	}

	// Documentation recommendations
	if meta.Documentation == "" {
		recommendations = append(recommendations, "Review with feature owner as documentation is limited")
	}

	// Related features
	if len(meta.RelatedFeatures) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Consider related features: %s", strings.Join(meta.RelatedFeatures[:minInt(3, len(meta.RelatedFeatures))], ", ")))
	}

	// Usage recommendations
	if feature.Usage != nil && feature.Usage.TotalReads < 100 {
		recommendations = append(recommendations, "Low usage - validate with peers before adopting")
	}

	return recommendations
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TemplateExplainer provides simple template-based explanations without LLM.
type TemplateExplainer struct {
	indexer *EnhancedIndexer
}

// NewTemplateExplainer creates a template-only explainer.
func NewTemplateExplainer(indexer *EnhancedIndexer) *TemplateExplainer {
	return &TemplateExplainer{indexer: indexer}
}

// Explain generates a template-based explanation.
func (t *TemplateExplainer) Explain(featureID, query string, score float64) (*FeatureExplanation, error) {
	enriched, err := t.indexer.GetEnrichedFeature(featureID)
	if err != nil {
		return nil, err
	}

	e := &Explainer{
		indexer: t.indexer,
		config:  DefaultExplainerConfig(),
	}

	explanation := e.generateTemplateExplanation(enriched, query, score)
	explanation.GeneratedBy = "template"

	return explanation, nil
}
