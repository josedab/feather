package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NLQueryEngine provides natural language query understanding for feature discovery.
type NLQueryEngine struct {
	mu sync.RWMutex

	// Core components
	discovery *FeatureDiscovery
	embedder  Embedder

	// Query understanding
	intentClassifier *IntentClassifier
	entityExtractor  *EntityExtractor
	queryParser      *QueryParser

	// LLM support (optional)
	llmClient LLMClient

	// Configuration
	config NLQueryConfig

	// Logging
	logger *slog.Logger
}

// NLQueryConfig configures the NL query engine.
type NLQueryConfig struct {
	// Intent classification
	MinIntentConfidence float64 `json:"min_intent_confidence"`
	FallbackToSearch    bool    `json:"fallback_to_search"`

	// Entity extraction
	EnableFuzzyMatch    bool    `json:"enable_fuzzy_match"`
	FuzzyMatchThreshold float64 `json:"fuzzy_match_threshold"`

	// LLM support
	UseLLMForParsing  bool          `json:"use_llm_for_parsing"`
	LLMParseTimeout   time.Duration `json:"llm_parse_timeout"`
	LLMFallbackOnFail bool          `json:"llm_fallback_on_fail"`

	// Context
	MaxContextLength int `json:"max_context_length"`

	// Caching
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
}

// DefaultNLQueryConfig returns sensible defaults.
func DefaultNLQueryConfig() NLQueryConfig {
	return NLQueryConfig{
		MinIntentConfidence: 0.6,
		FallbackToSearch:    true,
		EnableFuzzyMatch:    true,
		FuzzyMatchThreshold: 0.7,
		UseLLMForParsing:    false,
		LLMParseTimeout:     10 * time.Second,
		LLMFallbackOnFail:   true,
		MaxContextLength:    1000,
		CacheEnabled:        true,
		CacheTTL:            15 * time.Minute,
	}
}

// QueryIntent represents the detected intent of a query.
type QueryIntent string

const (
	IntentSearch       QueryIntent = "search"        // Find features matching criteria
	IntentSimilar      QueryIntent = "similar"       // Find similar features
	IntentRecommend    QueryIntent = "recommend"     // Get recommendations
	IntentExplain      QueryIntent = "explain"       // Explain a feature
	IntentCompare      QueryIntent = "compare"       // Compare features
	IntentList         QueryIntent = "list"          // List features
	IntentCount        QueryIntent = "count"         // Count features
	IntentFilter       QueryIntent = "filter"        // Filter by criteria
	IntentRelated      QueryIntent = "related"       // Find related features
	IntentTrending     QueryIntent = "trending"      // Find trending/popular features
	IntentRecent       QueryIntent = "recent"        // Find recently updated
	IntentQuality      QueryIntent = "quality"       // Find high quality features
	IntentOwned        QueryIntent = "owned"         // Find features by owner
	IntentUnknown      QueryIntent = "unknown"       // Unable to determine
)

// ParsedQuery represents a parsed natural language query.
type ParsedQuery struct {
	// Original query
	OriginalQuery string `json:"original_query"`

	// Intent classification
	Intent           QueryIntent `json:"intent"`
	IntentConfidence float64     `json:"intent_confidence"`
	SubIntent        string      `json:"sub_intent,omitempty"`

	// Extracted entities
	Entities *ExtractedEntities `json:"entities"`

	// Structured query
	StructuredQuery *DiscoveryQuery `json:"structured_query"`

	// Interpretation
	Interpretation string `json:"interpretation"`

	// Alternative interpretations
	Alternatives []AlternativeInterpretation `json:"alternatives,omitempty"`

	// Metadata
	ParsedAt time.Time `json:"parsed_at"`
	ParsedBy string    `json:"parsed_by"` // "rules", "llm", "hybrid"
}

// ExtractedEntities contains entities extracted from the query.
type ExtractedEntities struct {
	// Feature references
	FeatureNames []string `json:"feature_names,omitempty"`
	FeatureIDs   []string `json:"feature_ids,omitempty"`

	// Filters
	Categories  []string `json:"categories,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	EntityTypes []string `json:"entity_types,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Owners      []string `json:"owners,omitempty"`
	Teams       []string `json:"teams,omitempty"`
	DataTypes   []string `json:"data_types,omitempty"`
	UseCases    []string `json:"use_cases,omitempty"`

	// Quality/freshness
	MinQuality   *float32 `json:"min_quality,omitempty"`
	Freshness    []string `json:"freshness,omitempty"`

	// Time references
	TimeReference *TimeReference `json:"time_reference,omitempty"`

	// Numeric parameters
	Limit  *int `json:"limit,omitempty"`
	Offset *int `json:"offset,omitempty"`

	// Comparison targets
	CompareTargets []string `json:"compare_targets,omitempty"`

	// Keywords
	Keywords []string `json:"keywords,omitempty"`
}

