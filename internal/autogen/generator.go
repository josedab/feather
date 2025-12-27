package autogen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"
)

// LLMProvider represents an LLM provider type.
type LLMProvider string

const (
	ProviderOpenAI    LLMProvider = "openai"
	ProviderAnthropic LLMProvider = "anthropic"
	ProviderOllama    LLMProvider = "ollama"
	ProviderLocal     LLMProvider = "local"
)

// Config holds auto-generation configuration.
type Config struct {
	Provider    LLMProvider
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float32
	Timeout     time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Provider:    ProviderLocal,
		Model:       "gpt-4",
		MaxTokens:   2048,
		Temperature: 0.7,
		Timeout:     30 * time.Second,
	}
}

// Generator generates features using LLMs.
type Generator struct {
	mu         sync.RWMutex
	config     Config
	httpClient *http.Client
	templates  map[string]*template.Template
	history    []*GenerationResult
}

// SchemaField represents a field in a data schema.
type SchemaField struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Nullable    bool              `json:"nullable,omitempty"`
	Examples    []interface{}     `json:"examples,omitempty"`
	Constraints map[string]string `json:"constraints,omitempty"`
}

// DataSchema represents a data source schema.
type DataSchema struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Fields      []SchemaField `json:"fields"`
	Entity      string        `json:"entity,omitempty"`
	Source      string        `json:"source,omitempty"`
}

// FeatureSuggestion represents a suggested feature.
type FeatureSuggestion struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Expression  string            `json:"expression"`
	DataType    string            `json:"data_type"`
	Category    string            `json:"category"`
	Tags        []string          `json:"tags"`
	Confidence  float32           `json:"confidence"`
	Rationale   string            `json:"rationale"`
	Dependencies []string         `json:"dependencies,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// GenerationRequest represents a feature generation request.
type GenerationRequest struct {
	Schema         *DataSchema         `json:"schema"`
	ExistingFeatures []string          `json:"existing_features,omitempty"`
	UseCase        string              `json:"use_case,omitempty"`
	Constraints    *GenerationConstraints `json:"constraints,omitempty"`
	MaxSuggestions int                 `json:"max_suggestions"`
}

// GenerationConstraints defines constraints for feature generation.
type GenerationConstraints struct {
	AllowedTypes    []string `json:"allowed_types,omitempty"`
	ForbiddenFields []string `json:"forbidden_fields,omitempty"`
	RequireTags     []string `json:"require_tags,omitempty"`
	MinConfidence   float32  `json:"min_confidence,omitempty"`
	MaxComplexity   int      `json:"max_complexity,omitempty"`
}

// GenerationResult holds the result of feature generation.
type GenerationResult struct {
	ID          string              `json:"id"`
	Request     *GenerationRequest  `json:"request"`
	Suggestions []FeatureSuggestion `json:"suggestions"`
	CreatedAt   time.Time           `json:"created_at"`
	Duration    time.Duration       `json:"duration"`
	TokensUsed  int                 `json:"tokens_used,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// TransformationSuggestion represents a suggested transformation.
type TransformationSuggestion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputType   string   `json:"input_type"`
	OutputType  string   `json:"output_type"`
	Expression  string   `json:"expression"`
	Examples    []string `json:"examples,omitempty"`
}

// AggregationSuggestion represents a suggested aggregation.
type AggregationSuggestion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Function    string   `json:"function"`
	Window      string   `json:"window"`
	GroupBy     []string `json:"group_by,omitempty"`
	Expression  string   `json:"expression"`
}

// NewGenerator creates a new feature generator.
func NewGenerator(config Config) *Generator {
	g := &Generator{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		templates: make(map[string]*template.Template),
		history:   make([]*GenerationResult, 0),
	}

	g.initTemplates()
	return g
}

