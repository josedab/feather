package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// RecommendationEngine provides AI-powered feature recommendations.
type RecommendationEngine struct {
	mu sync.RWMutex

	// Core components
	discovery *FeatureDiscovery
	indexer   *EnhancedIndexer

	// Recommendation models
	collaborativeFilter *CollaborativeFilter
	contentBasedFilter  *ContentBasedFilter
	popularityModel     *PopularityModel
	contextModel        *ContextModel

	// User interaction history
	userInteractions map[string]*UserInteractionHistory

	// Model usage tracking
	modelFeatures map[string]*ModelFeatureUsage

	// Configuration
	config RecommendationConfig

	// Logging
	logger *slog.Logger
}

// RecommendationConfig configures the recommendation engine.
type RecommendationConfig struct {
	// Algorithm weights
	CollaborativeWeight float64 `json:"collaborative_weight"`
	ContentBasedWeight  float64 `json:"content_based_weight"`
	PopularityWeight    float64 `json:"popularity_weight"`
	ContextWeight       float64 `json:"context_weight"`

	// Collaborative filtering settings
	MinSimilarUsers         int     `json:"min_similar_users"`
	UserSimilarityThreshold float64 `json:"user_similarity_threshold"`

	// Content-based settings
	MinContentSimilarity float64 `json:"min_content_similarity"`

	// Popularity settings
	PopularityDecayDays int     `json:"popularity_decay_days"`
	MinPopularityScore  float64 `json:"min_popularity_score"`

	// Context settings
	UseContextBoost    bool    `json:"use_context_boost"`
	ContextBoostFactor float64 `json:"context_boost_factor"`

	// Diversity settings
	DiversityFactor      float64 `json:"diversity_factor"`
	MaxSameCategoryRatio float64 `json:"max_same_category_ratio"`

	// Cold start handling
	ColdStartStrategy string `json:"cold_start_strategy"` // "popularity", "content", "random"

	// General settings
	DefaultLimit int           `json:"default_limit"`
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
}

// DefaultRecommendationConfig returns sensible defaults.
func DefaultRecommendationConfig() RecommendationConfig {
	return RecommendationConfig{
		CollaborativeWeight:     0.35,
		ContentBasedWeight:      0.30,
		PopularityWeight:        0.20,
		ContextWeight:           0.15,
		MinSimilarUsers:         3,
		UserSimilarityThreshold: 0.3,
		MinContentSimilarity:    0.4,
		PopularityDecayDays:     30,
		MinPopularityScore:      0.1,
		UseContextBoost:         true,
		ContextBoostFactor:      1.3,
		DiversityFactor:         0.2,
		MaxSameCategoryRatio:    0.5,
		ColdStartStrategy:       "popularity",
		DefaultLimit:            10,
		CacheEnabled:            true,
		CacheTTL:                10 * time.Minute,
	}
}

// UserInteractionHistory tracks user interactions with features.
type UserInteractionHistory struct {
	UserID              string                         `json:"user_id"`
	ViewedFeatures      map[string]*FeatureInteraction `json:"viewed_features"`
	FavoriteFeatures    []string                       `json:"favorite_features"`
	UsedFeatures        map[string]time.Time           `json:"used_features"`
	SearchQueries       []string                       `json:"search_queries,omitempty"`
	PreferredCategories map[string]float64             `json:"preferred_categories"`
	PreferredDomains    map[string]float64             `json:"preferred_domains"`
	LastActivity        time.Time                      `json:"last_activity"`
	InteractionCount    int                            `json:"interaction_count"`
}

// FeatureInteraction represents an interaction with a feature.
type FeatureInteraction struct {
	FeatureID  string    `json:"feature_id"`
	ViewCount  int       `json:"view_count"`
	LastViewed time.Time `json:"last_viewed"`
	IsFavorite bool      `json:"is_favorite"`
	IsUsed     bool      `json:"is_used"`
	Rating     float64   `json:"rating,omitempty"` // Implicit rating based on interaction
}

