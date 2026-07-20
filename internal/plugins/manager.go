// Package plugins provides a third-party plugin system for GAIA.
// Plugins are standalone binaries that communicate via JSON-RPC over stdio
// (same transport as MCP). Each plugin directory contains a plugin.json
// manifest file declaring its tools and capabilities.
//
// Plugins are loaded from ~/.gaia/plugins/<name>/ and are language-agnostic
// — the only requirement is a valid plugin.json and an executable binary.
package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gaia/internal/core/domain"
)

// Manifest describes a plugin's metadata, entry point, and tool declarations.
type Manifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args,omitempty"`
	Tools       []string `json:"tools"`
	Subagents   []string `json:"subagents,omitempty"`
}

// Manager handles plugin discovery, loading, and lifecycle.
type Manager struct {
	pluginsDir string
	plugins    map[string]*PluginInstance
	mu         sync.RWMutex
}

// PluginInstance represents a loaded plugin with its manifest and controller.
type PluginInstance struct {
	Manifest Manifest
	Dir      string
	Enabled  bool
}

// NewManager creates a plugin manager scanning the given directory.
func NewManager(pluginsDir string) *Manager {
	return &Manager{
		pluginsDir: pluginsDir,
		plugins:    make(map[string]*PluginInstance),
	}
}

// Load discovers plugins in the plugins directory. Each subdirectory with a
// valid plugin.json is registered as an available plugin.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pluginsDir == "" {
		return nil
	}

	if _, err := os.Stat(m.pluginsDir); os.IsNotExist(err) {
		return nil // No plugins directory yet — not an error.
	}

	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return fmt.Errorf("plugins: read dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(m.pluginsDir, entry.Name())
		manifestPath := filepath.Join(dir, "plugin.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // No manifest — skip.
			}
			return fmt.Errorf("plugins: read manifest %s: %w", manifestPath, err)
		}

		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("plugins: parse manifest %s: %w", manifestPath, err)
		}

		if manifest.Name == "" {
			continue // Invalid manifest.
		}

		m.plugins[manifest.Name] = &PluginInstance{
			Manifest: manifest,
			Dir:      dir,
			Enabled:  true, // Enabled by default on first discovery.
		}
	}

	return nil
}

// List returns all discovered plugins sorted by name.
func (m *Manager) List() []PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Manifest.Name < result[j].Manifest.Name
	})
	return result
}

// Get returns a plugin instance by name.
func (m *Manager) Get(name string) (*PluginInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	return p, nil
}

// Enable marks a plugin as active.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Enabled = true
	return nil
}

// Disable marks a plugin as inactive.
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Enabled = false
	return nil
}

// Install creates a new plugin from a source directory by copying
// the plugin files into the plugins directory.
func (m *Manager) Install(name string, srcDir string) error {
	if err := os.MkdirAll(m.pluginsDir, 0755); err != nil {
		return fmt.Errorf("plugins: create dir: %w", err)
	}

	dstDir := filepath.Join(m.pluginsDir, name)
	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("plugin %q is already installed", name)
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("plugins: copy: %w", err)
	}

	return m.Load() // Reload to pick up the new plugin.
}

// Remove uninstalls a plugin by deleting its directory.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	if err := os.RemoveAll(p.Dir); err != nil {
		return fmt.Errorf("plugins: remove: %w", err)
	}

	delete(m.plugins, name)
	return nil
}

// ToolsForPlugin returns the tool domain.ToolCall definitions for a plugin.
// These are wrapper tools that delegate to the plugin binary via JSON-RPC.
func (m *Manager) ToolsForPlugin(name string) ([]domain.ToolCall, error) {
	p, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	tools := make([]domain.ToolCall, len(p.Manifest.Tools))
	for i, toolName := range p.Manifest.Tools {
		tools[i] = domain.ToolCall{
			Name: "plugin_" + name + "_" + toolName,
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Plugin tool: %s from %s", toolName, name),
				"plugin":      name,
				"tool_name":   toolName,
			},
		}
	}
	return tools, nil
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// Ensure context import is used (kept for future Execute methods).
var _ = context.Background
