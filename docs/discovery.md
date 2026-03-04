# AI-Powered Feature Discovery

Feather includes an intelligent feature discovery system that helps users find, explore, and understand features through semantic search, natural language queries, and personalized recommendations.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Feature Discovery System                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Semantic   │  │  NL Query    │  │   Recommendation     │  │
│  │   Search     │  │  Engine      │  │   Engine             │  │
│  │              │  │              │  │                      │  │
│  │ • Embeddings │  │ • Intent     │  │ • Collaborative      │  │
│  │ • TF-IDF     │  │ • Entities   │  │ • Content-Based      │  │
│  │ • Cosine Sim │  │ • LLM Parse  │  │ • Popularity         │  │
│  └──────┬───────┘  └──────┬───────┘  │ • Context-Aware      │  │
│         │                 │          └──────────┬───────────┘  │
│         └────────┬────────┴─────────────────────┘              │
│                  │                                              │
│         ┌────────▼────────┐                                    │
│         │    Discovery    │                                    │
│         │     Engine      │                                    │
│         │                 │                                    │
│         │ • Feature Graph │                                    │
│         │ • Personalization│                                   │
│         │ • AutoComplete  │                                    │
│         └────────┬────────┘                                    │
│                  │                                              │
│         ┌────────▼────────┐                                    │
│         │  Feature Store  │                                    │
│         │    Registry     │                                    │
│         └─────────────────┘                                    │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Semantic Search

The semantic search engine enables finding features by meaning rather than exact keyword matches.

**Location**: `internal/extensions/semantic/search.go`

#### Embeddings

Features are represented as vector embeddings for similarity matching:

```go
type Embedder interface {
    // Embed generates a vector embedding for the given text
    Embed(ctx context.Context, text string) ([]float64, error)

    // EmbedBatch generates embeddings for multiple texts
    EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

    // Dimension returns the embedding dimension
    Dimension() int
}
```

#### TF-IDF Fallback

When no external embedder is configured, the system uses TF-IDF (Term Frequency-Inverse Document Frequency):

```go
// TF-IDF calculation
tf := float64(termCount) / float64(totalTerms)
idf := math.Log(float64(totalDocs) / float64(docsWithTerm))
tfidf := tf * idf
```

#### Search Configuration

```go
type SearchConfig struct {
    // MinScore filters results below this threshold (0.0-1.0)
    MinScore float64

    // MaxResults limits the number of returned features
    MaxResults int

    // IncludeMetadata returns additional feature metadata
    IncludeMetadata bool

    // Filters narrow search scope
    Filters SearchFilters
}

type SearchFilters struct {
    Categories []string  // Filter by feature category
    Tags       []string  // Filter by tags
    Owner      string    // Filter by owner
    CreatedAfter  time.Time
    CreatedBefore time.Time
}
```

#### Similarity Calculation

Features are ranked using cosine similarity:

```go
// Cosine similarity between two vectors
similarity := dotProduct(a, b) / (magnitude(a) * magnitude(b))
```

### 2. Natural Language Query Engine

The NL Query Engine interprets human language questions about features.

**Location**: `internal/semantic/nlquery.go`

#### Intent Classification

The engine recognizes 13 different query intents:

| Intent | Description | Example Query |
|--------|-------------|---------------|
| `search` | Find features by description | "find user engagement features" |
| `similar` | Find similar features | "features like click_count" |
| `recommend` | Get personalized suggestions | "recommend features for me" |
| `explain` | Explain a feature | "what does purchase_amount mean" |
| `compare` | Compare features | "compare click_count vs view_count" |
| `list` | List features by criteria | "list all payment features" |
| `count` | Count matching features | "how many user features exist" |
| `filter` | Apply filters | "features with freshness > 0.9" |
| `related` | Find related features | "features related to user_id" |
| `trending` | Show popular features | "what features are trending" |
| `recent` | Show recently updated | "recently updated features" |
| `quality` | Filter by quality | "high quality features" |
| `owned` | Filter by owner | "features owned by team-ml" |

