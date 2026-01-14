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

// FeatureDiscovery provides AI-powered feature discovery capabilities.
// It combines semantic search, relationship graphs, and intelligent recommendations.
type FeatureDiscovery struct {
	mu sync.RWMutex

	// Core components
	indexer   *EnhancedIndexer
	ranker    *HybridRanker
	explainer *Explainer
	embedder  Embedder

	// Feature relationship graph
	graph *FeatureGraph

	// Discovery state
	queryHistory    []QueryHistoryEntry
	userPreferences map[string]*UserDiscoveryPreferences

	// Configuration
	config DiscoveryConfig

	// Logging
	logger *slog.Logger
}

// DiscoveryConfig configures the discovery engine.
type DiscoveryConfig struct {
	// Search configuration
	DefaultLimit        int     `json:"default_limit"`
	MaxLimit            int     `json:"max_limit"`
	MinSemanticScore    float32 `json:"min_semantic_score"`
	MinRecommendScore   float32 `json:"min_recommend_score"`
	EnableExplanations  bool    `json:"enable_explanations"`
	EnableAutoComplete  bool    `json:"enable_auto_complete"`
	EnableRelatedSearch bool    `json:"enable_related_search"`

	// Graph configuration
	MaxGraphDepth     int     `json:"max_graph_depth"`
	MinEdgeWeight     float32 `json:"min_edge_weight"`
	EnableGraphSearch bool    `json:"enable_graph_search"`

	// History configuration
	MaxQueryHistory       int           `json:"max_query_history"`
	QueryHistoryTTL       time.Duration `json:"query_history_ttl"`
	EnablePersonalization bool          `json:"enable_personalization"`
	PersonalizationWeight float64       `json:"personalization_weight"`

	// Performance
	ParallelQueries int           `json:"parallel_queries"`
	QueryTimeout    time.Duration `json:"query_timeout"`
	CacheEnabled    bool          `json:"cache_enabled"`
	CacheTTL        time.Duration `json:"cache_ttl"`
}

// DefaultDiscoveryConfig returns sensible defaults.
func DefaultDiscoveryConfig() DiscoveryConfig {
	return DiscoveryConfig{
		DefaultLimit:          10,
		MaxLimit:              100,
		MinSemanticScore:      0.3,
		MinRecommendScore:     0.4,
		EnableExplanations:    true,
		EnableAutoComplete:    true,
		EnableRelatedSearch:   true,
		MaxGraphDepth:         3,
		MinEdgeWeight:         0.3,
		EnableGraphSearch:     true,
		MaxQueryHistory:       1000,
		QueryHistoryTTL:       7 * 24 * time.Hour,
		EnablePersonalization: true,
		PersonalizationWeight: 0.15,
		ParallelQueries:       4,
		QueryTimeout:          30 * time.Second,
		CacheEnabled:          true,
		CacheTTL:              15 * time.Minute,
	}
}

// QueryHistoryEntry records a user query.
type QueryHistoryEntry struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Query        string    `json:"query"`
	ResultCount  int       `json:"result_count"`
	ClickedIDs   []string  `json:"clicked_ids,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	QueryType    string    `json:"query_type"` // "search", "similar", "recommend"
	ResponseTime int64     `json:"response_time_ms"`
}

// UserDiscoveryPreferences stores user preferences for personalization.
type UserDiscoveryPreferences struct {
	UserID           string            `json:"user_id"`
	PreferredDomains []string          `json:"preferred_domains,omitempty"`
	PreferredOwners  []string          `json:"preferred_owners,omitempty"`
	RecentCategories []string          `json:"recent_categories,omitempty"`
	FavoriteFeatures []string          `json:"favorite_features,omitempty"`
	FrequentTerms    map[string]int    `json:"frequent_terms,omitempty"`
	LastActivity     time.Time         `json:"last_activity"`
	Preferences      map[string]string `json:"preferences,omitempty"`
}

// FeatureGraph represents relationships between features.
type FeatureGraph struct {
	mu sync.RWMutex

	// Adjacency list with weighted edges
	edges map[string]map[string]*FeatureEdge

	// Node metadata
	nodes map[string]*FeatureNode

	// Reverse index for efficient lookup
	reverseEdges map[string]map[string]*FeatureEdge
}

// FeatureNode represents a feature in the graph.
type FeatureNode struct {
	FeatureID   string    `json:"feature_id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Domain      string    `json:"domain"`
	EntityType  string    `json:"entity_type"`
	Centrality  float64   `json:"centrality"` // PageRank-like score
	LastUpdated time.Time `json:"last_updated"`
}

