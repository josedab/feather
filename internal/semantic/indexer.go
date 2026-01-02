package semantic

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EnhancedIndexer extends the semantic search with rich feature metadata.
type EnhancedIndexer struct {
	search     *Search
	metadata   map[string]*FeatureMetadata
	statistics map[string]*FeatureStatistics
	lineage    map[string]*FeatureLineage
	usage      map[string]*FeatureUsage
	mu         sync.RWMutex
}

// FeatureMetadata contains rich metadata about a feature.
type FeatureMetadata struct {
	// Core info
	FeatureID   string `json:"feature_id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Schema info
	DataType   string `json:"data_type"`
	ValueType  string `json:"value_type"` // numeric, categorical, vector, boolean
	EntityType string `json:"entity_type"`
	Nullable   bool   `json:"nullable"`

	// Classification
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory,omitempty"`
	Tags        []string `json:"tags"`
	Labels      []string `json:"labels,omitempty"`

	// Domain info
	Domain       string   `json:"domain,omitempty"`        // e.g., "user", "product", "transaction"
	BusinessUnit string   `json:"business_unit,omitempty"` // e.g., "marketing", "fraud", "recommendations"
	UseCase      []string `json:"use_case,omitempty"`      // e.g., ["fraud_detection", "personalization"]

	// Ownership
	Owner       string   `json:"owner"`
	Team        string   `json:"team,omitempty"`
	Maintainers []string `json:"maintainers,omitempty"`

	// Quality
	QualityScore float32 `json:"quality_score"` // 0-1 based on completeness, freshness
	DataQuality  string  `json:"data_quality"`  // "high", "medium", "low"
	Completeness float32 `json:"completeness"`  // % of non-null values
	Freshness    string  `json:"freshness"`     // "real-time", "hourly", "daily", "weekly"

	// Temporal
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastComputedAt time.Time `json:"last_computed_at,omitempty"`

	// Source info
	Source         string `json:"source,omitempty"` // e.g., "kafka", "batch", "derived"
	SourceTable    string `json:"source_table,omitempty"`
	TransformLogic string `json:"transform_logic,omitempty"`

	// Documentation
	Documentation   string            `json:"documentation,omitempty"`
	Examples        []string          `json:"examples,omitempty"`
	RelatedFeatures []string          `json:"related_features,omitempty"`
	CustomFields    map[string]string `json:"custom_fields,omitempty"`
}

// FeatureStatistics contains statistical information about a feature.
type FeatureStatistics struct {
	FeatureID string `json:"feature_id"`

	// Numeric stats
	Min         float64            `json:"min,omitempty"`
	Max         float64            `json:"max,omitempty"`
	Mean        float64            `json:"mean,omitempty"`
	Median      float64            `json:"median,omitempty"`
	StdDev      float64            `json:"std_dev,omitempty"`
	Percentiles map[string]float64 `json:"percentiles,omitempty"` // "p50", "p90", "p99"

	// Categorical stats
	UniqueCount int64            `json:"unique_count,omitempty"`
	TopValues   []TopValue       `json:"top_values,omitempty"`
	ValueCounts map[string]int64 `json:"value_counts,omitempty"`

	// Distribution
	Distribution string  `json:"distribution,omitempty"` // "normal", "skewed", "bimodal", "uniform"
	Skewness     float64 `json:"skewness,omitempty"`
	Kurtosis     float64 `json:"kurtosis,omitempty"`

	// Nulls and outliers
	NullCount      int64   `json:"null_count"`
	NullPercentage float64 `json:"null_percentage"`
	OutlierCount   int64   `json:"outlier_count,omitempty"`

	// Time series stats
	Volatility  float64 `json:"volatility,omitempty"`
	Trend       string  `json:"trend,omitempty"` // "increasing", "decreasing", "stable"
	Seasonality string  `json:"seasonality,omitempty"`

	// Sample info
	SampleSize int64     `json:"sample_size"`
	ComputedAt time.Time `json:"computed_at"`
}

// TopValue represents a frequent value.
type TopValue struct {
	Value string  `json:"value"`
	Count int64   `json:"count"`
	Pct   float64 `json:"percentage"`
}

