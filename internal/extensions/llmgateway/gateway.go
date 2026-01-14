package llmgateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Provider identifies an LLM provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOllama    Provider = "ollama"
	ProviderCustom    Provider = "custom"
)

// Per-1K-token cost by provider (combined input+output estimate).
var providerCostPer1K = map[Provider]float64{
	ProviderOpenAI:    0.03,
	ProviderAnthropic: 0.025,
	ProviderOllama:    0.0,
	ProviderCustom:    0.01,
}

// GatewayConfig configures the LLM gateway.
type GatewayConfig struct {
	MaxCacheEntries     int
	CacheTTL            time.Duration
	DefaultProvider     Provider
	RateLimitPerMinute  int
	EnableABTesting     bool
	CostTrackingEnabled bool
}

// DefaultGatewayConfig returns sensible defaults.
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		MaxCacheEntries:     10000,
		CacheTTL:            1 * time.Hour,
		DefaultProvider:     ProviderOpenAI,
		RateLimitPerMinute:  60,
		EnableABTesting:     true,
		CostTrackingEnabled: true,
	}
}

// Gateway is the unified LLM feature gateway.
type Gateway struct {
	config      GatewayConfig
	mu          sync.RWMutex
	cache       map[string]*CacheEntry
	templates   map[string]*PromptTemplate
	abTests     map[string]*ABTest
	rateLimiter *RateLimiter
	costTracker *CostTracker
	stats       GatewayStats
}

// CacheEntry represents a cached LLM response.
type CacheEntry struct {
	Key       string   `json:"key"`
	Prompt    string   `json:"prompt"`
	Response  string   `json:"response"`
	Model     string   `json:"model"`
	Provider  Provider `json:"provider"`
	TokensIn  int      `json:"tokens_in"`
	TokensOut int      `json:"tokens_out"`
	CostUSD   float64  `json:"cost_usd"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	HitCount  int64    `json:"hit_count"`
}

// PromptTemplate defines a reusable prompt template with {{.variable}} syntax.
type PromptTemplate struct {
	Name        string            `json:"name"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables"`
	Provider    Provider          `json:"provider"`
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ABTest defines an A/B prompt experiment.
type ABTest struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	VariantA   ABVariant     `json:"variant_a"`
	VariantB   ABVariant     `json:"variant_b"`
	TrafficPct float64       `json:"traffic_pct_b"`
	Status     string        `json:"status"`
	Results    ABTestResults `json:"results"`
	CreatedAt  time.Time     `json:"created_at"`
}

// ABVariant describes one side of an A/B test.
type ABVariant struct {
	TemplateName string   `json:"template_name"`
	Model        string   `json:"model"`
	Provider     Provider `json:"provider"`
}

// ABTestResults tracks aggregate metrics for each variant.
type ABTestResults struct {
	VariantACalls      int64   `json:"variant_a_calls"`
	VariantBCalls      int64   `json:"variant_b_calls"`
	VariantAAvgCost    float64 `json:"variant_a_avg_cost"`
	VariantBAvgCost    float64 `json:"variant_b_avg_cost"`
	VariantAAvgLatency float64 `json:"variant_a_avg_latency_ms"`
	VariantBAvgLatency float64 `json:"variant_b_avg_latency_ms"`
}

// RateLimiter implements per-client token-bucket rate limiting.
type RateLimiter struct {
	mu     sync.Mutex
	tokens map[string]*tokenBucket
	rate   int // tokens per minute
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	rate       float64 // tokens per second
}

// CostTracker accumulates per-provider cost and savings records.
type CostTracker struct {
	mu      sync.RWMutex
	records map[string]*CostRecord
	total   float64
}

// CostRecord holds aggregate cost data for a single provider.
type CostRecord struct {
	Provider     Provider `json:"provider"`
	TotalCost    float64  `json:"total_cost_usd"`
	TotalTokens  int64    `json:"total_tokens"`
	RequestCount int64    `json:"request_count"`
	CostSaved    float64  `json:"cost_saved_usd"`
}

// GatewayStats tracks high-level gateway metrics.
type GatewayStats struct {
	CacheHits   atomic.Int64
	CacheMisses atomic.Int64
	TotalCalls  atomic.Int64
	RateLimited atomic.Int64
}

// LookupRequest is the input for a cache lookup.
type LookupRequest struct {
	Prompt   string   `json:"prompt"`
	Model    string   `json:"model,omitempty"`
	Provider Provider `json:"provider,omitempty"`
}

// LookupResponse is the result of a cache lookup.
type LookupResponse struct {
	Hit    bool        `json:"hit"`
	Entry  *CacheEntry `json:"entry,omitempty"`
	Source string      `json:"source"`
}

// StoreRequest is the input for storing a response in the cache.
type StoreRequest struct {
	Prompt    string   `json:"prompt"`
	Response  string   `json:"response"`
	Model     string   `json:"model"`
	Provider  Provider `json:"provider"`
	TokensIn  int      `json:"tokens_in"`
	TokensOut int      `json:"tokens_out"`
}