#### Entity Extraction

The engine extracts entities using pattern matching:

```go
// Entity patterns
patterns := map[string]*regexp.Regexp{
    "feature_name": regexp.MustCompile(`\b([a-z_]+_(?:count|sum|avg|rate|ratio))\b`),
    "category":     regexp.MustCompile(`\b(user|product|order|payment|session)\b`),
    "owner":        regexp.MustCompile(`\bowned?\s+by\s+(\S+)\b`),
    "time_range":   regexp.MustCompile(`\b(last|past)\s+(\d+)\s+(day|week|month)s?\b`),
    "threshold":    regexp.MustCompile(`\b(greater|less|above|below)\s+than?\s+([\d.]+)\b`),
}
```

#### LLM Integration (Optional)

For complex queries, an optional LLM can enhance parsing:

```go
type LLMParser interface {
    // Parse uses an LLM to understand the query
    Parse(ctx context.Context, query string) (*ParsedQuery, error)
}

type ParsedQuery struct {
    Intent      QueryIntent
    Entities    map[string]string
    Filters     map[string]interface{}
    Confidence  float64
}
```

#### Query Suggestions

The engine provides autocomplete suggestions:

```go
suggestions := nlEngine.Suggest(ctx, "user click")
// Returns: ["user_click_count", "user_click_rate", "user_click_through"]
```

### 3. Feature Relationship Graph

The discovery engine maintains a graph of feature relationships.

**Location**: `internal/semantic/discovery.go`

#### Relationship Types

```go
type EdgeType string

const (
    EdgeSimilar     EdgeType = "similar"     // Semantically similar features
    EdgeDerived     EdgeType = "derived"     // One derived from another
    EdgeCoUsed      EdgeType = "co_used"     // Frequently used together
    EdgeRelated     EdgeType = "related"     // General relationship
    EdgeAggregation EdgeType = "aggregation" // Aggregation relationship
)
```

#### Graph Structure

```go
type FeatureGraph struct {
    Nodes      map[string]*FeatureNode
    Edges      map[string][]*FeatureEdge
    Centrality map[string]float64  // PageRank-like scores
}

type FeatureNode struct {
    Name        string
    Category    string
    Description string
    Tags        []string
    Owner       string
    Quality     float64
    Popularity  float64
}

type FeatureEdge struct {
    Source   string
    Target   string
    Type     EdgeType
    Weight   float64
    Metadata map[string]interface{}
}
```

#### Centrality Computation

Features are ranked by importance using a PageRank-like algorithm:

```go
// Iterative centrality computation
for iteration := 0; iteration < maxIterations; iteration++ {
    for node := range nodes {
        sum := 0.0
        for _, incoming := range incomingEdges[node] {
            sum += centrality[incoming.Source] / outDegree[incoming.Source]
        }
        newCentrality[node] = damping + (1-damping) * sum
    }
}
```

### 4. Recommendation Engine

The recommendation engine provides personalized feature suggestions.

**Location**: `internal/semantic/recommend.go`

#### Recommendation Strategies

The engine combines four recommendation strategies:

| Strategy | Weight | Description |
|----------|--------|-------------|
| Collaborative | 35% | Based on similar users' preferences |
| Content-Based | 30% | Based on feature attributes |
| Popularity | 20% | Based on overall usage patterns |
| Context | 15% | Based on current session context |

#### Collaborative Filtering

```go
type CollaborativeFilter struct {
    UserSimilarity map[string]map[string]float64
    UserFeatures   map[string][]string
}

// Find similar users using Jaccard similarity
func (cf *CollaborativeFilter) SimilarUsers(userID string, k int) []string {
    similarities := make(map[string]float64)
    userFeatures := cf.UserFeatures[userID]

    for otherUser, otherFeatures := range cf.UserFeatures {
        if otherUser == userID {
            continue
        }
        // Jaccard similarity
        intersection := len(setIntersection(userFeatures, otherFeatures))
        union := len(setUnion(userFeatures, otherFeatures))
        similarities[otherUser] = float64(intersection) / float64(union)
    }

    return topK(similarities, k)
}
```

