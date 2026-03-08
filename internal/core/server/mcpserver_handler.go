package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/tools/mcp"
)

// MCPServerHandler exposes MCP server info and capabilities via HTTP.
type MCPServerHandler struct {
	info      *mcp.ServerInfo
	resources []mcp.Resource
	prompts   []mcp.PromptTemplate
}

// NewMCPServerHandler creates a new MCP server info handler.
func NewMCPServerHandler(info *mcp.ServerInfo, resources []mcp.Resource, prompts []mcp.PromptTemplate) *MCPServerHandler {
	return &MCPServerHandler{
		info:      info,
		resources: resources,
		prompts:   prompts,
	}
}

// RegisterRoutes registers MCP API routes.
func (h *MCPServerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/mcp/info", h.handleInfo)
	mux.HandleFunc("GET /v1/mcp/tools", h.handleTools)
	mux.HandleFunc("GET /v1/mcp/resources", h.handleResources)
	mux.HandleFunc("GET /v1/mcp/prompts", h.handlePrompts)
	mux.HandleFunc("GET /v1/mcp/capabilities", h.handleCapabilities)
}

func (h *MCPServerHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.info)
}

func (h *MCPServerHandler) handleTools(w http.ResponseWriter, r *http.Request) {
	tools := mcp.AdditionalTools()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"total": len(tools),
	})
}

func (h *MCPServerHandler) handleResources(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"resources": h.resources,
		"total":     len(h.resources),
	})
}

func (h *MCPServerHandler) handlePrompts(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"prompts": h.prompts,
		"total":   len(h.prompts),
	})
}

func (h *MCPServerHandler) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"protocol_version": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":     true,
			"resources": true,
			"prompts":   true,
		},
		"server": h.info,
	})
}
