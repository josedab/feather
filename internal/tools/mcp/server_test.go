package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/core/vector"
)

func TestNewServer(t *testing.T) {
	server := NewServer(ServerConfig{})
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestNewServer_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	server := NewServer(ServerConfig{
		Logger: logger,
	})

	if server.logger != logger {
		t.Error("logger was not set correctly")
	}
}

func TestServer_handleInitialize(t *testing.T) {
	server := NewServer(ServerConfig{})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(initializeResult)
	if !ok {
		t.Fatalf("result is not initializeResult: %T", resp.Result)
	}

	if result.ServerInfo.Name != "feather-mcp" {
		t.Errorf("ServerInfo.Name = %s, want feather-mcp", result.ServerInfo.Name)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("ProtocolVersion = %s, want 2024-11-05", result.ProtocolVersion)
	}
}

func TestServer_handleInitialized(t *testing.T) {
	server := NewServer(ServerConfig{})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "initialized",
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_handleToolsList(t *testing.T) {
	server := NewServer(ServerConfig{})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/list",
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(toolsListResult)
	if !ok {
		t.Fatalf("result is not toolsListResult: %T", resp.Result)
	}

	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"get_features", "put_features", "list_feature_groups", "health_check", "vector_search"}
	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool %q not found", expected)
		}
	}
}

func TestServer_handleToolCall_UnknownTool(t *testing.T) {
	server := NewServer(ServerConfig{})

	params, _ := json.Marshal(toolCallParams{
		Name:      "unknown_tool",
		Arguments: json.RawMessage(`{}`),
	})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("result is not toolCallResult: %T", resp.Result)
	}

	if !result.IsError {
		t.Error("expected IsError to be true for unknown tool")
	}
}

func TestServer_handleToolCall_InvalidParams(t *testing.T) {
	server := NewServer(ServerConfig{})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  json.RawMessage(`not valid json`),
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

func TestServer_handleUnknownMethod(t *testing.T) {
	server := NewServer(ServerConfig{})

	req := &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "unknown/method",
	}

	resp := server.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestServer_toolGetFeatures_NoStore(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:123",
		"features": []string{"age", "income"},
	})

	result := server.toolGetFeatures(context.Background(), args)

	if !result.IsError {
		t.Error("expected IsError for nil store")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "not available") {
		t.Errorf("expected error message about store not available")
	}
}

func TestServer_toolPutFeatures_NoStore(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{
		"entity": "user:123",
		"features": map[string]interface{}{
			"age": 25,
		},
	})

	result := server.toolPutFeatures(context.Background(), args)

	if !result.IsError {
		t.Error("expected IsError for nil store")
	}
}

func TestServer_toolListFeatureGroups_NoSchema(t *testing.T) {
	server := NewServer(ServerConfig{})

	result := server.toolListFeatureGroups(context.Background())

	if !result.IsError {
		t.Error("expected IsError for nil schema")
	}
}

func TestServer_toolGetFeatureGroup_NoSchema(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{
		"name": "user_features",
	})

	result := server.toolGetFeatureGroup(context.Background(), args)

	if !result.IsError {
		t.Error("expected IsError for nil schema")
	}
}

func TestServer_toolVectorSearch_NoVectors(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{
		"index":  "embeddings",
		"vector": []float32{0.1, 0.2, 0.3},
		"top_k":  5,
	})

	result := server.toolVectorSearch(context.Background(), args)

	if !result.IsError {
		t.Error("expected IsError for nil vectors")
	}
}

func TestServer_toolListVectorIndexes_NoVectors(t *testing.T) {
	server := NewServer(ServerConfig{})

	result := server.toolListVectorIndexes(context.Background())

	if !result.IsError {
		t.Error("expected IsError for nil vectors")
	}
}