#### Content-Based Filtering

```go
type ContentBasedFilter struct {
    FeatureProfiles map[string]*FeatureProfile
    UserProfiles    map[string]*UserProfile
}

type FeatureProfile struct {
    Categories  map[string]float64
    Tags        map[string]float64
    Attributes  map[string]float64
}

// Match user profile against feature profiles
func (cbf *ContentBasedFilter) Score(userID, featureID string) float64 {
    userProfile := cbf.UserProfiles[userID]
    featureProfile := cbf.FeatureProfiles[featureID]
    return cosineSimilarity(userProfile.Vector(), featureProfile.Vector())
}
```

#### Popularity Model

Features are scored by popularity with time decay:

```go
type PopularityModel struct {
    AccessCounts map[string]int64
    LastAccess   map[string]time.Time
    DecayFactor  float64  // e.g., 0.95 per day
}

func (pm *PopularityModel) Score(featureID string) float64 {
    count := pm.AccessCounts[featureID]
    age := time.Since(pm.LastAccess[featureID])
    decay := math.Pow(pm.DecayFactor, age.Hours()/24)
    return float64(count) * decay
}
```

#### Context-Aware Recommendations

```go
type ContextModel struct {
    SessionFeatures []string      // Features accessed this session
    CurrentTask     string        // Current user task/goal
    TimeOfDay       time.Time     // For time-sensitive recommendations
}

// Boost features related to current session
func (cm *ContextModel) Score(featureID string, graph *FeatureGraph) float64 {
    score := 0.0
    for _, sessionFeature := range cm.SessionFeatures {
        if edge := graph.GetEdge(sessionFeature, featureID); edge != nil {
            score += edge.Weight
        }
    }
    return score
}
```

#### Diversity Factor

To avoid recommending only features from the same category:

```go
type DiversityConfig struct {
    Factor         float64 // 0.0 = no diversity, 1.0 = max diversity
    MaxPerCategory int     // Maximum features per category
}

func applyDiversity(recommendations []Recommendation, config DiversityConfig) []Recommendation {
    categoryCount := make(map[string]int)
    result := make([]Recommendation, 0)

    for _, rec := range recommendations {
        category := rec.Feature.Category
        if categoryCount[category] < config.MaxPerCategory {
            // Apply diversity penalty
            rec.Score *= (1 - config.Factor * float64(categoryCount[category]) / float64(config.MaxPerCategory))
            result = append(result, rec)
            categoryCount[category]++
        }
    }

    return result
}
```

## Discovery Engine

The main discovery engine coordinates all components.

**Location**: `internal/semantic/discovery.go`

### Query Interface

```go
type DiscoveryQuery struct {
    // Text query (natural language or keywords)
    Query string

    // Filters
    Categories []string
    Tags       []string
    Owner      string
    MinQuality float64

    // Pagination
    Offset int
    Limit  int

    // Personalization
    UserID         string
    IncludeHistory bool

    // Result options
    IncludeGraph    bool
    IncludeFacets   bool
    IncludeRelated  bool
}
```

### Discovery Response

```go
type DiscoveryResult struct {
    // Matching features
    Features []DiscoveredFeature

    // Total count (for pagination)
    Total int

    // Search facets
    Facets *SearchFacets

    // Related features graph
    Graph *FeatureGraph

    // Query interpretation
    ParsedQuery *ParsedQuery

    // Suggestions for refinement
    Suggestions []string
}

type DiscoveredFeature struct {
    Name        string
    Description string
    Category    string
    Tags        []string
    Owner       string

    // Scores
    RelevanceScore    float64
    QualityScore      float64
    PopularityScore   float64

    // Relationship info
    RelatedFeatures   []string
    SimilarFeatures   []string
}

type SearchFacets struct {
    Categories map[string]int  // Category -> count
    Tags       map[string]int  // Tag -> count
    Owners     map[string]int  // Owner -> count
    Quality    []QualityBucket // Quality distribution
}
```

### Key Methods