// RenderRequest is the input for rendering a prompt template.
type RenderRequest struct {
	TemplateName string            `json:"template_name"`
	Variables    map[string]string `json:"variables"`
}

// RenderResponse is the result of rendering a prompt template.
type RenderResponse struct {
	RenderedPrompt string   `json:"rendered_prompt"`
	Model          string   `json:"model"`
	Provider       Provider `json:"provider"`
	MaxTokens      int      `json:"max_tokens"`
	EstimatedCost  float64  `json:"estimated_cost_usd"`
}

// NewGateway creates a new LLM feature gateway.
func NewGateway(cfg GatewayConfig) *Gateway {
	return &Gateway{
		config:    cfg,
		cache:     make(map[string]*CacheEntry),
		templates: make(map[string]*PromptTemplate),
		abTests:   make(map[string]*ABTest),
		rateLimiter: &RateLimiter{
			tokens: make(map[string]*tokenBucket),
			rate:   cfg.RateLimitPerMinute,
		},
		costTracker: &CostTracker{
			records: make(map[string]*CostRecord),
		},
	}
}

// Lookup checks the cache for a matching prompt.
func (g *Gateway) Lookup(req LookupRequest) *LookupResponse {
	g.stats.TotalCalls.Add(1)

	key := g.cacheKey(req.Prompt, req.Model, req.Provider)

	g.mu.RLock()
	entry, ok := g.cache[key]
	g.mu.RUnlock()

	if ok && time.Now().Before(entry.ExpiresAt) {
		g.stats.CacheHits.Add(1)

		g.mu.Lock()
		entry.HitCount++
		g.mu.Unlock()

		return &LookupResponse{Hit: true, Entry: entry, Source: "cache"}
	}

	g.stats.CacheMisses.Add(1)
	return &LookupResponse{Hit: false, Source: "miss"}
}

// Store saves an LLM response in the cache and tracks cost.
func (g *Gateway) Store(req StoreRequest) (*CacheEntry, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt must not be empty")
	}
	if req.Response == "" {
		return nil, fmt.Errorf("response must not be empty")
	}

	cost := estimateCost(req.Provider, req.TokensIn+req.TokensOut)
	key := g.cacheKey(req.Prompt, req.Model, req.Provider)

	entry := &CacheEntry{
		Key:       key,
		Prompt:    req.Prompt,
		Response:  req.Response,
		Model:     req.Model,
		Provider:  req.Provider,
		TokensIn:  req.TokensIn,
		TokensOut: req.TokensOut,
		CostUSD:   cost,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(g.config.CacheTTL),
	}

	g.mu.Lock()
	// Evict oldest when at capacity.
	if len(g.cache) >= g.config.MaxCacheEntries {
		g.evictOldest()
	}
	g.cache[key] = entry
	g.mu.Unlock()

	if g.config.CostTrackingEnabled {
		g.costTracker.record(req.Provider, cost, req.TokensIn+req.TokensOut)
	}

	return entry, nil
}

// RegisterTemplate registers a new prompt template.
func (g *Gateway) RegisterTemplate(tmpl PromptTemplate) error {
	if tmpl.Name == "" {
		return fmt.Errorf("template name must not be empty")
	}
	if tmpl.Template == "" {
		return fmt.Errorf("template body must not be empty")
	}

	tmpl.Variables = extractVariables(tmpl.Template)
	tmpl.CreatedAt = time.Now()

	g.mu.Lock()
	g.templates[tmpl.Name] = &tmpl
	g.mu.Unlock()

	return nil
}

// RenderTemplate renders a registered template with the given variables.
func (g *Gateway) RenderTemplate(req RenderRequest) (*RenderResponse, error) {
	g.mu.RLock()
	tmpl, ok := g.templates[req.TemplateName]
	g.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("template %q not found", req.TemplateName)
	}

	rendered := tmpl.Template
	for k, v := range req.Variables {
		rendered = strings.ReplaceAll(rendered, "{{."+k+"}}", v)
	}

	// Rough token estimate: ~4 chars per token.
	estimatedTokens := len(rendered) / 4
	if tmpl.MaxTokens > 0 {
		estimatedTokens += tmpl.MaxTokens
	}

	provider := tmpl.Provider
	if provider == "" {
		provider = g.config.DefaultProvider
	}

	return &RenderResponse{
		RenderedPrompt: rendered,
		Model:          tmpl.Model,
		Provider:       provider,
		MaxTokens:      tmpl.MaxTokens,
		EstimatedCost:  estimateCost(provider, estimatedTokens),
	}, nil
}

// ListTemplates returns all registered templates.
func (g *Gateway) ListTemplates() []*PromptTemplate {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make([]*PromptTemplate, 0, len(g.templates))
	for _, t := range g.templates {
		out = append(out, t)
	}
	return out
}