// TimeReference represents a time reference in the query.
type TimeReference struct {
	Type     string    `json:"type"` // "relative", "absolute", "range"
	Duration string    `json:"duration,omitempty"`
	Start    time.Time `json:"start,omitempty"`
	End      time.Time `json:"end,omitempty"`
}

// AlternativeInterpretation represents an alternative query interpretation.
type AlternativeInterpretation struct {
	Intent         QueryIntent     `json:"intent"`
	Interpretation string          `json:"interpretation"`
	Confidence     float64         `json:"confidence"`
	Query          *DiscoveryQuery `json:"query,omitempty"`
}

// IntentClassifier classifies query intent.
type IntentClassifier struct {
	patterns map[QueryIntent][]*regexp.Regexp
	keywords map[QueryIntent][]string
}

// EntityExtractor extracts entities from queries.
type EntityExtractor struct {
	categoryPatterns  []*regexp.Regexp
	domainPatterns    []*regexp.Regexp
	ownerPatterns     []*regexp.Regexp
	qualityPatterns   []*regexp.Regexp
	freshnessPatterns []*regexp.Regexp
	limitPatterns     []*regexp.Regexp
	timePatterns      []*regexp.Regexp

	// Known values for matching
	knownCategories  map[string]bool
	knownDomains     map[string]bool
	knownOwners      map[string]bool
	knownTags        map[string]bool
	knownEntityTypes map[string]bool
}

// QueryParser parses natural language into structured queries.
type QueryParser struct {
	classifier *IntentClassifier
	extractor  *EntityExtractor
	config     NLQueryConfig
}