// FeatureLineage tracks feature derivation and dependencies.
type FeatureLineage struct {
	FeatureID string `json:"feature_id"`

	// Upstream dependencies
	SourceFeatures  []string `json:"source_features,omitempty"`
	SourceTables    []string `json:"source_tables,omitempty"`
	ExternalSources []string `json:"external_sources,omitempty"`

	// Downstream dependents
	DependentFeatures []string `json:"dependent_features,omitempty"`
	DependentModels   []string `json:"dependent_models,omitempty"`

	// Transformation
	TransformationType string `json:"transformation_type,omitempty"` // "aggregation", "join", "derived", "raw"
	TransformationCode string `json:"transformation_code,omitempty"`
	Version            string `json:"version,omitempty"`

	// Pipeline info
	PipelineID   string    `json:"pipeline_id,omitempty"`
	PipelineName string    `json:"pipeline_name,omitempty"`
	LastRun      time.Time `json:"last_run,omitempty"`
}

// FeatureUsage tracks how a feature is being used.
type FeatureUsage struct {
	FeatureID string `json:"feature_id"`

	// Access patterns
	TotalReads    int64     `json:"total_reads"`
	ReadRate      float64   `json:"read_rate"` // per second
	LastReadAt    time.Time `json:"last_read_at"`
	UniqueReaders int64     `json:"unique_readers"` // unique clients/models

	// Model usage
	ModelsUsing []string `json:"models_using,omitempty"`
	ModelCount  int      `json:"model_count"`

	// Popularity
	PopularityScore float64 `json:"popularity_score"` // 0-1 based on usage
	TrendScore      float64 `json:"trend_score"`      // recent usage trend

	// Importance
	ImportanceScore float64  `json:"importance_score"` // based on model importance
	CriticalModels  []string `json:"critical_models,omitempty"`

	// Time window
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// NewEnhancedIndexer creates a new enhanced indexer.
func NewEnhancedIndexer(search *Search) *EnhancedIndexer {
	return &EnhancedIndexer{
		search:     search,
		metadata:   make(map[string]*FeatureMetadata),
		statistics: make(map[string]*FeatureStatistics),
		lineage:    make(map[string]*FeatureLineage),
		usage:      make(map[string]*FeatureUsage),
	}
}

// Search returns the underlying search instance.
func (i *EnhancedIndexer) Search() *Search {
	return i.search
}

// IndexFeatureWithMetadata indexes a feature with rich metadata.
func (i *EnhancedIndexer) IndexFeatureWithMetadata(ctx context.Context, meta *FeatureMetadata) error {
	if meta.FeatureID == "" {
		return fmt.Errorf("feature ID is required")
	}

	// Create feature document from metadata
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

	// Index in search
	if err := i.search.IndexFeature(ctx, doc); err != nil {
		return err
	}

	// Store metadata
	i.mu.Lock()
	i.metadata[meta.FeatureID] = meta
	i.mu.Unlock()

	return nil
}

// SetStatistics sets statistical information for a feature.
func (i *EnhancedIndexer) SetStatistics(featureID string, stats *FeatureStatistics) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.metadata[featureID]; !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	stats.FeatureID = featureID
	if stats.ComputedAt.IsZero() {
		stats.ComputedAt = time.Now()
	}
	i.statistics[featureID] = stats

	return nil
}

// SetLineage sets lineage information for a feature.
func (i *EnhancedIndexer) SetLineage(featureID string, lineage *FeatureLineage) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.metadata[featureID]; !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	lineage.FeatureID = featureID
	i.lineage[featureID] = lineage

	return nil
}

// SetUsage sets usage information for a feature.
func (i *EnhancedIndexer) SetUsage(featureID string, usage *FeatureUsage) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.metadata[featureID]; !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	usage.FeatureID = featureID
	i.usage[featureID] = usage

	return nil
}

// RecordRead records a read access for a feature.
func (i *EnhancedIndexer) RecordRead(featureID string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	usage, ok := i.usage[featureID]
	if !ok {
		usage = &FeatureUsage{
			FeatureID:   featureID,
			WindowStart: time.Now(),
		}
		i.usage[featureID] = usage
	}

	usage.TotalReads++
	usage.LastReadAt = time.Now()
}

// GetMetadata returns metadata for a feature.
func (i *EnhancedIndexer) GetMetadata(featureID string) (*FeatureMetadata, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	meta, ok := i.metadata[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}
	return meta, nil
}

// GetStatistics returns statistics for a feature.
func (i *EnhancedIndexer) GetStatistics(featureID string) (*FeatureStatistics, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	stats, ok := i.statistics[featureID]
	if !ok {
		return nil, fmt.Errorf("statistics not found: %s", featureID)
	}
	return stats, nil
}