```go
// Main discovery entry point
func (d *FeatureDiscovery) Discover(ctx context.Context, query DiscoveryQuery) (*DiscoveryResult, error)

// Find similar features
func (d *FeatureDiscovery) FindSimilar(ctx context.Context, featureName string, limit int) ([]DiscoveredFeature, error)

// Autocomplete suggestions
func (d *FeatureDiscovery) AutoComplete(ctx context.Context, prefix string, limit int) ([]string, error)

// Get feature relationship graph
func (d *FeatureDiscovery) GetFeatureGraph(ctx context.Context, rootFeature string, depth int) (*FeatureGraph, error)

// Get personalized recommendations
func (d *FeatureDiscovery) Recommend(ctx context.Context, userID string, limit int) ([]Recommendation, error)
```

## API Reference

### HTTP Endpoints

#### Search Features

```http
GET /v1/discovery/search?q={query}&limit={limit}&offset={offset}
```

Query Parameters:
- `q` - Search query (natural language or keywords)
- `limit` - Maximum results (default: 20, max: 100)
- `offset` - Pagination offset
- `categories` - Filter by categories (comma-separated)
- `tags` - Filter by tags (comma-separated)
- `owner` - Filter by owner
- `min_quality` - Minimum quality score (0.0-1.0)

Response:
```json
{
  "success": true,
  "data": {
    "features": [
      {
        "name": "user_click_count",
        "description": "Total clicks by user in last 24h",
        "category": "engagement",
        "tags": ["user", "clicks", "real-time"],
        "owner": "team-ml",
        "relevance_score": 0.95,
        "quality_score": 0.88,
        "popularity_score": 0.72
      }
    ],
    "total": 42,
    "facets": {
      "categories": {"engagement": 15, "conversion": 12, "user": 10},
      "tags": {"real-time": 20, "aggregated": 18},
      "owners": {"team-ml": 25, "team-data": 17}
    },
    "parsed_query": {
      "intent": "search",
      "entities": {"category": "user"},
      "confidence": 0.92
    },
    "suggestions": ["user_session_count", "user_page_views"]
  }
}
```

#### Natural Language Query

```http
POST /v1/discovery/query
Content-Type: application/json

{
  "query": "show me features similar to click_count that are owned by team-ml",
  "user_id": "user123",
  "include_graph": true
}
```

Response:
```json
{
  "success": true,
  "data": {
    "parsed_query": {
      "intent": "similar",
      "entities": {
        "feature_name": "click_count",
        "owner": "team-ml"
      },
      "confidence": 0.88
    },
    "features": [...],
    "graph": {
      "nodes": [...],
      "edges": [...]
    }
  }
}
```

#### Get Recommendations

```http
GET /v1/discovery/recommend?user_id={user_id}&limit={limit}
```

Query Parameters:
- `user_id` - User identifier for personalization
- `limit` - Maximum recommendations (default: 10)
- `context` - Current context/task (optional)
- `diversity` - Diversity factor 0.0-1.0 (default: 0.3)

Response:
```json
{
  "success": true,
  "data": {
    "recommendations": [
      {
        "feature": {
          "name": "user_purchase_probability",
          "description": "ML-predicted purchase probability",
          "category": "prediction"
        },
        "score": 0.89,
        "reason": "Similar users frequently use this feature",
        "strategy": "collaborative"
      }
    ]
  }
}
```

#### Find Similar Features

```http
GET /v1/discovery/similar/{feature_name}?limit={limit}
```

Response:
```json
{
  "success": true,
  "data": {
    "feature": "click_count",
    "similar": [
      {
        "name": "tap_count",
        "similarity": 0.94,
        "relationship": "similar"
      },
      {
        "name": "click_rate",
        "similarity": 0.87,
        "relationship": "derived"
      }
    ]
  }
}
```

#### Autocomplete

```http
GET /v1/discovery/autocomplete?prefix={prefix}&limit={limit}
```

Response:
```json
{
  "success": true,
  "data": {
    "suggestions": [
      "user_click_count",
      "user_click_rate",
      "user_click_through_rate"
    ]
  }
}
```