// NewNLQueryEngine creates a new natural language query engine.
func NewNLQueryEngine(
	discovery *FeatureDiscovery,
	embedder Embedder,
	llmClient LLMClient,
	config NLQueryConfig,
	logger *slog.Logger,
) (*NLQueryEngine, error) {
	if discovery == nil {
		return nil, fmt.Errorf("discovery engine is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	classifier := newIntentClassifier()
	extractor := newEntityExtractor()
	parser := &QueryParser{
		classifier: classifier,
		extractor:  extractor,
		config:     config,
	}

	engine := &NLQueryEngine{
		discovery:        discovery,
		embedder:         embedder,
		intentClassifier: classifier,
		entityExtractor:  extractor,
		queryParser:      parser,
		llmClient:        llmClient,
		config:           config,
		logger:           logger,
	}

	// Populate known values from indexed features
	engine.populateKnownValues()

	return engine, nil
}

func newIntentClassifier() *IntentClassifier {
	classifier := &IntentClassifier{
		patterns: make(map[QueryIntent][]*regexp.Regexp),
		keywords: make(map[QueryIntent][]string),
	}

	// Search intent patterns
	classifier.patterns[IntentSearch] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(find|search|look\s*for|get|show|what|which)\s+`),
		regexp.MustCompile(`(?i)\s+(feature|features)\s+(for|about|related\s+to|like)`),
		regexp.MustCompile(`(?i)^(i\s+need|i\s+want|give\s+me|show\s+me)\s+`),
	}
	classifier.keywords[IntentSearch] = []string{"find", "search", "look", "get", "show", "what", "which"}

	// Similar intent patterns
	classifier.patterns[IntentSimilar] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(similar|like|same\s+as|resembl|related\s+to)\s+`),
		regexp.MustCompile(`(?i)features?\s+(similar|like|resembling)\s+`),
		regexp.MustCompile(`(?i)(alternatives?|substitutes?|replacements?)\s+(for|to)`),
	}
	classifier.keywords[IntentSimilar] = []string{"similar", "like", "same as", "alternatives", "resembling"}

	// Recommend intent patterns
	classifier.patterns[IntentRecommend] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(recommend|suggest|propose)\s+`),
		regexp.MustCompile(`(?i)(what\s+should|what\s+would)\s+`),
		regexp.MustCompile(`(?i)best\s+features?\s+for`),
		regexp.MustCompile(`(?i)features?\s+i\s+should\s+use`),
	}
	classifier.keywords[IntentRecommend] = []string{"recommend", "suggest", "propose", "best", "should use"}

	// Explain intent patterns
	classifier.patterns[IntentExplain] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(explain|describe|what\s+is|tell\s+me\s+about|details?\s+about)`),
		regexp.MustCompile(`(?i)(how\s+does|what\s+does)\s+.+\s+(work|mean|do)`),
	}
	classifier.keywords[IntentExplain] = []string{"explain", "describe", "what is", "tell me about", "details"}

	// Compare intent patterns
	classifier.patterns[IntentCompare] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(compare|difference|vs\.?|versus|between)\s+`),
		regexp.MustCompile(`(?i)(how\s+does|how\s+do)\s+.+\s+(differ|compare)`),
	}
	classifier.keywords[IntentCompare] = []string{"compare", "difference", "vs", "versus", "between", "differ"}

	// List intent patterns
	classifier.patterns[IntentList] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(list|show\s+all|enumerate|display)\s+`),
		regexp.MustCompile(`(?i)(all|every)\s+features?`),
	}
	classifier.keywords[IntentList] = []string{"list", "all", "every", "enumerate", "display all"}

	// Count intent patterns
	classifier.patterns[IntentCount] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(count|how\s+many|number\s+of|total)\s+`),
	}
	classifier.keywords[IntentCount] = []string{"count", "how many", "number of", "total"}

	// Filter intent patterns
	classifier.patterns[IntentFilter] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(filter|narrow|refine|only)\s+`),
		regexp.MustCompile(`(?i)(where|with|having)\s+`),
	}
	classifier.keywords[IntentFilter] = []string{"filter", "narrow", "refine", "only", "where"}

	// Related intent patterns
	classifier.patterns[IntentRelated] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(related|connected|linked|dependent)\s+`),
		regexp.MustCompile(`(?i)features?\s+(used\s+with|combined\s+with)`),
	}
	classifier.keywords[IntentRelated] = []string{"related", "connected", "linked", "dependent", "used with"}

	// Trending intent patterns
	classifier.patterns[IntentTrending] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(trending|popular|most\s+used|frequently\s+used|hot)`),
		regexp.MustCompile(`(?i)(top|best\s+performing)\s+features?`),
	}
	classifier.keywords[IntentTrending] = []string{"trending", "popular", "most used", "frequently", "hot", "top"}

	// Recent intent patterns
	classifier.patterns[IntentRecent] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(recent|new|latest|updated|fresh)\s+`),
		regexp.MustCompile(`(?i)(added|created|modified)\s+(recently|today|this\s+week)`),
	}
	classifier.keywords[IntentRecent] = []string{"recent", "new", "latest", "updated", "fresh"}

	// Quality intent patterns
	classifier.patterns[IntentQuality] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(high\s+quality|reliable|accurate|well.documented)`),
		regexp.MustCompile(`(?i)(quality|completeness)\s+(above|greater|over|at\s+least)`),
	}
	classifier.keywords[IntentQuality] = []string{"high quality", "reliable", "accurate", "well documented"}

	// Owned intent patterns
	classifier.patterns[IntentOwned] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(owned|maintained|created)\s+by`),
		regexp.MustCompile(`(?i)(my|team's|our)\s+features?`),
	}
	classifier.keywords[IntentOwned] = []string{"owned by", "maintained by", "my features", "team features"}

	return classifier
}

func newEntityExtractor() *EntityExtractor {
	extractor := &EntityExtractor{
		knownCategories:  make(map[string]bool),
		knownDomains:     make(map[string]bool),
		knownOwners:      make(map[string]bool),
		knownTags:        make(map[string]bool),
		knownEntityTypes: make(map[string]bool),
	}

	// Category extraction patterns
	extractor.categoryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)category[:\s]+["']?(\w+)["']?`),
		regexp.MustCompile(`(?i)in\s+(?:the\s+)?["']?(\w+)["']?\s+category`),
		regexp.MustCompile(`(?i)(?:type|kind)\s+(?:of\s+)?["']?(\w+)["']?`),
	}

	// Domain extraction patterns
	extractor.domainPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)domain[:\s]+["']?(\w+)["']?`),
		regexp.MustCompile(`(?i)in\s+(?:the\s+)?["']?(\w+)["']?\s+domain`),
		regexp.MustCompile(`(?i)for\s+["']?(\w+)["']?\s+(?:entity|entities)`),
	}

	// Owner extraction patterns
	extractor.ownerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:owned|maintained|created)\s+by\s+["']?([^"'\s,]+)["']?`),
		regexp.MustCompile(`(?i)owner[:\s]+["']?([^"'\s,]+)["']?`),
		regexp.MustCompile(`(?i)from\s+(?:the\s+)?["']?([^"'\s,]+)["']?\s+team`),
	}

	// Quality extraction patterns
	extractor.qualityPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)quality\s*(?:>=?|>|above|over|at\s+least)\s*(\d+(?:\.\d+)?)`),
		regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:\+|%)\s*quality`),
	}

	// Freshness extraction patterns
	extractor.freshnessPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(real[- ]?time|hourly|daily|weekly)\s+(?:update|refresh|freshness)`),
		regexp.MustCompile(`(?i)updated?\s+(real[- ]?time|hourly|daily|weekly)`),
	}

	// Limit extraction patterns
	extractor.limitPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:top|first|show|limit)\s+(\d+)`),
		regexp.MustCompile(`(?i)(\d+)\s+(?:results?|features?)`),
	}

	// Time extraction patterns
	extractor.timePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(today|yesterday|this\s+week|last\s+week|this\s+month|last\s+month)`),
		regexp.MustCompile(`(?i)(?:in\s+the\s+)?(?:last|past)\s+(\d+)\s+(hour|day|week|month)s?`),
	}

	return extractor
}

