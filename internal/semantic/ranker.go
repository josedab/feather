package semantic

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

// HybridRanker combines vector similarity with metadata-based ranking.
type HybridRanker struct {
	indexer *EnhancedIndexer
	config  RankerConfig
}

// RankerConfig configures the hybrid ranking algorithm.
type RankerConfig struct {
	// Weight distribution (should sum to 1.0)
	SemanticWeight   float64 `json:"semantic_weight"`   // Weight for vector similarity
	MetadataWeight   float64 `json:"metadata_weight"`   // Weight for metadata matching
	PopularityWeight float64 `json:"popularity_weight"` // Weight for usage popularity
	QualityWeight    float64 `json:"quality_weight"`    // Weight for data quality
	FreshnessWeight  float64 `json:"freshness_weight"`  // Weight for data freshness
	LineageWeight    float64 `json:"lineage_weight"`    // Weight for lineage relevance

	// Boosting factors
	ExactMatchBoost    float64 `json:"exact_match_boost"`    // Boost for exact name/tag matches
	CategoryMatchBoost float64 `json:"category_match_boost"` // Boost for category matches
	DomainMatchBoost   float64 `json:"domain_match_boost"`   // Boost for domain matches
	UseCaseMatchBoost  float64 `json:"use_case_match_boost"` // Boost for use case matches
	OwnerMatchBoost    float64 `json:"owner_match_boost"`    // Boost for owner matches

	// Penalty factors
	StalePenalty      float64 `json:"stale_penalty"`       // Penalty for stale features
	LowQualityPenalty float64 `json:"low_quality_penalty"` // Penalty for low quality
	DeprecatedPenalty float64 `json:"deprecated_penalty"`  // Penalty for deprecated features

	// Thresholds
	MinSemanticScore float64       `json:"min_semantic_score"` // Minimum semantic similarity
	MaxResultAge     time.Duration `json:"max_result_age"`     // Maximum age for freshness
}

// DefaultRankerConfig returns sensible default configuration.
func DefaultRankerConfig() RankerConfig {
	return RankerConfig{
		SemanticWeight:   0.40,
		MetadataWeight:   0.25,
		PopularityWeight: 0.15,
		QualityWeight:    0.10,
		FreshnessWeight:  0.05,
		LineageWeight:    0.05,

		ExactMatchBoost:    1.5,
		CategoryMatchBoost: 1.2,
		DomainMatchBoost:   1.3,
		UseCaseMatchBoost:  1.4,
		OwnerMatchBoost:    1.1,

		StalePenalty:      0.8,
		LowQualityPenalty: 0.7,
		DeprecatedPenalty: 0.5,

		MinSemanticScore: 0.3,
		MaxResultAge:     30 * 24 * time.Hour, // 30 days
	}
}

// NewHybridRanker creates a new hybrid ranker.
func NewHybridRanker(indexer *EnhancedIndexer, config RankerConfig) *HybridRanker {
	return &HybridRanker{
		indexer: indexer,
		config:  config,
	}
}

