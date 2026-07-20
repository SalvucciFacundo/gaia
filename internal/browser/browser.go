// Package browser provides an optional browser automation module for GAIA.
// It wraps an MCP-compatible browser server to provide web navigation,
// screenshot, and interaction tools to the agent.
package browser

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
	"gaia/internal/mcp"
)

// Module implements ports.Module for browser automation via MCP.
// It connects to a browser MCP server (e.g., playwright-mcp or puppeteer-mcp)
// and exposes its tools through the agent's ToolRegistry.
type Module struct {
	client           *mcp.Client
	config           domain.BrowserToolsConfig
	mcpModule        *mcp.MCPModule
}

// NewModule creates a browser tools module wrapping an MCP client.
func NewModule(cfg domain.BrowserToolsConfig) (*Module, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("browser tools: not enabled")
	}
	if cfg.Command == "" {
		return nil, fmt.Errorf("browser tools: command is required")
	}

	mcpCfg := domain.MCPServerConfig{
		Name:    "browser",
		Command: cfg.Command,
	}

	client := mcp.NewClient(mcpCfg)
	return &Module{
		client:    client,
		config:    cfg,
		mcpModule: mcp.NewMCPModule(client),
	}, nil
}

// Name returns the module identifier.
func (m *Module) Name() string {
	return "browser"
}

// Description returns a human-readable summary.
func (m *Module) Description() string {
	return "Browser automation tools (navigation, screenshots, interactions)"
}

// Discover connects to the browser MCP server and loads its tools.
func (m *Module) Discover(ctx context.Context) error {
	return m.mcpModule.Discover(ctx)
}

// GetTools returns the discovered tool definitions.
func (m *Module) GetTools() []domain.ToolCall {
	return m.mcpModule.GetTools()
}

// Execute dispatches a browser tool call.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	return m.mcpModule.Execute(ctx, toolName, args)
}

// Close terminates the browser MCP server connection.
func (m *Module) Close() error {
	return m.mcpModule.Close()
}