func (e *NLQueryEngine) populateKnownValues() {
	// Get all metadata from indexer
	allMetadata := e.discovery.indexer.ListMetadata()

	for _, meta := range allMetadata {
		if meta.Category != "" {
			e.entityExtractor.knownCategories[strings.ToLower(meta.Category)] = true
		}
		if meta.Domain != "" {
			e.entityExtractor.knownDomains[strings.ToLower(meta.Domain)] = true
		}
		if meta.Owner != "" {
			e.entityExtractor.knownOwners[strings.ToLower(meta.Owner)] = true
		}
		if meta.EntityType != "" {
			e.entityExtractor.knownEntityTypes[strings.ToLower(meta.EntityType)] = true
		}
		for _, tag := range meta.Tags {
			e.entityExtractor.knownTags[strings.ToLower(tag)] = true
		}
	}
}

// Parse parses a natural language query into a structured form.
func (e *NLQueryEngine) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	startTime := time.Now()
	query = strings.TrimSpace(query)

	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	var parsed *ParsedQuery
	var err error

	// Try LLM parsing if enabled and available
	if e.config.UseLLMForParsing && e.llmClient != nil && e.llmClient.IsAvailable() {
		parsed, err = e.parseLLM(ctx, query)
		if err != nil && !e.config.LLMFallbackOnFail {
			return nil, err
		}
		if parsed != nil {
			parsed.ParsedBy = "llm"
		}
	}

	// Fallback to rule-based parsing
	if parsed == nil {
		parsed, err = e.parseRules(query)
		if err != nil {
			return nil, err
		}
		parsed.ParsedBy = "rules"
	}

	parsed.ParsedAt = startTime
	return parsed, nil
}

func (e *NLQueryEngine) parseRules(query string) (*ParsedQuery, error) {
	parsed := &ParsedQuery{
		OriginalQuery: query,
		Entities:      &ExtractedEntities{},
	}

	// Classify intent
	intent, confidence := e.intentClassifier.Classify(query)
	parsed.Intent = intent
	parsed.IntentConfidence = confidence

	// Extract entities
	entities := e.entityExtractor.Extract(query)
	parsed.Entities = entities

	// Build structured query
	structured := e.buildStructuredQuery(intent, entities, query)
	parsed.StructuredQuery = structured

	// Generate interpretation
	parsed.Interpretation = e.generateInterpretation(intent, entities)

	// Generate alternatives
	parsed.Alternatives = e.generateAlternatives(query, intent, entities)

	return parsed, nil
}