// FeatureEdge represents a relationship between features.
type FeatureEdge struct {
	SourceID     string            `json:"source_id"`
	TargetID     string            `json:"target_id"`
	Weight       float32           `json:"weight"` // 0-1 strength
	EdgeType     FeatureEdgeType   `json:"edge_type"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	LastAccessed time.Time         `json:"last_accessed"`
}

// FeatureEdgeType defines types of relationships.
type FeatureEdgeType string

// Feature edge types for relationship graph metadata.
const (
	EdgeTypeSimilar     FeatureEdgeType = "similar"     // Semantically similar
	EdgeTypeDerived     FeatureEdgeType = "derived"     // One derived from another
	EdgeTypeCoUsed      FeatureEdgeType = "co_used"     // Used together in models
	EdgeTypeRelated     FeatureEdgeType = "related"     // Related by domain/entity
	EdgeTypeAggregation FeatureEdgeType = "aggregation" // Aggregation relationship
)

// NewFeatureDiscovery creates a new feature discovery engine.
func NewFeatureDiscovery(
	indexer *EnhancedIndexer,
	embedder Embedder,
	config DiscoveryConfig,
	logger *slog.Logger,
) (*FeatureDiscovery, error) {
	if indexer == nil {
		return nil, fmt.Errorf("indexer is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	ranker := NewHybridRanker(indexer, DefaultRankerConfig())
	explainer := NewExplainer(indexer, nil, DefaultExplainerConfig())

	fd := &FeatureDiscovery{
		indexer:         indexer,
		ranker:          ranker,
		explainer:       explainer,
		embedder:        embedder,
		graph:           newFeatureGraph(),
		queryHistory:    make([]QueryHistoryEntry, 0),
		userPreferences: make(map[string]*UserDiscoveryPreferences),
		config:          config,
		logger:          logger,
	}

	return fd, nil
}

func newFeatureGraph() *FeatureGraph {
	return &FeatureGraph{
		edges:        make(map[string]map[string]*FeatureEdge),
		nodes:        make(map[string]*FeatureNode),
		reverseEdges: make(map[string]map[string]*FeatureEdge),
	}
}

// DiscoveryQuery represents a discovery query.
type DiscoveryQuery struct {
	// Core query
	Query  string `json:"query"`
	UserID string `json:"user_id,omitempty"`

	// Filters
	Categories  []string `json:"categories,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	EntityTypes []string `json:"entity_types,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Owners      []string `json:"owners,omitempty"`
	Teams       []string `json:"teams,omitempty"`
	UseCases    []string `json:"use_cases,omitempty"`
	DataTypes   []string `json:"data_types,omitempty"`

	// Quality filters
	MinQuality   float32 `json:"min_quality,omitempty"`
	OnlyFresh    bool    `json:"only_fresh,omitempty"`
	MinFreshness string  `json:"min_freshness,omitempty"` // "real-time", "hourly", "daily"

	// Exclusions
	ExcludeFeatures   []string `json:"exclude_features,omitempty"`
	ExcludeCategories []string `json:"exclude_categories,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
	ExcludeDeprecated bool     `json:"exclude_deprecated,omitempty"`

	// Result options
	Limit              int  `json:"limit,omitempty"`
	Offset             int  `json:"offset,omitempty"`
	IncludeExplanation bool `json:"include_explanation,omitempty"`
	IncludeRelated     bool `json:"include_related,omitempty"`
	IncludeStats       bool `json:"include_stats,omitempty"`

	// Personalization
	UsePersonalization bool `json:"use_personalization,omitempty"`
}

// DiscoveryResult represents discovery results.
type DiscoveryResult struct {
	// Query info
	Query         string `json:"query"`
	TotalResults  int    `json:"total_results"`
	ReturnedCount int    `json:"returned_count"`

	// Results
	Features    []DiscoveredFeature `json:"features"`
	Related     []RelatedResult     `json:"related,omitempty"`
	Suggestions []QuerySuggestion   `json:"suggestions,omitempty"`

	// Facets for filtering
	Facets *DiscoveryFacets `json:"facets,omitempty"`

	// Metadata
	ResponseTime int64     `json:"response_time_ms"`
	Timestamp    time.Time `json:"timestamp"`
}

// DiscoveredFeature represents a discovered feature with enrichment.
type DiscoveredFeature struct {
	// Feature info
	Feature *EnrichedFeature `json:"feature"`

	// Scoring
	Score          float64         `json:"score"`
	ScoreBreakdown *ScoreBreakdown `json:"score_breakdown,omitempty"`

	// Explanation
	Explanation *FeatureExplanation `json:"explanation,omitempty"`

	// Relationships
	RelatedCount int      `json:"related_count"`
	TopRelated   []string `json:"top_related,omitempty"`

	// Actions
	Bookmarked bool       `json:"bookmarked,omitempty"`
	LastViewed *time.Time `json:"last_viewed,omitempty"`
}

// ScoreBreakdown provides detailed score analysis.
type ScoreBreakdown struct {
	SemanticScore        float64 `json:"semantic_score"`
	MetadataScore        float64 `json:"metadata_score"`
	PopularityScore      float64 `json:"popularity_score"`
	QualityScore         float64 `json:"quality_score"`
	FreshnessScore       float64 `json:"freshness_score"`
	PersonalizationScore float64 `json:"personalization_score,omitempty"`
	BoostMultiplier      float64 `json:"boost_multiplier"`
	PenaltyMultiplier    float64 `json:"penalty_multiplier"`
}

// RelatedResult represents related search results.
type RelatedResult struct {
	FeatureID string  `json:"feature_id"`
	Name      string  `json:"name"`
	Relation  string  `json:"relation"` // "similar", "derived", "co_used"
	Score     float64 `json:"score"`
}

// QuerySuggestion suggests query refinements.
type QuerySuggestion struct {
	Query       string  `json:"query"`
	Type        string  `json:"type"` // "refine", "broaden", "related"
	Description string  `json:"description,omitempty"`
	Score       float64 `json:"score"`
}

// DiscoveryFacets provides filtering options.
type DiscoveryFacets struct {
	Categories  []FacetValue `json:"categories"`
	Domains     []FacetValue `json:"domains"`
	EntityTypes []FacetValue `json:"entity_types"`
	Tags        []FacetValue `json:"tags"`
	Owners      []FacetValue `json:"owners"`
	DataTypes   []FacetValue `json:"data_types"`
	Quality     []FacetValue `json:"quality"`
}

// FacetValue represents a facet option with count.
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Discover performs AI-powered feature discovery.
func (fd *FeatureDiscovery) Discover(ctx context.Context, query DiscoveryQuery) (*DiscoveryResult, error) {
	startTime := time.Now()

	// Apply defaults
	if query.Limit <= 0 {
		query.Limit = fd.config.DefaultLimit
	}
	if query.Limit > fd.config.MaxLimit {
		query.Limit = fd.config.MaxLimit
	}

	// Build rank request from discovery query
	rankReq := fd.buildRankRequest(query)

	// Apply personalization if enabled
	if query.UsePersonalization && query.UserID != "" && fd.config.EnablePersonalization {
		fd.applyPersonalization(&rankReq, query.UserID)
	}

	// Execute search
	rankedResults, err := fd.ranker.Rank(ctx, rankReq)
	if err != nil {
		return nil, fmt.Errorf("ranking failed: %w", err)
	}

	// Build result
	result := &DiscoveryResult{
		Query:         query.Query,
		TotalResults:  len(rankedResults),
		ReturnedCount: min(len(rankedResults), query.Limit),
		Features:      make([]DiscoveredFeature, 0),
		Timestamp:     time.Now(),
	}

	// Apply offset and limit
	start := query.Offset
	end := query.Offset + query.Limit
	if start > len(rankedResults) {
		start = len(rankedResults)
	}
	if end > len(rankedResults) {
		end = len(rankedResults)
	}
	paginatedResults := rankedResults[start:end]

	// Convert to discovered features
	for _, rr := range paginatedResults {
		df := fd.convertToDiscoveredFeature(ctx, rr, query)
		result.Features = append(result.Features, df)
	}

	// Add related results if requested
	if query.IncludeRelated && fd.config.EnableRelatedSearch && len(result.Features) > 0 {
		result.Related = fd.findRelated(ctx, result.Features[:min(3, len(result.Features))])
	}

	// Generate query suggestions
	result.Suggestions = fd.generateSuggestions(query, rankedResults)

	// Build facets
	result.Facets = fd.buildFacets(rankedResults)

	// Record query
	result.ResponseTime = time.Since(startTime).Milliseconds()
	fd.recordQuery(query, result)

	return result, nil
}

func (fd *FeatureDiscovery) buildRankRequest(query DiscoveryQuery) RankRequest {
	req := RankRequest{
		Query:           query.Query,
		Categories:      query.Categories,
		Domains:         query.Domains,
		EntityTypes:     query.EntityTypes,
		Tags:            query.Tags,
		UseCases:        query.UseCases,
		MinQuality:      query.MinQuality,
		OnlyFresh:       query.OnlyFresh,
		ExcludeFeatures: query.ExcludeFeatures,
		Limit:           query.Limit * 3, // Get more for filtering
	}

	if len(query.Owners) > 0 {
		req.Owner = query.Owners[0]
	}
	if len(query.Teams) > 0 {
		req.Team = query.Teams[0]
	}

	return req
}

func (fd *FeatureDiscovery) applyPersonalization(req *RankRequest, userID string) {
	fd.mu.RLock()
	prefs, ok := fd.userPreferences[userID]
	fd.mu.RUnlock()

	if !ok {
		return
	}

	// Add preferred domains if not already specified
	if len(req.Domains) == 0 && len(prefs.PreferredDomains) > 0 {
		req.Domains = prefs.PreferredDomains[:min(3, len(prefs.PreferredDomains))]
	}

	// Boost owner matches
	if len(prefs.PreferredOwners) > 0 && req.Owner == "" {
		req.Owner = prefs.PreferredOwners[0]
	}
}

func (fd *FeatureDiscovery) convertToDiscoveredFeature(ctx context.Context, rr RankedResult, query DiscoveryQuery) DiscoveredFeature {
	df := DiscoveredFeature{
		Feature: rr.Feature,
		Score:   rr.TotalScore,
		ScoreBreakdown: &ScoreBreakdown{
			SemanticScore:     rr.SemanticScore,
			MetadataScore:     rr.MetadataScore,
			PopularityScore:   rr.PopularityScore,
			QualityScore:      rr.QualityScore,
			FreshnessScore:    rr.FreshnessScore,
			BoostMultiplier:   1.0,
			PenaltyMultiplier: 1.0,
		},
	}

	// Calculate boost/penalty multipliers
	if len(rr.BoostFactors) > 0 {
		df.ScoreBreakdown.BoostMultiplier = 1.0 + float64(len(rr.BoostFactors))*0.1
	}
	if len(rr.Penalties) > 0 {
		df.ScoreBreakdown.PenaltyMultiplier = 1.0 - float64(len(rr.Penalties))*0.1
	}

	// Add explanation if requested
	if query.IncludeExplanation && fd.config.EnableExplanations {
		explanation, err := fd.explainer.Explain(ctx, rr.Feature.Metadata.FeatureID, query.Query, rr.TotalScore)
		if err == nil {
			df.Explanation = explanation
		}
	}

	// Get related count from graph
	if fd.config.EnableGraphSearch {
		fd.graph.mu.RLock()
		if edges, ok := fd.graph.edges[rr.Feature.Metadata.FeatureID]; ok {
			df.RelatedCount = len(edges)
			// Get top related
			var topRelated []string
			for targetID := range edges {
				topRelated = append(topRelated, targetID)
				if len(topRelated) >= 3 {
					break
				}
			}
			df.TopRelated = topRelated
		}
		fd.graph.mu.RUnlock()
	}

	return df
}

func (fd *FeatureDiscovery) findRelated(ctx context.Context, features []DiscoveredFeature) []RelatedResult {
	var related []RelatedResult
	seen := make(map[string]bool)

	for _, f := range features {
		seen[f.Feature.Metadata.FeatureID] = true
	}

	fd.graph.mu.RLock()
	defer fd.graph.mu.RUnlock()

	for _, f := range features {
		if edges, ok := fd.graph.edges[f.Feature.Metadata.FeatureID]; ok {
			for targetID, edge := range edges {
				if seen[targetID] {
					continue
				}
				seen[targetID] = true

				// Get feature name
				name := targetID
				if node, ok := fd.graph.nodes[targetID]; ok {
					name = node.Name
				}

				related = append(related, RelatedResult{
					FeatureID: targetID,
					Name:      name,
					Relation:  string(edge.EdgeType),
					Score:     float64(edge.Weight),
				})
			}
		}
	}

	// Sort by score
	sort.Slice(related, func(i, j int) bool {
		return related[i].Score > related[j].Score
	})

	// Limit results
	if len(related) > 10 {
		related = related[:10]
	}

	return related
}

func (fd *FeatureDiscovery) generateSuggestions(query DiscoveryQuery, results []RankedResult) []QuerySuggestion {
	var suggestions []QuerySuggestion

	// Suggest refinements based on top result categories
	if len(results) > 0 {
		categories := make(map[string]int)
		domains := make(map[string]int)

		for _, r := range results[:min(10, len(results))] {
			if r.Feature.Metadata.Category != "" {
				categories[r.Feature.Metadata.Category]++
			}
			if r.Feature.Metadata.Domain != "" {
				domains[r.Feature.Metadata.Domain]++
			}
		}

		// Suggest category refinements
		for cat, count := range categories {
			if count >= 2 && !contains(query.Categories, cat) {
				suggestions = append(suggestions, QuerySuggestion{
					Query:       fmt.Sprintf("%s category:%s", query.Query, cat),
					Type:        "refine",
					Description: fmt.Sprintf("Filter by %s category (%d results)", cat, count),
					Score:       float64(count) / float64(len(results)),
				})
			}
		}

		// Suggest domain refinements
		for domain, count := range domains {
			if count >= 2 && !contains(query.Domains, domain) {
				suggestions = append(suggestions, QuerySuggestion{
					Query:       fmt.Sprintf("%s domain:%s", query.Query, domain),
					Type:        "refine",
					Description: fmt.Sprintf("Filter by %s domain (%d results)", domain, count),
					Score:       float64(count) / float64(len(results)),
				})
			}
		}
	}

	// Suggest broadening if few results
	if len(results) < 5 {
		// Suggest removing filters
		if len(query.Categories) > 0 {
			suggestions = append(suggestions, QuerySuggestion{
				Query:       query.Query,
				Type:        "broaden",
				Description: "Remove category filter to see more results",
				Score:       0.5,
			})
		}
	}

	// Sort by score
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	// Limit suggestions
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

func (fd *FeatureDiscovery) buildFacets(results []RankedResult) *DiscoveryFacets {
	facets := &DiscoveryFacets{
		Categories:  make([]FacetValue, 0),
		Domains:     make([]FacetValue, 0),
		EntityTypes: make([]FacetValue, 0),
		Tags:        make([]FacetValue, 0),
		Owners:      make([]FacetValue, 0),
		DataTypes:   make([]FacetValue, 0),
		Quality:     make([]FacetValue, 0),
	}

	// Count facets
	categories := make(map[string]int)
	domains := make(map[string]int)
	entityTypes := make(map[string]int)
	tags := make(map[string]int)
	owners := make(map[string]int)
	dataTypes := make(map[string]int)
	quality := make(map[string]int)

	for _, r := range results {
		meta := r.Feature.Metadata
		if meta.Category != "" {
			categories[meta.Category]++
		}
		if meta.Domain != "" {
			domains[meta.Domain]++
		}
		if meta.EntityType != "" {
			entityTypes[meta.EntityType]++
		}
		for _, tag := range meta.Tags {
			tags[tag]++
		}
		if meta.Owner != "" {
			owners[meta.Owner]++
		}
		if meta.DataType != "" {
			dataTypes[meta.DataType]++
		}
		if meta.DataQuality != "" {
			quality[meta.DataQuality]++
		}
	}

	// Convert to facet values
	facets.Categories = mapToFacetValues(categories)
	facets.Domains = mapToFacetValues(domains)
	facets.EntityTypes = mapToFacetValues(entityTypes)
	facets.Tags = mapToFacetValues(tags)
	facets.Owners = mapToFacetValues(owners)
	facets.DataTypes = mapToFacetValues(dataTypes)
	facets.Quality = mapToFacetValues(quality)

	return facets
}

func mapToFacetValues(m map[string]int) []FacetValue {
	values := make([]FacetValue, 0, len(m))
	for v, c := range m {
		values = append(values, FacetValue{Value: v, Count: c})
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Count > values[j].Count
	})
	return values
}

func (fd *FeatureDiscovery) recordQuery(query DiscoveryQuery, result *DiscoveryResult) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	entry := QueryHistoryEntry{
		ID:           fmt.Sprintf("q_%d", time.Now().UnixNano()),
		UserID:       query.UserID,
		Query:        query.Query,
		ResultCount:  result.TotalResults,
		Timestamp:    result.Timestamp,
		QueryType:    "search",
		ResponseTime: result.ResponseTime,
	}

	fd.queryHistory = append(fd.queryHistory, entry)

	// Trim history if too large
	if len(fd.queryHistory) > fd.config.MaxQueryHistory {
		fd.queryHistory = fd.queryHistory[len(fd.queryHistory)-fd.config.MaxQueryHistory:]
	}

	// Update user preferences
	if query.UserID != "" && fd.config.EnablePersonalization {
		fd.updateUserPreferences(query, result)
	}
}

func (fd *FeatureDiscovery) updateUserPreferences(query DiscoveryQuery, result *DiscoveryResult) {
	prefs, ok := fd.userPreferences[query.UserID]
	if !ok {
		prefs = &UserDiscoveryPreferences{
			UserID:        query.UserID,
			FrequentTerms: make(map[string]int),
			Preferences:   make(map[string]string),
		}
		fd.userPreferences[query.UserID] = prefs
	}

	prefs.LastActivity = time.Now()

	// Track query terms
	terms := strings.Fields(strings.ToLower(query.Query))
	for _, term := range terms {
		if len(term) > 2 {
			prefs.FrequentTerms[term]++
		}
	}

	// Track categories from results
	for _, f := range result.Features[:min(5, len(result.Features))] {
		if f.Feature.Metadata.Category != "" {
			prefs.RecentCategories = appendUnique(prefs.RecentCategories, f.Feature.Metadata.Category)
		}
		if f.Feature.Metadata.Domain != "" {
			prefs.PreferredDomains = appendUnique(prefs.PreferredDomains, f.Feature.Metadata.Domain)
		}
	}

	// Limit tracked items
	if len(prefs.RecentCategories) > 10 {
		prefs.RecentCategories = prefs.RecentCategories[len(prefs.RecentCategories)-10:]
	}
	if len(prefs.PreferredDomains) > 5 {
		prefs.PreferredDomains = prefs.PreferredDomains[len(prefs.PreferredDomains)-5:]
	}
}

// FindSimilar finds features similar to a given feature.
func (fd *FeatureDiscovery) FindSimilar(ctx context.Context, featureID string, limit int) ([]DiscoveredFeature, error) {
	if limit <= 0 {
		limit = fd.config.DefaultLimit
	}

	// Get source feature
	source, err := fd.indexer.GetEnrichedFeature(featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Use ranker's similar suggestion
	rankedResults, err := fd.ranker.SuggestSimilar(ctx, featureID, limit)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}

	// Also check graph relationships
	if fd.config.EnableGraphSearch {
		graphSimilar := fd.findGraphSimilar(featureID, limit)
		rankedResults = fd.mergeResults(rankedResults, graphSimilar)
	}

	// Convert to discovered features
	result := make([]DiscoveredFeature, 0, len(rankedResults))
	for _, rr := range rankedResults {
		df := DiscoveredFeature{
			Feature: rr.Feature,
			Score:   rr.TotalScore,
			ScoreBreakdown: &ScoreBreakdown{
				SemanticScore:   rr.SemanticScore,
				MetadataScore:   rr.MetadataScore,
				PopularityScore: rr.PopularityScore,
				QualityScore:    rr.QualityScore,
				FreshnessScore:  rr.FreshnessScore,
			},
		}
		result = append(result, df)
	}

	// Record relationship in graph
	fd.recordSimilarityEdges(source.Metadata.FeatureID, result)

	return result, nil
}

func (fd *FeatureDiscovery) findGraphSimilar(featureID string, limit int) []RankedResult {
	fd.graph.mu.RLock()
	defer fd.graph.mu.RUnlock()

	edges, ok := fd.graph.edges[featureID]
	if !ok {
		return nil
	}

	results := make([]RankedResult, 0, len(edges))

	for targetID, edge := range edges {
		if edge.EdgeType != EdgeTypeSimilar && edge.EdgeType != EdgeTypeRelated {
			continue
		}

		// Get enriched feature
		enriched, err := fd.indexer.GetEnrichedFeature(targetID)
		if err != nil {
			continue
		}

		results = append(results, RankedResult{
			Feature:    enriched,
			TotalScore: float64(edge.Weight),
		})
	}

	// Sort and limit
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalScore > results[j].TotalScore
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func (fd *FeatureDiscovery) mergeResults(primary, secondary []RankedResult) []RankedResult {
	seen := make(map[string]bool)
	result := make([]RankedResult, 0)

	for _, r := range primary {
		seen[r.Feature.Metadata.FeatureID] = true
		result = append(result, r)
	}

	for _, r := range secondary {
		if !seen[r.Feature.Metadata.FeatureID] {
			result = append(result, r)
		}
	}

	return result
}

func (fd *FeatureDiscovery) recordSimilarityEdges(sourceID string, similar []DiscoveredFeature) {
	fd.graph.mu.Lock()
	defer fd.graph.mu.Unlock()

	if _, ok := fd.graph.edges[sourceID]; !ok {
		fd.graph.edges[sourceID] = make(map[string]*FeatureEdge)
	}

	for _, sf := range similar {
		targetID := sf.Feature.Metadata.FeatureID
		if targetID == sourceID {
			continue
		}

		// Create or update edge
		edge, ok := fd.graph.edges[sourceID][targetID]
		if !ok {
			edge = &FeatureEdge{
				SourceID:  sourceID,
				TargetID:  targetID,
				EdgeType:  EdgeTypeSimilar,
				CreatedAt: time.Now(),
			}
			fd.graph.edges[sourceID][targetID] = edge

			// Add reverse edge
			if _, ok := fd.graph.reverseEdges[targetID]; !ok {
				fd.graph.reverseEdges[targetID] = make(map[string]*FeatureEdge)
			}
			fd.graph.reverseEdges[targetID][sourceID] = edge
		}

		edge.Weight = float32(sf.Score)
		edge.LastAccessed = time.Now()
	}
}

// AutoComplete provides query suggestions based on prefix.
func (fd *FeatureDiscovery) AutoComplete(ctx context.Context, prefix string, limit int) ([]string, error) {
	if !fd.config.EnableAutoComplete {
		return nil, nil
	}

	if limit <= 0 {
		limit = 10
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, nil
	}

	suggestions := make([]string, 0)
	seen := make(map[string]bool)

	// Search feature names
	for _, meta := range fd.indexer.ListMetadata() {
		nameLower := strings.ToLower(meta.Name)
		if strings.HasPrefix(nameLower, prefix) {
			if !seen[meta.Name] {
				suggestions = append(suggestions, meta.Name)
				seen[meta.Name] = true
			}
		}

		// Also check tags
		for _, tag := range meta.Tags {
			tagLower := strings.ToLower(tag)
			if strings.HasPrefix(tagLower, prefix) && !seen[tag] {
				suggestions = append(suggestions, tag)
				seen[tag] = true
			}
		}

		// Check categories
		catLower := strings.ToLower(meta.Category)
		if strings.HasPrefix(catLower, prefix) && !seen[meta.Category] {
			suggestions = append(suggestions, meta.Category)
			seen[meta.Category] = true
		}

		if len(suggestions) >= limit*2 {
			break
		}
	}

	// Sort by relevance (exact prefix match first, then alphabetically)
	sort.Slice(suggestions, func(i, j int) bool {
		iExact := strings.HasPrefix(strings.ToLower(suggestions[i]), prefix)
		jExact := strings.HasPrefix(strings.ToLower(suggestions[j]), prefix)
		if iExact != jExact {
			return iExact
		}
		return suggestions[i] < suggestions[j]
	})

	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// AddFeatureRelationship adds a relationship between features.
func (fd *FeatureDiscovery) AddFeatureRelationship(sourceID, targetID string, edgeType FeatureEdgeType, weight float32) error {
	if sourceID == targetID {
		return fmt.Errorf("cannot create self-referential edge")
	}

	// Verify features exist
	if _, err := fd.indexer.GetMetadata(sourceID); err != nil {
		return fmt.Errorf("source feature not found: %w", err)
	}
	if _, err := fd.indexer.GetMetadata(targetID); err != nil {
		return fmt.Errorf("target feature not found: %w", err)
	}

	fd.graph.mu.Lock()
	defer fd.graph.mu.Unlock()

	if _, ok := fd.graph.edges[sourceID]; !ok {
		fd.graph.edges[sourceID] = make(map[string]*FeatureEdge)
	}

	edge := &FeatureEdge{
		SourceID:  sourceID,
		TargetID:  targetID,
		Weight:    weight,
		EdgeType:  edgeType,
		CreatedAt: time.Now(),
	}

	fd.graph.edges[sourceID][targetID] = edge

	// Add reverse edge
	if _, ok := fd.graph.reverseEdges[targetID]; !ok {
		fd.graph.reverseEdges[targetID] = make(map[string]*FeatureEdge)
	}
	fd.graph.reverseEdges[targetID][sourceID] = edge

	return nil
}

// GetFeatureGraph returns the feature relationship graph for a feature.
func (fd *FeatureDiscovery) GetFeatureGraph(featureID string, depth int) (*FeatureSubgraph, error) {
	if depth <= 0 {
		depth = 1
	}
	if depth > fd.config.MaxGraphDepth {
		depth = fd.config.MaxGraphDepth
	}

	// Verify feature exists
	meta, err := fd.indexer.GetMetadata(featureID)
	if err != nil {
		return nil, err
	}

	subgraph := &FeatureSubgraph{
		RootID: featureID,
		Nodes:  make(map[string]*FeatureNode),
		Edges:  make([]*FeatureEdge, 0),
	}

	// BFS to collect nodes and edges
	visited := make(map[string]bool)
	queue := []struct {
		id    string
		depth int
	}{{featureID, 0}}

	fd.graph.mu.RLock()
	defer fd.graph.mu.RUnlock()

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.id] {
			continue
		}
		visited[current.id] = true

		// Add node
		if node, ok := fd.graph.nodes[current.id]; ok {
			subgraph.Nodes[current.id] = node
		} else {
			// Create node from metadata
			nodeMeta, err := fd.indexer.GetMetadata(current.id)
			if err != nil {
				continue
			}
			subgraph.Nodes[current.id] = &FeatureNode{
				FeatureID:   nodeMeta.FeatureID,
				Name:        nodeMeta.Name,
				Category:    nodeMeta.Category,
				Domain:      nodeMeta.Domain,
				EntityType:  nodeMeta.EntityType,
				LastUpdated: nodeMeta.UpdatedAt,
			}
		}

		// Add edges if not at max depth
		if current.depth < depth {
			if edges, ok := fd.graph.edges[current.id]; ok {
				for targetID, edge := range edges {
					subgraph.Edges = append(subgraph.Edges, edge)
					if !visited[targetID] {
						queue = append(queue, struct {
							id    string
							depth int
						}{targetID, current.depth + 1})
					}
				}
			}

			// Add reverse edges
			if reverseEdges, ok := fd.graph.reverseEdges[current.id]; ok {
				for sourceID, edge := range reverseEdges {
					subgraph.Edges = append(subgraph.Edges, edge)
					if !visited[sourceID] {
						queue = append(queue, struct {
							id    string
							depth int
						}{sourceID, current.depth + 1})
					}
				}
			}
		}
	}

	// Add root node if not present
	if _, ok := subgraph.Nodes[featureID]; !ok {
		subgraph.Nodes[featureID] = &FeatureNode{
			FeatureID:   meta.FeatureID,
			Name:        meta.Name,
			Category:    meta.Category,
			Domain:      meta.Domain,
			EntityType:  meta.EntityType,
			LastUpdated: meta.UpdatedAt,
		}
	}

	return subgraph, nil
}

// FeatureSubgraph represents a subgraph around a feature.
type FeatureSubgraph struct {
	RootID string                  `json:"root_id"`
	Nodes  map[string]*FeatureNode `json:"nodes"`
	Edges  []*FeatureEdge          `json:"edges"`
}

// ComputeGraphCentrality computes centrality scores for all features.
func (fd *FeatureDiscovery) ComputeGraphCentrality() {
	fd.graph.mu.Lock()
	defer fd.graph.mu.Unlock()

	// Simple PageRank-like algorithm
	const iterations = 10
	const dampingFactor = 0.85

	// Initialize scores
	scores := make(map[string]float64)
	for id := range fd.graph.nodes {
		scores[id] = 1.0
	}

	// Also include nodes that only appear in edges
	for sourceID := range fd.graph.edges {
		if _, ok := scores[sourceID]; !ok {
			scores[sourceID] = 1.0
		}
		for targetID := range fd.graph.edges[sourceID] {
			if _, ok := scores[targetID]; !ok {
				scores[targetID] = 1.0
			}
		}
	}

	// Iterate
	for i := 0; i < iterations; i++ {
		newScores := make(map[string]float64)

		for id := range scores {
			// Base score
			newScores[id] = (1 - dampingFactor) / float64(len(scores))

			// Add contributions from incoming edges
			if reverseEdges, ok := fd.graph.reverseEdges[id]; ok {
				for sourceID, edge := range reverseEdges {
					outDegree := len(fd.graph.edges[sourceID])
					if outDegree > 0 {
						contribution := dampingFactor * scores[sourceID] * float64(edge.Weight) / float64(outDegree)
						newScores[id] += contribution
					}
				}
			}
		}

		scores = newScores
	}

	// Normalize and store
	maxScore := 0.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}

	for id, score := range scores {
		normalized := score / maxScore

		node, ok := fd.graph.nodes[id]
		if !ok {
			// Create node
			meta, err := fd.indexer.GetMetadata(id)
			if err != nil {
				continue
			}
			node = &FeatureNode{
				FeatureID:   meta.FeatureID,
				Name:        meta.Name,
				Category:    meta.Category,
				Domain:      meta.Domain,
				EntityType:  meta.EntityType,
				LastUpdated: meta.UpdatedAt,
			}
			fd.graph.nodes[id] = node
		}

		node.Centrality = normalized
	}
}

// GetMostCentralFeatures returns features with highest centrality.
func (fd *FeatureDiscovery) GetMostCentralFeatures(limit int) []*FeatureNode {
	if limit <= 0 {
		limit = 10
	}

	fd.graph.mu.RLock()
	defer fd.graph.mu.RUnlock()

	nodes := make([]*FeatureNode, 0, len(fd.graph.nodes))
	for _, node := range fd.graph.nodes {
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Centrality > nodes[j].Centrality
	})

	if len(nodes) > limit {
		nodes = nodes[:limit]
	}

	return nodes
}

// GetQueryHistory returns recent query history.
func (fd *FeatureDiscovery) GetQueryHistory(userID string, limit int) []QueryHistoryEntry {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var history []QueryHistoryEntry

	// Filter by user if specified
	for i := len(fd.queryHistory) - 1; i >= 0 && len(history) < limit; i-- {
		entry := fd.queryHistory[i]
		if userID == "" || entry.UserID == userID {
			history = append(history, entry)
		}
	}

	return history
}

// GetUserPreferences returns user discovery preferences.
func (fd *FeatureDiscovery) GetUserPreferences(userID string) (*UserDiscoveryPreferences, error) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	prefs, ok := fd.userPreferences[userID]
	if !ok {
		return nil, fmt.Errorf("no preferences for user: %s", userID)
	}

	return prefs, nil
}

// SetUserPreferences sets user discovery preferences.
func (fd *FeatureDiscovery) SetUserPreferences(prefs *UserDiscoveryPreferences) error {
	if prefs.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Initialize maps if nil
	if prefs.FrequentTerms == nil {
		prefs.FrequentTerms = make(map[string]int)
	}
	if prefs.Preferences == nil {
		prefs.Preferences = make(map[string]string)
	}

	fd.mu.Lock()
	defer fd.mu.Unlock()

	fd.userPreferences[prefs.UserID] = prefs
	return nil
}

// RecordFeatureClick records when a user clicks on a feature.
func (fd *FeatureDiscovery) RecordFeatureClick(userID, featureID, queryID string) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	// Update query history
	for i := range fd.queryHistory {
		if fd.queryHistory[i].ID == queryID {
			fd.queryHistory[i].ClickedIDs = append(fd.queryHistory[i].ClickedIDs, featureID)
			break
		}
	}

	// Update user preferences
	if prefs, ok := fd.userPreferences[userID]; ok {
		prefs.FavoriteFeatures = appendUnique(prefs.FavoriteFeatures, featureID)
		if len(prefs.FavoriteFeatures) > 50 {
			prefs.FavoriteFeatures = prefs.FavoriteFeatures[len(prefs.FavoriteFeatures)-50:]
		}
	}

	// Record read in indexer
	fd.indexer.RecordRead(featureID)
}

// GetDiscoveryStats returns discovery engine statistics.
func (fd *FeatureDiscovery) GetDiscoveryStats() map[string]interface{} {
	fd.mu.RLock()
	fd.graph.mu.RLock()
	defer fd.mu.RUnlock()
	defer fd.graph.mu.RUnlock()

	// Count edges by type
	edgesByType := make(map[string]int)
	totalEdges := 0
	for _, edges := range fd.graph.edges {
		for _, edge := range edges {
			edgesByType[string(edge.EdgeType)]++
			totalEdges++
		}
	}

	return map[string]interface{}{
		"total_queries":    len(fd.queryHistory),
		"unique_users":     len(fd.userPreferences),
		"graph_nodes":      len(fd.graph.nodes),
		"graph_edges":      totalEdges,
		"edges_by_type":    edgesByType,
		"indexed_features": fd.indexer.GetStats()["total_features"],
		"config":           fd.config,
	}
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// BuildFeatureEmbeddingIndex builds/rebuilds the embedding index from all features.
func (fd *FeatureDiscovery) BuildFeatureEmbeddingIndex(ctx context.Context) error {
	if fd.embedder == nil {
		fd.logger.Warn("No embedder configured, using fallback TF-IDF")
	}

	// Get all metadata
	allMetadata := fd.indexer.ListMetadata()
	fd.logger.Info("Building embedding index", "feature_count", len(allMetadata))

	// Convert to feature documents and index
	docs := make([]*FeatureDocument, 0, len(allMetadata))
	for _, meta := range allMetadata {
		doc := &FeatureDocument{
			ID:          meta.FeatureID,
			Name:        meta.Name,
			Description: meta.Description,
			Tags:        meta.Tags,
			Category:    meta.Category,
			DataType:    meta.DataType,
			Owner:       meta.Owner,
			Metadata:    meta.CustomFields,
			CreatedAt:   meta.CreatedAt,
			UpdatedAt:   meta.UpdatedAt,
		}
		docs = append(docs, doc)
	}

	// Index in batches
	batchSize := 100
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]

		if err := fd.indexer.Search().IndexBatch(ctx, batch); err != nil {
			return fmt.Errorf("indexing batch %d: %w", i/batchSize, err)
		}

		fd.logger.Debug("Indexed batch", "batch", i/batchSize+1, "features", len(batch))
	}

	// Compute initial graph centrality
	fd.ComputeGraphCentrality()

	fd.logger.Info("Embedding index built successfully", "total_features", len(docs))
	return nil
}

// InferFeatureRelationships automatically infers relationships between features.
func (fd *FeatureDiscovery) InferFeatureRelationships(ctx context.Context) error {
	fd.logger.Info("Inferring feature relationships")

	allMetadata := fd.indexer.ListMetadata()

	// Group features by entity type
	byEntity := make(map[string][]*FeatureMetadata)
	for _, meta := range allMetadata {
		if meta.EntityType != "" {
			byEntity[meta.EntityType] = append(byEntity[meta.EntityType], meta)
		}
	}

	// Create related edges for features with same entity type
	for _, features := range byEntity {
		for i := 0; i < len(features); i++ {
			for j := i + 1; j < len(features); j++ {
				// Calculate similarity based on metadata
				sim := calculateMetadataSimilarity(features[i], features[j])
				if sim >= fd.config.MinEdgeWeight {
					if err := fd.AddFeatureRelationship(features[i].FeatureID, features[j].FeatureID, EdgeTypeRelated, sim); err != nil {
						fd.logger.Warn("failed to add related edge", "source", features[i].FeatureID, "target", features[j].FeatureID, "error", err)
					}
					if err := fd.AddFeatureRelationship(features[j].FeatureID, features[i].FeatureID, EdgeTypeRelated, sim); err != nil {
						fd.logger.Warn("failed to add related edge", "source", features[j].FeatureID, "target", features[i].FeatureID, "error", err)
					}
				}
			}
		}
	}

	// Infer from lineage
	for _, meta := range allMetadata {
		lineage, err := fd.indexer.GetLineage(meta.FeatureID)
		if err != nil {
			continue
		}

		// Add derived edges
		for _, sourceID := range lineage.SourceFeatures {
			if err := fd.AddFeatureRelationship(sourceID, meta.FeatureID, EdgeTypeDerived, 0.8); err != nil {
				fd.logger.Warn("failed to add derived edge", "source", sourceID, "target", meta.FeatureID, "error", err)
			}
		}
	}

	// Infer from usage (co-used features)
	modelToFeatures := make(map[string][]string)
	for _, meta := range allMetadata {
		usage, err := fd.indexer.GetUsage(meta.FeatureID)
		if err != nil {
			continue
		}
		for _, model := range usage.ModelsUsing {
			modelToFeatures[model] = append(modelToFeatures[model], meta.FeatureID)
		}
	}

	addedPairs := make(map[string]struct{})
	for _, features := range modelToFeatures {
		if len(features) < 2 {
			continue
		}
		for i := 0; i < len(features); i++ {
			for j := i + 1; j < len(features); j++ {
				source := features[i]
				target := features[j]
				if source == target {
					continue
				}

				keyA, keyB := source, target
				if keyA > keyB {
					keyA, keyB = keyB, keyA
				}
				pairKey := keyA + "|" + keyB
				if _, ok := addedPairs[pairKey]; ok {
					continue
				}
				addedPairs[pairKey] = struct{}{}

				if err := fd.AddFeatureRelationship(source, target, EdgeTypeCoUsed, 0.7); err != nil {
					fd.logger.Warn("failed to add co-used edge", "source", source, "target", target, "error", err)
				}
				if err := fd.AddFeatureRelationship(target, source, EdgeTypeCoUsed, 0.7); err != nil {
					fd.logger.Warn("failed to add co-used edge", "source", target, "target", source, "error", err)
				}
			}
		}
	}

	// Recompute centrality
	fd.ComputeGraphCentrality()

	fd.logger.Info("Feature relationships inferred", "edge_count", fd.GetDiscoveryStats()["graph_edges"])
	return nil
}

func calculateMetadataSimilarity(a, b *FeatureMetadata) float32 {
	var score float32
	var factors float32

	// Same category
	if a.Category != "" && a.Category == b.Category {
		score += 0.3
	}
	factors += 0.3

	// Same domain
	if a.Domain != "" && a.Domain == b.Domain {
		score += 0.3
	}
	factors += 0.3

	// Overlapping tags
	tagOverlap := 0
	for _, tagA := range a.Tags {
		for _, tagB := range b.Tags {
			if strings.EqualFold(tagA, tagB) {
				tagOverlap++
				break
			}
		}
	}
	if len(a.Tags) > 0 || len(b.Tags) > 0 {
		maxTags := math.Max(float64(len(a.Tags)), float64(len(b.Tags)))
		score += 0.2 * float32(float64(tagOverlap)/maxTags)
	}
	factors += 0.2

	// Same use case
	useCaseOverlap := 0
	for _, ucA := range a.UseCase {
		for _, ucB := range b.UseCase {
			if strings.EqualFold(ucA, ucB) {
				useCaseOverlap++
				break
			}
		}
	}
	if len(a.UseCase) > 0 || len(b.UseCase) > 0 {
		maxUC := math.Max(float64(len(a.UseCase)), float64(len(b.UseCase)))
		score += 0.2 * float32(float64(useCaseOverlap)/maxUC)
	}
	factors += 0.2

	return score / factors
}