#### Get Feature Graph

```http
GET /v1/discovery/graph/{feature_name}?depth={depth}
```

Response:
```json
{
  "success": true,
  "data": {
    "root": "user_click_count",
    "nodes": [
      {
        "name": "user_click_count",
        "category": "engagement",
        "centrality": 0.85
      },
      {
        "name": "user_view_count",
        "category": "engagement",
        "centrality": 0.72
      }
    ],
    "edges": [
      {
        "source": "user_click_count",
        "target": "user_view_count",
        "type": "co_used",
        "weight": 0.78
      }
    ]
  }
}
```

## Configuration

### Discovery Configuration

```yaml
discovery:
  # Semantic search settings
  search:
    # Minimum similarity score (0.0-1.0)
    min_score: 0.3

    # Maximum results per query
    max_results: 100

    # Embedding configuration
    embedder:
      # Type: "openai", "sentence-transformers", "tfidf"
      type: "tfidf"

      # For OpenAI embeddings
      openai:
        api_key: "${OPENAI_API_KEY}"
        model: "text-embedding-3-small"

      # For local sentence transformers
      sentence_transformers:
        model: "all-MiniLM-L6-v2"
        endpoint: "http://localhost:8000"

  # Natural language query settings
  nlquery:
    # Enable LLM parsing for complex queries
    llm_enabled: false

    # LLM configuration (if enabled)
    llm:
      provider: "openai"
      model: "gpt-4"
      api_key: "${OPENAI_API_KEY}"

    # Confidence threshold for intent classification
    confidence_threshold: 0.7

  # Recommendation settings
  recommendations:
    # Strategy weights (must sum to 1.0)
    weights:
      collaborative: 0.35
      content_based: 0.30
      popularity: 0.20
      context: 0.15

    # Diversity settings
    diversity:
      factor: 0.3
      max_per_category: 3

    # Popularity decay (per day)
    popularity_decay: 0.95

  # Feature graph settings
  graph:
    # Maximum graph depth for traversal
    max_depth: 5

    # Minimum edge weight to include
    min_edge_weight: 0.1

    # PageRank damping factor
    damping_factor: 0.85

    # PageRank iterations
    max_iterations: 100

  # Personalization settings
  personalization:
    # Enable user history tracking
    enabled: true

    # Maximum history items per user
    max_history: 1000

    # History retention period
    retention: "30d"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `FEATHER_DISCOVERY_ENABLED` | Enable discovery features | `true` |
| `FEATHER_DISCOVERY_MIN_SCORE` | Minimum similarity score | `0.3` |
| `FEATHER_DISCOVERY_MAX_RESULTS` | Maximum results | `100` |
| `FEATHER_EMBEDDER_TYPE` | Embedder type | `tfidf` |
| `FEATHER_NLQUERY_LLM_ENABLED` | Enable LLM parsing | `false` |
| `OPENAI_API_KEY` | OpenAI API key | - |

## Usage Examples

### Basic Search

```bash
# Simple keyword search
curl "http://localhost:8080/v1/discovery/search?q=user+engagement"

# Search with filters
curl "http://localhost:8080/v1/discovery/search?q=click&categories=engagement&min_quality=0.8"
```

### Natural Language Queries

```bash
# Find features by intent
curl -X POST "http://localhost:8080/v1/discovery/query" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "what features are similar to purchase_amount?",
    "user_id": "analyst-1"
  }'

# Complex query with multiple intents
curl -X POST "http://localhost:8080/v1/discovery/query" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "show trending features in the payment category owned by team-risk"
  }'
```

### Get Recommendations

```bash
# Personalized recommendations
curl "http://localhost:8080/v1/discovery/recommend?user_id=analyst-1&limit=10"

# With context
curl "http://localhost:8080/v1/discovery/recommend?user_id=analyst-1&context=fraud-detection"
```

### Explore Feature Relationships

```bash
# Get similar features
curl "http://localhost:8080/v1/discovery/similar/user_click_count?limit=5"