func (c *IntentClassifier) Classify(query string) (QueryIntent, float64) {
	query = strings.ToLower(query)

	bestIntent := IntentUnknown
	bestScore := 0.0

	for intent, patterns := range c.patterns {
		score := 0.0

		// Check pattern matches
		for _, pattern := range patterns {
			if pattern.MatchString(query) {
				score += 0.5
				break
			}
		}

		// Check keyword matches
		keywords := c.keywords[intent]
		keywordMatches := 0
		for _, keyword := range keywords {
			if strings.Contains(query, keyword) {
				keywordMatches++
			}
		}
		if len(keywords) > 0 {
			score += 0.5 * float64(keywordMatches) / float64(len(keywords))
		}

		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	// Default to search if no clear intent
	if bestIntent == IntentUnknown || bestScore < 0.3 {
		bestIntent = IntentSearch
		bestScore = 0.5
	}

	return bestIntent, bestScore
}

func (e *EntityExtractor) Extract(query string) *ExtractedEntities {
	entities := &ExtractedEntities{
		Keywords: make([]string, 0),
	}

	// Extract categories
	for _, pattern := range e.categoryPatterns {
		matches := pattern.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				entities.Categories = append(entities.Categories, match[1])
			}
		}
	}

	// Also check known categories in text
	queryLower := strings.ToLower(query)
	for cat := range e.knownCategories {
		if strings.Contains(queryLower, cat) && !contains(entities.Categories, cat) {
			entities.Categories = append(entities.Categories, cat)
		}
	}

	// Extract domains
	for _, pattern := range e.domainPatterns {
		matches := pattern.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				entities.Domains = append(entities.Domains, match[1])
			}
		}
	}

	// Check known domains
	for domain := range e.knownDomains {
		if strings.Contains(queryLower, domain) && !contains(entities.Domains, domain) {
			entities.Domains = append(entities.Domains, domain)
		}
	}

	// Extract owners
	for _, pattern := range e.ownerPatterns {
		matches := pattern.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				entities.Owners = append(entities.Owners, match[1])
			}
		}
	}

	// Extract quality threshold
	for _, pattern := range e.qualityPatterns {
		match := pattern.FindStringSubmatch(query)
		if len(match) > 1 {
			if val, err := strconv.ParseFloat(match[1], 32); err == nil {
				// Normalize to 0-1 if percentage
				if val > 1 {
					val = val / 100
				}
				qualityVal := float32(val)
				entities.MinQuality = &qualityVal
			}
		}
	}

	// Extract freshness requirements
	for _, pattern := range e.freshnessPatterns {
		match := pattern.FindStringSubmatch(query)
		if len(match) > 1 {
			freshness := strings.ToLower(strings.ReplaceAll(match[1], " ", "-"))
			entities.Freshness = append(entities.Freshness, freshness)
		}
	}

	// Extract limit
	for _, pattern := range e.limitPatterns {
		match := pattern.FindStringSubmatch(query)
		if len(match) > 1 {
			if val, err := strconv.Atoi(match[1]); err == nil {
				entities.Limit = &val
			}
		}
	}

	// Extract time references
	for _, pattern := range e.timePatterns {
		match := pattern.FindStringSubmatch(query)
		if len(match) > 0 {
			entities.TimeReference = parseTimeReference(match[0])
			break
		}
	}

	// Extract known tags
	for tag := range e.knownTags {
		if strings.Contains(queryLower, tag) {
			entities.Tags = append(entities.Tags, tag)
		}
	}

	// Extract known entity types
	for et := range e.knownEntityTypes {
		if strings.Contains(queryLower, et) {
			entities.EntityTypes = append(entities.EntityTypes, et)
		}
	}

	// Extract keywords (remaining significant words)
	entities.Keywords = extractKeywords(query)

	return entities
}

func parseTimeReference(text string) *TimeReference {
	text = strings.ToLower(text)
	ref := &TimeReference{}

	if strings.Contains(text, "today") {
		ref.Type = "relative"
		ref.Duration = "1d"
	} else if strings.Contains(text, "yesterday") {
		ref.Type = "relative"
		ref.Duration = "2d"
	} else if strings.Contains(text, "this week") {
		ref.Type = "relative"
		ref.Duration = "7d"
	} else if strings.Contains(text, "last week") {
		ref.Type = "relative"
		ref.Duration = "14d"
	} else if strings.Contains(text, "this month") {
		ref.Type = "relative"
		ref.Duration = "30d"
	} else if strings.Contains(text, "last month") {
		ref.Type = "relative"
		ref.Duration = "60d"
	} else {
		// Parse "last N units" pattern
		pattern := regexp.MustCompile(`(\d+)\s*(hour|day|week|month)s?`)
		match := pattern.FindStringSubmatch(text)
		if len(match) > 2 {
			ref.Type = "relative"
			ref.Duration = match[1] + string(match[2][0])
		}
	}

	return ref
}

func extractKeywords(query string) []string {
	// Remove common stop words and extract significant terms
	stopWords := map[string]bool{
		"find": true, "search": true, "show": true, "get": true, "list": true,
		"me": true, "the": true, "a": true, "an": true, "for": true, "with": true,
		"in": true, "of": true, "to": true, "and": true, "or": true, "that": true,
		"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "must": true, "i": true, "you": true, "we": true, "they": true,
		"features": true, "feature": true, "all": true, "any": true, "some": true,
	}

	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	})

	var keywords []string
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

