package semantic

import (
	"context"
	"fmt"
	"log/slog"
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
