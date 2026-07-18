package core

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// ToolEntry is a named tool with its owning module.
type ToolEntry struct {
	Name   string
	Module ports.Module
}

// ToolRegistry holds a flat map of tool names to implementations.
type ToolRegistry struct {
	tools map[string]ToolEntry
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]ToolEntry)}
}

// Register adds all tools from a module to the registry.
func (r *ToolRegistry) Register(mod ports.Module) {
	for _, def := range mod.GetTools() {
		r.tools[def.Name] = ToolEntry{Name: def.Name, Module: mod}
	}
}

// Execute dispatches a tool call by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	entry, ok := r.tools[name]
	if !ok {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}, nil
	}
	return entry.Module.Execute(ctx, name, args)
}
