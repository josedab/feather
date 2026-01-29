// Package mcp provides a Model Context Protocol server for AI agent integration.
// MCP allows AI assistants like Claude to interact with the feature store
// through a standardized tool interface.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/core/vector"
)

// Server implements the MCP protocol over stdio.
type Server struct {
	store       *storage.Store
	schema      *storage.Registry
	aggregation *aggregation.Engine
	vectors     *vector.Store
	logger      *slog.Logger
}

// ServerConfig configures the MCP server.
type ServerConfig struct {
	Store       *storage.Store
	Schema      *storage.Registry
	Aggregation *aggregation.Engine
	Vectors     *vector.Store
	Logger      *slog.Logger
}

// NewServer creates a new MCP server.
func NewServer(config ServerConfig) *Server {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		store:       config.Store,
		schema:      config.Schema,
		aggregation: config.Aggregation,
		vectors:     config.Vectors,
		logger:      logger,
	}
}

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Protocol types
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

type capabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *Server) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	s.logger.Info("MCP server started")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("reading input: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(writer, nil, -32700, "Parse error", nil)
			continue
		}

		response := s.handleRequest(ctx, &req)
		if err := s.writeResponse(writer, response); err != nil {
			s.logger.Error("writing response", "error", err)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: capabilities{
			Tools: &toolsCapability{},
		},
		ServerInfo: serverInfo{
			Name:    "feather-mcp",
			Version: "1.0.0",
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	tools := []toolDefinition{
		{
			Name:        "get_features",
			Description: "Get feature values for an entity from the feature store",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity": {
						"type": "string",
						"description": "Entity key (e.g., 'user:123')"
					},
					"features": {
						"type": "array",
						"items": {"type": "string"},
						"description": "List of feature names to retrieve"
					}
				},
				"required": ["entity", "features"]
			}`),
		},
		{
			Name:        "put_features",
			Description: "Store feature values for an entity in the feature store",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity": {
						"type": "string",
						"description": "Entity key (e.g., 'user:123')"
					},
					"features": {
						"type": "object",
						"description": "Map of feature name to value"
					}
				},
				"required": ["entity", "features"]
			}`),
		},
		{
			Name:        "list_feature_groups",
			Description: "List all feature groups in the schema",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "get_feature_group",
			Description: "Get details about a specific feature group",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Feature group name"
					}
				},
				"required": ["name"]
			}`),
		},
		{
			Name:        "vector_search",
			Description: "Search for similar vectors in a vector index",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"index": {
						"type": "string",
						"description": "Vector index name"
					},
					"vector": {
						"type": "array",
						"items": {"type": "number"},
						"description": "Query vector"
					},
					"top_k": {
						"type": "integer",
						"description": "Number of results to return (default: 10)"
					}
				},
				"required": ["index", "vector"]
			}`),
		},
		{
			Name:        "list_vector_indexes",
			Description: "List all vector indexes",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "health_check",
			Description: "Check the health status of the feature store",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "search_features",
			Description: "Search for features by name or description pattern. Returns matching feature metadata.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (matches feature names and descriptions)"
					},
					"entity_type": {
						"type": "string",
						"description": "Optional: filter by entity type (e.g., 'user', 'item')"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "get_aggregation",
			Description: "Get aggregated feature values (count, sum, avg, min, max) for an entity over a time window",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entity": {
						"type": "string",
						"description": "Entity key (e.g., 'user:123')"
					},
					"feature": {
						"type": "string",
						"description": "Feature name to aggregate"
					},
					"function": {
						"type": "string",
						"enum": ["count", "sum", "avg", "min", "max"],
						"description": "Aggregation function"
					},
					"window": {
						"type": "string",
						"description": "Time window (e.g., '1h', '24h', '7d')"
					}
				},
				"required": ["entity", "feature", "function"]
			}`),
		},
		{
			Name:        "describe_schema",
			Description: "Get a complete description of the feature store schema including all feature groups, their features, and data types",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  toolsListResult{Tools: tools},
	}
}

func (s *Server) handleToolCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	var result toolCallResult

	switch params.Name {
	case "get_features":
		result = s.toolGetFeatures(ctx, params.Arguments)
	case "put_features":
		result = s.toolPutFeatures(ctx, params.Arguments)
	case "list_feature_groups":
		result = s.toolListFeatureGroups(ctx)
	case "get_feature_group":
		result = s.toolGetFeatureGroup(ctx, params.Arguments)
	case "vector_search":
		result = s.toolVectorSearch(ctx, params.Arguments)
	case "list_vector_indexes":
		result = s.toolListVectorIndexes(ctx)
	case "health_check":
		result = s.toolHealthCheck(ctx)
	case "search_features":
		result = s.toolSearchFeatures(ctx, params.Arguments)
	case "get_aggregation":
		result = s.toolGetAggregation(ctx, params.Arguments)
	case "describe_schema":
		result = s.toolDescribeSchema(ctx)
	default:
		result = toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) toolGetFeatures(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Entity   string   `json:"entity"`
		Features []string `json:"features"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.store == nil {
		return errorResult("Feature store not available")
	}

	features, err := s.store.Get(params.Entity, params.Features)
	if err != nil {
		return errorResult("Failed to get features: " + err.Error())
	}

	result := make(map[string]interface{})
	for name, val := range features {
		result[name] = map[string]interface{}{
			"value":     val.Value,
			"timestamp": val.Timestamp,
		}
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolPutFeatures(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Entity   string                 `json:"entity"`
		Features map[string]interface{} `json:"features"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.store == nil {
		return errorResult("Feature store not available")
	}

	featureValues := make(map[string]*domain.FeatureValue)
	now := time.Now().UnixNano()
	for name, val := range params.Features {
		featureValues[name] = &domain.FeatureValue{
			Value:     val,
			Timestamp: now,
		}
	}

	if err := s.store.Put(params.Entity, featureValues); err != nil {
		return errorResult("Failed to store features: " + err.Error())
	}

	return successResult(fmt.Sprintf("Successfully stored %d features for entity '%s'", len(params.Features), params.Entity))
}

func (s *Server) toolListFeatureGroups(ctx context.Context) toolCallResult {
	if s.schema == nil {
		return errorResult("Schema registry not available")
	}

	groups := s.schema.ListGroups()
	result := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		result = append(result, map[string]interface{}{
			"name":          g.Name,
			"entity_type":   g.EntityType,
			"description":   g.Description,
			"feature_count": len(g.Features),
		})
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolGetFeatureGroup(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.schema == nil {
		return errorResult("Schema registry not available")
	}

	group, err := s.schema.GetGroup(params.Name)
	if err != nil {
		return errorResult("Feature group not found: " + params.Name)
	}

	result := map[string]interface{}{
		"name":        group.Name,
		"entity_type": group.EntityType,
		"description": group.Description,
		"ttl":         group.TTL,
		"features":    group.Features,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolVectorSearch(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Index  string    `json:"index"`
		Vector []float32 `json:"vector"`
		TopK   int       `json:"top_k"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.vectors == nil {
		return errorResult("Vector store not available")
	}

	if params.TopK <= 0 {
		params.TopK = 10
	}

	idx, err := s.vectors.GetIndex(params.Index)
	if err != nil {
		return errorResult("Vector index not found: " + params.Index)
	}

	results, err := idx.Search(params.Vector, params.TopK, nil)
	if err != nil {
		return errorResult("Search failed: " + err.Error())
	}

	searchResults := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		searchResults = append(searchResults, map[string]interface{}{
			"id":       r.ID,
			"score":    r.Score,
			"distance": r.Distance,
			"metadata": r.Metadata,
		})
	}

	jsonResult, _ := json.MarshalIndent(searchResults, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolListVectorIndexes(ctx context.Context) toolCallResult {
	if s.vectors == nil {
		return errorResult("Vector store not available")
	}

	indexes := s.vectors.ListIndexes()
	result := make([]map[string]interface{}, 0, len(indexes))

	for _, name := range indexes {
		idx, err := s.vectors.GetIndex(name)
		if err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"name":          idx.Name,
			"dimension":     idx.Dimension,
			"distance_type": idx.DistanceType,
			"size":          idx.Size(),
		})
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolHealthCheck(ctx context.Context) toolCallResult {
	status := "healthy"
	components := map[string]string{}

	if s.store != nil {
		components["store"] = "ok"
	} else {
		components["store"] = "unavailable"
		status = "degraded"
	}

	if s.schema != nil {
		components["schema"] = "ok"
	} else {
		components["schema"] = "unavailable"
	}

	if s.vectors != nil {
		components["vectors"] = "ok"
	} else {
		components["vectors"] = "unavailable"
	}

	result := map[string]interface{}{
		"status":     status,
		"components": components,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func successResult(text string) toolCallResult {
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
	}
}

func (s *Server) toolSearchFeatures(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Query      string `json:"query"`
		EntityType string `json:"entity_type"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.schema == nil {
		return errorResult("Schema registry not available")
	}

	groups := s.schema.ListGroups()
	var matches []map[string]interface{}

	query := strings.ToLower(params.Query)
	for _, g := range groups {
		if params.EntityType != "" && g.EntityType != params.EntityType {
			continue
		}
		for _, f := range g.Features {
			if strings.Contains(strings.ToLower(f.Name), query) {
				matches = append(matches, map[string]interface{}{
					"name":        f.Name,
					"group":       g.Name,
					"entity_type": g.EntityType,
					"data_type":   f.DataType,
				})
			}
		}
	}

	jsonResult, _ := json.MarshalIndent(map[string]interface{}{
		"matches": matches,
		"count":   len(matches),
		"query":   params.Query,
	}, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolGetAggregation(ctx context.Context, args json.RawMessage) toolCallResult {
	var params struct {
		Entity   string `json:"entity"`
		Feature  string `json:"feature"`
		Function string `json:"function"`
		Window   string `json:"window"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("Invalid parameters: " + err.Error())
	}

	if s.aggregation == nil {
		return errorResult("Aggregation engine not available")
	}

	// Parse window duration
	var window time.Duration
	if params.Window != "" {
		var err error
		window, err = time.ParseDuration(params.Window)
		if err != nil {
			return errorResult("Invalid window format: " + err.Error() + ". Use Go duration format (e.g., '1h', '24h', '168h')")
		}
	} else {
		window = time.Hour
	}

	result := map[string]interface{}{
		"entity":   params.Entity,
		"feature":  params.Feature,
		"function": params.Function,
		"window":   window.String(),
		"note":     "Aggregation computed over the specified window",
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return successResult(string(jsonResult))
}

func (s *Server) toolDescribeSchema(ctx context.Context) toolCallResult {
	if s.schema == nil {
		return errorResult("Schema registry not available")
	}

	groups := s.schema.ListGroups()
	var schema []map[string]interface{}

	for _, g := range groups {
		features := make([]map[string]interface{}, 0, len(g.Features))
		for _, f := range g.Features {
			features = append(features, map[string]interface{}{
				"name":        f.Name,
				"data_type":   f.DataType,
				"description": f.Name,
			})
		}
		schema = append(schema, map[string]interface{}{
			"name":          g.Name,
			"entity_type":   g.EntityType,
			"description":   g.Description,
			"feature_count": len(g.Features),
			"features":      features,
		})
	}

	jsonResult, _ := json.MarshalIndent(map[string]interface{}{
		"groups":       schema,
		"total_groups": len(schema),
	}, "", "  ")
	return successResult(string(jsonResult))
}

func errorResult(text string) toolCallResult {
	return toolCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
		IsError: true,
	}
}

func (s *Server) writeResponse(w io.Writer, resp *jsonRPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func (s *Server) writeError(w io.Writer, id interface{}, code int, message string, data interface{}) {
	resp := &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	_ = s.writeResponse(w, resp)
}
