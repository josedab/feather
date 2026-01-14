package transform

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// ErrTemplateNotFound indicates the requested template does not exist.
var ErrTemplateNotFound = errors.New("transform template not found")

// TemplateCategory groups related transforms.
type TemplateCategory string

const (
	CategoryEncoding      TemplateCategory = "encoding"
	CategoryNormalization TemplateCategory = "normalization"
	CategoryAggregation   TemplateCategory = "aggregation"
	CategoryString        TemplateCategory = "string"
	CategoryMath          TemplateCategory = "math"
	CategoryTemporal      TemplateCategory = "temporal"
)

// Template is a reusable transform definition.
type Template struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    TemplateCategory       `json:"category"`
	Type        Type                   `json:"type"`
	Expression  string                 `json:"expression"`
	InputTypes  []domain.DataType      `json:"input_types"`
	OutputType  domain.DataType        `json:"output_type"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Version     int                    `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	BuiltIn     bool                   `json:"built_in"`
}

// Catalog manages transform templates.
type Catalog struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewCatalog creates a new catalog initialized with built-in templates.
func NewCatalog() *Catalog {
	c := &Catalog{
		templates: make(map[string]*Template),
	}

	now := time.Now()
	builtIns := []*Template{
		{ID: "one_hot", Name: "One-Hot Encoding", Description: "One-hot encode categorical features into vector representation", Category: CategoryEncoding, Type: TypeCustom, Expression: "one_hot(input)", InputTypes: []domain.DataType{domain.DataTypeString}, OutputType: domain.DataTypeVector, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "bucket", Name: "Numeric Bucketing", Description: "Bucket numeric values into discrete categories", Category: CategoryEncoding, Type: TypeCustom, Expression: "bucket(input, boundaries)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeString, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "min_max_scale", Name: "Min-Max Normalization", Description: "Scale values to [0,1] range using min-max normalization", Category: CategoryNormalization, Type: TypeArithmetic, Expression: "(input - min) / (max - min)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "z_score", Name: "Z-Score Normalization", Description: "Normalize values using z-score (mean and standard deviation)", Category: CategoryNormalization, Type: TypeArithmetic, Expression: "(input - mean) / stddev", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "log_transform", Name: "Log Transform", Description: "Apply natural logarithm transformation", Category: CategoryMath, Type: TypeArithmetic, Expression: "log(input)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "abs", Name: "Absolute Value", Description: "Compute absolute value of numeric input", Category: CategoryMath, Type: TypeArithmetic, Expression: "abs(input)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "power", Name: "Power Transform", Description: "Raise input to a specified power", Category: CategoryMath, Type: TypeArithmetic, Expression: "pow(input, exponent)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "lower", Name: "Lowercase String", Description: "Convert string to lowercase", Category: CategoryString, Type: TypeString, Expression: "lower(input)", InputTypes: []domain.DataType{domain.DataTypeString}, OutputType: domain.DataTypeString, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "upper", Name: "Uppercase String", Description: "Convert string to uppercase", Category: CategoryString, Type: TypeString, Expression: "upper(input)", InputTypes: []domain.DataType{domain.DataTypeString}, OutputType: domain.DataTypeString, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "trim", Name: "Trim Whitespace", Description: "Remove leading and trailing whitespace from string", Category: CategoryString, Type: TypeString, Expression: "trim(input)", InputTypes: []domain.DataType{domain.DataTypeString}, OutputType: domain.DataTypeString, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "day_of_week", Name: "Day of Week", Description: "Extract day of week from timestamp (0=Sunday)", Category: CategoryTemporal, Type: TypeTimestamp, Expression: "day_of_week(input)", InputTypes: []domain.DataType{domain.DataTypeTimestamp}, OutputType: domain.DataTypeInt64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "hour_of_day", Name: "Hour of Day", Description: "Extract hour from timestamp (0-23)", Category: CategoryTemporal, Type: TypeTimestamp, Expression: "hour_of_day(input)", InputTypes: []domain.DataType{domain.DataTypeTimestamp}, OutputType: domain.DataTypeInt64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "is_weekend", Name: "Is Weekend", Description: "Check if timestamp falls on a weekend", Category: CategoryTemporal, Type: TypeTimestamp, Expression: "is_weekend(input)", InputTypes: []domain.DataType{domain.DataTypeTimestamp}, OutputType: domain.DataTypeBool, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "count", Name: "Count Aggregation", Description: "Count the number of values in a group", Category: CategoryAggregation, Type: TypeAggregation, Expression: "count(input)", InputTypes: []domain.DataType{domain.DataTypeInt64}, OutputType: domain.DataTypeInt64, Version: 1, CreatedAt: now, BuiltIn: true},
		{ID: "mean", Name: "Mean Aggregation", Description: "Compute arithmetic mean of values in a group", Category: CategoryAggregation, Type: TypeAggregation, Expression: "mean(input)", InputTypes: []domain.DataType{domain.DataTypeFloat64}, OutputType: domain.DataTypeFloat64, Version: 1, CreatedAt: now, BuiltIn: true},
	}

	for _, t := range builtIns {
		c.templates[t.ID] = t
	}

	return c
}

// Register adds a custom template to the catalog.
func (c *Catalog) Register(template *Template) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.templates[template.ID] = template
	return nil
}

// Get retrieves a template by ID.
func (c *Catalog) Get(id string) (*Template, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	t, ok := c.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

// List returns all templates in the catalog.
func (c *Catalog) List() []*Template {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Template, 0, len(c.templates))
	for _, t := range c.templates {
		result = append(result, t)
	}
	return result
}

// ListByCategory returns templates matching the given category.
func (c *Catalog) ListByCategory(category TemplateCategory) []*Template {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Template
	for _, t := range c.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// Remove deletes a template from the catalog.
func (c *Catalog) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	delete(c.templates, id)
	return nil
}

// Search finds templates whose name or description contains the query (case-insensitive).
func (c *Catalog) Search(query string) []*Template {
	c.mu.RLock()
	defer c.mu.RUnlock()

	q := strings.ToLower(query)
	var result []*Template
	for _, t := range c.templates {
		if strings.Contains(strings.ToLower(t.Name), q) || strings.Contains(strings.ToLower(t.Description), q) {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the number of templates in the catalog.
func (c *Catalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.templates)
}