// CreateABTest creates a new A/B prompt test.
func (g *Gateway) CreateABTest(test ABTest) (*ABTest, error) {
	if test.ID == "" {
		return nil, fmt.Errorf("test ID must not be empty")
	}
	if test.VariantA.TemplateName == "" || test.VariantB.TemplateName == "" {
		return nil, fmt.Errorf("both variants must specify a template name")
	}
	if test.TrafficPct < 0 || test.TrafficPct > 100 {
		return nil, fmt.Errorf("traffic_pct_b must be between 0 and 100")
	}

	test.Status = "active"
	test.CreatedAt = time.Now()

	g.mu.Lock()
	g.abTests[test.ID] = &test
	g.mu.Unlock()

	return &test, nil
}

// ResolveABTest deterministically assigns an entity to a variant.
func (g *Gateway) ResolveABTest(testID, entityID string) (*ABVariant, string, error) {
	g.mu.RLock()
	test, ok := g.abTests[testID]
	g.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("A/B test %q not found", testID)
	}
	if test.Status != "active" {
		return nil, "", fmt.Errorf("A/B test %q is not active", testID)
	}

	bucket := hashToBucket(entityID)
	threshold := int(test.TrafficPct)

	g.mu.Lock()
	defer g.mu.Unlock()

	if bucket < threshold {
		test.Results.VariantBCalls++
		return &test.VariantB, "B", nil
	}
	test.Results.VariantACalls++
	return &test.VariantA, "A", nil
}

// GetABTestResults returns the current state of an A/B test.
func (g *Gateway) GetABTestResults(testID string) (*ABTest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	test, ok := g.abTests[testID]
	if !ok {
		return nil, fmt.Errorf("A/B test %q not found", testID)
	}
	return test, nil
}

// ListABTests returns all A/B tests.
func (g *Gateway) ListABTests() []*ABTest {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make([]*ABTest, 0, len(g.abTests))
	for _, t := range g.abTests {
		out = append(out, t)
	}
	return out
}

// AllowRequest returns true if the client has not exceeded its rate limit.
func (g *Gateway) AllowRequest(clientID string) bool {
	g.rateLimiter.mu.Lock()
	defer g.rateLimiter.mu.Unlock()

	now := time.Now()
	b, ok := g.rateLimiter.tokens[clientID]
	if !ok {
		b = &tokenBucket{
			tokens:     float64(g.rateLimiter.rate),
			lastRefill: now,
			rate:       float64(g.rateLimiter.rate) / 60.0, // per second
		}
		g.rateLimiter.tokens[clientID] = b
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > float64(g.rateLimiter.rate) {
		b.tokens = float64(g.rateLimiter.rate)
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens--
		return true
	}

	g.stats.RateLimited.Add(1)
	return false
}

// GetCosts returns per-provider cost records.
func (g *Gateway) GetCosts() map[string]*CostRecord {
	g.costTracker.mu.RLock()
	defer g.costTracker.mu.RUnlock()

	out := make(map[string]*CostRecord, len(g.costTracker.records))
	for k, v := range g.costTracker.records {
		cp := *v
		out[k] = &cp
	}
	return out
}

// GetStats returns a snapshot of gateway metrics.
func (g *Gateway) GetStats() map[string]interface{} {
	hits := g.stats.CacheHits.Load()
	misses := g.stats.CacheMisses.Load()
	total := hits + misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	g.mu.RLock()
	cacheSize := len(g.cache)
	templateCount := len(g.templates)
	abTestCount := len(g.abTests)
	g.mu.RUnlock()

	return map[string]interface{}{
		"cache_hits":   hits,
		"cache_misses": misses,
		"hit_rate":     hitRate,
		"total_calls":  g.stats.TotalCalls.Load(),
		"rate_limited": g.stats.RateLimited.Load(),
		"cache_size":   cacheSize,
		"templates":    templateCount,
		"ab_tests":     abTestCount,
	}
}

// --------------- internal helpers ---------------

func (g *Gateway) cacheKey(prompt, model string, provider Provider) string {
	h := sha256.New()
	h.Write([]byte(string(provider)))
	h.Write([]byte{0})
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))
}

func (g *Gateway) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range g.cache {
		if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.CreatedAt
		}
	}
	if oldestKey != "" {
		delete(g.cache, oldestKey)
	}
}

func (ct *CostTracker) record(provider Provider, cost float64, tokens int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	key := string(provider)
	rec, ok := ct.records[key]
	if !ok {
		rec = &CostRecord{Provider: provider}
		ct.records[key] = rec
	}
	rec.TotalCost += cost
	rec.TotalTokens += int64(tokens)
	rec.RequestCount++
	ct.total += cost
}

func estimateCost(provider Provider, tokens int) float64 {
	rate, ok := providerCostPer1K[provider]
	if !ok {
		rate = providerCostPer1K[ProviderCustom]
	}
	return float64(tokens) / 1000.0 * rate
}

var varPattern = regexp.MustCompile(`\{\{\.(\w+)\}\}`)

func extractVariables(tmpl string) []string {
	matches := varPattern.FindAllStringSubmatch(tmpl, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	return vars
}

func hashToBucket(entityID string) int {
	h := sha256.Sum256([]byte(entityID))
	val := int(h[0])<<8 | int(h[1])
	return val % 100
}