func (e *NLQueryEngine) buildStructuredQuery(intent QueryIntent, entities *ExtractedEntities, originalQuery string) *DiscoveryQuery {
	query := &DiscoveryQuery{
		Query:       originalQuery,
		Categories:  entities.Categories,
		Domains:     entities.Domains,
		EntityTypes: entities.EntityTypes,
		Tags:        entities.Tags,
		Owners:      entities.Owners,
		UseCases:    entities.UseCases,
		DataTypes:   entities.DataTypes,
	}

	// Apply quality filter
	if entities.MinQuality != nil {
		query.MinQuality = *entities.MinQuality
	}

	// Apply freshness filter
	if len(entities.Freshness) > 0 {
		query.MinFreshness = entities.Freshness[0]
		query.OnlyFresh = true
	}

	// Apply limit
	if entities.Limit != nil {
		query.Limit = *entities.Limit
	} else {
		query.Limit = 10
	}

	// Adjust based on intent
	switch intent {
	case IntentSimilar:
		// For similar intent, use feature names as the basis
		if len(entities.FeatureNames) > 0 {
			query.Query = entities.FeatureNames[0]
		} else if len(entities.Keywords) > 0 {
			query.Query = strings.Join(entities.Keywords, " ")
		}
		query.IncludeRelated = true

	case IntentRecommend:
		query.IncludeRelated = true
		// Higher quality for recommendations
		if query.MinQuality == 0 {
			query.MinQuality = 0.7
		}

	case IntentQuality:
		if query.MinQuality == 0 {
			query.MinQuality = 0.8
		}

	case IntentTrending:
		// Would need popularity sorting (handled at discovery level)
		query.Limit = 20

	case IntentRecent:
		query.OnlyFresh = true

	case IntentList:
		query.Limit = 50 // More results for list

	case IntentCount:
		query.Limit = 1000 // Get all for count

	case IntentOwned:
		// Owners already extracted

	default:
		// Use keywords for general search
		if len(entities.Keywords) > 0 {
			query.Query = strings.Join(entities.Keywords, " ")
		}
	}

	return query
}

func (e *NLQueryEngine) generateInterpretation(intent QueryIntent, entities *ExtractedEntities) string {
	var parts []string

	// Describe intent
	switch intent {
	case IntentSearch:
		parts = append(parts, "Searching for features")
	case IntentSimilar:
		if len(entities.FeatureNames) > 0 {
			parts = append(parts, fmt.Sprintf("Finding features similar to '%s'", entities.FeatureNames[0]))
		} else {
			parts = append(parts, "Finding similar features")
		}
	case IntentRecommend:
		parts = append(parts, "Recommending features")
	case IntentExplain:
		parts = append(parts, "Explaining feature details")
	case IntentCompare:
		parts = append(parts, "Comparing features")
	case IntentList:
		parts = append(parts, "Listing all features")
	case IntentCount:
		parts = append(parts, "Counting features")
	case IntentFilter:
		parts = append(parts, "Filtering features")
	case IntentRelated:
		parts = append(parts, "Finding related features")
	case IntentTrending:
		parts = append(parts, "Finding trending features")
	case IntentRecent:
		parts = append(parts, "Finding recently updated features")
	case IntentQuality:
		parts = append(parts, "Finding high-quality features")
	case IntentOwned:
		parts = append(parts, "Finding features by owner")
	default:
		parts = append(parts, "Processing query")
	}

	// Add filter descriptions
	if len(entities.Categories) > 0 {
		parts = append(parts, fmt.Sprintf("in category '%s'", strings.Join(entities.Categories, ", ")))
	}
	if len(entities.Domains) > 0 {
		parts = append(parts, fmt.Sprintf("in domain '%s'", strings.Join(entities.Domains, ", ")))
	}
	if len(entities.Owners) > 0 {
		parts = append(parts, fmt.Sprintf("owned by '%s'", strings.Join(entities.Owners, ", ")))
	}
	if entities.MinQuality != nil {
		parts = append(parts, fmt.Sprintf("with quality >= %.0f%%", *entities.MinQuality*100))
	}
	if len(entities.Freshness) > 0 {
		parts = append(parts, fmt.Sprintf("with %s freshness", entities.Freshness[0]))
	}
	if entities.Limit != nil {
		parts = append(parts, fmt.Sprintf("(limit %d)", *entities.Limit))
	}

	return strings.Join(parts, " ")
}

func (e *NLQueryEngine) generateAlternatives(query string, mainIntent QueryIntent, entities *ExtractedEntities) []AlternativeInterpretation {
	var alternatives []AlternativeInterpretation

	// Generate alternative intents
	allIntents := []QueryIntent{
		IntentSearch, IntentSimilar, IntentRecommend, IntentList, IntentTrending,
	}

	for _, intent := range allIntents {
		if intent == mainIntent {
			continue
		}

		confidence := 0.3 // Lower confidence for alternatives

		altQuery := e.buildStructuredQuery(intent, entities, query)
		altInterp := e.generateInterpretation(intent, entities)

		alternatives = append(alternatives, AlternativeInterpretation{
			Intent:         intent,
			Interpretation: altInterp,
			Confidence:     confidence,
			Query:          altQuery,
		})
	}

	// Sort by confidence and limit
	if len(alternatives) > 3 {
		alternatives = alternatives[:3]
	}

	return alternatives
}

