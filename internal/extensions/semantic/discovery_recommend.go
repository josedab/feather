package semantic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

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
