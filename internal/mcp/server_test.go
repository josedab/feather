package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
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
