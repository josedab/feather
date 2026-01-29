package catalog

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// FeatureStatus represents the lifecycle state of a catalog feature.
type FeatureStatus string

const (
	// FeatureStatusActive is an actively used feature.
	FeatureStatusActive FeatureStatus = "active"
	// FeatureStatusDeprecated is a feature scheduled for removal.
	FeatureStatusDeprecated FeatureStatus = "deprecated"
	// FeatureStatusExperimental is a feature still under development.
	FeatureStatusExperimental FeatureStatus = "experimental"
	// FeatureStatusArchived is a feature that has been retired.
	FeatureStatusArchived FeatureStatus = "archived"
)

// QualityScore represents quality metrics for a catalog feature.
type QualityScore struct {
	Overall      float64 `json:"overall"`      // 0-1
	Freshness    float64 `json:"freshness"`    // 0-1
	Completeness float64 `json:"completeness"` // 0-1
	Consistency  float64 `json:"consistency"`  // 0-1
}

// CatalogEntry represents a feature registered in the catalog.
type CatalogEntry struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DataType     string            `json:"data_type"`
	Entity       string            `json:"entity"`
	Owner        string            `json:"owner"`
	Tags         []string          `json:"tags"`
	Source       string            `json:"source"`
	Status       FeatureStatus     `json:"status"`
	Quality      QualityScore      `json:"quality"`
	UsageCount   int64             `json:"usage_count"`
	LastAccessed time.Time         `json:"last_accessed"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Dependencies []string          `json:"dependencies"`
	Dependents   []string          `json:"dependents"`
	SampleValues []interface{}     `json:"sample_values,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SearchQuery defines criteria for catalog searches.
