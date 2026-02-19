package openapisync

import (
	"encoding/json"
	"strings"
	"sync"
)

// RouteInfo describes a single HTTP route for OpenAPI spec generation.
type RouteInfo struct {
	Method      string           `json:"method"`
	Path        string           `json:"path"`
	Summary     string           `json:"summary"`
	Description string           `json:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Parameters  []Parameter      `json:"parameters,omitempty"`
	RequestBody *SchemaRef       `json:"request_body,omitempty"`
	Responses   map[int]SchemaRef `json:"responses,omitempty"`
}

// Parameter describes an API parameter.
type Parameter struct {
	Name        string    `json:"name"`
	In          string    `json:"in"`
	Description string    `json:"description,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Schema      SchemaRef `json:"schema"`
}

// SchemaRef describes a JSON schema.
type SchemaRef struct {
	Type        string               `json:"type"`
	Properties  map[string]SchemaRef `json:"properties,omitempty"`
	Items       *SchemaRef           `json:"items,omitempty"`
	Description string               `json:"description,omitempty"`
}

// OpenAPISpec represents a complete OpenAPI 3.1 specification.
type OpenAPISpec struct {
	OpenAPI string                            `json:"openapi"`
	Info    SpecInfo                          `json:"info"`
	Paths   map[string]map[string]OperationSpec `json:"paths"`
	Tags    []TagSpec                         `json:"tags,omitempty"`
}

// SpecInfo holds API metadata.
type SpecInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// OperationSpec describes a single API operation.
type OperationSpec struct {
	Summary     string                    `json:"summary"`
	Description string                    `json:"description,omitempty"`
	OperationID string                    `json:"operationId"`
	Tags        []string                  `json:"tags,omitempty"`
	Parameters  []Parameter               `json:"parameters,omitempty"`
	RequestBody *RequestBodySpec          `json:"requestBody,omitempty"`
	Responses   map[string]ResponseSpec   `json:"responses"`
}

// RequestBodySpec describes a request body.
type RequestBodySpec struct {
	Description string                   `json:"description,omitempty"`
	Content     map[string]MediaTypeSpec `json:"content"`
}

// MediaTypeSpec describes a media type.
type MediaTypeSpec struct {
	Schema SchemaRef `json:"schema"`
}

// ResponseSpec describes an API response.
type ResponseSpec struct {
	Description string                   `json:"description"`
	Content     map[string]MediaTypeSpec `json:"content,omitempty"`
}

// TagSpec describes an API tag.
type TagSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GeneratorConfig configures the OpenAPI spec generator.
type GeneratorConfig struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	BasePath    string `json:"base_path"`
}

// DefaultGeneratorConfig returns sensible defaults.
func DefaultGeneratorConfig() GeneratorConfig {
	return GeneratorConfig{
		Title:       "Feather Feature Store API",
		Description: "High-performance real-time feature store",
		Version:     "1.0.0",
		BasePath:    "/v1",
	}
}

// GeneratorStats holds generator statistics.
type GeneratorStats struct {
	TotalRoutes int `json:"total_routes"`
	TotalPaths  int `json:"total_paths"`
	TotalTags   int `json:"total_tags"`
}

// Generator builds OpenAPI specifications from registered routes.
type Generator struct {
	mu     sync.RWMutex
	config GeneratorConfig
	routes []RouteInfo
}

// NewGenerator creates a new OpenAPI spec generator.
func NewGenerator(config GeneratorConfig) *Generator {
	return &Generator{
		config: config,
		routes: make([]RouteInfo, 0),
	}
}

// AddRoute registers a route for inclusion in the generated spec.
func (g *Generator) AddRoute(route RouteInfo) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.routes = append(g.routes, route)
}

// AddRouteFromPattern parses a URL pattern and extracts path parameters.
func (g *Generator) AddRouteFromPattern(method, pattern, summary string, tags []string) {
	var params []Parameter
	parts := strings.Split(pattern, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params = append(params, Parameter{
				Name:     name,
				In:       "path",
				Required: true,
				Schema:   SchemaRef{Type: "string"},
			})
		}
	}

	route := RouteInfo{
		Method:     method,
		Path:       pattern,
		Summary:    summary,
		Tags:       tags,
		Parameters: params,
	}

	g.AddRoute(route)
}

// GenerateSpec builds an OpenAPI specification from all registered routes.
func (g *Generator) GenerateSpec() *OpenAPISpec {
	g.mu.RLock()
	defer g.mu.RUnlock()

	spec := &OpenAPISpec{
		OpenAPI: "3.1.0",
		Info: SpecInfo{
			Title:       g.config.Title,
			Description: g.config.Description,
			Version:     g.config.Version,
		},
		Paths: make(map[string]map[string]OperationSpec),
	}

	tagSet := make(map[string]bool)

	for _, route := range g.routes {
		if spec.Paths[route.Path] == nil {
			spec.Paths[route.Path] = make(map[string]OperationSpec)
		}

		op := OperationSpec{
			Summary:     route.Summary,
			Description: route.Description,
			OperationID: g.operationID(route.Method, route.Path),
			Tags:        route.Tags,
			Parameters:  route.Parameters,
			Responses: map[string]ResponseSpec{
				"200": {Description: "Successful response"},
			},
		}

		if route.RequestBody != nil {
			op.RequestBody = &RequestBodySpec{
				Content: map[string]MediaTypeSpec{
					"application/json": {Schema: *route.RequestBody},
				},
			}
		}

		spec.Paths[route.Path][strings.ToLower(route.Method)] = op

		for _, tag := range route.Tags {
			tagSet[tag] = true
		}
	}

	for tag := range tagSet {
		spec.Tags = append(spec.Tags, TagSpec{Name: tag})
	}

	return spec
}

// GenerateJSON returns the OpenAPI spec as a JSON string.
func (g *Generator) GenerateJSON() (string, error) {
	spec := g.GenerateSpec()
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetRouteCount returns the number of registered routes.
func (g *Generator) GetRouteCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.routes)
}

// ListRoutes returns all registered routes.
func (g *Generator) ListRoutes() []RouteInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]RouteInfo, len(g.routes))
	copy(result, g.routes)
	return result
}

// Stats returns generator statistics.
func (g *Generator) Stats() GeneratorStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	paths := make(map[string]bool)
	tags := make(map[string]bool)
	for _, r := range g.routes {
		paths[r.Path] = true
		for _, t := range r.Tags {
			tags[t] = true
		}
	}

	return GeneratorStats{
		TotalRoutes: len(g.routes),
		TotalPaths:  len(paths),
		TotalTags:   len(tags),
	}
}

func (g *Generator) operationID(method, path string) string {
	// Convert "GET /v1/features/{id}" to "get_v1_features_id"
	clean := strings.NewReplacer(
		"/", "_", "{", "", "}", "",
	).Replace(path)
	clean = strings.TrimPrefix(clean, "_")
	return strings.ToLower(method) + "_" + clean
}
