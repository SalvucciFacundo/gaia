package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// LoadToolRunner implements lazy tool schema loading leveraging ToolCatalog.
type LoadToolRunner struct {
	catalog *ToolCatalog
}

// NewLoadToolRunner creates a LoadToolRunner backed by a ToolCatalog.
func NewLoadToolRunner(catalog *ToolCatalog) *LoadToolRunner {
	return &LoadToolRunner{
		catalog: catalog,
	}
}

// Backend implements tool backend identifier.
func (l *LoadToolRunner) Backend() string {
	return "internal"
}

// Name implements tool identifier.
func (l *LoadToolRunner) Name() string {
	return "load_tool"
}

// Description implements tool capability description.
func (l *LoadToolRunner) Description() string {
	return "Load the full schema and parameter definitions for a specific tool from the catalog."
}

// LoadToolOutput represents the structured response of load_tool.
type LoadToolOutput struct {
	Status      string      `json:"status"`
	Name        string      `json:"name"`
	Category    string      `json:"category,omitempty"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Schema      interface{} `json:"schema,omitempty"`
}

// Run looks up the requested tool and returns full schema and parameters confirmation.
func (l *LoadToolRunner) Run(_ context.Context, input string) (string, error) {
	if l.catalog == nil {
		return "", errors.New("tool catalog not available")
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("parsing load_tool arguments: %w", err)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", errors.New("tool name is required")
	}

	entry, found := l.catalog.Get(name)
	if !found {
		return "", fmt.Errorf("tool %q not found", name)
	}

	params := entry.Definition.Parameters
	if params == nil {
		params = make(map[string]interface{})
	}

	resp := LoadToolOutput{
		Status:      "loaded",
		Name:        entry.Name,
		Category:    entry.Category,
		Description: entry.Description,
		Parameters:  params,
		Schema:      params,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("marshaling load_tool response: %w", err)
	}

	return string(data), nil
}
