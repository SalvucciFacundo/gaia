package lsp

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
)

// Module implements ports.Module, wrapping an LSP client's diagnostic
// and analysis capabilities for registration in the Brain's ToolRegistry.
type Module struct {
	client     *Client
	serverName string
}

// NewModule creates an LSP module wrapping a connected client.
func NewModule(client *Client) *Module {
	return &Module{
		client:     client,
		serverName: client.cfg.Name,
	}
}

// Name returns the module identifier, prefixed for collision avoidance.
func (m *Module) Name() string {
	return "lsp_" + m.serverName
}

// Description returns a human-readable summary.
func (m *Module) Description() string {
	return "LSP diagnostics and analysis from " + m.serverName
}

// GetTools returns the tool definitions this module provides.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{
			Name: fmt.Sprintf("lsp_%s_diagnostics", m.serverName),
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Get LSP diagnostics from %s", m.serverName),
				"server":      m.serverName,
			},
		},
	}
}

// Execute dispatches an LSP tool call by name.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch toolName {
	case fmt.Sprintf("lsp_%s_diagnostics", m.serverName):
		diags, err := m.client.Diagnostics(ctx)
		if err != nil {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("LSP diagnostics error: %v", err),
			}, nil
		}
		return &domain.ToolResult{
			Success: true,
			Output:  FormatDiagnostics(diags),
		}, nil
	default:
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown LSP tool: %s", toolName),
		}, nil
	}
}

// Close closes the underlying LSP client.
func (m *Module) Close() error {
	return m.client.Close()
}