// GetLineage returns lineage for a feature.
func (i *EnhancedIndexer) GetLineage(featureID string) (*FeatureLineage, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	lineage, ok := i.lineage[featureID]
	if !ok {
		return nil, fmt.Errorf("lineage not found: %s", featureID)
	}
	return lineage, nil
}

// GetUsage returns usage for a feature.
func (i *EnhancedIndexer) GetUsage(featureID string) (*FeatureUsage, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	usage, ok := i.usage[featureID]
	if !ok {
		return nil, fmt.Errorf("usage not found: %s", featureID)
	}
	return usage, nil
}

// GetEnrichedFeature returns a feature with all enrichment data.
func (i *EnhancedIndexer) GetEnrichedFeature(featureID string) (*EnrichedFeature, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	meta, ok := i.metadata[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	return &EnrichedFeature{
		Metadata:   meta,
		Statistics: i.statistics[featureID],
		Lineage:    i.lineage[featureID],
		Usage:      i.usage[featureID],
	}, nil
}

// EnrichedFeature combines all feature information.
type EnrichedFeature struct {
	Metadata   *FeatureMetadata   `json:"metadata"`
	Statistics *FeatureStatistics `json:"statistics,omitempty"`
	Lineage    *FeatureLineage    `json:"lineage,omitempty"`
	Usage      *FeatureUsage      `json:"usage,omitempty"`
}

// ListMetadata returns all metadata.
func (i *EnhancedIndexer) ListMetadata() []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := make([]*FeatureMetadata, 0, len(i.metadata))
	for _, m := range i.metadata {
		result = append(result, m)
	}
	return result
}

// FindByEntityType returns features for a specific entity type.
func (i *EnhancedIndexer) FindByEntityType(entityType string) []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var result []*FeatureMetadata
	for _, m := range i.metadata {
		if m.EntityType == entityType {
			result = append(result, m)
		}
	}
	return result
}

// FindByDomain returns features for a specific domain.
func (i *EnhancedIndexer) FindByDomain(domain string) []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var result []*FeatureMetadata
	for _, m := range i.metadata {
		if m.Domain == domain {
			result = append(result, m)
		}
	}
	return result
}

// FindByUseCase returns features for a specific use case.
func (i *EnhancedIndexer) FindByUseCase(useCase string) []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var result []*FeatureMetadata
	for _, m := range i.metadata {
		for _, uc := range m.UseCase {
			if uc == useCase {
				result = append(result, m)
				break
			}
		}
	}
	return result
}

// GetMostPopular returns the most popular features.
func (i *EnhancedIndexer) GetMostPopular(limit int) []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	type scored struct {
		meta  *FeatureMetadata
		score float64
	}

	var scored_list []scored
	for id, meta := range i.metadata {
		usage := i.usage[id]
		var score float64
		if usage != nil {
			score = usage.PopularityScore
		}
		scored_list = append(scored_list, scored{meta: meta, score: score})
	}

	sort.Slice(scored_list, func(a, b int) bool {
		return scored_list[a].score > scored_list[b].score
	})

	if len(scored_list) > limit {
		scored_list = scored_list[:limit]
	}

	result := make([]*FeatureMetadata, len(scored_list))
	for idx, s := range scored_list {
		result[idx] = s.meta
	}
	return result
}

// GetHighQuality returns high quality features.
func (i *EnhancedIndexer) GetHighQuality(minScore float32) []*FeatureMetadata {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var result []*FeatureMetadata
	for _, m := range i.metadata {
		if m.QualityScore >= minScore {
			result = append(result, m)
		}
	}
	return result
}

// GetStats returns indexer statistics.
func (i *EnhancedIndexer) GetStats() map[string]interface{} {
	i.mu.RLock()
	defer i.mu.RUnlock()

	byEntityType := make(map[string]int)
	byDomain := make(map[string]int)
	byCategory := make(map[string]int)
	byDataQuality := make(map[string]int)

	for _, m := range i.metadata {
		byEntityType[m.EntityType]++
		byDomain[m.Domain]++
		byCategory[m.Category]++
		byDataQuality[m.DataQuality]++
	}

	return map[string]interface{}{
		"total_features":  len(i.metadata),
		"with_statistics": len(i.statistics),
		"with_lineage":    len(i.lineage),
		"with_usage":      len(i.usage),
		"by_entity_type":  byEntityType,
		"by_domain":       byDomain,
		"by_category":     byCategory,
		"by_data_quality": byDataQuality,
	}
}
