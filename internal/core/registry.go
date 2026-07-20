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

// Filtered returns a new ToolRegistry containing only the specified tool names.
// If allowed is nil or empty, a copy with all tools is returned.
func (r *ToolRegistry) Filtered(allowed []string) *ToolRegistry {
	filtered := NewToolRegistry()
	if len(allowed) == 0 {
		for k, v := range r.tools {
			filtered.tools[k] = v
		}
		return filtered
	}
	for _, name := range allowed {
		if entry, ok := r.tools[name]; ok {
			filtered.tools[name] = entry
		}
	}
	return filtered
}

// Tools returns a copy of the current tool names.
func (r *ToolRegistry) Tools() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