type SearchQuery struct {
	Text     string   `json:"text"`
	Tags     []string `json:"tags,omitempty"`
	Owner    string   `json:"owner,omitempty"`
	Status   string   `json:"status,omitempty"`
	DataType string   `json:"data_type,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// SearchResult contains paginated search results.
type SearchResult struct {
	Entries []*CatalogEntry `json:"entries"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

// UsageRecord tracks access patterns for a single feature.
type UsageRecord struct {
	FeatureName string    `json:"feature_name"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	Consumers   []string  `json:"consumers"`
	Trend       string    `json:"trend"` // up/down/stable
}

// LineageNode represents a node in the lineage graph.
type LineageNode struct {
	Name     string `json:"name"`
	NodeType string `json:"type"` // feature/source/model
	Level    int    `json:"level"`
}

// LineageEdge represents a directed edge in the lineage graph.
type LineageEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // derives_from/feeds_into
}

// LineageResult contains the lineage subgraph for a feature.
type LineageResult struct {
	Nodes []LineageNode `json:"nodes"`
	Edges []LineageEdge `json:"edges"`
	Root  string        `json:"root"`
	Depth int           `json:"depth"`
}

// CatalogStats contains aggregate catalog statistics.
type CatalogStats struct {
	TotalFeatures   int            `json:"total_features"`
	ActiveFeatures  int            `json:"active_features"`
	DeprecatedCount int            `json:"deprecated_count"`
	ByOwner         map[string]int `json:"by_owner"`
	ByStatus        map[string]int `json:"by_status"`
	ByDataType      map[string]int `json:"by_data_type"`
	TopUsed         []string       `json:"top_used"`
	AvgQualityScore float64        `json:"avg_quality_score"`
}

// Config holds configuration for the catalog service.
type Config struct {
	MaxSearchResults    int
	EnableLineage       bool
	EnableUsageTracking bool
	RetentionDays       int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxSearchResults:    100,
		EnableLineage:       true,
		EnableUsageTracking: true,
		RetentionDays:       90,
	}
}

// UsageTracker tracks feature access patterns.
type UsageTracker struct {
	mu      sync.RWMutex
	records map[string]*UsageRecord
}

// LineageGraph tracks feature dependency relationships.
type LineageGraph struct {
	mu    sync.RWMutex
	nodes map[string]*LineageNode
	edges []LineageEdge
}

// Service provides feature catalog functionality including
// search, lineage visualization, usage analytics, and team collaboration.
type Service struct {
	config       Config
	mu           sync.RWMutex
	features     map[string]*CatalogEntry
	tags         map[string][]string // tag -> feature names
	owners       map[string][]string // team -> feature names
	usageTracker *UsageTracker
	lineageGraph *LineageGraph
}

// NewService creates a new catalog service.
func NewService(cfg Config) *Service {
	return &Service{
		config:   cfg,
		features: make(map[string]*CatalogEntry),
		tags:     make(map[string][]string),
		owners:   make(map[string][]string),
		usageTracker: &UsageTracker{
			records: make(map[string]*UsageRecord),
		},
		lineageGraph: &LineageGraph{
			nodes: make(map[string]*LineageNode),
		},
	}
}

// Register adds or updates a feature in the catalog.
func (s *Service) Register(entry CatalogEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("feature name is required")
	}
	if entry.DataType == "" {
		return fmt.Errorf("feature data type is required")
	}
	if entry.Owner == "" {
		return fmt.Errorf("feature owner is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if existing, ok := s.features[entry.Name]; ok {
		entry.CreatedAt = existing.CreatedAt
		entry.UsageCount = existing.UsageCount
	} else {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	if entry.Status == "" {
		entry.Status = FeatureStatusActive
	}

	s.features[entry.Name] = &entry

	// Update tag index.
	for _, tag := range entry.Tags {
		s.tags[tag] = addUnique(s.tags[tag], entry.Name)
	}

	// Update owner index.
	s.owners[entry.Owner] = addUnique(s.owners[entry.Owner], entry.Name)

	// Update lineage graph.
	if s.config.EnableLineage {
		s.lineageGraph.mu.Lock()
		s.lineageGraph.nodes[entry.Name] = &LineageNode{
			Name:     entry.Name,
			NodeType: "feature",
		}
		for _, dep := range entry.Dependencies {
			if _, ok := s.lineageGraph.nodes[dep]; !ok {
				s.lineageGraph.nodes[dep] = &LineageNode{
					Name:     dep,
					NodeType: "source",
				}
			}
			s.lineageGraph.edges = append(s.lineageGraph.edges, LineageEdge{
				From: dep,
				To:   entry.Name,
				Type: "derives_from",
			})
		}
		s.lineageGraph.mu.Unlock()
	}

	return nil
}

// Get retrieves a feature by name.
func (s *Service) Get(name string) (*CatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.features[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not found", name)
	}
	return entry, nil
}

// Search finds features matching the given query.
func (s *Service) Search(query SearchQuery) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := query.Limit
	if limit <= 0 {
		limit = s.config.MaxSearchResults
	}
	if limit > s.config.MaxSearchResults {
		limit = s.config.MaxSearchResults
	}

	var matched []*CatalogEntry
	for _, entry := range s.features {
		if !s.matchesQuery(entry, query) {
			continue
		}
		matched = append(matched, entry)
	}

	// Sort by name for deterministic results.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})

	total := len(matched)
	offset := query.Offset
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := matched[offset:end]

	return &SearchResult{
		Entries: page,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// Delete removes a feature from the catalog.
func (s *Service) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.features[name]
	if !ok {
		return fmt.Errorf("feature %q not found", name)
	}

	// Remove from tag index.
	for _, tag := range entry.Tags {
		s.tags[tag] = removeFromSlice(s.tags[tag], name)
	}

	// Remove from owner index.
	s.owners[entry.Owner] = removeFromSlice(s.owners[entry.Owner], name)

	// Remove from lineage graph.
	if s.config.EnableLineage {
		s.lineageGraph.mu.Lock()
		delete(s.lineageGraph.nodes, name)
		filtered := s.lineageGraph.edges[:0]
		for _, e := range s.lineageGraph.edges {
			if e.From != name && e.To != name {
				filtered = append(filtered, e)
			}
		}
		s.lineageGraph.edges = filtered
		s.lineageGraph.mu.Unlock()
	}

	delete(s.features, name)
	return nil
}

// ListAll returns all catalog entries.
func (s *Service) ListAll() []*CatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CatalogEntry, 0, len(s.features))
	for _, entry := range s.features {
		result = append(result, entry)
	}
	return result
}

// GetByOwner returns all features owned by the given team.
func (s *Service) GetByOwner(owner string) []*CatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := s.owners[owner]
	result := make([]*CatalogEntry, 0, len(names))
	for _, name := range names {
		if entry, ok := s.features[name]; ok {
			result = append(result, entry)
		}
	}
	return result
}

// GetByTag returns all features with the given tag.
func (s *Service) GetByTag(tag string) []*CatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := s.tags[tag]
	result := make([]*CatalogEntry, 0, len(names))
	for _, name := range names {
		if entry, ok := s.features[name]; ok {
			result = append(result, entry)
		}
	}
	return result
}

// RecordUsage records a feature access by a consumer.
func (s *Service) RecordUsage(featureName, consumer string) {
	s.mu.RLock()
	entry, ok := s.features[featureName]
	s.mu.RUnlock()
	if !ok {
		return
	}

	now := time.Now()

	s.mu.Lock()
	entry.UsageCount++
	entry.LastAccessed = now
	s.mu.Unlock()

	if !s.config.EnableUsageTracking {
		return
	}

	s.usageTracker.mu.Lock()
	defer s.usageTracker.mu.Unlock()

	rec, ok := s.usageTracker.records[featureName]
	if !ok {
		rec = &UsageRecord{
			FeatureName: featureName,
			Trend:       "stable",
		}
		s.usageTracker.records[featureName] = rec
	}
	rec.AccessCount++
	rec.LastAccess = now
	rec.Consumers = addUnique(rec.Consumers, consumer)
}

// GetUsageStats returns usage records for all tracked features.
func (s *Service) GetUsageStats() map[string]*UsageRecord {
	s.usageTracker.mu.RLock()
	defer s.usageTracker.mu.RUnlock()

	result := make(map[string]*UsageRecord, len(s.usageTracker.records))
	for k, v := range s.usageTracker.records {
		result[k] = v
	}
	return result
}

// GetLineage returns the lineage subgraph for a feature.
func (s *Service) GetLineage(featureName string) (*LineageResult, error) {
	if !s.config.EnableLineage {
		return nil, fmt.Errorf("lineage tracking is disabled")
	}

	s.mu.RLock()
	_, ok := s.features[featureName]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("feature %q not found", featureName)
	}

	s.lineageGraph.mu.RLock()
	defer s.lineageGraph.mu.RUnlock()

	visited := make(map[string]bool)
	var nodes []LineageNode
	var edges []LineageEdge
	maxDepth := 0

	// Traverse upstream (dependencies).
	s.traverseUpstream(featureName, visited, &nodes, &edges, 0, &maxDepth)
	// Traverse downstream (dependents).
	s.traverseDownstream(featureName, visited, &nodes, &edges, 0, &maxDepth)

	return &LineageResult{
		Nodes: nodes,
		Edges: edges,
		Root:  featureName,
		Depth: maxDepth,
	}, nil
}

// Stats returns aggregate catalog statistics.
func (s *Service) Stats() CatalogStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := CatalogStats{
		TotalFeatures: len(s.features),
		ByOwner:       make(map[string]int),
		ByStatus:      make(map[string]int),
		ByDataType:    make(map[string]int),
	}

	var qualitySum float64
	type usageEntry struct {
		name  string
		count int64
	}
	var usageList []usageEntry

	for _, entry := range s.features {
		switch entry.Status {
		case FeatureStatusActive:
			stats.ActiveFeatures++
		case FeatureStatusDeprecated:
			stats.DeprecatedCount++
		}
		stats.ByOwner[entry.Owner]++
		stats.ByStatus[string(entry.Status)]++
		stats.ByDataType[entry.DataType]++
		qualitySum += entry.Quality.Overall
		usageList = append(usageList, usageEntry{name: entry.Name, count: entry.UsageCount})
	}

	if stats.TotalFeatures > 0 {
		stats.AvgQualityScore = qualitySum / float64(stats.TotalFeatures)
	}

	sort.Slice(usageList, func(i, j int) bool {
		return usageList[i].count > usageList[j].count
	})
	topN := 5
	if len(usageList) < topN {
		topN = len(usageList)
	}
	stats.TopUsed = make([]string, topN)
	for i := 0; i < topN; i++ {
		stats.TopUsed[i] = usageList[i].name
	}

	return stats
}

// matchesQuery checks if an entry matches the search criteria.
func (s *Service) matchesQuery(entry *CatalogEntry, q SearchQuery) bool {
	if q.Status != "" && string(entry.Status) != q.Status {
		return false
	}
	if q.Owner != "" && entry.Owner != q.Owner {
		return false
	}
	if q.DataType != "" && entry.DataType != q.DataType {
		return false
	}
	if len(q.Tags) > 0 && !hasAnyTag(entry.Tags, q.Tags) {
		return false
	}
	if q.Text != "" {
		if !containsIgnoreCase(entry.Name, q.Text) &&
			!containsIgnoreCase(entry.Description, q.Text) &&
			!tagsContain(entry.Tags, q.Text) {
			return false
		}
	}
	return true
}

// traverseUpstream walks upstream dependencies recursively.
func (s *Service) traverseUpstream(name string, visited map[string]bool, nodes *[]LineageNode, edges *[]LineageEdge, depth int, maxDepth *int) {
	if visited[name] {
		return
	}
	visited[name] = true

	if depth > *maxDepth {
		*maxDepth = depth
	}

	if node, ok := s.lineageGraph.nodes[name]; ok {
		n := *node
		n.Level = -depth // negative for upstream
		*nodes = append(*nodes, n)
	}

	for _, e := range s.lineageGraph.edges {
		if e.To == name {
			*edges = append(*edges, e)
			s.traverseUpstream(e.From, visited, nodes, edges, depth+1, maxDepth)
		}
	}
}

// traverseDownstream walks downstream dependents recursively.
func (s *Service) traverseDownstream(name string, visited map[string]bool, nodes *[]LineageNode, edges *[]LineageEdge, depth int, maxDepth *int) {
	if visited[name] {
		return
	}
	visited[name] = true

	if depth > *maxDepth {
		*maxDepth = depth
	}

	if node, ok := s.lineageGraph.nodes[name]; ok {
		n := *node
		n.Level = depth
		*nodes = append(*nodes, n)
	}

	for _, e := range s.lineageGraph.edges {
		if e.From == name {
			*edges = append(*edges, e)
			s.traverseDownstream(e.To, visited, nodes, edges, depth+1, maxDepth)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func tagsContain(tags []string, text string) bool {
	lower := strings.ToLower(text)
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), lower) {
			return true
		}
	}
	return false
}

func hasAnyTag(tags, filter []string) bool {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	for _, f := range filter {
		if tagSet[f] {
			return true
		}
	}
	return false
}

func addUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func removeFromSlice(slice []string, val string) []string {
	for i, s := range slice {
		if s == val {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