// RankRequest represents a ranking request with query context.
type RankRequest struct {
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

// RankedResult represents a ranked search result with score breakdown.
type RankedResult struct {
	Feature         *EnrichedFeature `json:"feature"`
	TotalScore      float64          `json:"total_score"`
	SemanticScore   float64          `json:"semantic_score"`
	MetadataScore   float64          `json:"metadata_score"`
	PopularityScore float64          `json:"popularity_score"`
	QualityScore    float64          `json:"quality_score"`
	FreshnessScore  float64          `json:"freshness_score"`
	LineageScore    float64          `json:"lineage_score"`
	BoostFactors    []string         `json:"boost_factors,omitempty"`
	Penalties       []string         `json:"penalties,omitempty"`
}

// Rank performs hybrid ranking on search results.
func (r *HybridRanker) Rank(ctx context.Context, req RankRequest) ([]RankedResult, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// First, get semantic search results
	searchOpts := SearchOptions{
		Limit:      req.Limit * 3, // Get more for filtering
		MinScore:   float32(r.config.MinSemanticScore),
		Categories: req.Categories,
		Tags:       req.Tags,
		Owner:      req.Owner,
	}

	semanticResults, err := r.indexer.Search().Search(ctx, req.Query, searchOpts)
	if err != nil {
		return nil, err
	}

	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, id := range req.ExcludeFeatures {
		excludeSet[id] = true
	}

	// Score and rank each result
	var rankedResults []RankedResult

	for _, sr := range semanticResults {
		if excludeSet[sr.Feature.ID] {
			continue
		}

		// Get enriched feature
		enriched, err := r.indexer.GetEnrichedFeature(sr.Feature.ID)
		if err != nil {
			continue
		}

		// Apply filters
		if !r.matchesFilters(enriched.Metadata, req) {
			continue
		}

		// Calculate scores
		result := r.calculateScores(enriched, req, float64(sr.Score))
		rankedResults = append(rankedResults, result)
	}

	// Sort by total score
	sort.Slice(rankedResults, func(i, j int) bool {
		return rankedResults[i].TotalScore > rankedResults[j].TotalScore
	})

	// Limit results
	if len(rankedResults) > req.Limit {
		rankedResults = rankedResults[:req.Limit]
	}

	return rankedResults, nil
}

func (r *HybridRanker) matchesFilters(meta *FeatureMetadata, req RankRequest) bool {
	// Entity type filter
	if len(req.EntityTypes) > 0 {
		found := false
		for _, et := range req.EntityTypes {
			if strings.EqualFold(meta.EntityType, et) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Domain filter
	if len(req.Domains) > 0 {
		found := false
		for _, d := range req.Domains {
			if strings.EqualFold(meta.Domain, d) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Use case filter
	if len(req.UseCases) > 0 {
		found := false
		for _, uc := range req.UseCases {
			for _, muc := range meta.UseCase {
				if strings.EqualFold(muc, uc) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Team filter
	if req.Team != "" && !strings.EqualFold(meta.Team, req.Team) {
		return false
	}

	// Quality filter
	if req.MinQuality > 0 && meta.QualityScore < req.MinQuality {
		return false
	}

	// Freshness filter
	if req.OnlyFresh {
		if meta.Freshness != "real-time" && meta.Freshness != "hourly" {
			return false
		}
	}

	return true
}

func (r *HybridRanker) calculateScores(feature *EnrichedFeature, req RankRequest, semanticScore float64) RankedResult {
	result := RankedResult{
		Feature:       feature,
		SemanticScore: semanticScore,
	}

	meta := feature.Metadata

	// Calculate metadata score
	result.MetadataScore = r.calculateMetadataScore(meta, req)

	// Calculate popularity score
	result.PopularityScore = r.calculatePopularityScore(feature.Usage)

	// Calculate quality score
	result.QualityScore = r.calculateQualityScore(meta)

	// Calculate freshness score
	result.FreshnessScore = r.calculateFreshnessScore(meta)

	// Calculate lineage score
	result.LineageScore = r.calculateLineageScore(feature.Lineage, req)

	// Apply boosts
	boostMultiplier := 1.0
	var boostFactors []string

	// Exact name match boost
	if strings.Contains(strings.ToLower(meta.Name), strings.ToLower(req.Query)) {
		boostMultiplier *= r.config.ExactMatchBoost
		boostFactors = append(boostFactors, "exact_name_match")
	}

	// Tag match boost
	for _, tag := range meta.Tags {
		if strings.Contains(strings.ToLower(req.Query), strings.ToLower(tag)) {
			boostMultiplier *= 1.1
			boostFactors = append(boostFactors, "tag_match:"+tag)
			break
		}
	}

	// Category match boost
	if len(req.Categories) > 0 {
		for _, cat := range req.Categories {
			if strings.EqualFold(meta.Category, cat) {
				boostMultiplier *= r.config.CategoryMatchBoost
				boostFactors = append(boostFactors, "category_match")
				break
			}
		}
	}

	// Domain match boost
	if len(req.Domains) > 0 {
		for _, domain := range req.Domains {
			if strings.EqualFold(meta.Domain, domain) {
				boostMultiplier *= r.config.DomainMatchBoost
				boostFactors = append(boostFactors, "domain_match")
				break
			}
		}
	}

	// Use case match boost
	if len(req.UseCases) > 0 {
		for _, uc := range req.UseCases {
			for _, muc := range meta.UseCase {
				if strings.EqualFold(muc, uc) {
					boostMultiplier *= r.config.UseCaseMatchBoost
					boostFactors = append(boostFactors, "use_case_match")
					break
				}
			}
		}
	}

	result.BoostFactors = boostFactors

	// Apply penalties
	penaltyMultiplier := 1.0
	var penalties []string

	// Stale penalty
	if time.Since(meta.UpdatedAt) > r.config.MaxResultAge {
		penaltyMultiplier *= r.config.StalePenalty
		penalties = append(penalties, "stale")
	}

	// Low quality penalty
	if meta.QualityScore < 0.5 {
		penaltyMultiplier *= r.config.LowQualityPenalty
		penalties = append(penalties, "low_quality")
	}

	// Check for deprecated tag
	for _, tag := range meta.Tags {
		if strings.EqualFold(tag, "deprecated") {
			penaltyMultiplier *= r.config.DeprecatedPenalty
			penalties = append(penalties, "deprecated")
			break
		}
	}

	result.Penalties = penalties

	// Calculate total score
	baseScore := r.config.SemanticWeight*result.SemanticScore +
		r.config.MetadataWeight*result.MetadataScore +
		r.config.PopularityWeight*result.PopularityScore +
		r.config.QualityWeight*result.QualityScore +
		r.config.FreshnessWeight*result.FreshnessScore +
		r.config.LineageWeight*result.LineageScore

	result.TotalScore = baseScore * boostMultiplier * penaltyMultiplier

	return result
}

func (r *HybridRanker) calculateMetadataScore(meta *FeatureMetadata, req RankRequest) float64 {
	var score float64
	var factors int

	// Documentation completeness
	if meta.Description != "" {
		score += 0.3
	}
	if meta.Documentation != "" {
		score += 0.2
	}
	if len(meta.Examples) > 0 {
		score += 0.1
	}
	factors += 3

	// Tags and labels
	if len(meta.Tags) > 0 {
		score += math.Min(float64(len(meta.Tags))*0.05, 0.2)
	}
	factors++

	// Owner/team info
	if meta.Owner != "" {
		score += 0.1
	}
	if meta.Team != "" {
		score += 0.1
	}
	factors += 2

	return score / float64(factors) * float64(factors) // Normalize to 0-1
}

func (r *HybridRanker) calculatePopularityScore(usage *FeatureUsage) float64 {
	if usage == nil {
		return 0.5 // Neutral for unknown
	}

	// Use popularity score if available
	if usage.PopularityScore > 0 {
		return usage.PopularityScore
	}

	// Calculate from raw metrics
	var score float64

	// Read count factor (log scale)
	if usage.TotalReads > 0 {
		score += math.Min(math.Log10(float64(usage.TotalReads))/6, 0.5) // Cap at 1M reads
	}

	// Model count factor
	if usage.ModelCount > 0 {
		score += math.Min(float64(usage.ModelCount)*0.1, 0.3)
	}

	// Recency factor
	if !usage.LastReadAt.IsZero() {
		daysSinceRead := time.Since(usage.LastReadAt).Hours() / 24
		if daysSinceRead < 7 {
			score += 0.2
		} else if daysSinceRead < 30 {
			score += 0.1
		}
	}

	return math.Min(score, 1.0)
}

func (r *HybridRanker) calculateQualityScore(meta *FeatureMetadata) float64 {
	// Use stored quality score if available
	if meta.QualityScore > 0 {
		return float64(meta.QualityScore)
	}

	var score float64

	// Completeness
	if meta.Completeness > 0 {
		score += float64(meta.Completeness) * 0.4
	} else {
		score += 0.2 // Assume partial for unknown
	}

	// Data quality label
	switch meta.DataQuality {
	case "high":
		score += 0.4
	case "medium":
		score += 0.25
	case "low":
		score += 0.1
	default:
		score += 0.2 // Unknown
	}

	// Documentation factor
	if meta.Description != "" {
		score += 0.1
	}
	if meta.Documentation != "" {
		score += 0.1
	}

	return math.Min(score, 1.0)
}

func (r *HybridRanker) calculateFreshnessScore(meta *FeatureMetadata) float64 {
	// Check freshness label
	switch meta.Freshness {
	case "real-time":
		return 1.0
	case "hourly":
		return 0.9
	case "daily":
		return 0.7
	case "weekly":
		return 0.5
	}

	// Fallback to last update time
	if meta.UpdatedAt.IsZero() {
		return 0.5
	}

	daysSinceUpdate := time.Since(meta.UpdatedAt).Hours() / 24

	if daysSinceUpdate < 1 {
		return 1.0
	} else if daysSinceUpdate < 7 {
		return 0.9
	} else if daysSinceUpdate < 30 {
		return 0.7
	} else if daysSinceUpdate < 90 {
		return 0.5
	}

	return 0.3
}

func (r *HybridRanker) calculateLineageScore(lineage *FeatureLineage, req RankRequest) float64 {
	if lineage == nil {
		return 0.5 // Neutral for unknown
	}

	var score float64

	// Has clear source
	if len(lineage.SourceFeatures) > 0 || len(lineage.SourceTables) > 0 {
		score += 0.3
	}

	// Has dependents (indicates importance)
	if len(lineage.DependentFeatures) > 0 || len(lineage.DependentModels) > 0 {
		score += 0.3
	}

	// Has transformation logic documented
	if lineage.TransformationCode != "" || lineage.TransformationType != "" {
		score += 0.2
	}

	// Recent pipeline run
	if !lineage.LastRun.IsZero() {
		if time.Since(lineage.LastRun) < 24*time.Hour {
			score += 0.2
		}
	}

	return math.Min(score, 1.0)
}

// SuggestSimilar suggests similar features to a given feature using hybrid ranking.
func (r *HybridRanker) SuggestSimilar(ctx context.Context, featureID string, limit int) ([]RankedResult, error) {
	if limit <= 0 {
		limit = 5
	}

	// Get source feature
	source, err := r.indexer.GetEnrichedFeature(featureID)
	if err != nil {
		return nil, err
	}

	// Build a query from source feature
	queryParts := []string{source.Metadata.Name}
	if source.Metadata.Description != "" {
		queryParts = append(queryParts, source.Metadata.Description)
	}
	query := strings.Join(queryParts, " ")

	// Search with context from source feature
	req := RankRequest{
		Query:           query,
		Categories:      []string{source.Metadata.Category},
		EntityTypes:     []string{source.Metadata.EntityType},
		Domains:         []string{source.Metadata.Domain},
		ExcludeFeatures: []string{featureID},
		Limit:           limit,
	}

	return r.Rank(ctx, req)
}

// RecommendForModel recommends features for a model based on existing features.
func (r *HybridRanker) RecommendForModel(ctx context.Context, existingFeatures []string, modelUseCase string, limit int) ([]RankedResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Collect patterns from existing features
	var categories []string
	var domains []string
	var entityTypes []string
	var queryParts []string

	categorySet := make(map[string]bool)
	domainSet := make(map[string]bool)
	entityTypeSet := make(map[string]bool)

	for _, fid := range existingFeatures {
		enriched, err := r.indexer.GetEnrichedFeature(fid)
		if err != nil {
			continue
		}

		meta := enriched.Metadata
		if meta.Category != "" && !categorySet[meta.Category] {
			categories = append(categories, meta.Category)
			categorySet[meta.Category] = true
		}
		if meta.Domain != "" && !domainSet[meta.Domain] {
			domains = append(domains, meta.Domain)
			domainSet[meta.Domain] = true
		}
		if meta.EntityType != "" && !entityTypeSet[meta.EntityType] {
			entityTypes = append(entityTypes, meta.EntityType)
			entityTypeSet[meta.EntityType] = true
		}

		if meta.Description != "" {
			queryParts = append(queryParts, meta.Description)
		}
	}

	// Build query
	query := modelUseCase
	if len(queryParts) > 0 {
		// Add context from existing features
		query += " " + strings.Join(queryParts[:min(len(queryParts), 3)], " ")
	}

	req := RankRequest{
		Query:           query,
		Categories:      categories,
		Domains:         domains,
		EntityTypes:     entityTypes,
		UseCases:        []string{modelUseCase},
		ExcludeFeatures: existingFeatures,
		Limit:           limit,
	}

	return r.Rank(ctx, req)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
