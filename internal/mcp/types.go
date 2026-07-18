// Package mcp provides a Model Context Protocol (MCP) client for GAIA.
// It connects to MCP servers via stdio transport, discovers tools,
// and wraps them as ports.Module implementations for the ToolRegistry.
package mcp

import (
	"gaia/internal/core/domain"
)

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPCallParams holds parameters for an MCP tools/call request.
type MCPCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPCallResult holds the result of an MCP tools/call.
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// MCPContent represents a content block in an MCP result.
type MCPContent struct {
	Type string `json:"type"` // "text" or "resource"
	Text string `json:"text"`
}

// MCPJSONRPCRequest is a JSON-RPC 2.0 request.
type MCPJSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPJSONRPCResponse is a JSON-RPC 2.0 response.
type MCPJSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServerInfo holds MCP server metadata from the initialize response.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GetToolsResult is the result of the tools/list method.
type GetToolsResult struct {
	Tools []MCPTool `json:"tools"`
}

// ToolCallParams is an alias for compatibility with the domain layer.
type ToolCallParams = MCPCallParams

// ToolCallResult is an alias for compatibility with the domain layer.
type ToolCallResult = MCPCallResult

// WrapResult converts an MCP call result to a domain.ToolResult.
func (r *MCPCallResult) WrapResult() *domain.ToolResult {
	if r.IsError {
		msg := "MCP tool error"
		if len(r.Content) > 0 {
			msg = r.Content[0].Text
		}
		return &domain.ToolResult{
			Success: false,
			Error:   msg,
		}
	}

	output := ""
	for _, c := range r.Content {
		if c.Type == "text" {
			output += c.Text
		}
	}

	return &domain.ToolResult{
		Success: true,
		Output:  output,
	}
}