func TestServer_toolHealthCheck(t *testing.T) {
	server := NewServer(ServerConfig{})

	result := server.toolHealthCheck(context.Background())

	if result.IsError {
		t.Error("health check should not return error")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Should contain status
	if !strings.Contains(result.Content[0].Text, "status") {
		t.Error("expected status in health check response")
	}

	// Should be degraded since store is nil
	if !strings.Contains(result.Content[0].Text, "degraded") {
		t.Error("expected degraded status for nil store")
	}
}

func TestSuccessResult(t *testing.T) {
	result := successResult("test message")

	if result.IsError {
		t.Error("IsError should be false for success")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %s, want text", result.Content[0].Type)
	}
	if result.Content[0].Text != "test message" {
		t.Errorf("content text = %s, want test message", result.Content[0].Text)
	}
}

func TestErrorResult(t *testing.T) {
	result := errorResult("error message")

	if !result.IsError {
		t.Error("IsError should be true for error")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Text != "error message" {
		t.Errorf("content text = %s, want error message", result.Content[0].Text)
	}
}

func TestServer_writeResponse(t *testing.T) {
	server := NewServer(ServerConfig{})

	var buf bytes.Buffer
	resp := &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "test",
	}

	err := server.writeResponse(&buf, resp)
	if err != nil {
		t.Fatalf("writeResponse failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"jsonrpc":"2.0"`) {
		t.Errorf("output should contain jsonrpc, got: %s", output)
	}
	if !strings.Contains(output, `"result":"test"`) {
		t.Errorf("output should contain result, got: %s", output)
	}
}

func TestServer_writeError(t *testing.T) {
	server := NewServer(ServerConfig{})

	var buf bytes.Buffer
	server.writeError(&buf, 1, -32700, "Parse error", nil)

	output := buf.String()
	if !strings.Contains(output, `"error"`) {
		t.Errorf("output should contain error, got: %s", output)
	}
	if !strings.Contains(output, `"code":-32700`) {
		t.Errorf("output should contain error code, got: %s", output)
	}
}

// --- Helper to create server with real store ---

func newTestServerWithStore(t *testing.T) *Server {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:       1024 * 1024 * 10,
		WarmInMemory:     true,
		TTLCheckInterval: time.Hour,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	schema := storage.NewRegistry()
	schema.RegisterGroup(&domain.FeatureGroup{
		Name:        "user_features",
		EntityType:  "user",
		Description: "User features",
		Features: []domain.FeatureSpec{
			{Name: "clicks", DataType: domain.DataTypeInt64},
			{Name: "views", DataType: domain.DataTypeInt64},
		},
	})

	return NewServer(ServerConfig{
		Store:       store,
		Schema:      schema,
		Aggregation: aggregation.NewEngine(),
	})
}

// --- toolSearchFeatures tests ---

func TestToolSearchFeatures_PatternMatch(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"query": "click"})
	result := server.toolSearchFeatures(context.Background(), args)

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "clicks") {
		t.Error("expected to find 'clicks' matching 'click'")
	}
}

func TestToolSearchFeatures_NoResults(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"query": "zzz_nonexistent"})
	result := server.toolSearchFeatures(context.Background(), args)

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, `"count": 0`) {
		t.Error("expected 0 matches")
	}
}

func TestToolSearchFeatures_EntityTypeFilter(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"query": "click", "entity_type": "user"})
	result := server.toolSearchFeatures(context.Background(), args)

	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "clicks") {
		t.Error("expected clicks for user entity type")
	}
}

func TestToolSearchFeatures_EntityTypeFilter_NoMatch(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"query": "click", "entity_type": "item"})
	result := server.toolSearchFeatures(context.Background(), args)

	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, `"count": 0`) {
		t.Error("expected 0 matches for non-matching entity type")
	}
}

func TestToolSearchFeatures_NoSchema(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{"query": "test"})
	result := server.toolSearchFeatures(context.Background(), args)

	if !result.IsError {
		t.Error("expected error for nil schema")
	}
}

func TestToolSearchFeatures_InvalidParams(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolSearchFeatures(context.Background(), json.RawMessage(`invalid`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolGetAggregation tests ---

func TestToolGetAggregation_ValidFunction(t *testing.T) {
	server := newTestServerWithStore(t)

	for _, fn := range []string{"count", "sum", "avg", "min", "max"} {
		args, _ := json.Marshal(map[string]interface{}{
			"entity":   "user:1",
			"feature":  "clicks",
			"function": fn,
			"window":   "1h",
		})
		result := server.toolGetAggregation(context.Background(), args)
		if result.IsError {
			t.Errorf("toolGetAggregation(%s) unexpected error", fn)
		}
		if !strings.Contains(result.Content[0].Text, fn) {
			t.Errorf("expected function %s in result", fn)
		}
	}
}

func TestToolGetAggregation_InvalidWindow(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:1",
		"feature":  "clicks",
		"function": "sum",
		"window":   "invalid",
	})
	result := server.toolGetAggregation(context.Background(), args)
	if !result.IsError {
		t.Error("expected error for invalid window")
	}
}

func TestToolGetAggregation_DefaultWindow(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:1",
		"feature":  "clicks",
		"function": "sum",
	})
	result := server.toolGetAggregation(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "1h0m0s") {
		t.Error("expected default 1h window")
	}
}

func TestToolGetAggregation_NoEngine(t *testing.T) {
	server := NewServer(ServerConfig{})

	args, _ := json.Marshal(map[string]interface{}{
		"entity": "user:1", "feature": "clicks", "function": "sum",
	})
	result := server.toolGetAggregation(context.Background(), args)
	if !result.IsError {
		t.Error("expected error for nil aggregation engine")
	}
}

func TestToolGetAggregation_InvalidParams(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolGetAggregation(context.Background(), json.RawMessage(`{bad`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolDescribeSchema tests ---

func TestToolDescribeSchema_Populated(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolDescribeSchema(context.Background())
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "user_features") {
		t.Error("expected user_features in schema")
	}
	if !strings.Contains(result.Content[0].Text, "clicks") {
		t.Error("expected clicks feature in schema")
	}
}

func TestToolDescribeSchema_NoSchema(t *testing.T) {
	server := NewServer(ServerConfig{})

	result := server.toolDescribeSchema(context.Background())
	if !result.IsError {
		t.Error("expected error for nil schema")
	}
}

// --- toolGetFeatures with store ---

func TestToolGetFeatures_WithStore(t *testing.T) {
	server := newTestServerWithStore(t)

	// Store a feature first
	ctx := context.Background()
	server.store.Put(ctx, "user:1", map[string]*domain.FeatureValue{
		"clicks": {Value: int64(42), Timestamp: time.Now().UnixNano()},
	})

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:1",
		"features": []string{"clicks"},
	})
	result := server.toolGetFeatures(ctx, args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "clicks") {
		t.Error("expected clicks in result")
	}
}

