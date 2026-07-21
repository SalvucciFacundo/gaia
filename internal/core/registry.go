package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// ToolEntry is a named tool with its owning module.
type ToolEntry struct {
	Name   string
	Module ports.Module
}

// ToolRegistry manages tool definitions and dispatches execution with caching.
type ToolRegistry struct {
	tools map[string]ToolEntry
	cache *ToolCache
}

// NewToolRegistry creates an empty tool registry with caching.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolEntry),
		cache: NewToolCache(),
	}
}

// Register adds all tools from a module to the registry.
func (r *ToolRegistry) Register(mod ports.Module) {
	for _, def := range mod.GetTools() {
		r.tools[def.Name] = ToolEntry{Name: def.Name, Module: mod}
	}
}

// Execute dispatches a tool call by name, with caching for read-only tools.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	entry, ok := r.tools[name]
	if !ok {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}, nil
	}

	// Try cache for read-only tools
	if r.cache != nil && isReadOnlyTool(name) {
		cacheKey := buildCacheKey(name, args)
		if cached, ok := r.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	result, execErr := entry.Module.Execute(ctx, name, args)
	if execErr != nil {
		result = &domain.ToolResult{
			Success: false,
			Error:   execErr.Error(),
		}
	}

	// Cache read-only results
	if r.cache != nil && isReadOnlyTool(name) && result != nil {
		cacheKey := buildCacheKey(name, args)
		r.cache.Set(cacheKey, result)
	}

	return result, nil
}

// Filtered returns a new ToolRegistry containing only the specified tool names.
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
	sort.Strings(names)
	return names
}

// ToolInfo holds display info for a registered tool.
type ToolInfo struct {
	Name        string
	Module      string
	Description string
}

// ListToolInfo returns all registered tools sorted by name.
func (r *ToolRegistry) ListToolInfo() []ToolInfo {
	seen := make(map[string]bool)
	var tools []ToolInfo
	for _, entry := range r.tools {
		if seen[entry.Name] {
			continue
		}
		seen[entry.Name] = true
		desc := ""
		for _, def := range entry.Module.GetTools() {
			if def.Name == entry.Name {
				if descMap, ok := def.Arguments["description"]; ok {
					if d, ok := descMap.(string); ok {
						desc = d
					}
				}
				break
			}
		}
		tools = append(tools, ToolInfo{
			Name:        entry.Name,
			Module:      entry.Module.Name(),
			Description: desc,
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

// SearchTools finds tools whose name or module matches the query.
func (r *ToolRegistry) SearchTools(query string) []ToolInfo {
	all := r.ListToolInfo()
	if query == "" {
		return all
	}
	lower := strings.ToLower(query)
	var results []ToolInfo
	for _, t := range all {
		if strings.Contains(strings.ToLower(t.Name), lower) ||
			strings.Contains(strings.ToLower(t.Module), lower) {
			results = append(results, t)
		}
	}
	return results
}

// ---- Tool Cache ----

// readOnlyTools whose output is deterministic within a short window.
var readOnlyTools = map[string]bool{
	"read": true, "glob": true, "grep": true,
	"file_info": true, "list_dir": true,
}

func isReadOnlyTool(name string) bool {
	return readOnlyTools[strings.ToLower(name)]
}

func buildCacheKey(name string, args map[string]interface{}) string {
	key := name
	for k, v := range args {
		key += fmt.Sprintf("|%s=%v", k, v)
	}
	return key
}

type cacheItem struct {
	result    *domain.ToolResult
	expiresAt time.Time
}

const defaultToolCacheTTL = 5 * time.Second

// ToolCache provides TTL-based caching for tool outputs.
type ToolCache struct {
	mu    sync.Mutex
	items map[string]*cacheItem
}

func NewToolCache() *ToolCache {
	return &ToolCache{items: make(map[string]*cacheItem)}
}

func (c *ToolCache) Get(key string) (*domain.ToolResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		delete(c.items, key)
		return nil, false
	}
	return item.result, true
}

func (c *ToolCache) Set(key string, result *domain.ToolResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &cacheItem{
		result:    result,
		expiresAt: time.Now().Add(defaultToolCacheTTL),
	}
}

// Flush clears all cached tool results (call after write operations like git commit).
func (c *ToolCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}
