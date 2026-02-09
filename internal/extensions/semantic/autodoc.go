package semantic

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AutoDocConfig configures auto-documentation generation.
type AutoDocConfig struct {
	MinUsageCount     int     `json:"min_usage_count"`
	SimilarityThresh  float64 `json:"similarity_threshold"`
	MaxRelatedFeatures int    `json:"max_related_features"`
}

// DefaultAutoDocConfig returns sensible defaults.
func DefaultAutoDocConfig() AutoDocConfig {
	return AutoDocConfig{
		MinUsageCount:      5,
		SimilarityThresh:   0.7,
		MaxRelatedFeatures: 10,
	}
}

// FeatureDocumentation is auto-generated documentation for a feature.
type FeatureDocumentation struct {
	FeatureName     string              `json:"feature_name"`
	FeatureGroup    string              `json:"feature_group,omitempty"`
	Summary         string              `json:"summary"`
	Description     string              `json:"description"`
	DataType        string              `json:"data_type,omitempty"`
	UsagePatterns   []UsagePattern      `json:"usage_patterns,omitempty"`
	RelatedFeatures []RelatedFeature    `json:"related_features,omitempty"`
	LineageInfo     *LineageDocInfo      `json:"lineage,omitempty"`
	QualityMetrics  *QualityDocMetrics   `json:"quality_metrics,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// UsagePattern describes how a feature is used.
type UsagePattern struct {
	Pattern     string  `json:"pattern"`     // "batch_prediction", "online_serving", "analytics"
	Frequency   int64   `json:"frequency"`
	Description string  `json:"description"`
}

// RelatedFeature describes a feature related by usage or lineage.
type RelatedFeature struct {
	Name       string  `json:"name"`
	Similarity float64 `json:"similarity"`
	Relation   string  `json:"relation"` // "co-used", "derived", "upstream", "downstream"
}

// LineageDocInfo describes feature lineage for documentation.
type LineageDocInfo struct {
	Sources      []string `json:"sources,omitempty"`
	Transforms   []string `json:"transforms,omitempty"`
	Consumers    []string `json:"consumers,omitempty"`
}

// QualityDocMetrics describes feature quality for documentation.
type QualityDocMetrics struct {
	Completeness float64 `json:"completeness"`
	Freshness    string  `json:"freshness"`
	DriftStatus  string  `json:"drift_status"`
}

// AutoDocGenerator generates documentation from usage patterns and metadata.
type AutoDocGenerator struct {
	mu            sync.RWMutex
	config        AutoDocConfig
	usageTracker  map[string][]UsageEvent  // feature -> usage events
	coUsageMatrix map[string]map[string]int // feature -> co-used features -> count
	docs          map[string]*FeatureDocumentation
}

// UsageEvent tracks a feature usage event.
type UsageEvent struct {
	FeatureName string    `json:"feature_name"`
	UsedWith    []string  `json:"used_with,omitempty"`
	Context     string    `json:"context"` // "prediction", "training", "analytics"
	Timestamp   time.Time `json:"timestamp"`
}

// NewAutoDocGenerator creates a new auto-documentation generator.
func NewAutoDocGenerator(config AutoDocConfig) *AutoDocGenerator {
	if config.MinUsageCount == 0 {
		config = DefaultAutoDocConfig()
	}
	return &AutoDocGenerator{
		config:        config,
		usageTracker:  make(map[string][]UsageEvent),
		coUsageMatrix: make(map[string]map[string]int),
		docs:          make(map[string]*FeatureDocumentation),
	}
}

// RecordUsage records a feature usage event for documentation analysis.
func (g *AutoDocGenerator) RecordUsage(event UsageEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.usageTracker[event.FeatureName] = append(g.usageTracker[event.FeatureName], event)

	// Track co-usage
	for _, coFeature := range event.UsedWith {
		if g.coUsageMatrix[event.FeatureName] == nil {
			g.coUsageMatrix[event.FeatureName] = make(map[string]int)
		}
		g.coUsageMatrix[event.FeatureName][coFeature]++

		if g.coUsageMatrix[coFeature] == nil {
			g.coUsageMatrix[coFeature] = make(map[string]int)
		}
		g.coUsageMatrix[coFeature][event.FeatureName]++
	}
}

// GenerateDoc auto-generates documentation for a feature.
func (g *AutoDocGenerator) GenerateDoc(featureName string) *FeatureDocumentation {
	g.mu.RLock()
	defer g.mu.RUnlock()

	doc := &FeatureDocumentation{
		FeatureName: featureName,
		GeneratedAt: time.Now(),
	}

	events := g.usageTracker[featureName]
	if len(events) == 0 {
		doc.Summary = fmt.Sprintf("Feature %s (no usage data available)", featureName)
		doc.Description = "This feature has not been used yet. Documentation will be auto-generated as usage patterns emerge."
		return doc
	}

	// Analyze usage patterns
	contextCounts := make(map[string]int64)
	for _, e := range events {
		contextCounts[e.Context]++
	}

	var patterns []UsagePattern
	for ctx, count := range contextCounts {
		patterns = append(patterns, UsagePattern{
			Pattern:     ctx,
			Frequency:   count,
			Description: describeContext(ctx, count),
		})
	}
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})
	doc.UsagePatterns = patterns

	// Find related features from co-usage
	coUsage := g.coUsageMatrix[featureName]
	type coUsagePair struct {
		name  string
		count int
	}
	var pairs []coUsagePair
	for name, count := range coUsage {
		pairs = append(pairs, coUsagePair{name, count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	maxRelated := g.config.MaxRelatedFeatures
	if len(pairs) < maxRelated {
		maxRelated = len(pairs)
	}
	for _, p := range pairs[:maxRelated] {
		sim := float64(p.count) / float64(len(events))
		if sim > 1.0 {
			sim = 1.0
		}
		doc.RelatedFeatures = append(doc.RelatedFeatures, RelatedFeature{
			Name:       p.name,
			Similarity: sim,
			Relation:   "co-used",
		})
	}

	// Generate summary and description
	primaryContext := "general"
	if len(patterns) > 0 {
		primaryContext = patterns[0].Pattern
	}
	doc.Summary = fmt.Sprintf("%s — primarily used for %s (%d total uses)", featureName, primaryContext, len(events))
	doc.Description = generateDescription(featureName, patterns, doc.RelatedFeatures)

	// Generate tags from usage context
	for ctx := range contextCounts {
		doc.Tags = append(doc.Tags, ctx)
	}

	// Cache the doc
	g.mu.RUnlock()
	g.mu.Lock()
	g.docs[featureName] = doc
	g.mu.Unlock()
	g.mu.RLock()

	return doc
}

// GenerateAll generates documentation for all tracked features.
func (g *AutoDocGenerator) GenerateAll() map[string]*FeatureDocumentation {
	g.mu.RLock()
	features := make([]string, 0, len(g.usageTracker))
	for f := range g.usageTracker {
		features = append(features, f)
	}
	g.mu.RUnlock()

	result := make(map[string]*FeatureDocumentation, len(features))
	for _, f := range features {
		result[f] = g.GenerateDoc(f)
	}
	return result
}

// GetDoc returns cached documentation for a feature.
func (g *AutoDocGenerator) GetDoc(featureName string) (*FeatureDocumentation, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	doc, exists := g.docs[featureName]
	return doc, exists
}

func describeContext(ctx string, count int64) string {
	switch ctx {
	case "prediction":
		return fmt.Sprintf("Used in %d prediction requests for real-time inference", count)
	case "training":
		return fmt.Sprintf("Used in %d training jobs for model development", count)
	case "analytics":
		return fmt.Sprintf("Used in %d analytics queries for reporting", count)
	case "batch_prediction":
		return fmt.Sprintf("Used in %d batch prediction pipelines", count)
	default:
		return fmt.Sprintf("Used %d times in %s context", count, ctx)
	}
}

func generateDescription(name string, patterns []UsagePattern, related []RelatedFeature) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("**%s** is a feature", name))

	if len(patterns) > 0 {
		contexts := make([]string, 0, len(patterns))
		for _, p := range patterns {
			contexts = append(contexts, p.Pattern)
		}
		parts = append(parts, fmt.Sprintf("used in %s workflows", strings.Join(contexts, ", ")))
	}

	if len(related) > 0 {
		relNames := make([]string, 0, 3)
		for i, r := range related {
			if i >= 3 {
				break
			}
			relNames = append(relNames, r.Name)
		}
		parts = append(parts, fmt.Sprintf("Commonly used alongside: %s", strings.Join(relNames, ", ")))
	}

	return strings.Join(parts, ". ") + "."
}
