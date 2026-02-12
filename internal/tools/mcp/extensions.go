package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ResourceType identifies MCP resource types.
type ResourceType string

const (
	ResourceSchema     ResourceType = "schema"
	ResourceFeature    ResourceType = "feature"
	ResourceStats      ResourceType = "stats"
	ResourceIndex      ResourceType = "vector_index"
)

// Resource represents an MCP resource exposed to AI agents.
type Resource struct {
	URI         string       `json:"uri"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	MimeType    string       `json:"mimeType"`
	Type        ResourceType `json:"type"`
}

// PromptTemplate provides pre-built prompts for common feature store tasks.
type PromptTemplate struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Template    string           `json:"template"`
}

// PromptArgument describes a template parameter.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// AdditionalTools returns extended tool definitions for the MCP server.
func AdditionalTools() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "create_feature_group",
			Description: "Create a new feature group schema with specified features and types",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "Feature group name"},
					"entity_type": {"type": "string", "description": "Entity type (e.g., user, product)"},
					"features": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"name": {"type": "string"},
								"data_type": {"type": "string", "enum": ["int64", "float64", "string", "bool", "timestamp", "vector"]}
							},
							"required": ["name", "data_type"]
						}
					},
					"ttl": {"type": "string", "description": "TTL duration (e.g., 24h)"}
				},
				"required": ["name", "entity_type", "features"]
			}`),
		},
		{
			Name:        "delete_features",
			Description: "Delete features for a specific entity",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity_key": {"type": "string", "description": "Entity key to delete features for"}
				},
				"required": ["entity_key"]
			}`),
		},
		{
			Name:        "get_feature_history",
			Description: "Get historical values of a feature at a specific point in time",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity_key": {"type": "string"},
					"features": {"type": "array", "items": {"type": "string"}},
					"as_of": {"type": "string", "description": "RFC3339 timestamp for point-in-time query"}
				},
				"required": ["entity_key", "features", "as_of"]
			}`),
		},
		{
			Name:        "batch_get_features",
			Description: "Retrieve features for multiple entities in a single request",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entities": {"type": "array", "items": {"type": "string"}, "description": "List of entity keys"},
					"features": {"type": "array", "items": {"type": "string"}, "description": "Feature names to retrieve"}
				},
				"required": ["entities", "features"]
			}`),
		},
		{
			Name:        "get_store_metrics",
			Description: "Get current store performance metrics including hit rates, latency, and memory usage",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "explain_feature",
			Description: "Get detailed information about a specific feature including schema, statistics, and lineage",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"feature_name": {"type": "string", "description": "Name of the feature to explain"}
				},
				"required": ["feature_name"]
			}`),
		},
	}
}

// BuiltinResources returns the list of MCP resources the server exposes.
func BuiltinResources() []Resource {
	return []Resource{
		{
			URI:         "feather://schema/groups",
			Name:        "Feature Groups",
			Description: "All feature group definitions including types and TTLs",
			MimeType:    "application/json",
			Type:        ResourceSchema,
		},
		{
			URI:         "feather://stats/store",
			Name:        "Store Statistics",
			Description: "Current storage tier metrics, hit rates, and memory usage",
			MimeType:    "application/json",
			Type:        ResourceStats,
		},
		{
			URI:         "feather://stats/vectors",
			Name:        "Vector Index Statistics",
			Description: "HNSW index sizes, dimensions, and query statistics",
			MimeType:    "application/json",
			Type:        ResourceIndex,
		},
	}
}

// BuiltinPrompts returns pre-built prompt templates for common tasks.
func BuiltinPrompts() []PromptTemplate {
	return []PromptTemplate{
		{
			Name:        "analyze_feature_usage",
			Description: "Analyze how a feature is being used and suggest optimizations",
			Arguments: []PromptArgument{
				{Name: "feature_name", Description: "Feature to analyze", Required: true},
			},
			Template: "Analyze the feature '{{feature_name}}' in the Feather feature store. Check its schema, recent values, hit rate, and access patterns. Suggest optimizations for performance and cost.",
		},
		{
			Name:        "debug_missing_features",
			Description: "Debug why features might be missing or stale for an entity",
			Arguments: []PromptArgument{
				{Name: "entity_key", Description: "Entity key to debug", Required: true},
			},
			Template: "Debug why features for entity '{{entity_key}}' might be missing or stale. Check the entity exists, verify feature groups are configured, check hot/warm tier status, and look for ingestion issues.",
		},
		{
			Name:        "generate_feature_schema",
			Description: "Generate a feature group schema based on a description of the use case",
			Arguments: []PromptArgument{
				{Name: "use_case", Description: "Description of the ML use case", Required: true},
				{Name: "entity_type", Description: "Type of entity (user, product, etc.)", Required: true},
			},
			Template: "Generate a Feather feature group YAML schema for the following use case: {{use_case}}. The entity type is '{{entity_type}}'. Include appropriate data types, TTLs, and aggregation windows.",
		},
	}
}

// ServerInfo provides extended information about the MCP server.
type ServerInfo struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities"`
	ToolCount    int       `json:"tool_count"`
	ResourceCount int     `json:"resource_count"`
	PromptCount  int       `json:"prompt_count"`
	StartedAt    time.Time `json:"started_at"`
}

// GetServerInfo returns information about this MCP server instance.
func GetServerInfo() *ServerInfo {
	tools := AdditionalTools()
	resources := BuiltinResources()
	prompts := BuiltinPrompts()

	return &ServerInfo{
		Name:          "feather-mcp",
		Version:       "1.0.0",
		Description:   "Feather Feature Store MCP Server - Enables AI agents to discover, query, and manage ML features",
		Capabilities:  []string{"tools", "resources", "prompts"},
		ToolCount:     len(tools) + 10, // 10 existing + new
		ResourceCount: len(resources),
		PromptCount:   len(prompts),
		StartedAt:     time.Now(),
	}
}

// FormatFeatureTable formats features as a human-readable markdown table.
func FormatFeatureTable(features map[string]interface{}) string {
	if len(features) == 0 {
		return "No features found."
	}

	keys := make([]string, 0, len(features))
	for k := range features {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := "| Feature | Value |\n|---------|-------|\n"
	for _, k := range keys {
		result += fmt.Sprintf("| %s | %v |\n", k, features[k])
	}
	return result
}
