package semantic

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// NLDiscoveryResult represents a natural language query result.
type NLDiscoveryResult struct {
	Query          string              `json:"query"`
	Intent         string              `json:"intent"`
	Features       []NLFeatureMatch    `json:"features"`
	Suggestions    []string            `json:"suggestions,omitempty"`
	Pipeline       *NLPipeline         `json:"pipeline,omitempty"`
	Confidence     float64             `json:"confidence"`
	ProcessedAt    time.Time           `json:"processed_at"`
}

// NLFeatureMatch is a feature matched by NL search.
type NLFeatureMatch struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	EntityType  string            `json:"entity_type,omitempty"`
	Score       float64           `json:"score"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Reason      string            `json:"reason"`
}

// NLPipeline represents an auto-composed feature pipeline.
type NLPipeline struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Steps       []NLPipelineStep `json:"steps"`
	FeatherQL   string       `json:"featherql,omitempty"`
}

// NLPipelineStep is one step in a composed pipeline.
type NLPipelineStep struct {
	Type       string   `json:"type"` // source, transform, aggregate, join
	Feature    string   `json:"feature,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Inputs     []string `json:"inputs,omitempty"`
	Window     string   `json:"window,omitempty"`
}

// ConversationMessage represents a chat message.
type ConversationMessage struct {
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation tracks a multi-turn feature discovery session.
type Conversation struct {
	ID       string                `json:"id"`
	Messages []ConversationMessage `json:"messages"`
	Context  map[string]string     `json:"context,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// NLDiscoveryEngine provides natural language feature discovery.
type NLDiscoveryEngine struct {
	mu            sync.RWMutex
	search        *Search
	conversations map[string]*Conversation
	queryHistory  []NLDiscoveryResult
	maxHistory    int

	// Known feature catalog for matching
	featureCatalog []catalogEntry
}

type catalogEntry struct {
	name        string
	description string
	entityType  string
	tags        []string
}

// NewNLDiscoveryEngine creates a new NL discovery engine.
func NewNLDiscoveryEngine(search *Search) *NLDiscoveryEngine {
	return &NLDiscoveryEngine{
		search:        search,
		conversations: make(map[string]*Conversation),
		maxHistory:    1000,
	}
}

// RegisterFeature adds a feature to the discovery catalog.
func (e *NLDiscoveryEngine) RegisterFeature(name, description, entityType string, tags []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.featureCatalog = append(e.featureCatalog, catalogEntry{
		name:        name,
		description: description,
		entityType:  entityType,
		tags:        tags,
	})
}

// Query processes a natural language feature query.
func (e *NLDiscoveryEngine) Query(query string) *NLDiscoveryResult {
	e.mu.RLock()
	catalog := make([]catalogEntry, len(e.featureCatalog))
	copy(catalog, e.featureCatalog)
	e.mu.RUnlock()

	result := &NLDiscoveryResult{
		Query:       query,
		ProcessedAt: time.Now(),
	}

	// Detect intent
	result.Intent = detectIntent(query)

	// Score features against query
	queryLower := strings.ToLower(query)
	queryTerms := nlTokenize(queryLower)

	var matches []NLFeatureMatch
	for _, entry := range catalog {
		score := scoreMatch(queryTerms, entry)
		if score > 0.1 {
			matches = append(matches, NLFeatureMatch{
				Name:        entry.name,
				Description: entry.description,
				EntityType:  entry.entityType,
				Score:       score,
				Tags:        entry.tags,
				Reason:      explainMatch(queryTerms, entry),
			})
		}
	}

	// Sort by score
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	if len(matches) > 20 {
		matches = matches[:20]
	}
	result.Features = matches

	// Generate suggestions
	result.Suggestions = generateSuggestions(query, result.Intent, matches)

	// Auto-compose pipeline if applicable
	if result.Intent == "compose" || result.Intent == "aggregate" {
		result.Pipeline = composePipeline(query, matches)
	}

	// Confidence
	if len(matches) > 0 {
		result.Confidence = matches[0].Score
	}

	// Store in history
	e.mu.Lock()
	e.queryHistory = append(e.queryHistory, *result)
	if len(e.queryHistory) > e.maxHistory {
		e.queryHistory = e.queryHistory[1:]
	}
	e.mu.Unlock()

	return result
}

// Chat processes a conversational message.
func (e *NLDiscoveryEngine) Chat(convID, message string) (*NLDiscoveryResult, *Conversation) {
	e.mu.Lock()
	conv, exists := e.conversations[convID]
	if !exists {
		conv = &Conversation{
			ID:        convID,
			Messages:  []ConversationMessage{},
			Context:   make(map[string]string),
			CreatedAt: time.Now(),
		}
		e.conversations[convID] = conv
	}

	conv.Messages = append(conv.Messages, ConversationMessage{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	})
	conv.UpdatedAt = time.Now()
	e.mu.Unlock()

	// Process query with conversation context
	result := e.Query(message)

	// Generate assistant response
	var response string
	if len(result.Features) > 0 {
		response = fmt.Sprintf("I found %d relevant features. The best match is '%s'",
			len(result.Features), result.Features[0].Name)
		if result.Features[0].Description != "" {
			response += fmt.Sprintf(" — %s", result.Features[0].Description)
		}
		response += "."
		if len(result.Suggestions) > 0 {
			response += fmt.Sprintf(" You might also want to explore: %s", strings.Join(result.Suggestions[:min(3, len(result.Suggestions))], ", "))
		}
	} else {
		response = "I couldn't find matching features. Try describing what data or metrics you need."
	}

	e.mu.Lock()
	conv.Messages = append(conv.Messages, ConversationMessage{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})
	e.mu.Unlock()

	return result, conv
}

// GetConversation returns a conversation by ID.
func (e *NLDiscoveryEngine) GetConversation(id string) (*Conversation, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	conv, exists := e.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation %s not found", id)
	}
	return conv, nil
}

// GetQueryHistory returns recent query results.
func (e *NLDiscoveryEngine) GetQueryHistory(limit int) []NLDiscoveryResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.queryHistory) {
		limit = len(e.queryHistory)
	}
	start := len(e.queryHistory) - limit
	out := make([]NLDiscoveryResult, limit)
	copy(out, e.queryHistory[start:])
	return out
}

// --- Internal helpers ---

func detectIntent(query string) string {
	q := strings.ToLower(query)
	switch {
	case strings.Contains(q, "compose") || strings.Contains(q, "pipeline") || strings.Contains(q, "create"):
		return "compose"
	case strings.Contains(q, "aggregate") || strings.Contains(q, "sum") || strings.Contains(q, "average") || strings.Contains(q, "count"):
		return "aggregate"
	case strings.Contains(q, "find") || strings.Contains(q, "search") || strings.Contains(q, "show"):
		return "search"
	case strings.Contains(q, "how") || strings.Contains(q, "what") || strings.Contains(q, "explain"):
		return "explore"
	default:
		return "search"
	}
}

func nlTokenize(s string) []string {
	words := strings.FieldsFunc(s, func(c rune) bool {
		return !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_')
	})
	var tokens []string
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"for": true, "to": true, "of": true, "in": true, "and": true,
		"or": true, "with": true, "that": true, "this": true, "me": true,
		"all": true, "my": true, "i": true, "can": true, "you": true,
	}
	for _, w := range words {
		if !stopWords[w] && len(w) > 1 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func scoreMatch(queryTerms []string, entry catalogEntry) float64 {
	if len(queryTerms) == 0 {
		return 0
	}

	var totalScore float64
	nameLower := strings.ToLower(entry.name)
	descLower := strings.ToLower(entry.description)
	entityLower := strings.ToLower(entry.entityType)
	tagsLower := strings.ToLower(strings.Join(entry.tags, " "))

	for _, term := range queryTerms {
		var termScore float64
		if strings.Contains(nameLower, term) {
			termScore = 1.0
		}
		if strings.Contains(descLower, term) {
			termScore = math.Max(termScore, 0.7)
		}
		if strings.Contains(entityLower, term) {
			termScore = math.Max(termScore, 0.5)
		}
		if strings.Contains(tagsLower, term) {
			termScore = math.Max(termScore, 0.4)
		}
		totalScore += termScore
	}

	return totalScore / float64(len(queryTerms))
}

func explainMatch(queryTerms []string, entry catalogEntry) string {
	nameLower := strings.ToLower(entry.name)
	for _, term := range queryTerms {
		if strings.Contains(nameLower, term) {
			return fmt.Sprintf("Name matches '%s'", term)
		}
	}
	descLower := strings.ToLower(entry.description)
	for _, term := range queryTerms {
		if strings.Contains(descLower, term) {
			return fmt.Sprintf("Description mentions '%s'", term)
		}
	}
	return "Partial match"
}

func generateSuggestions(query, intent string, matches []NLFeatureMatch) []string {
	var suggestions []string
	switch intent {
	case "search":
		if len(matches) > 3 {
			suggestions = append(suggestions, "Try narrowing your search with entity type or tags")
		}
		suggestions = append(suggestions, "Try: 'show all user features'")
	case "compose":
		suggestions = append(suggestions, "Try: 'create pipeline combining purchase_count and return_rate'")
	case "aggregate":
		suggestions = append(suggestions, "Try: 'aggregate click_count with sum over 24h window'")
	}
	return suggestions
}

func composePipeline(query string, matches []NLFeatureMatch) *NLPipeline {
	if len(matches) == 0 {
		return nil
	}

	pipeline := &NLPipeline{
		Name:        "auto_composed_pipeline",
		Description: fmt.Sprintf("Auto-composed from query: %s", query),
	}

	// Build pipeline steps from matched features
	for i, m := range matches {
		if i >= 5 {
			break
		}
		step := NLPipelineStep{
			Type:    "source",
			Feature: m.Name,
		}
		if i > 0 {
			step.Type = "transform"
			step.Inputs = []string{matches[0].Name}
		}
		pipeline.Steps = append(pipeline.Steps, step)
	}

	// Generate FeatherQL
	if len(matches) > 0 {
		cols := make([]string, 0)
		for i, m := range matches {
			if i >= 5 {
				break
			}
			cols = append(cols, m.Name)
		}
		entityType := "events"
		if matches[0].EntityType != "" {
			entityType = matches[0].EntityType
		}
		pipeline.FeatherQL = fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), entityType)
	}

	return pipeline
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
