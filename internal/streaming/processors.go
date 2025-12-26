package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// FilterProcessor filters events based on conditions.
type FilterProcessor struct {
	name       string
	conditions []FilterCondition
}

// FilterCondition defines a filter condition.
type FilterCondition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// NewFilterProcessor creates a new filter processor.
func NewFilterProcessor(name string, conditions []FilterCondition) *FilterProcessor {
	return &FilterProcessor{
		name:       name,
		conditions: conditions,
	}
}

func (p *FilterProcessor) Name() string {
	return p.name
}

func (p *FilterProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	for _, cond := range p.conditions {
		if !p.evaluateCondition(event, cond) {
			return nil, nil // Filter out
		}
	}
	return event, nil
}

func (p *FilterProcessor) evaluateCondition(event *Event, cond FilterCondition) bool {
	value, ok := event.Data[cond.Field]
	if !ok {
		return false
	}

	switch cond.Operator {
	case OpEquals:
		return compareEqual(value, cond.Value)
	case OpNotEquals:
		return !compareEqual(value, cond.Value)
	case OpGreaterThan:
		return compareNumeric(value, cond.Value) > 0
	case OpLessThan:
		return compareNumeric(value, cond.Value) < 0
	case OpGreaterOrEqual:
		return compareNumeric(value, cond.Value) >= 0
	case OpLessOrEqual:
		return compareNumeric(value, cond.Value) <= 0
	default:
		return false
	}
}

// TransformProcessor transforms event data.
type TransformProcessor struct {
	name           string
	transformations []Transformation
}

// Transformation defines a transformation operation.
type Transformation struct {
	Type        TransformationType `json:"type"`
	SourceField string             `json:"source_field"`
	TargetField string             `json:"target_field"`
	Expression  string             `json:"expression,omitempty"`
	Config      json.RawMessage    `json:"config,omitempty"`
}

// TransformationType indicates the type of transformation.
type TransformationType string

const (
	TransformRename    TransformationType = "rename"
	TransformCopy      TransformationType = "copy"
	TransformDelete    TransformationType = "delete"
	TransformCast      TransformationType = "cast"
	TransformExtract   TransformationType = "extract"
	TransformConcat    TransformationType = "concat"
	TransformSplit     TransformationType = "split"
	TransformDefault   TransformationType = "default"
	TransformCompute   TransformationType = "compute"
)

// NewTransformProcessor creates a new transform processor.
func NewTransformProcessor(name string, transformations []Transformation) *TransformProcessor {
	return &TransformProcessor{
		name:           name,
		transformations: transformations,
	}
}

func (p *TransformProcessor) Name() string {
	return p.name
}

func (p *TransformProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	// Clone event data
	newData := make(map[string]interface{})
	for k, v := range event.Data {
		newData[k] = v
	}

	for _, t := range p.transformations {
		switch t.Type {
		case TransformRename:
			if val, ok := newData[t.SourceField]; ok {
				newData[t.TargetField] = val
				delete(newData, t.SourceField)
			}

		case TransformCopy:
			if val, ok := newData[t.SourceField]; ok {
				newData[t.TargetField] = val
			}

		case TransformDelete:
			delete(newData, t.SourceField)

		case TransformCast:
			if val, ok := newData[t.SourceField]; ok {
				newData[t.TargetField] = p.castValue(val, t.Expression)
			}

		case TransformExtract:
			if val, ok := newData[t.SourceField].(string); ok {
				extracted := p.extractValue(val, t.Expression)
				if extracted != "" {
					newData[t.TargetField] = extracted
				}
			}

		case TransformDefault:
			if _, ok := newData[t.TargetField]; !ok {
				var defaultVal interface{}
				json.Unmarshal(t.Config, &defaultVal)
				newData[t.TargetField] = defaultVal
			}
		}
	}

	return &Event{
		ID:        event.ID,
		Type:      event.Type,
		EntityID:  event.EntityID,
		Timestamp: event.Timestamp,
		Data:      newData,
	}, nil
}