func TestToolGetFeatures_MissingEntity(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:999",
		"features": []string{"clicks"},
	})
	result := server.toolGetFeatures(context.Background(), args)
	// Should succeed but with empty result
	if result.IsError {
		t.Error("expected success for missing entity (returns empty)")
	}
}

func TestToolGetFeatures_InvalidParams(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolGetFeatures(context.Background(), json.RawMessage(`{invalid`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolPutFeatures with store ---

func TestToolPutFeatures_WithStore(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{
		"entity":   "user:1",
		"features": map[string]interface{}{"clicks": 42},
	})
	result := server.toolPutFeatures(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Successfully stored") {
		t.Error("expected success message")
	}
}

func TestToolPutFeatures_InvalidParams(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolPutFeatures(context.Background(), json.RawMessage(`{bad`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolVectorSearch with store ---

func TestToolVectorSearch_WithIndex(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	vs.CreateIndex("test-idx", 3, "cosine")
	idx, _ := vs.GetIndex("test-idx")
	idx.Upsert("v1", []float32{1, 0, 0}, nil)

	server := NewServer(ServerConfig{Vectors: vs})

	args, _ := json.Marshal(map[string]interface{}{
		"index":  "test-idx",
		"vector": []float32{1, 0, 0},
		"top_k":  5,
	})
	result := server.toolVectorSearch(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
}

func TestToolVectorSearch_IndexNotFound(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	server := NewServer(ServerConfig{Vectors: vs})

	args, _ := json.Marshal(map[string]interface{}{
		"index":  "nonexistent",
		"vector": []float32{1, 0, 0},
	})
	result := server.toolVectorSearch(context.Background(), args)
	if !result.IsError {
		t.Error("expected error for missing index")
	}
}

func TestToolVectorSearch_DefaultTopK(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	vs.CreateIndex("idx", 3, "cosine")
	server := NewServer(ServerConfig{Vectors: vs})

	args, _ := json.Marshal(map[string]interface{}{
		"index":  "idx",
		"vector": []float32{1, 0, 0},
	})
	result := server.toolVectorSearch(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}
}

func TestToolVectorSearch_InvalidParams(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	server := NewServer(ServerConfig{Vectors: vs})

	result := server.toolVectorSearch(context.Background(), json.RawMessage(`{bad`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolListVectorIndexes with store ---

func TestToolListVectorIndexes_Empty(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	server := NewServer(ServerConfig{Vectors: vs})

	result := server.toolListVectorIndexes(context.Background())
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "[]") {
		t.Error("expected empty list")
	}
}

func TestToolListVectorIndexes_Populated(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	vs.CreateIndex("embeddings", 128, "cosine")
	server := NewServer(ServerConfig{Vectors: vs})

	result := server.toolListVectorIndexes(context.Background())
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "embeddings") {
		t.Error("expected embeddings index in list")
	}
}

// --- toolListFeatureGroups with schema ---

func TestToolListFeatureGroups_WithSchema(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolListFeatureGroups(context.Background())
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "user_features") {
		t.Error("expected user_features group")
	}
}

// --- toolGetFeatureGroup with schema ---

func TestToolGetFeatureGroup_WithSchema(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"name": "user_features"})
	result := server.toolGetFeatureGroup(context.Background(), args)
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "user_features") {
		t.Error("expected user_features in result")
	}
}

func TestToolGetFeatureGroup_NotFound(t *testing.T) {
	server := newTestServerWithStore(t)

	args, _ := json.Marshal(map[string]interface{}{"name": "nonexistent"})
	result := server.toolGetFeatureGroup(context.Background(), args)
	if !result.IsError {
		t.Error("expected error for nonexistent group")
	}
}

func TestToolGetFeatureGroup_InvalidParams(t *testing.T) {
	server := newTestServerWithStore(t)

	result := server.toolGetFeatureGroup(context.Background(), json.RawMessage(`{bad`))
	if !result.IsError {
		t.Error("expected error for invalid params")
	}
}

// --- toolHealthCheck with components ---

func TestToolHealthCheck_AllComponents(t *testing.T) {
	vs := vector.NewStore(vector.StoreConfig{})
	server := newTestServerWithStore(t)
	server.vectors = vs

	result := server.toolHealthCheck(context.Background())
	if result.IsError {
		t.Fatal("unexpected error")
	}
	if !strings.Contains(result.Content[0].Text, "healthy") {
		t.Error("expected healthy status with all components")
	}
}
