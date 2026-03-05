package graphqlfederation

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceConfig defines a federated GraphQL service.
type ServiceConfig struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Schema   string `json:"schema,omitempty"`
	Priority int    `json:"priority"`
}

// FederatedField maps a field to its owning service.
type FederatedField struct {
	FieldName   string `json:"field_name"`
	TypeName    string `json:"type_name"`
	ServiceName string `json:"service_name"`
	External    bool   `json:"external"`
	Requires    string `json:"requires,omitempty"`
	Provides    string `json:"provides,omitempty"`
}

// QueryPlan describes how a federated query will be resolved.
type QueryPlan struct {
	ID         string      `json:"id"`
	Query      string      `json:"query"`
	Steps      []QueryStep `json:"steps"`
	CreatedAt  time.Time   `json:"created_at"`
	EstimatedMs int        `json:"estimated_ms"`
}

// QueryStep is a single step in the query execution plan.
type QueryStep struct {
	ServiceName string   `json:"service_name"`
	Fields      []string `json:"fields"`
	DependsOn   []int    `json:"depends_on,omitempty"`
	Parallel    bool     `json:"parallel"`
}

// BatchRequest groups multiple entity lookups for efficient resolution.
type BatchRequest struct {
	Entities  []EntityRef `json:"entities"`
	Fields    []string    `json:"fields"`
}

// EntityRef identifies an entity for batch resolution.
type EntityRef struct {
	TypeName string                 `json:"__typename"`
	KeyFields map[string]interface{} `json:"key_fields"`
}

// GatewayConfig configures the federation gateway.
type GatewayConfig struct {
	MaxBatchSize     int           `json:"max_batch_size"`
	QueryTimeout     time.Duration `json:"query_timeout_ns"`
	CacheEnabled     bool          `json:"cache_enabled"`
	CacheTTL         time.Duration `json:"cache_ttl_ns"`
	IntrospectionEnabled bool      `json:"introspection_enabled"`
}

// DefaultGatewayConfig returns sensible defaults.
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		MaxBatchSize:     100,
		QueryTimeout:     10 * time.Second,
		CacheEnabled:     true,
		CacheTTL:         5 * time.Minute,
		IntrospectionEnabled: true,
	}
}

// GatewayStats tracks federation gateway statistics.
type GatewayStats struct {
	TotalQueries     int64              `json:"total_queries"`
	TotalBatches     int64              `json:"total_batches"`
	CacheHits        int64              `json:"cache_hits"`
	CacheMisses      int64              `json:"cache_misses"`
	AvgLatencyMs     float64            `json:"avg_latency_ms"`
	ServiceCalls     map[string]int64   `json:"service_calls"`
	ErrorsByService  map[string]int64   `json:"errors_by_service"`
}

// Gateway manages the GraphQL federation gateway.
type Gateway struct {
	mu       sync.RWMutex
	config   GatewayConfig
	services map[string]*ServiceConfig
	fields   map[string]*FederatedField // typeName.fieldName -> field
	stats    GatewayStats
	plans    []QueryPlan
}

// NewGateway creates a new federation gateway.
func NewGateway(config GatewayConfig) *Gateway {
	return &Gateway{
		config:   config,
		services: make(map[string]*ServiceConfig),
		fields:   make(map[string]*FederatedField),
		stats: GatewayStats{
			ServiceCalls:    make(map[string]int64),
			ErrorsByService: make(map[string]int64),
		},
	}
}

// RegisterService adds a federated service.
func (g *Gateway) RegisterService(svc ServiceConfig) error {
	if svc.Name == "" || svc.URL == "" {
		return fmt.Errorf("service name and url are required")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.services[svc.Name] = &svc
	return nil
}

// RemoveService removes a federated service.
func (g *Gateway) RemoveService(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.services[name]; !exists {
		return fmt.Errorf("service %s not found", name)
	}
	delete(g.services, name)

	// Remove associated fields
	for key, f := range g.fields {
		if f.ServiceName == name {
			delete(g.fields, key)
		}
	}
	return nil
}

// ListServices returns all registered services.
func (g *Gateway) ListServices() []ServiceConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]ServiceConfig, 0, len(g.services))
	for _, s := range g.services {
		result = append(result, *s)
	}
	return result
}

// RegisterField registers a federated field mapping.
func (g *Gateway) RegisterField(field FederatedField) error {
	if field.FieldName == "" || field.TypeName == "" || field.ServiceName == "" {
		return fmt.Errorf("field_name, type_name, and service_name are required")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.services[field.ServiceName]; !exists {
		return fmt.Errorf("service %s not registered", field.ServiceName)
	}

	key := field.TypeName + "." + field.FieldName
	g.fields[key] = &field
	return nil
}

// ListFields returns all registered federated fields.
func (g *Gateway) ListFields() []FederatedField {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]FederatedField, 0, len(g.fields))
	for _, f := range g.fields {
		result = append(result, *f)
	}
	return result
}

// PlanQuery creates a query execution plan for a federated query.
func (g *Gateway) PlanQuery(query string) (*QueryPlan, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Group fields by service
	serviceFields := make(map[string][]string)
	for _, f := range g.fields {
		serviceFields[f.ServiceName] = append(serviceFields[f.ServiceName], f.FieldName)
	}

	var steps []QueryStep
	for svc, fields := range serviceFields {
		steps = append(steps, QueryStep{
			ServiceName: svc,
			Fields:      fields,
			Parallel:    true,
		})
	}

	plan := QueryPlan{
		ID:          fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		Query:       query,
		Steps:       steps,
		CreatedAt:   time.Now(),
		EstimatedMs: len(steps) * 10,
	}

	g.plans = append(g.plans, plan)
	if len(g.plans) > 100 {
		g.plans = g.plans[1:]
	}

	return &plan, nil
}

// ExecuteQuery executes a federated query.
func (g *Gateway) ExecuteQuery(_ context.Context, query string) (map[string]interface{}, error) {
	g.mu.Lock()
	g.stats.TotalQueries++
	g.mu.Unlock()

	plan, err := g.PlanQuery(query)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"data": map[string]interface{}{
			"_plan":    plan,
			"_query":   query,
			"_message": "query planned across federated services",
		},
	}
	return result, nil
}

// BatchResolve resolves entities in batch across services.
func (g *Gateway) BatchResolve(_ context.Context, batch BatchRequest) ([]map[string]interface{}, error) {
	g.mu.Lock()
	g.stats.TotalBatches++
	g.mu.Unlock()

	if len(batch.Entities) > g.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds max %d", len(batch.Entities), g.config.MaxBatchSize)
	}

	results := make([]map[string]interface{}, len(batch.Entities))
	for i, entity := range batch.Entities {
		results[i] = map[string]interface{}{
			"__typename": entity.TypeName,
			"keys":       entity.KeyFields,
			"resolved":   true,
		}
	}
	return results, nil
}

// GetSchema returns the composed federated schema.
func (g *Gateway) GetSchema() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	types := make(map[string][]map[string]interface{})
	for _, f := range g.fields {
		types[f.TypeName] = append(types[f.TypeName], map[string]interface{}{
			"name":     f.FieldName,
			"service":  f.ServiceName,
			"external": f.External,
		})
	}

	return map[string]interface{}{
		"services":    len(g.services),
		"types":       types,
		"total_fields": len(g.fields),
	}
}

// Stats returns gateway statistics.
func (g *Gateway) Stats() GatewayStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.stats
}