func (g *Generator) initTemplates() {
	// Feature generation prompt template
	featurePrompt := `You are an expert feature engineer. Given a data schema, suggest useful features for machine learning.

Data Schema: {{.SchemaName}}
Description: {{.SchemaDescription}}

Fields:
{{range .Fields}}- {{.Name}} ({{.Type}}): {{.Description}}
{{end}}

{{if .UseCase}}Use Case: {{.UseCase}}{{end}}

{{if .ExistingFeatures}}
Existing Features (do not duplicate):
{{range .ExistingFeatures}}- {{.}}
{{end}}
{{end}}

Please suggest up to {{.MaxSuggestions}} new features. For each feature, provide:
1. A unique ID (snake_case)
2. A descriptive name
3. A clear description
4. The expression/formula to compute it
5. The resulting data type
6. A category (user_behavior, product_metrics, temporal, aggregation, etc.)
7. Relevant tags
8. Confidence score (0-1)
9. Rationale for why this feature would be useful

Respond in JSON format with an array of feature suggestions.`

	g.templates["feature_generation"] = template.Must(template.New("feature_generation").Parse(featurePrompt))

	// Transformation suggestion prompt
	transformPrompt := `You are an expert data engineer. Suggest useful transformations for the given feature.

Feature: {{.FeatureName}}
Type: {{.FeatureType}}
Description: {{.FeatureDescription}}

Current Value Examples:
{{range .Examples}}- {{.}}
{{end}}

Suggest transformations that would make this feature more useful for machine learning.
Consider: normalization, binning, encoding, time-based transformations, etc.

Respond in JSON format with an array of transformation suggestions.`

	g.templates["transformation"] = template.Must(template.New("transformation").Parse(transformPrompt))

	// Aggregation suggestion prompt
	aggPrompt := `You are an expert feature engineer. Suggest useful aggregations for the given entity.

Entity: {{.Entity}}
Available Fields:
{{range .Fields}}- {{.Name}} ({{.Type}})
{{end}}

{{if .TimeField}}Time Field: {{.TimeField}}{{end}}

Suggest aggregations that would be useful for:
- Understanding user/entity behavior over time
- Detecting patterns and anomalies
- Building predictive features

Consider windows: 1h, 24h, 7d, 30d, etc.
Consider functions: count, sum, avg, min, max, std, percentile, etc.

Respond in JSON format with an array of aggregation suggestions.`

	g.templates["aggregation"] = template.Must(template.New("aggregation").Parse(aggPrompt))
}

