// Package fileops provides a file read/write/list module for GAIA.
// Every operation validates paths through the security package to
// prevent directory traversal and symlink escapes.
package fileops

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/modules/security"
)

// Module implements ports.Module for safe file operations.
type Module struct {
	projectRoot string
}

// NewModule creates a fileops module scoped to the given project root.
func NewModule(projectRoot string) *Module {
	return &Module{projectRoot: projectRoot}
}

// Name returns the module identifier.
func (m *Module) Name() string { return "fileops" }

// Description returns a human-readable summary of the module.
func (m *Module) Description() string {
	return "Read, write, and list files with path-traversal protection"
}

// GetTools returns tool definitions registered by this module.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{
			Name: "file_read",
			Arguments: map[string]interface{}{
				"path": "string — relative or absolute path to the file to read",
			},
		},
		{
			Name: "file_write",
			Arguments: map[string]interface{}{
				"path":    "string — relative or absolute path to the file to write",
				"content": "string — content to write",
			},
		},
		{
			Name: "file_list",
			Arguments: map[string]interface{}{
				"path": "string — directory path to list (defaults to project root)",
			},
		},
	}
}

// Execute dispatches a tool call by name.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch toolName {
	case "file_read":
		return m.readFile(args)
	case "file_write":
		return m.writeFile(args)
	case "file_list":
		return m.listFiles(args)
	default:
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", toolName)}, nil
	}
}

// readFile validates the path and reads the file content.
func (m *Module) readFile(args map[string]interface{}) (*domain.ToolResult, error) {
	path, err := m.resolveArg(args, "path")
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}

	safePath, err := security.ValidatePath(m.projectRoot, path)
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}

	data, err := ioutil.ReadFile(safePath)
	if err != nil {
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("read failed: %v", err)}, nil
	}

	return &domain.ToolResult{
		Success: true,
		Output:  string(data),
	}, nil
}

// writeFile validates the path and writes content.
func (m *Module) writeFile(args map[string]interface{}) (*domain.ToolResult, error) {
	path, err := m.resolveArg(args, "path")
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}
	content, err := m.resolveArg(args, "content")
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}

	safePath, err := security.ValidatePath(m.projectRoot, path)
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}

	// Ensure parent directory exists
	parent := filepath.Dir(safePath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("create parent dir: %v", err)}, nil
	}

	if err := ioutil.WriteFile(safePath, []byte(content), 0644); err != nil {
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("write failed: %v", err)}, nil
	}

	return &domain.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("wrote %d bytes to %s", len(content), path),
	}, nil
}

// listFiles validates the path and lists directory contents.
func (m *Module) listFiles(args map[string]interface{}) (*domain.ToolResult, error) {
	dirPath := m.projectRoot
	if v, ok := args["path"].(string); ok && v != "" {
		dirPath = v
	}

	safePath, err := security.ValidatePath(m.projectRoot, dirPath)
	if err != nil {
		return &domain.ToolResult{Success: false, Error: err.Error()}, nil
	}

	entries, err := ioutil.ReadDir(safePath)
	if err != nil {
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("list failed: %v", err)}, nil
	}

	var b strings.Builder
	for _, e := range entries {
		prefix := "-"
		if e.IsDir() {
			prefix = "d"
		}
		fmt.Fprintf(&b, "%s  %s\n", prefix, e.Name())
	}

	return &domain.ToolResult{
		Success: true,
		Output:  b.String(),
	}, nil
}

// resolveArg extracts a string argument value by name.
func (m *Module) resolveArg(args map[string]interface{}, name string) (string, error) {
	v, ok := args[name]
	if !ok || v == nil {
		return "", fmt.Errorf("missing required argument: %s", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", name)
	}
	if s == "" {
		return "", fmt.Errorf("argument %q must not be empty", name)
	}
	return s, nil
}
