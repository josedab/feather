package semantic

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// CatalogEntry represents a feature in the semantic catalog with auto-generated embeddings.
type CatalogEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	EntityType  string            `json:"entity_type,omitempty"`
	DataType    string            `json:"data_type,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Group       string            `json:"group,omitempty"`
	Embedding   []float64         `json:"-"`
	IndexedAt   time.Time         `json:"indexed_at"`
	UsageCount  int64             `json:"usage_count"`
}

// CatalogSearchResult is a ranked search result from the catalog.
type CatalogSearchResult struct {
	Entry      CatalogEntry `json:"entry"`
	Score      float64      `json:"score"`
	MatchType  string       `json:"match_type"` // "semantic", "keyword", "hybrid"
	Highlights []string     `json:"highlights,omitempty"`
}

// DuplicateCandidate represents a pair of potentially duplicate features.
type DuplicateCandidate struct {
	FeatureA   string  `json:"feature_a"`
	FeatureB   string  `json:"feature_b"`
	Similarity float64 `json:"similarity"`
	Reason     string  `json:"reason"`
}

// CatalogConfig configures the semantic catalog.
type CatalogConfig struct {
	MaxEntries          int     `json:"max_entries"`
	DuplicateThreshold  float64 `json:"duplicate_threshold"`
	MinSearchScore      float64 `json:"min_search_score"`
	EmbeddingDimensions int     `json:"embedding_dimensions"`
}

// DefaultCatalogConfig returns sensible defaults.
func DefaultCatalogConfig() CatalogConfig {
	return CatalogConfig{
		MaxEntries:          100000,
		DuplicateThreshold:  0.85,
		MinSearchScore:      0.1,
		EmbeddingDimensions: 64,
	}
}

// Catalog provides semantic search and duplicate detection over feature metadata.
type Catalog struct {
	mu      sync.RWMutex
	config  CatalogConfig
	entries map[string]*CatalogEntry
}

// NewCatalog creates a new semantic catalog.
func NewCatalog(cfg CatalogConfig) *Catalog {
	if cfg.MaxEntries <= 0 {
		cfg = DefaultCatalogConfig()
	}
	return &Catalog{
		config:  cfg,
		entries: make(map[string]*CatalogEntry),
	}
}

// Index adds or updates a feature in the catalog, auto-generating its embedding.
func (c *Catalog) Index(entry CatalogEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("feature name is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.config.MaxEntries {
		if _, exists := c.entries[entry.Name]; !exists {
			return fmt.Errorf("catalog at capacity (%d entries)", c.config.MaxEntries)
		}
	}

	// Auto-generate embedding from name, description, entity type, and tags
	entry.Embedding = generateEmbedding(entry, c.config.EmbeddingDimensions)
	entry.IndexedAt = time.Now()

	c.entries[entry.Name] = &entry
	return nil
}

// Search performs semantic + keyword hybrid search over the catalog.
func (c *Catalog) Search(query string, limit int) []CatalogSearchResult {
	if limit <= 0 {
		limit = 20
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	queryEmb := generateQueryEmbedding(query, c.config.EmbeddingDimensions)
	queryTokens := catalogTokenize(strings.ToLower(query))

	var results []CatalogSearchResult
	for _, entry := range c.entries {
		// Semantic score via cosine similarity
		semanticScore := catalogCosineSimilarity(queryEmb, entry.Embedding)

		// Keyword score
		keywordScore := catalogKeywordMatch(queryTokens, entry)

		// Hybrid score
		hybridScore := 0.6*semanticScore + 0.4*keywordScore

		if hybridScore < c.config.MinSearchScore {
			continue
		}

		matchType := "hybrid"
		if semanticScore > keywordScore*2 {
			matchType = "semantic"
		} else if keywordScore > semanticScore*2 {
			matchType = "keyword"
		}

		results = append(results, CatalogSearchResult{
			Entry:     *entry,
			Score:     hybridScore,
			MatchType: matchType,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// DetectDuplicates finds semantically similar features that may be duplicates.
func (c *Catalog) DetectDuplicates() []DuplicateCandidate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]*CatalogEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, e)
	}

	var candidates []DuplicateCandidate
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			sim := catalogCosineSimilarity(entries[i].Embedding, entries[j].Embedding)
			if sim >= c.config.DuplicateThreshold {
				reason := "high semantic similarity"
				// Check name similarity too
				if levenshteinRatio(entries[i].Name, entries[j].Name) > 0.7 {
					reason = "similar names and semantics"
				}
				candidates = append(candidates, DuplicateCandidate{
					FeatureA:   entries[i].Name,
					FeatureB:   entries[j].Name,
					Similarity: sim,
					Reason:     reason,
				})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Similarity > candidates[j].Similarity
	})

	return candidates
}

// Get returns a catalog entry by name.
func (c *Catalog) Get(name string) (*CatalogEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not found in catalog", name)
	}
	cp := *entry
	return &cp, nil
}

// List returns all catalog entries.
func (c *Catalog) List() []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]CatalogEntry, 0, len(c.entries))
	for _, e := range c.entries {
		result = append(result, *e)
	}
	return result
}

// RecordUsage increments the usage counter for a feature.
func (c *Catalog) RecordUsage(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[name]; ok {
		e.UsageCount++
	}
}

// CatalogStats returns catalog statistics.
type CatalogStats struct {
	TotalEntries  int `json:"total_entries"`
	TotalGroups   int `json:"total_groups"`
	AvgEmbDim     int `json:"avg_embedding_dimensions"`
	DuplicateRisk int `json:"duplicate_risk_count"`
}

// Stats returns catalog statistics.
func (c *Catalog) Stats() CatalogStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	groups := make(map[string]bool)
	for _, e := range c.entries {
		if e.Group != "" {
			groups[e.Group] = true
		}
	}

	return CatalogStats{
		TotalEntries: len(c.entries),
		TotalGroups:  len(groups),
		AvgEmbDim:    c.config.EmbeddingDimensions,
	}
}

// generateEmbedding creates a simple bag-of-characters embedding from feature metadata.
// In production, this would use a proper embedding model (e.g., sentence-transformers).
func generateEmbedding(entry CatalogEntry, dims int) []float64 {
	text := strings.ToLower(entry.Name + " " + entry.Description + " " + entry.EntityType)
	for k, v := range entry.Tags {
		text += " " + k + " " + v
	}
	return hashEmbedding(text, dims)
}

func generateQueryEmbedding(query string, dims int) []float64 {
	return hashEmbedding(strings.ToLower(query), dims)
}

// hashEmbedding creates a deterministic embedding by hashing character n-grams.
func hashEmbedding(text string, dims int) []float64 {
	emb := make([]float64, dims)
	// Character trigram hashing
	for i := 0; i < len(text)-2; i++ {
		trigram := text[i : i+3]
		hash := 0
		for _, c := range trigram {
			hash = hash*31 + int(c)
		}
		idx := abs(hash) % dims
		emb[idx] += 1.0
	}

	// Also hash individual words
	for _, word := range strings.Fields(text) {
		hash := 0
		for _, c := range word {
			hash = hash*37 + int(c)
		}
		idx := abs(hash) % dims
		emb[idx] += 0.5
	}

	// L2 normalize
	norm := 0.0
	for _, v := range emb {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range emb {
			emb[i] /= norm
		}
	}
	return emb
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func catalogCosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func catalogKeywordMatch(queryTokens []string, entry *CatalogEntry) float64 {
	text := strings.ToLower(entry.Name + " " + entry.Description + " " + entry.EntityType)
	matches := 0
	for _, token := range queryTokens {
		if strings.Contains(text, token) {
			matches++
		}
	}
	if len(queryTokens) == 0 {
		return 0
	}
	return float64(matches) / float64(len(queryTokens))
}

func catalogTokenize(text string) []string {
	return strings.Fields(text)
}

func levenshteinRatio(a, b string) float64 {
	if a == b {
		return 1.0
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshtein(a, b)
	return 1.0 - float64(dist)/float64(maxLen)
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = catalogMinInt(d[i-1][j]+1, catalogMinInt(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[la][lb]
}

func catalogMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
