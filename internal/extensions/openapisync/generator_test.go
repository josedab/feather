package openapisync

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	cfg := DefaultGeneratorConfig()
	g := NewGenerator(cfg)
	require.NotNil(t, g)
	assert.Equal(t, "Feather Feature Store API", g.config.Title)
	assert.Equal(t, "1.0.0", g.config.Version)
}

func TestAddRoute(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRoute(RouteInfo{
		Method:  "GET",
		Path:    "/v1/features",
		Summary: "List features",
		Tags:    []string{"features"},
	})

	assert.Equal(t, 1, g.GetRouteCount())
	routes := g.ListRoutes()
	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/v1/features", routes[0].Path)
}

func TestAddRouteFromPattern(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRouteFromPattern("GET", "/v1/features/{id}", "Get feature by ID", []string{"features"})

	routes := g.ListRoutes()
	require.Len(t, routes, 1)
	require.Len(t, routes[0].Parameters, 1)
	assert.Equal(t, "id", routes[0].Parameters[0].Name)
	assert.Equal(t, "path", routes[0].Parameters[0].In)
	assert.True(t, routes[0].Parameters[0].Required)
}

func TestGenerateSpec(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRoute(RouteInfo{
		Method:  "GET",
		Path:    "/v1/features",
		Summary: "List features",
		Tags:    []string{"features"},
	})
	g.AddRoute(RouteInfo{
		Method:  "POST",
		Path:    "/v1/features",
		Summary: "Create feature",
		Tags:    []string{"features"},
	})
	g.AddRouteFromPattern("GET", "/v1/features/{id}", "Get feature", []string{"features"})

	spec := g.GenerateSpec()
	assert.Equal(t, "3.1.0", spec.OpenAPI)
	assert.Equal(t, "Feather Feature Store API", spec.Info.Title)
	assert.Len(t, spec.Paths, 2) // /v1/features and /v1/features/{id}
	assert.Len(t, spec.Paths["/v1/features"], 2) // GET and POST
}

func TestGenerateJSON(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRoute(RouteInfo{
		Method:  "GET",
		Path:    "/v1/health",
		Summary: "Health check",
		Tags:    []string{"system"},
	})

	out, err := g.GenerateJSON()
	require.NoError(t, err)

	var spec OpenAPISpec
	require.NoError(t, json.Unmarshal([]byte(out), &spec))
	assert.Equal(t, "3.1.0", spec.OpenAPI)
	assert.Contains(t, spec.Paths, "/v1/health")
}

func TestMultipleMethods(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRoute(RouteInfo{Method: "GET", Path: "/v1/items", Summary: "List items", Tags: []string{"items"}})
	g.AddRoute(RouteInfo{Method: "POST", Path: "/v1/items", Summary: "Create item", Tags: []string{"items"}})

	spec := g.GenerateSpec()
	pathOps := spec.Paths["/v1/items"]
	require.Len(t, pathOps, 2)
	assert.Contains(t, pathOps, "get")
	assert.Contains(t, pathOps, "post")
}

func TestStats(t *testing.T) {
	g := NewGenerator(DefaultGeneratorConfig())

	g.AddRoute(RouteInfo{Method: "GET", Path: "/v1/a", Tags: []string{"alpha"}})
	g.AddRoute(RouteInfo{Method: "POST", Path: "/v1/a", Tags: []string{"alpha"}})
	g.AddRoute(RouteInfo{Method: "GET", Path: "/v1/b", Tags: []string{"beta"}})

	stats := g.Stats()
	assert.Equal(t, 3, stats.TotalRoutes)
	assert.Equal(t, 2, stats.TotalPaths)
	assert.Equal(t, 2, stats.TotalTags)
}
