package pipelinebuilder

import (
	"fmt"
	"strings"
	"sync"
)

// ParameterDef describes a parameter accepted by a transform.
type ParameterDef struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description"`
}

// TransformDef defines a reusable transformation.
type TransformDef struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Description  string            `json:"description"`
	InputSchema  map[string]string `json:"input_schema"`
	OutputSchema map[string]string `json:"output_schema"`
	Parameters   []ParameterDef   `json:"parameters,omitempty"`
}

// TransformRegistry is a thread-safe registry of transform definitions.
type TransformRegistry struct {
	mu         sync.RWMutex
	transforms map[string]*TransformDef
}

// NewTransformRegistry creates a registry pre-loaded with built-in transforms.
func NewTransformRegistry() *TransformRegistry {
	r := &TransformRegistry{transforms: make(map[string]*TransformDef)}
	r.registerBuiltins()
	return r
}

// Register adds a transform to the registry.
func (r *TransformRegistry) Register(def *TransformDef) error {
	if def.ID == "" {
		return fmt.Errorf("transform ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.transforms[def.ID]; exists {
		return fmt.Errorf("transform %q already registered", def.ID)
	}
	r.transforms[def.ID] = def
	return nil
}

// Get returns a transform by ID.
func (r *TransformRegistry) Get(id string) (*TransformDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.transforms[id]
	if !ok {
		return nil, fmt.Errorf("transform %q not found", id)
	}
	return t, nil
}

// List returns all registered transforms.
func (r *TransformRegistry) List() []*TransformDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*TransformDef, 0, len(r.transforms))
	for _, t := range r.transforms {
		out = append(out, t)
	}
	return out
}

// ListByCategory returns transforms matching the given category.
func (r *TransformRegistry) ListByCategory(category string) []*TransformDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*TransformDef
	for _, t := range r.transforms {
		if t.Category == category {
			out = append(out, t)
		}
	}
	return out
}

// Search returns transforms whose name or description contains the query.
func (r *TransformRegistry) Search(query string) []*TransformDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := strings.ToLower(query)
	var out []*TransformDef
	for _, t := range r.transforms {
		if strings.Contains(strings.ToLower(t.Name), q) || strings.Contains(strings.ToLower(t.Description), q) {
			out = append(out, t)
		}
	}
	return out
}