func (e *NLQueryEngine) parseLLM(ctx context.Context, query string) (*ParsedQuery, error) {
	if e.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured")
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, e.config.LLMParseTimeout)
	defer cancel()

	prompt := e.buildParsePrompt(query)

	response, err := e.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse LLM response
	return e.parseLLMResponse(response, query)
}

func (e *NLQueryEngine) buildParsePrompt(query string) string {
	var sb strings.Builder

	sb.WriteString("Parse the following natural language query about features in a feature store.\n\n")
	sb.WriteString(fmt.Sprintf("Query: \"%s\"\n\n", query))
	sb.WriteString("Extract the following information:\n")
	sb.WriteString("1. Intent: What does the user want to do? (search, similar, recommend, explain, compare, list, count, filter, related, trending, recent, quality, owned)\n")
	sb.WriteString("2. Entities: Extract any mentioned categories, domains, entity types, owners, tags, quality thresholds, freshness requirements\n")
	sb.WriteString("3. Keywords: Key terms for search\n\n")

	// List known values to help the LLM
	if len(e.entityExtractor.knownCategories) > 0 {
		categories := make([]string, 0)
		for cat := range e.entityExtractor.knownCategories {
			categories = append(categories, cat)
		}
		sb.WriteString(fmt.Sprintf("Known categories: %s\n", strings.Join(categories, ", ")))
	}

	if len(e.entityExtractor.knownDomains) > 0 {
		domains := make([]string, 0)
		for domain := range e.entityExtractor.knownDomains {
			domains = append(domains, domain)
		}
		sb.WriteString(fmt.Sprintf("Known domains: %s\n", strings.Join(domains, ", ")))
	}

	sb.WriteString("\nRespond in a structured format with clear labels.")

	return sb.String()
}

func (e *NLQueryEngine) parseLLMResponse(response, originalQuery string) (*ParsedQuery, error) {
	// Simple parsing of LLM response
	// In a real implementation, you'd want structured output (JSON mode)
	parsed := &ParsedQuery{
		OriginalQuery: originalQuery,
		Entities:      &ExtractedEntities{},
	}

	response = strings.ToLower(response)

	// Extract intent from response
	intents := map[string]QueryIntent{
		"search":    IntentSearch,
		"similar":   IntentSimilar,
		"recommend": IntentRecommend,
		"explain":   IntentExplain,
		"compare":   IntentCompare,
		"list":      IntentList,
		"count":     IntentCount,
		"filter":    IntentFilter,
		"related":   IntentRelated,
		"trending":  IntentTrending,
		"recent":    IntentRecent,
		"quality":   IntentQuality,
		"owned":     IntentOwned,
	}

	parsed.Intent = IntentSearch // Default
	parsed.IntentConfidence = 0.7

	for keyword, intent := range intents {
		if strings.Contains(response, keyword) {
			parsed.Intent = intent
			parsed.IntentConfidence = 0.85
			break
		}
	}

	// Fall back to rule-based extraction for entities
	parsed.Entities = e.entityExtractor.Extract(originalQuery)

	// Build structured query
	parsed.StructuredQuery = e.buildStructuredQuery(parsed.Intent, parsed.Entities, originalQuery)
	parsed.Interpretation = e.generateInterpretation(parsed.Intent, parsed.Entities)

	return parsed, nil
}