// ModelFeatureUsage tracks which features are used by which models.
type ModelFeatureUsage struct {
	ModelID     string    `json:"model_id"`
	ModelName   string    `json:"model_name"`
	UseCase     string    `json:"use_case"`
	Features    []string  `json:"features"`
	Performance float64   `json:"performance,omitempty"`
	LastUpdated time.Time `json:"last_updated"`
}

// CollaborativeFilter implements collaborative filtering.
type CollaborativeFilter struct {
	userSimilarities map[string]map[string]float64 // user -> similar users with scores
	lastComputed     time.Time
}

// ContentBasedFilter implements content-based filtering.
type ContentBasedFilter struct {
	featureVectors map[string][]float32
}

// PopularityModel tracks feature popularity.
type PopularityModel struct {
	mu sync.RWMutex

	viewCounts  map[string]int64
	usageCounts map[string]int64
	trendScores map[string]float64
	recentViews map[string][]time.Time
}

// ContextModel provides context-aware recommendations.
type ContextModel struct {
	// Context patterns
	useCaseFeatures  map[string][]string
	domainFeatures   map[string][]string
	categoryFeatures map[string][]string
	entityFeatures   map[string][]string
}

// NewRecommendationEngine creates a new recommendation engine.
func NewRecommendationEngine(
	discovery *FeatureDiscovery,
	config RecommendationConfig,
	logger *slog.Logger,
) (*RecommendationEngine, error) {
	if discovery == nil {
		return nil, fmt.Errorf("discovery engine is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	engine := &RecommendationEngine{
		discovery:           discovery,
		indexer:             discovery.indexer,
		collaborativeFilter: newCollaborativeFilter(),
		contentBasedFilter:  newContentBasedFilter(),
		popularityModel:     newPopularityModel(),
		contextModel:        newContextModel(),
		userInteractions:    make(map[string]*UserInteractionHistory),
		modelFeatures:       make(map[string]*ModelFeatureUsage),
		config:              config,
		logger:              logger,
	}

	// Initialize context model from indexed features
	engine.initializeContextModel()

	return engine, nil
}

func newCollaborativeFilter() *CollaborativeFilter {
	return &CollaborativeFilter{
		userSimilarities: make(map[string]map[string]float64),
	}
}

func newContentBasedFilter() *ContentBasedFilter {
	return &ContentBasedFilter{
		featureVectors: make(map[string][]float32),
	}
}

func newPopularityModel() *PopularityModel {
	return &PopularityModel{
		viewCounts:  make(map[string]int64),
		usageCounts: make(map[string]int64),
		trendScores: make(map[string]float64),
		recentViews: make(map[string][]time.Time),
	}
}

func newContextModel() *ContextModel {
	return &ContextModel{
		useCaseFeatures:  make(map[string][]string),
		domainFeatures:   make(map[string][]string),
		categoryFeatures: make(map[string][]string),
		entityFeatures:   make(map[string][]string),
	}
}

func (e *RecommendationEngine) initializeContextModel() {
	allMetadata := e.indexer.ListMetadata()

	for _, meta := range allMetadata {
		// Index by use case
		for _, uc := range meta.UseCase {
			e.contextModel.useCaseFeatures[uc] = append(e.contextModel.useCaseFeatures[uc], meta.FeatureID)
		}

		// Index by domain
		if meta.Domain != "" {
			e.contextModel.domainFeatures[meta.Domain] = append(e.contextModel.domainFeatures[meta.Domain], meta.FeatureID)
		}

		// Index by category
		if meta.Category != "" {
			e.contextModel.categoryFeatures[meta.Category] = append(e.contextModel.categoryFeatures[meta.Category], meta.FeatureID)
		}

		// Index by entity type
		if meta.EntityType != "" {
			e.contextModel.entityFeatures[meta.EntityType] = append(e.contextModel.entityFeatures[meta.EntityType], meta.FeatureID)
		}
	}
}

// RecommendationRequest specifies a recommendation request.
type RecommendationRequest struct {
	// User context
	UserID string `json:"user_id,omitempty"`

	// Recommendation context
	UseCase         string   `json:"use_case,omitempty"`
	Domain          string   `json:"domain,omitempty"`
	EntityType      string   `json:"entity_type,omitempty"`
	Category        string   `json:"category,omitempty"`
	CurrentFeatures []string `json:"current_features,omitempty"` // Features already used

	// Filters
	ExcludeFeatures []string `json:"exclude_features,omitempty"`
	MinQuality      float32  `json:"min_quality,omitempty"`
	OnlyFresh       bool     `json:"only_fresh,omitempty"`
	Tags            []string `json:"tags,omitempty"`

	// Algorithm preferences
	Strategy        string  `json:"strategy,omitempty"` // "balanced", "collaborative", "content", "popularity", "context"
	DiversityFactor float64 `json:"diversity_factor,omitempty"`

	// Output
	Limit          int  `json:"limit,omitempty"`
	IncludeScores  bool `json:"include_scores,omitempty"`
	IncludeReasons bool `json:"include_reasons,omitempty"`
}

// Recommendation represents a single recommendation.
type Recommendation struct {
	Feature        *EnrichedFeature      `json:"feature"`
	Score          float64               `json:"score"`
	ScoreBreakdown *RecommendationScores `json:"score_breakdown,omitempty"`
	Reasons        []string              `json:"reasons,omitempty"`
	Confidence     float64               `json:"confidence"`
}

// RecommendationScores breaks down the recommendation score.
type RecommendationScores struct {
	CollaborativeScore float64 `json:"collaborative_score"`
	ContentScore       float64 `json:"content_score"`
	PopularityScore    float64 `json:"popularity_score"`
	ContextScore       float64 `json:"context_score"`
	QualityScore       float64 `json:"quality_score"`
	DiversityPenalty   float64 `json:"diversity_penalty"`
	FinalScore         float64 `json:"final_score"`
}

// RecommendationResult contains the recommendation results.
type RecommendationResult struct {
	Recommendations []Recommendation `json:"recommendations"`
	TotalCandidates int              `json:"total_candidates"`
	Strategy        string           `json:"strategy"`
	ResponseTime    int64            `json:"response_time_ms"`
	Timestamp       time.Time        `json:"timestamp"`
}

// Recommend generates feature recommendations.
func (e *RecommendationEngine) Recommend(ctx context.Context, req RecommendationRequest) (*RecommendationResult, error) {
	startTime := time.Now()

	// Apply defaults
	if req.Limit <= 0 {
		req.Limit = e.config.DefaultLimit
	}
	if req.Strategy == "" {
		req.Strategy = "balanced"
	}
	if req.DiversityFactor == 0 {
		req.DiversityFactor = e.config.DiversityFactor
	}

	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, id := range req.ExcludeFeatures {
		excludeSet[id] = true
	}
	for _, id := range req.CurrentFeatures {
		excludeSet[id] = true
	}

	// Get candidate features
	candidates := e.getCandidates(ctx, req, excludeSet)

	// Score candidates
	scored := e.scoreRecommendations(ctx, candidates, req)

	// Apply diversity
	diverse := e.applyDiversity(scored, req)

	// Build result
	result := &RecommendationResult{
		Recommendations: diverse[:min(len(diverse), req.Limit)],
		TotalCandidates: len(candidates),
		Strategy:        req.Strategy,
		ResponseTime:    time.Since(startTime).Milliseconds(),
		Timestamp:       time.Now(),
	}

	return result, nil
}

func (e *RecommendationEngine) getCandidates(ctx context.Context, req RecommendationRequest, excludeSet map[string]bool) []*EnrichedFeature {
	allMetadata := e.indexer.ListMetadata()
	candidates := make([]*EnrichedFeature, 0, len(allMetadata))

	for _, meta := range allMetadata {
		if excludeSet[meta.FeatureID] {
			continue
		}

		// Apply filters
		if req.MinQuality > 0 && meta.QualityScore < req.MinQuality {
			continue
		}

		if req.OnlyFresh && meta.Freshness != "real-time" && meta.Freshness != "hourly" {
			continue
		}

		if req.Domain != "" && !strings.EqualFold(meta.Domain, req.Domain) {
			continue
		}

		if req.EntityType != "" && !strings.EqualFold(meta.EntityType, req.EntityType) {
			continue
		}

		if req.Category != "" && !strings.EqualFold(meta.Category, req.Category) {
			continue
		}

		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range req.Tags {
				for _, metaTag := range meta.Tags {
					if strings.EqualFold(tag, metaTag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// Get enriched feature
		enriched, err := e.indexer.GetEnrichedFeature(meta.FeatureID)
		if err != nil {
			continue
		}

		candidates = append(candidates, enriched)
	}

	return candidates
}

func (e *RecommendationEngine) scoreRecommendations(ctx context.Context, candidates []*EnrichedFeature, req RecommendationRequest) []Recommendation {
	recommendations := make([]Recommendation, 0, len(candidates))

	// Get user history if available
	var userHistory *UserInteractionHistory
	if req.UserID != "" {
		e.mu.RLock()
		userHistory = e.userInteractions[req.UserID]
		e.mu.RUnlock()
	}

	// Determine weights based on strategy
	weights := e.getStrategyWeights(req.Strategy)

	// Check for cold start
	isColdStart := userHistory == nil || userHistory.InteractionCount < 5

	for _, candidate := range candidates {
		scores := &RecommendationScores{}

		// Collaborative score
		if !isColdStart && weights.collaborative > 0 {
			scores.CollaborativeScore = e.computeCollaborativeScore(candidate, userHistory)
		}

		// Content-based score
		if weights.contentBased > 0 {
			scores.ContentScore = e.computeContentScore(candidate, req.CurrentFeatures, userHistory)
		}

		// Popularity score
		if weights.popularity > 0 {
			scores.PopularityScore = e.computePopularityScore(candidate)
		}

		// Context score
		if weights.context > 0 {
			scores.ContextScore = e.computeContextScore(candidate, req)
		}

		// Quality score
		scores.QualityScore = float64(candidate.Metadata.QualityScore)

		// Compute final score
		scores.FinalScore = weights.collaborative*scores.CollaborativeScore +
			weights.contentBased*scores.ContentScore +
			weights.popularity*scores.PopularityScore +
			weights.context*scores.ContextScore +
			0.1*scores.QualityScore // Small quality boost

		// Generate reasons
		var reasons []string
		if req.IncludeReasons {
			reasons = e.generateReasons(candidate, scores, req)
		}

		recommendations = append(recommendations, Recommendation{
			Feature:        candidate,
			Score:          scores.FinalScore,
			ScoreBreakdown: scores,
			Reasons:        reasons,
			Confidence:     e.computeConfidence(scores, isColdStart),
		})
	}

	// Sort by score
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	return recommendations
}

type strategyWeights struct {
	collaborative float64
	contentBased  float64
	popularity    float64
	context       float64
}

func (e *RecommendationEngine) getStrategyWeights(strategy string) strategyWeights {
	switch strategy {
	case "collaborative":
		return strategyWeights{collaborative: 0.7, contentBased: 0.2, popularity: 0.05, context: 0.05}
	case "content":
		return strategyWeights{collaborative: 0.1, contentBased: 0.7, popularity: 0.1, context: 0.1}
	case "popularity":
		return strategyWeights{collaborative: 0.1, contentBased: 0.1, popularity: 0.7, context: 0.1}
	case "context":
		return strategyWeights{collaborative: 0.1, contentBased: 0.2, popularity: 0.1, context: 0.6}
	default: // balanced
		return strategyWeights{
			collaborative: e.config.CollaborativeWeight,
			contentBased:  e.config.ContentBasedWeight,
			popularity:    e.config.PopularityWeight,
			context:       e.config.ContextWeight,
		}
	}
}

func (e *RecommendationEngine) computeCollaborativeScore(candidate *EnrichedFeature, userHistory *UserInteractionHistory) float64 {
	if userHistory == nil {
		return 0
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Get similar users
	similarUsers, ok := e.collaborativeFilter.userSimilarities[userHistory.UserID]
	if !ok || len(similarUsers) == 0 {
		return 0
	}

	var score float64
	var totalWeight float64

	for similarUserID, similarity := range similarUsers {
		if similarity < e.config.UserSimilarityThreshold {
			continue
		}

		similarHistory, ok := e.userInteractions[similarUserID]
		if !ok {
			continue
		}

		// Check if similar user interacted with this feature
		if interaction, ok := similarHistory.ViewedFeatures[candidate.Metadata.FeatureID]; ok {
			// Weight by similarity and interaction strength
			interactionScore := float64(interaction.ViewCount) * 0.1
			if interaction.IsFavorite {
				interactionScore += 0.5
			}
			if interaction.IsUsed {
				interactionScore += 0.3
			}
			interactionScore = math.Min(interactionScore, 1.0)

			score += similarity * interactionScore
			totalWeight += similarity
		}
	}

	if totalWeight > 0 {
		return score / totalWeight
	}
	return 0
}

func (e *RecommendationEngine) computeContentScore(candidate *EnrichedFeature, currentFeatures []string, userHistory *UserInteractionHistory) float64 {
	if len(currentFeatures) == 0 && (userHistory == nil || len(userHistory.ViewedFeatures) == 0) {
		return 0.5 // Neutral score
	}

	var maxSimilarity float64

	// Compare with current features
	for _, featureID := range currentFeatures {
		sim := e.computeFeatureSimilarity(candidate.Metadata.FeatureID, featureID)
		if sim > maxSimilarity {
			maxSimilarity = sim
		}
	}

	// Compare with user's viewed features
	if userHistory != nil {
		for featureID, interaction := range userHistory.ViewedFeatures {
			sim := e.computeFeatureSimilarity(candidate.Metadata.FeatureID, featureID)

			// Weight by interaction strength
			weight := 1.0
			if interaction.IsFavorite {
				weight = 1.5
			}
			if interaction.IsUsed {
				weight *= 1.3
			}

			sim *= weight
			if sim > maxSimilarity {
				maxSimilarity = sim
			}
		}
	}

	return math.Min(maxSimilarity, 1.0)
}

func (e *RecommendationEngine) computeFeatureSimilarity(featureID1, featureID2 string) float64 {
	if featureID1 == featureID2 {
		return 1.0
	}

	meta1, err1 := e.indexer.GetMetadata(featureID1)
	meta2, err2 := e.indexer.GetMetadata(featureID2)
	if err1 != nil || err2 != nil {
		return 0
	}

	var score float64
	var factors float64

	// Same category
	if meta1.Category != "" && meta1.Category == meta2.Category {
		score += 0.3
	}
	factors += 0.3

	// Same domain
	if meta1.Domain != "" && meta1.Domain == meta2.Domain {
		score += 0.3
	}
	factors += 0.3

	// Same entity type
	if meta1.EntityType != "" && meta1.EntityType == meta2.EntityType {
		score += 0.2
	}
	factors += 0.2

	// Tag overlap
	tagOverlap := 0
	for _, tag1 := range meta1.Tags {
		for _, tag2 := range meta2.Tags {
			if strings.EqualFold(tag1, tag2) {
				tagOverlap++
				break
			}
		}
	}
	if len(meta1.Tags) > 0 || len(meta2.Tags) > 0 {
		maxTags := math.Max(float64(len(meta1.Tags)), float64(len(meta2.Tags)))
		score += 0.2 * (float64(tagOverlap) / maxTags)
	}
	factors += 0.2

	return score / factors
}

func (e *RecommendationEngine) computePopularityScore(candidate *EnrichedFeature) float64 {
	e.popularityModel.mu.RLock()
	defer e.popularityModel.mu.RUnlock()

	featureID := candidate.Metadata.FeatureID

	// Get view count
	views := float64(e.popularityModel.viewCounts[featureID])
	usage := float64(e.popularityModel.usageCounts[featureID])

	// Calculate recency-weighted score
	recentViews := e.popularityModel.recentViews[featureID]
	recentScore := 0.0
	now := time.Now()
	decayDays := float64(e.config.PopularityDecayDays)

	for _, viewTime := range recentViews {
		daysSince := now.Sub(viewTime).Hours() / 24
		if daysSince < decayDays {
			recentScore += math.Exp(-daysSince / decayDays)
		}
	}

	// Combine signals
	viewScore := math.Log10(views+1) / 6  // Normalize assuming max ~1M views
	usageScore := math.Log10(usage+1) / 4 // Normalize assuming max ~10K usage
	trendScore := recentScore / 10        // Normalize

	// Use stored usage data if available
	if candidate.Usage != nil {
		if candidate.Usage.PopularityScore > 0 {
			return candidate.Usage.PopularityScore
		}
	}

	return math.Min(viewScore*0.3+usageScore*0.4+trendScore*0.3, 1.0)
}

func (e *RecommendationEngine) computeContextScore(candidate *EnrichedFeature, req RecommendationRequest) float64 {
	var score float64
	var factors int

	meta := candidate.Metadata

	// Use case match
	if req.UseCase != "" {
		for _, uc := range meta.UseCase {
			if strings.EqualFold(uc, req.UseCase) {
				score += 1.0
				break
			}
		}
		factors++
	}

	// Domain match (already filtered, but boost exact matches)
	if req.Domain != "" && strings.EqualFold(meta.Domain, req.Domain) {
		score += 0.5
		factors++
	}

	// Entity type match
	if req.EntityType != "" && strings.EqualFold(meta.EntityType, req.EntityType) {
		score += 0.5
		factors++
	}

	// Category match
	if req.Category != "" && strings.EqualFold(meta.Category, req.Category) {
		score += 0.5
		factors++
	}

	// Co-usage with current features
	if len(req.CurrentFeatures) > 0 {
		coUsageScore := e.computeCoUsageScore(candidate.Metadata.FeatureID, req.CurrentFeatures)
		score += coUsageScore
		factors++
	}

	if factors == 0 {
		return 0.5 // Neutral
	}

	return score / float64(factors)
}

func (e *RecommendationEngine) computeCoUsageScore(candidateID string, currentFeatures []string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	coUsageCount := 0
	totalModels := 0

	for _, modelUsage := range e.modelFeatures {
		hasCandidate := false
		hasCurrentFeature := false

		for _, f := range modelUsage.Features {
			if f == candidateID {
				hasCandidate = true
			}
			for _, cf := range currentFeatures {
				if f == cf {
					hasCurrentFeature = true
					break
				}
			}
		}

		if hasCurrentFeature {
			totalModels++
			if hasCandidate {
				coUsageCount++
			}
		}
	}

	if totalModels == 0 {
		return 0
	}

	return float64(coUsageCount) / float64(totalModels)
}

func (e *RecommendationEngine) generateReasons(candidate *EnrichedFeature, scores *RecommendationScores, req RecommendationRequest) []string {
	var reasons []string

	meta := candidate.Metadata

	// Collaborative reason
	if scores.CollaborativeScore > 0.5 {
		reasons = append(reasons, "Popular among users with similar interests")
	}

	// Content reason
	if scores.ContentScore > 0.5 && len(req.CurrentFeatures) > 0 {
		reasons = append(reasons, "Similar to features you're already using")
	}

	// Popularity reason
	if scores.PopularityScore > 0.7 {
		reasons = append(reasons, "Highly popular feature")
	} else if scores.PopularityScore > 0.4 {
		reasons = append(reasons, "Frequently used feature")
	}

	// Context reasons
	if req.UseCase != "" {
		for _, uc := range meta.UseCase {
			if strings.EqualFold(uc, req.UseCase) {
				reasons = append(reasons, fmt.Sprintf("Designed for %s", req.UseCase))
				break
			}
		}
	}

	if req.Domain != "" && strings.EqualFold(meta.Domain, req.Domain) {
		reasons = append(reasons, fmt.Sprintf("Belongs to %s domain", meta.Domain))
	}

	// Quality reason
	if scores.QualityScore > 0.8 {
		reasons = append(reasons, "High data quality")
	}

	// Freshness reason
	if meta.Freshness == "real-time" {
		reasons = append(reasons, "Real-time updates")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Matches your criteria")
	}

	return reasons
}

func (e *RecommendationEngine) computeConfidence(scores *RecommendationScores, isColdStart bool) float64 {
	// Base confidence on signal availability
	signals := 0
	if scores.CollaborativeScore > 0 {
		signals++
	}
	if scores.ContentScore > 0 {
		signals++
	}
	if scores.PopularityScore > 0 {
		signals++
	}
	if scores.ContextScore > 0 {
		signals++
	}

	baseConfidence := float64(signals) / 4.0

	// Penalize cold start
	if isColdStart {
		baseConfidence *= 0.7
	}

	// Boost if score is high
	if scores.FinalScore > 0.7 {
		baseConfidence *= 1.1
	}

	return math.Min(baseConfidence, 1.0)
}

func (e *RecommendationEngine) applyDiversity(recommendations []Recommendation, req RecommendationRequest) []Recommendation {
	if req.DiversityFactor <= 0 || len(recommendations) <= 1 {
		return recommendations
	}

	// Track categories in result
	categoryCount := make(map[string]int)
	maxPerCategory := int(float64(req.Limit) * e.config.MaxSameCategoryRatio)

	diverse := make([]Recommendation, 0, len(recommendations))

	for _, rec := range recommendations {
		category := rec.Feature.Metadata.Category

		// Check category limit
		if categoryCount[category] >= maxPerCategory {
			// Apply diversity penalty
			rec.Score *= (1 - req.DiversityFactor)
			if rec.ScoreBreakdown != nil {
				rec.ScoreBreakdown.DiversityPenalty = req.DiversityFactor
			}
		}

		diverse = append(diverse, rec)
		categoryCount[category]++
	}

	// Re-sort after diversity adjustment
	sort.Slice(diverse, func(i, j int) bool {
		return diverse[i].Score > diverse[j].Score
	})

	return diverse
}

// RecordInteraction records a user interaction with a feature.
func (e *RecommendationEngine) RecordInteraction(userID, featureID string, interactionType string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get or create user history
	history, ok := e.userInteractions[userID]
	if !ok {
		history = &UserInteractionHistory{
			UserID:              userID,
			ViewedFeatures:      make(map[string]*FeatureInteraction),
			UsedFeatures:        make(map[string]time.Time),
			PreferredCategories: make(map[string]float64),
			PreferredDomains:    make(map[string]float64),
		}
		e.userInteractions[userID] = history
	}

	history.LastActivity = time.Now()
	history.InteractionCount++

	// Get or create feature interaction
	interaction, ok := history.ViewedFeatures[featureID]
	if !ok {
		interaction = &FeatureInteraction{
			FeatureID: featureID,
		}
		history.ViewedFeatures[featureID] = interaction
	}

	interaction.LastViewed = time.Now()

	switch interactionType {
	case "view":
		interaction.ViewCount++
	case "favorite":
		interaction.IsFavorite = true
		if !containsString(history.FavoriteFeatures, featureID) {
			history.FavoriteFeatures = append(history.FavoriteFeatures, featureID)
		}
	case "unfavorite":
		interaction.IsFavorite = false
		history.FavoriteFeatures = removeString(history.FavoriteFeatures, featureID)
	case "use":
		interaction.IsUsed = true
		history.UsedFeatures[featureID] = time.Now()
	}

	// Update preferences
	if meta, err := e.indexer.GetMetadata(featureID); err == nil {
		if meta.Category != "" {
			history.PreferredCategories[meta.Category] += 0.1
		}
		if meta.Domain != "" {
			history.PreferredDomains[meta.Domain] += 0.1
		}
	}

	// Update popularity model
	e.popularityModel.mu.Lock()
	e.popularityModel.viewCounts[featureID]++
	e.popularityModel.recentViews[featureID] = append(e.popularityModel.recentViews[featureID], time.Now())
	if interactionType == "use" {
		e.popularityModel.usageCounts[featureID]++
	}
	e.popularityModel.mu.Unlock()
}

// RegisterModel registers a model's feature usage.
func (e *RecommendationEngine) RegisterModel(modelID, modelName, useCase string, features []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.modelFeatures[modelID] = &ModelFeatureUsage{
		ModelID:     modelID,
		ModelName:   modelName,
		UseCase:     useCase,
		Features:    features,
		LastUpdated: time.Now(),
	}

	// Update context model
	if useCase != "" {
		for _, featureID := range features {
			e.contextModel.useCaseFeatures[useCase] = appendUniqueString(e.contextModel.useCaseFeatures[useCase], featureID)
		}
	}
}

// ComputeUserSimilarities computes similarity between users.
func (e *RecommendationEngine) ComputeUserSimilarities() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get all users with enough interactions
	users := make([]string, 0)
	for userID, history := range e.userInteractions {
		if history.InteractionCount >= 5 {
			users = append(users, userID)
		}
	}

	// Compute pairwise similarities
	for _, user1 := range users {
		if e.collaborativeFilter.userSimilarities[user1] == nil {
			e.collaborativeFilter.userSimilarities[user1] = make(map[string]float64)
		}

		history1 := e.userInteractions[user1]

		for _, user2 := range users {
			if user1 >= user2 {
				continue
			}

			history2 := e.userInteractions[user2]

			// Compute Jaccard similarity on viewed features
			similarity := computeJaccardSimilarity(history1.ViewedFeatures, history2.ViewedFeatures)

			if similarity >= e.config.UserSimilarityThreshold {
				e.collaborativeFilter.userSimilarities[user1][user2] = similarity
				if e.collaborativeFilter.userSimilarities[user2] == nil {
					e.collaborativeFilter.userSimilarities[user2] = make(map[string]float64)
				}
				e.collaborativeFilter.userSimilarities[user2][user1] = similarity
			}
		}
	}

	e.collaborativeFilter.lastComputed = time.Now()
	e.logger.Info("User similarities computed", "user_count", len(users))
}

func computeJaccardSimilarity(set1, set2 map[string]*FeatureInteraction) float64 {
	if len(set1) == 0 || len(set2) == 0 {
		return 0
	}

	intersection := 0
	for key := range set1 {
		if _, ok := set2[key]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// RecommendForUseCase gets recommendations for a specific use case.
func (e *RecommendationEngine) RecommendForUseCase(ctx context.Context, useCase string, limit int) (*RecommendationResult, error) {
	return e.Recommend(ctx, RecommendationRequest{
		UseCase:  useCase,
		Strategy: "context",
		Limit:    limit,
	})
}

// RecommendForUser gets personalized recommendations for a user.
func (e *RecommendationEngine) RecommendForUser(ctx context.Context, userID string, limit int) (*RecommendationResult, error) {
	return e.Recommend(ctx, RecommendationRequest{
		UserID:   userID,
		Strategy: "balanced",
		Limit:    limit,
	})
}

// RecommendSimilarTo gets recommendations similar to given features.
func (e *RecommendationEngine) RecommendSimilarTo(ctx context.Context, featureIDs []string, limit int) (*RecommendationResult, error) {
	return e.Recommend(ctx, RecommendationRequest{
		CurrentFeatures: featureIDs,
		ExcludeFeatures: featureIDs,
		Strategy:        "content",
		Limit:           limit,
	})
}

// RecommendTrending gets trending/popular recommendations.
func (e *RecommendationEngine) RecommendTrending(ctx context.Context, limit int) (*RecommendationResult, error) {
	return e.Recommend(ctx, RecommendationRequest{
		Strategy: "popularity",
		Limit:    limit,
	})
}

// GetUserHistory returns a user's interaction history.
func (e *RecommendationEngine) GetUserHistory(userID string) (*UserInteractionHistory, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	history, ok := e.userInteractions[userID]
	if !ok {
		return nil, fmt.Errorf("no history for user: %s", userID)
	}

	return history, nil
}

// GetStats returns recommendation engine statistics.
func (e *RecommendationEngine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.popularityModel.mu.RLock()
	defer e.popularityModel.mu.RUnlock()

	return map[string]interface{}{
		"user_count":              len(e.userInteractions),
		"model_count":             len(e.modelFeatures),
		"tracked_features":        len(e.popularityModel.viewCounts),
		"user_similarities_count": len(e.collaborativeFilter.userSimilarities),
		"use_case_count":          len(e.contextModel.useCaseFeatures),
		"domain_count":            len(e.contextModel.domainFeatures),
		"category_count":          len(e.contextModel.categoryFeatures),
		"config":                  e.config,
	}
}

// Helper functions

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func appendUniqueString(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