func (r *TransformRegistry) registerBuiltins() {
	numIn := map[string]string{"value": "float64"}
	numOut := map[string]string{"result": "float64"}
	strIn := map[string]string{"value": "string"}
	strOut := map[string]string{"result": "string"}
	tsIn := map[string]string{"timestamp": "time.Time"}
	tsOut := map[string]string{"result": "float64"}
	aggIn := map[string]string{"values": "[]float64"}
	aggOut := map[string]string{"result": "float64"}
	encIn := map[string]string{"value": "string"}
	encOut := map[string]string{"result": "[]float64"}

	builtins := []*TransformDef{
		// Math
		{ID: "math_log", Name: "Log", Category: "math", Description: "Natural logarithm", InputSchema: numIn, OutputSchema: numOut},
		{ID: "math_sqrt", Name: "Sqrt", Category: "math", Description: "Square root", InputSchema: numIn, OutputSchema: numOut},
		{ID: "math_square", Name: "Square", Category: "math", Description: "Square (x²)", InputSchema: numIn, OutputSchema: numOut},
		{ID: "math_abs", Name: "Abs", Category: "math", Description: "Absolute value", InputSchema: numIn, OutputSchema: numOut},
		{ID: "math_round", Name: "Round", Category: "math", Description: "Round to nearest integer", InputSchema: numIn, OutputSchema: numOut, Parameters: []ParameterDef{{Name: "decimals", Type: "int", Required: false, Default: 0, Description: "Decimal places"}}},
		{ID: "math_ceil", Name: "Ceil", Category: "math", Description: "Ceiling", InputSchema: numIn, OutputSchema: numOut},
		{ID: "math_floor", Name: "Floor", Category: "math", Description: "Floor", InputSchema: numIn, OutputSchema: numOut},
		// String
		{ID: "string_lower", Name: "Lower", Category: "string", Description: "Convert to lowercase", InputSchema: strIn, OutputSchema: strOut},
		{ID: "string_upper", Name: "Upper", Category: "string", Description: "Convert to uppercase", InputSchema: strIn, OutputSchema: strOut},
		{ID: "string_trim", Name: "Trim", Category: "string", Description: "Trim whitespace", InputSchema: strIn, OutputSchema: strOut},
		{ID: "string_concat", Name: "Concat", Category: "string", Description: "Concatenate strings", InputSchema: map[string]string{"a": "string", "b": "string"}, OutputSchema: strOut, Parameters: []ParameterDef{{Name: "separator", Type: "string", Required: false, Default: "", Description: "Separator between values"}}},
		{ID: "string_split", Name: "Split", Category: "string", Description: "Split string into parts", InputSchema: strIn, OutputSchema: map[string]string{"result": "[]string"}, Parameters: []ParameterDef{{Name: "delimiter", Type: "string", Required: true, Description: "Split delimiter"}}},
		{ID: "string_replace", Name: "Replace", Category: "string", Description: "Replace substring", InputSchema: strIn, OutputSchema: strOut, Parameters: []ParameterDef{{Name: "old", Type: "string", Required: true, Description: "Substring to replace"}, {Name: "new", Type: "string", Required: true, Description: "Replacement"}}},
		// Temporal
		{ID: "temporal_extract_hour", Name: "ExtractHour", Category: "temporal", Description: "Extract hour from timestamp", InputSchema: tsIn, OutputSchema: tsOut},
		{ID: "temporal_extract_day", Name: "ExtractDay", Category: "temporal", Description: "Extract day of month from timestamp", InputSchema: tsIn, OutputSchema: tsOut},
		{ID: "temporal_time_since", Name: "TimeSince", Category: "temporal", Description: "Duration since timestamp in seconds", InputSchema: tsIn, OutputSchema: tsOut},
		{ID: "temporal_date_diff", Name: "DateDiff", Category: "temporal", Description: "Difference between two timestamps in seconds", InputSchema: map[string]string{"start": "time.Time", "end": "time.Time"}, OutputSchema: tsOut},
		// Aggregation
		{ID: "agg_sum", Name: "Sum", Category: "aggregation", Description: "Sum of values", InputSchema: aggIn, OutputSchema: aggOut},
		{ID: "agg_avg", Name: "Avg", Category: "aggregation", Description: "Average of values", InputSchema: aggIn, OutputSchema: aggOut},
		{ID: "agg_count", Name: "Count", Category: "aggregation", Description: "Count of values", InputSchema: aggIn, OutputSchema: aggOut},
		{ID: "agg_min", Name: "Min", Category: "aggregation", Description: "Minimum value", InputSchema: aggIn, OutputSchema: aggOut},
		{ID: "agg_max", Name: "Max", Category: "aggregation", Description: "Maximum value", InputSchema: aggIn, OutputSchema: aggOut},
		{ID: "agg_stddev", Name: "StdDev", Category: "aggregation", Description: "Standard deviation", InputSchema: aggIn, OutputSchema: aggOut},
		// Encoding
		{ID: "enc_one_hot", Name: "OneHot", Category: "encoding", Description: "One-hot encoding", InputSchema: encIn, OutputSchema: encOut, Parameters: []ParameterDef{{Name: "categories", Type: "[]string", Required: true, Description: "Category list"}}},
		{ID: "enc_label", Name: "LabelEncode", Category: "encoding", Description: "Label encoding", InputSchema: encIn, OutputSchema: map[string]string{"result": "int"}, Parameters: []ParameterDef{{Name: "categories", Type: "[]string", Required: true, Description: "Category list"}}},
		{ID: "enc_hash", Name: "HashEncode", Category: "encoding", Description: "Feature hashing", InputSchema: encIn, OutputSchema: map[string]string{"result": "int"}, Parameters: []ParameterDef{{Name: "num_buckets", Type: "int", Required: false, Default: 1024, Description: "Number of hash buckets"}}},
		{ID: "enc_binary", Name: "BinaryEncode", Category: "encoding", Description: "Binary encoding", InputSchema: encIn, OutputSchema: encOut},
	}

	for _, b := range builtins {
		r.transforms[b.ID] = b
	}
}