// Execute parses and executes a natural language query.
func (e *NLQueryEngine) Execute(ctx context.Context, query string, userID string) (*NLQueryResult, error) {
	// Parse the query
	parsed, err := e.Parse(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	// Execute based on intent
	result := &NLQueryResult{
		ParsedQuery: parsed,
		Timestamp:   time.Now(),
	}

	// Add user ID to structured query
	if parsed.StructuredQuery != nil {
		parsed.StructuredQuery.UserID = userID
	}

	switch parsed.Intent {
	case IntentSimilar:
		// If we have a specific feature, find similar
		if len(parsed.Entities.FeatureIDs) > 0 {
			similar, err := e.discovery.FindSimilar(ctx, parsed.Entities.FeatureIDs[0], parsed.StructuredQuery.Limit)
			if err == nil {
				result.SimilarFeatures = similar
				result.ResultType = "similar"
			}
		} else {
			// Fall back to search
			discoveryResult, err := e.discovery.Discover(ctx, *parsed.StructuredQuery)
			if err == nil {
				result.DiscoveryResult = discoveryResult
				result.ResultType = "search"
			}
		}

	case IntentExplain:
		// Get feature explanation
		if len(parsed.Entities.FeatureIDs) > 0 {
			explanation, err := e.discovery.explainer.Explain(ctx, parsed.Entities.FeatureIDs[0], query, 1.0)
			if err == nil {
				result.Explanation = explanation
				result.ResultType = "explain"
			}
		} else {
			// Search first, then explain top result
			discoveryResult, err := e.discovery.Discover(ctx, *parsed.StructuredQuery)
			if err == nil && len(discoveryResult.Features) > 0 {
				result.DiscoveryResult = discoveryResult
				result.ResultType = "search_with_explanation"
			}
		}

	case IntentCount:
		// Get count
		parsed.StructuredQuery.Limit = 1000 // Get all
		discoveryResult, err := e.discovery.Discover(ctx, *parsed.StructuredQuery)
		if err == nil {
			result.Count = discoveryResult.TotalResults
			result.ResultType = "count"
		}

	case IntentTrending:
		// Get most popular
		central := e.discovery.GetMostCentralFeatures(parsed.StructuredQuery.Limit)
		if len(central) > 0 {
			result.TrendingFeatures = central
			result.ResultType = "trending"
		} else {
			// Fall back to search
			discoveryResult, err := e.discovery.Discover(ctx, *parsed.StructuredQuery)
			if err == nil {
				result.DiscoveryResult = discoveryResult
				result.ResultType = "search"
			}
		}

	default:
		// Default to search
		discoveryResult, err := e.discovery.Discover(ctx, *parsed.StructuredQuery)
		if err != nil {
			return nil, fmt.Errorf("discovery failed: %w", err)
		}
		result.DiscoveryResult = discoveryResult
		result.ResultType = "search"
	}

	result.ResponseTime = time.Since(result.Timestamp).Milliseconds()
	return result, nil
}

// NLQueryResult represents the result of a natural language query execution.
type NLQueryResult struct {
	// Parsed query info
	ParsedQuery *ParsedQuery `json:"parsed_query"`

	// Result type
	ResultType string `json:"result_type"`

	// Results (one of these will be populated)
	DiscoveryResult  *DiscoveryResult      `json:"discovery_result,omitempty"`
	SimilarFeatures  []DiscoveredFeature   `json:"similar_features,omitempty"`
	Explanation      *FeatureExplanation   `json:"explanation,omitempty"`
	TrendingFeatures []*FeatureNode        `json:"trending_features,omitempty"`
	Count            int                   `json:"count,omitempty"`

	// Metadata
	ResponseTime int64     `json:"response_time_ms"`
	Timestamp    time.Time `json:"timestamp"`
}

// GetSuggestions returns query suggestions based on partial input.
func (e *NLQueryEngine) GetSuggestions(ctx context.Context, partial string) ([]QuerySuggestion, error) {
	partial = strings.ToLower(strings.TrimSpace(partial))
	if partial == "" {
		return nil, nil
	}

	suggestions := make([]QuerySuggestion, 0)

	// Suggest based on common query patterns
	patterns := []struct {
		prefix     string
		completion string
		desc       string
	}{
		{"find", "find features for", "Search for features"},
		{"similar", "similar to", "Find similar features"},
		{"recommend", "recommend features for", "Get recommendations"},
		{"list", "list all", "List all features"},
		{"show", "show me", "Show features"},
		{"trending", "trending features", "Find popular features"},
		{"high quality", "high quality features", "Find reliable features"},
		{"owned by", "owned by ", "Find by owner"},
		{"in domain", "in domain ", "Filter by domain"},
		{"category", "category ", "Filter by category"},
	}

	for _, p := range patterns {
		if strings.HasPrefix(p.prefix, partial) || strings.Contains(p.prefix, partial) {
			suggestions = append(suggestions, QuerySuggestion{
				Query:       p.completion,
				Type:        "complete",
				Description: p.desc,
				Score:       0.8,
			})
		}
	}

	// Add autocomplete from feature names
	autocompletions, _ := e.discovery.AutoComplete(ctx, partial, 5)
	for _, ac := range autocompletions {
		suggestions = append(suggestions, QuerySuggestion{
			Query:       ac,
			Type:        "feature",
			Description: "Feature or tag",
			Score:       0.7,
		})
	}

	// Limit results
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions, nil
}

// RefreshKnownValues updates the known entity values from the index.
func (e *NLQueryEngine) RefreshKnownValues() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.populateKnownValues()
}

// GetStats returns NL query engine statistics.
func (e *NLQueryEngine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"known_categories":   len(e.entityExtractor.knownCategories),
		"known_domains":      len(e.entityExtractor.knownDomains),
		"known_owners":       len(e.entityExtractor.knownOwners),
		"known_tags":         len(e.entityExtractor.knownTags),
		"known_entity_types": len(e.entityExtractor.knownEntityTypes),
		"llm_enabled":        e.llmClient != nil,
		"config":             e.config,
	}
}
