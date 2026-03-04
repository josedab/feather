package semantic

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// FeatureSubgraph represents a subgraph around a feature.
type FeatureSubgraph struct {
	RootID string                  `json:"root_id"`
	Nodes  map[string]*FeatureNode `json:"nodes"`
	Edges  []*FeatureEdge          `json:"edges"`
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
