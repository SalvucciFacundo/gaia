package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"gaia/internal/core/domain"
)

// MCPModule implements ports.Module, wrapping an MCP server's tools
// for registration in the Brain's ToolRegistry.
type MCPModule struct {
	client     *Client
	serverName string
	tools      []domain.ToolCall
	mu         sync.RWMutex
	discovered bool
}

// NewMCPModule creates a module wrapping an MCP client.
// Call Discover() to populate the tool list before registering.
func NewMCPModule(client *Client) *MCPModule {
	return &MCPModule{
		client:     client,
		serverName: client.ServerName(),
	}
}

// Name returns the module identifier, prefixed for collision avoidance.
func (m *MCPModule) Name() string {
	return "mcp_" + m.serverName
}

// Description returns a human-readable summary.
func (m *MCPModule) Description() string {
	return "MCP tools from " + m.serverName
}

// Discover connects to the MCP server and loads its tools.
func (m *MCPModule) Discover(ctx context.Context) error {
	if err := m.client.Connect(ctx); err != nil {
		return fmt.Errorf("mcp discover: %w", err)
	}

	mcpTools, err := m.client.DiscoverTools(ctx)
	if err != nil {
		return fmt.Errorf("mcp discover tools: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.tools = make([]domain.ToolCall, len(mcpTools))
	for i, t := range mcpTools {
		// Prefix tool name to avoid collisions: mcp_{server}_{tool}
		prefixedName := "mcp_" + m.serverName + "_" + sanitizeToolName(t.Name)

		m.tools[i] = domain.ToolCall{
			Name: prefixedName,
			Arguments: map[string]interface{}{
				"description": t.Description,
				"name":        t.Name,    // preserve original name for calling
				"server":      m.serverName,
			},
		}
	}

	m.discovered = true
	return nil
}

// GetTools returns the discovered tool definitions.
func (m *MCPModule) GetTools() []domain.ToolCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tools
}

// Execute dispatches an MCP tool call by name.
func (m *MCPModule) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	m.mu.RLock()
	discovered := m.discovered
	m.mu.RUnlock()

	if !discovered {
		return &domain.ToolResult{Success: false, Error: "MCP module not yet discovered"}, nil
	}

	// Extract the original tool name from the prefixed version
	// mcp_{server}_{toolName} → toolName
	originalName := extractOriginalName(toolName, m.serverName)

	result, err := m.client.CallTool(ctx, originalName, args)
	if err != nil {
		return &domain.ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return result.WrapResult(), nil
}

// Close closes the underlying MCP client.
func (m *MCPModule) Close() error {
	return m.client.Close()
}

// sanitizeToolName replaces non-alphanumeric characters to make a valid tool name.
func sanitizeToolName(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
}

// extractOriginalName reverses the prefixing to get the original MCP tool name.
func extractOriginalName(prefixed, serverName string) string {
	prefix := "mcp_" + serverName + "_"
	if strings.HasPrefix(prefixed, prefix) {
		return prefixed[len(prefix):]
	}
	return prefixed
}