# Get feature graph
curl "http://localhost:8080/v1/discovery/graph/user_click_count?depth=2"
```

### SDK Usage

**Go:**
```go
client := feather.NewClient("http://localhost:8080")

// Search
results, err := client.Discovery.Search(ctx, feather.SearchRequest{
    Query:      "user engagement features",
    Categories: []string{"engagement"},
    Limit:      20,
})

// Natural language query
results, err := client.Discovery.Query(ctx,
    "find features similar to click_count with quality above 0.8")

// Recommendations
recs, err := client.Discovery.Recommend(ctx, feather.RecommendRequest{
    UserID: "analyst-1",
    Limit:  10,
})

// Feature graph
graph, err := client.Discovery.GetGraph(ctx, "user_click_count", 2)
```

**Python:**
```python
from feather import Client

client = Client("http://localhost:8080")

# Search
results = client.discovery.search(
    query="user engagement features",
    categories=["engagement"],
    limit=20
)

# Natural language query
results = client.discovery.query(
    "find features similar to click_count with quality above 0.8"
)

# Recommendations
recs = client.discovery.recommend(user_id="analyst-1", limit=10)

# Feature graph
graph = client.discovery.get_graph("user_click_count", depth=2)
```

## Best Practices

### 1. Feature Metadata

Good metadata improves discovery quality:

```json
{
  "name": "user_7d_purchase_count",
  "description": "Number of purchases made by user in the last 7 days",
  "category": "transaction",
  "tags": ["user", "purchase", "rolling-window", "7-day"],
  "owner": "team-commerce",
  "documentation": "https://wiki.example.com/features/purchase-count"
}
```

### 2. Query Optimization

- Use specific terms for better results
- Combine filters with search queries
- Leverage autocomplete for exploration

### 3. Personalization

- Enable user history for better recommendations
- Provide context when available
- Use feedback to improve results

### 4. Graph Exploration

- Start with central features (high centrality)
- Explore related features for discovery
- Use graph for impact analysis

## Troubleshooting

### Poor Search Results

1. **Check embedding quality:**
   ```bash
   curl "http://localhost:8080/v1/discovery/debug/embedding?text=click+count"
   ```

2. **Verify feature metadata:**
   ```bash
   curl "http://localhost:8080/v1/schema/groups"
   ```

3. **Lower minimum score:**
   ```yaml
   discovery:
     search:
       min_score: 0.2  # Lower threshold
   ```

### Slow Discovery Queries

1. **Check embedding cache:**
   ```bash
   curl "http://localhost:8080/v1/discovery/stats"
   ```

2. **Reduce result limits:**
   ```yaml
   discovery:
     search:
       max_results: 50
   ```

3. **Use TF-IDF for faster (but less accurate) results:**
   ```yaml
   discovery:
     search:
       embedder:
         type: "tfidf"
   ```

### Recommendation Quality Issues

1. **Verify user history exists:**
   ```bash
   curl "http://localhost:8080/v1/discovery/users/{user_id}/history"
   ```

2. **Adjust strategy weights:**
   ```yaml
   discovery:
     recommendations:
       weights:
         collaborative: 0.20  # Reduce if few users
         content_based: 0.40  # Increase for cold start
         popularity: 0.30
         context: 0.10
   ```

3. **Increase diversity for varied results:**
   ```yaml
   discovery:
     recommendations:
       diversity:
         factor: 0.5
   ```

## Metrics

The discovery system exposes Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `feather_discovery_queries_total` | Counter | Total discovery queries |
| `feather_discovery_query_duration_seconds` | Histogram | Query latency |
| `feather_discovery_results_count` | Histogram | Results per query |
| `feather_discovery_cache_hits_total` | Counter | Embedding cache hits |
| `feather_discovery_cache_misses_total` | Counter | Embedding cache misses |
| `feather_recommendations_total` | Counter | Recommendation requests |
| `feather_nlquery_intents_total` | Counter | Intents by type |
| `feather_graph_traversals_total` | Counter | Graph traversal operations |