func (p *TransformProcessor) castValue(val interface{}, targetType string) interface{} {
	switch targetType {
	case "string":
		return fmt.Sprintf("%v", val)
	case "int":
		if f, ok := toFloat64(val); ok {
			return int64(f)
		}
	case "float":
		if f, ok := toFloat64(val); ok {
			return f
		}
	case "bool":
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1" || v == "yes"
		case int, int64, int32:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return val
}

func (p *TransformProcessor) extractValue(val, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}

	matches := re.FindStringSubmatch(val)
	if len(matches) > 1 {
		return matches[1]
	} else if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// EnrichProcessor enriches events with additional data.
type EnrichProcessor struct {
	name      string
	enricher  Enricher
}

// Enricher provides data enrichment.
type Enricher interface {
	Enrich(ctx context.Context, event *Event) (map[string]interface{}, error)
}

// NewEnrichProcessor creates a new enrich processor.
func NewEnrichProcessor(name string, enricher Enricher) *EnrichProcessor {
	return &EnrichProcessor{
		name:     name,
		enricher: enricher,
	}
}

func (p *EnrichProcessor) Name() string {
	return p.name
}

func (p *EnrichProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	if p.enricher == nil {
		return event, nil
	}

	enrichment, err := p.enricher.Enrich(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("enrichment failed: %w", err)
	}

	// Merge enrichment into event data
	newData := make(map[string]interface{})
	for k, v := range event.Data {
		newData[k] = v
	}
	for k, v := range enrichment {
		newData[k] = v
	}

	return &Event{
		ID:        event.ID,
		Type:      event.Type,
		EntityID:  event.EntityID,
		Timestamp: event.Timestamp,
		Data:      newData,
	}, nil
}

// MapEnricher is a simple map-based enricher for testing.
type MapEnricher struct {
	data map[string]map[string]interface{}
}

// NewMapEnricher creates a map enricher.
func NewMapEnricher(data map[string]map[string]interface{}) *MapEnricher {
	return &MapEnricher{data: data}
}

func (e *MapEnricher) Enrich(ctx context.Context, event *Event) (map[string]interface{}, error) {
	if enrichment, ok := e.data[event.EntityID]; ok {
		return enrichment, nil
	}
	return nil, nil
}

// DeduplicateProcessor removes duplicate events.
type DeduplicateProcessor struct {
	name     string
	seen     map[string]bool
	capacity int
}

// NewDeduplicateProcessor creates a deduplication processor.
func NewDeduplicateProcessor(name string, capacity int) *DeduplicateProcessor {
	return &DeduplicateProcessor{
		name:     name,
		seen:     make(map[string]bool),
		capacity: capacity,
	}
}

func (p *DeduplicateProcessor) Name() string {
	return p.name
}

func (p *DeduplicateProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	if p.seen[event.ID] {
		return nil, nil // Duplicate, filter out
	}

	// Maintain capacity
	if len(p.seen) >= p.capacity {
		// Simple eviction: clear half
		count := 0
		for k := range p.seen {
			delete(p.seen, k)
			count++
			if count >= p.capacity/2 {
				break
			}
		}
	}

	p.seen[event.ID] = true
	return event, nil
}

// RouterProcessor routes events to different processors based on conditions.
type RouterProcessor struct {
	name   string
	routes []Route
}

// Route defines a routing rule.
type Route struct {
	Condition FilterCondition
	Processor Processor
}

// NewRouterProcessor creates a router processor.
func NewRouterProcessor(name string, routes []Route) *RouterProcessor {
	return &RouterProcessor{
		name:   name,
		routes: routes,
	}
}

func (p *RouterProcessor) Name() string {
	return p.name
}

func (p *RouterProcessor) Process(ctx context.Context, event *Event) (*Event, error) {
	for _, route := range p.routes {
		value, ok := event.Data[route.Condition.Field]
		if !ok {
			continue
		}

		if compareEqual(value, route.Condition.Value) {
			return route.Processor.Process(ctx, event)
		}
	}

	return event, nil // No route matched, pass through
}