// GenerateFeatures generates feature suggestions from a schema.
func (g *Generator) GenerateFeatures(ctx context.Context, req *GenerationRequest) (*GenerationResult, error) {
	startTime := time.Now()

	result := &GenerationResult{
		ID:        fmt.Sprintf("gen-%d", time.Now().UnixNano()),
		Request:   req,
		CreatedAt: time.Now(),
	}

	if req.MaxSuggestions <= 0 {
		req.MaxSuggestions = 5
	}

	// Build prompt
	prompt, err := g.buildFeaturePrompt(req)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Call LLM
	response, tokensUsed, err := g.callLLM(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.TokensUsed = tokensUsed

	// Parse response
	suggestions, err := g.parseFeatureSuggestions(response)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Filter by constraints
	if req.Constraints != nil {
		suggestions = g.filterByConstraints(suggestions, req.Constraints)
	}

	result.Suggestions = suggestions
	result.Duration = time.Since(startTime)

	// Store in history
	g.mu.Lock()
	g.history = append(g.history, result)
	g.mu.Unlock()

	return result, nil
}

func (g *Generator) buildFeaturePrompt(req *GenerationRequest) (string, error) {
	data := map[string]interface{}{
		"SchemaName":        req.Schema.Name,
		"SchemaDescription": req.Schema.Description,
		"Fields":            req.Schema.Fields,
		"UseCase":           req.UseCase,
		"ExistingFeatures":  req.ExistingFeatures,
		"MaxSuggestions":    req.MaxSuggestions,
	}

	var buf bytes.Buffer
	if err := g.templates["feature_generation"].Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

func (g *Generator) callLLM(ctx context.Context, prompt string) (string, int, error) {
	switch g.config.Provider {
	case ProviderOpenAI:
		return g.callOpenAI(ctx, prompt)
	case ProviderAnthropic:
		return g.callAnthropic(ctx, prompt)
	case ProviderOllama:
		return g.callOllama(ctx, prompt)
	case ProviderLocal:
		return g.generateLocal(prompt)
	default:
		return "", 0, fmt.Errorf("unsupported provider: %s", g.config.Provider)
	}
}

func (g *Generator) callOpenAI(ctx context.Context, prompt string) (string, int, error) {
	baseURL := g.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	reqBody := map[string]interface{}{
		"model":       g.config.Model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens":  g.config.MaxTokens,
		"temperature": g.config.Temperature,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.config.APIKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("OpenAI API error: %s", string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (g *Generator) callAnthropic(ctx context.Context, prompt string) (string, int, error) {
	baseURL := g.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	reqBody := map[string]interface{}{
		"model":      g.config.Model,
		"max_tokens": g.config.MaxTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("Anthropic API error: %s", string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	if len(result.Content) == 0 {
		return "", 0, fmt.Errorf("no response from Anthropic")
	}

	totalTokens := result.Usage.InputTokens + result.Usage.OutputTokens
	return result.Content[0].Text, totalTokens, nil
}

func (g *Generator) callOllama(ctx context.Context, prompt string) (string, int, error) {
	baseURL := g.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	reqBody := map[string]interface{}{
		"model":  g.config.Model,
		"prompt": prompt,
		"stream": false,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("Ollama API error: %s", string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	return result.Response, 0, nil
}

// generateLocal generates features locally without calling an LLM.
// This is a rule-based fallback for when no LLM is available.
func (g *Generator) generateLocal(prompt string) (string, int, error) {
	// Extract schema info from prompt for local generation
	// This is a simplified version that generates common feature patterns

	suggestions := []FeatureSuggestion{}

	// Parse the schema from the prompt (simplified)
	lines := strings.Split(prompt, "\n")
	schemaName := ""
	var fields []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Data Schema:") {
			schemaName = strings.TrimPrefix(line, "Data Schema:")
			schemaName = strings.TrimSpace(schemaName)
		}
		if strings.HasPrefix(line, "- ") && strings.Contains(line, "(") {
			// Extract field name
			parts := strings.SplitN(line[2:], " (", 2)
			if len(parts) > 0 {
				fields = append(fields, parts[0])
			}
		}
	}

	// Generate common patterns based on field names
	for i, field := range fields {
		fieldLower := strings.ToLower(field)

		// Temporal features
		if strings.Contains(fieldLower, "timestamp") || strings.Contains(fieldLower, "date") || strings.Contains(fieldLower, "time") {
			suggestions = append(suggestions, FeatureSuggestion{
				ID:          fmt.Sprintf("%s_hour_of_day", field),
				Name:        fmt.Sprintf("Hour of Day from %s", field),
				Description: fmt.Sprintf("Extract hour of day (0-23) from %s for temporal analysis", field),
				Expression:  fmt.Sprintf("EXTRACT(HOUR FROM %s)", field),
				DataType:    "int",
				Category:    "temporal",
				Tags:        []string{"temporal", "derived"},
				Confidence:  0.9,
				Rationale:   "Hour of day captures daily patterns in behavior",
			})
			suggestions = append(suggestions, FeatureSuggestion{
				ID:          fmt.Sprintf("%s_day_of_week", field),
				Name:        fmt.Sprintf("Day of Week from %s", field),
				Description: fmt.Sprintf("Extract day of week (0-6) from %s", field),
				Expression:  fmt.Sprintf("EXTRACT(DOW FROM %s)", field),
				DataType:    "int",
				Category:    "temporal",
				Tags:        []string{"temporal", "derived"},
				Confidence:  0.9,
				Rationale:   "Day of week captures weekly patterns",
			})
		}

		// Numeric features
		if strings.Contains(fieldLower, "amount") || strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "value") || strings.Contains(fieldLower, "count") {
			suggestions = append(suggestions, FeatureSuggestion{
				ID:          fmt.Sprintf("%s_log", field),
				Name:        fmt.Sprintf("Log of %s", field),
				Description: fmt.Sprintf("Natural logarithm of %s for better distribution", field),
				Expression:  fmt.Sprintf("LOG(%s + 1)", field),
				DataType:    "float64",
				Category:    "transformation",
				Tags:        []string{"numeric", "normalized"},
				Confidence:  0.85,
				Rationale:   "Log transform handles skewed distributions",
			})
		}

		// ID-based features
		if strings.Contains(fieldLower, "user") || strings.Contains(fieldLower, "customer") {
			suggestions = append(suggestions, FeatureSuggestion{
				ID:          fmt.Sprintf("%s_activity_count_24h", field),
				Name:        fmt.Sprintf("%s Activity Count (24h)", field),
				Description: fmt.Sprintf("Count of activities for this %s in the last 24 hours", field),
				Expression:  fmt.Sprintf("COUNT(*) OVER (PARTITION BY %s ORDER BY timestamp RANGE INTERVAL '24 hours' PRECEDING)", field),
				DataType:    "int",
				Category:    "aggregation",
				Tags:        []string{"user_behavior", "windowed"},
				Confidence:  0.88,
				Rationale:   "Recent activity count indicates engagement level",
			})
		}

		// Limit suggestions
		if len(suggestions) >= 5 {
			break
		}

		// Add an interaction feature if we have multiple numeric fields
		if i > 0 && (strings.Contains(fieldLower, "amount") || strings.Contains(fieldLower, "price")) {
			prevField := fields[i-1]
			if strings.Contains(strings.ToLower(prevField), "count") || strings.Contains(strings.ToLower(prevField), "quantity") {
				suggestions = append(suggestions, FeatureSuggestion{
					ID:          fmt.Sprintf("%s_%s_ratio", prevField, field),
					Name:        fmt.Sprintf("%s to %s Ratio", prevField, field),
					Description: fmt.Sprintf("Ratio of %s to %s for normalization", prevField, field),
					Expression:  fmt.Sprintf("CASE WHEN %s > 0 THEN %s / %s ELSE 0 END", field, prevField, field),
					DataType:    "float64",
					Category:    "derived",
					Tags:        []string{"ratio", "interaction"},
					Confidence:  0.75,
					Rationale:   "Ratios normalize for entity size differences",
				})
			}
		}
	}

	// If no specific patterns found, generate generic suggestions
	if len(suggestions) == 0 && len(fields) > 0 {
		suggestions = append(suggestions, FeatureSuggestion{
			ID:          fmt.Sprintf("%s_is_null", fields[0]),
			Name:        fmt.Sprintf("%s Is Null", fields[0]),
			Description: fmt.Sprintf("Boolean flag indicating if %s is null", fields[0]),
			Expression:  fmt.Sprintf("%s IS NULL", fields[0]),
			DataType:    "bool",
			Category:    "data_quality",
			Tags:        []string{"null_indicator", "data_quality"},
			Confidence:  0.7,
			Rationale:   "Null patterns can indicate data quality issues or special cases",
		})
	}

	_ = schemaName // Used for context

	// Marshal to JSON
	result, err := json.Marshal(suggestions)
	if err != nil {
		return "", 0, err
	}

	return string(result), 0, nil
}

func (g *Generator) parseFeatureSuggestions(response string) ([]FeatureSuggestion, error) {
	// Clean up response - find JSON array
	response = strings.TrimSpace(response)

	// Find JSON array in response
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")

	if start == -1 || end == -1 || end <= start {
		// Try to parse as single object
		var single FeatureSuggestion
		if err := json.Unmarshal([]byte(response), &single); err == nil {
			return []FeatureSuggestion{single}, nil
		}
		return nil, fmt.Errorf("could not find valid JSON array in response")
	}

	jsonStr := response[start : end+1]

	var suggestions []FeatureSuggestion
	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to parse suggestions: %w", err)
	}

	return suggestions, nil
}

func (g *Generator) filterByConstraints(suggestions []FeatureSuggestion, constraints *GenerationConstraints) []FeatureSuggestion {
	filtered := make([]FeatureSuggestion, 0, len(suggestions))

	for _, s := range suggestions {
		// Check minimum confidence
		if constraints.MinConfidence > 0 && s.Confidence < constraints.MinConfidence {
			continue
		}

		// Check allowed types
		if len(constraints.AllowedTypes) > 0 {
			allowed := false
			for _, t := range constraints.AllowedTypes {
				if t == s.DataType {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		// Check required tags
		if len(constraints.RequireTags) > 0 {
			hasRequired := false
			for _, required := range constraints.RequireTags {
				for _, tag := range s.Tags {
					if tag == required {
						hasRequired = true
						break
					}
				}
				if hasRequired {
					break
				}
			}
			if !hasRequired {
				continue
			}
		}

		filtered = append(filtered, s)
	}

	return filtered
}

// SuggestTransformations suggests transformations for a feature.
func (g *Generator) SuggestTransformations(ctx context.Context, featureName, featureType, description string, examples []interface{}) ([]TransformationSuggestion, error) {
	// Build transformation-specific suggestions based on type
	suggestions := []TransformationSuggestion{}

	switch featureType {
	case "float64", "float32", "int64", "int32":
		suggestions = append(suggestions,
			TransformationSuggestion{
				Name:        "normalize_minmax",
				Description: "Min-max normalization to [0, 1] range",
				InputType:   featureType,
				OutputType:  "float64",
				Expression:  "(value - min) / (max - min)",
			},
			TransformationSuggestion{
				Name:        "normalize_zscore",
				Description: "Z-score normalization (subtract mean, divide by std)",
				InputType:   featureType,
				OutputType:  "float64",
				Expression:  "(value - mean) / std",
			},
			TransformationSuggestion{
				Name:        "log_transform",
				Description: "Natural logarithm for skewed distributions",
				InputType:   featureType,
				OutputType:  "float64",
				Expression:  "log(value + 1)",
			},
			TransformationSuggestion{
				Name:        "binning_quantile",
				Description: "Quantile-based binning into categories",
				InputType:   featureType,
				OutputType:  "int",
				Expression:  "quantile_bin(value, [0.25, 0.5, 0.75])",
			},
		)

	case "string":
		suggestions = append(suggestions,
			TransformationSuggestion{
				Name:        "length",
				Description: "String length",
				InputType:   "string",
				OutputType:  "int",
				Expression:  "len(value)",
			},
			TransformationSuggestion{
				Name:        "lowercase",
				Description: "Convert to lowercase",
				InputType:   "string",
				OutputType:  "string",
				Expression:  "lower(value)",
			},
			TransformationSuggestion{
				Name:        "hash_bucket",
				Description: "Hash into N buckets for high-cardinality strings",
				InputType:   "string",
				OutputType:  "int",
				Expression:  "hash(value) % n_buckets",
			},
		)

	case "bool":
		suggestions = append(suggestions,
			TransformationSuggestion{
				Name:        "to_int",
				Description: "Convert boolean to integer (0/1)",
				InputType:   "bool",
				OutputType:  "int",
				Expression:  "int(value)",
			},
		)
	}

	return suggestions, nil
}

// SuggestAggregations suggests aggregations for an entity.
func (g *Generator) SuggestAggregations(ctx context.Context, entity string, fields []SchemaField, timeField string) ([]AggregationSuggestion, error) {
	suggestions := []AggregationSuggestion{}

	windows := []string{"1h", "24h", "7d", "30d"}

	for _, field := range fields {
		fieldLower := strings.ToLower(field.Name)

		switch field.Type {
		case "float64", "float32", "int64", "int32":
			// Numeric aggregations
			for _, window := range windows[:2] { // Only first 2 windows for brevity
				suggestions = append(suggestions,
					AggregationSuggestion{
						Name:        fmt.Sprintf("%s_sum_%s", field.Name, window),
						Description: fmt.Sprintf("Sum of %s over %s window", field.Name, window),
						Function:    "sum",
						Window:      window,
						GroupBy:     []string{entity},
						Expression:  fmt.Sprintf("SUM(%s) OVER (PARTITION BY %s ORDER BY %s RANGE '%s' PRECEDING)", field.Name, entity, timeField, window),
					},
					AggregationSuggestion{
						Name:        fmt.Sprintf("%s_avg_%s", field.Name, window),
						Description: fmt.Sprintf("Average of %s over %s window", field.Name, window),
						Function:    "avg",
						Window:      window,
						GroupBy:     []string{entity},
						Expression:  fmt.Sprintf("AVG(%s) OVER (PARTITION BY %s ORDER BY %s RANGE '%s' PRECEDING)", field.Name, entity, timeField, window),
					},
				)
			}

		case "bool":
			// Count of true values
			suggestions = append(suggestions,
				AggregationSuggestion{
					Name:        fmt.Sprintf("%s_count_true_24h", field.Name),
					Description: fmt.Sprintf("Count of true values for %s in 24h", field.Name),
					Function:    "sum",
					Window:      "24h",
					GroupBy:     []string{entity},
					Expression:  fmt.Sprintf("SUM(CASE WHEN %s THEN 1 ELSE 0 END) OVER (PARTITION BY %s ORDER BY %s RANGE '24 hours' PRECEDING)", field.Name, entity, timeField),
				},
			)
		}

		// Special cases based on field name
		if strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "amount") {
			suggestions = append(suggestions,
				AggregationSuggestion{
					Name:        fmt.Sprintf("%s_max_7d", field.Name),
					Description: fmt.Sprintf("Maximum %s in 7 days", field.Name),
					Function:    "max",
					Window:      "7d",
					GroupBy:     []string{entity},
					Expression:  fmt.Sprintf("MAX(%s) OVER (PARTITION BY %s ORDER BY %s RANGE '7 days' PRECEDING)", field.Name, entity, timeField),
				},
			)
		}

		// Limit suggestions per field
		if len(suggestions) > 10 {
			break
		}
	}

	return suggestions, nil
}

// GetHistory returns generation history.
func (g *Generator) GetHistory() []*GenerationResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	history := make([]*GenerationResult, len(g.history))
	copy(history, g.history)
	return history
}

// GetStats returns generator statistics.
func (g *Generator) GetStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	totalSuggestions := 0
	totalTokens := 0
	for _, r := range g.history {
		totalSuggestions += len(r.Suggestions)
		totalTokens += r.TokensUsed
	}

	return map[string]interface{}{
		"provider":          string(g.config.Provider),
		"model":             g.config.Model,
		"total_generations": len(g.history),
		"total_suggestions": totalSuggestions,
		"total_tokens":      totalTokens,
	}
}
