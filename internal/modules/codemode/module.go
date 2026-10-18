package codemode

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core"
	"gaia/internal/core/domain"
)

// Module implements the codemode programmatic tool execution plugin.
type Module struct {
	engine *Engine
}

// NewModule creates a new codemode module for GAIA.
func NewModule(projectRoot string, registry *core.ToolRegistry) *Module {
	return &Module{
		engine: NewEngine(projectRoot, registry),
	}
}

// Name returns the module identifier.
func (m *Module) Name() string {
	return "codemode"
}

// Description returns a concise summary of the module.
func (m *Module) Description() string {
	return "Executes dynamic Go scripts in an embedded interpreter (Yaegi) for high-performance multi-step operations."
}

// GetTools returns the advertised tool definitions for registry registration.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{
			Name: "execute_code",
		},
	}
}

// Tools returns detailed tool schemas for LLM provider advertisements.
func (m *Module) Tools() []domain.ToolDef {
	return []domain.ToolDef{
		{
			Name:        "execute_code",
			Description: "Execute a Go script dynamically in an embedded interpreter to perform multi-step operations (file inspection, filtering, loops, memory search) in a single turn. You can write bare statements using the 'gaia' SDK: gaia.ReadFile(path), gaia.WriteFile(path, content), gaia.Find(pattern), gaia.Grep(pattern, files...), gaia.Exec(cmd), gaia.Print(args...), gaia.Printf(format, args...). Standard Go packages (fmt, strings, regexp, os, filepath) are pre-imported.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "The Go code script or statements to execute.",
					},
				},
				"required": []string{"code"},
			},
		},
	}
}

// Execute handles tool dispatch for execute_code.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	if toolName != "execute_code" {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}

	codeVal, ok := args["code"]
	if !ok {
		return &domain.ToolResult{
			Success: false,
			Error:   "missing required argument 'code'",
		}, nil
	}

	code, ok := codeVal.(string)
	if !ok || strings.TrimSpace(code) == "" {
		return &domain.ToolResult{
			Success: false,
			Error:   "argument 'code' must be a non-empty string",
		}, nil
	}

	out, err := m.engine.Execute(ctx, code)
	if err != nil {
		return &domain.ToolResult{
			Success: false,
			Output:  out,
			Error:   err.Error(),
		}, nil
	}

	return &domain.ToolResult{
		Success: true,
		Output:  out,
	}, nil
}
