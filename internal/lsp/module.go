package lsp

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

// Module implements ports.Module, wrapping an LSP client's diagnostic
// and active refactoring capabilities for registration in the Brain's ToolRegistry.
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
	return "LSP diagnostics and active refactoring tools from " + m.serverName
}

// GetTools returns the tool definitions this module provides.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{
			Name: fmt.Sprintf("lsp_%s_diagnostics", m.serverName),
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Get workspace diagnostics from %s", m.serverName),
				"server":      m.serverName,
			},
		},
		{
			Name: fmt.Sprintf("lsp_%s_rename_symbol", m.serverName),
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Safely rename a symbol across the workspace using %s", m.serverName),
				"file":        "string — file path (e.g. main.go)",
				"line":        "int — 0-based line number",
				"character":   "int — 0-based character offset",
				"new_name":    "string — new identifier name",
			},
		},
		{
			Name: fmt.Sprintf("lsp_%s_find_references", m.serverName),
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Find all references to a symbol using %s", m.serverName),
				"file":        "string — file path",
				"line":        "int — 0-based line number",
				"character":   "int — 0-based character offset",
			},
		},
		{
			Name: fmt.Sprintf("lsp_%s_code_actions", m.serverName),
			Arguments: map[string]interface{}{
				"description": fmt.Sprintf("Get available quick-fixes and refactorings from %s", m.serverName),
				"file":        "string — file path",
				"start_line":  "int — 0-based start line",
				"start_char":  "int — 0-based start character",
				"end_line":    "int — 0-based end line",
				"end_char":    "int — 0-based end character",
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

	case fmt.Sprintf("lsp_%s_rename_symbol", m.serverName):
		filePath, _ := args["file"].(string)
		line, _ := getIntArg(args, "line")
		char, _ := getIntArg(args, "character")
		newName, _ := args["new_name"].(string)

		if filePath == "" || newName == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "file and new_name are required",
			}, nil
		}

		uri := PathToURI(filePath)
		pos := Position{Line: line, Character: char}

		edit, err := m.client.RenameSymbol(ctx, uri, pos, newName)
		if err != nil {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("rename failed: %v", err),
			}, nil
		}

		res, applyErr := m.client.ApplyWorkspaceEdit(ctx, edit)
		if applyErr != nil {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("apply rename edit failed: %v", applyErr),
			}, nil
		}

		return &domain.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Successfully renamed symbol to %q. Modified %d file(s) (%d total edits).", newName, len(res.ModifiedFiles), res.TotalEdits),
		}, nil

	case fmt.Sprintf("lsp_%s_find_references", m.serverName):
		filePath, _ := args["file"].(string)
		line, _ := getIntArg(args, "line")
		char, _ := getIntArg(args, "character")

		if filePath == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "file is required",
			}, nil
		}

		uri := PathToURI(filePath)
		pos := Position{Line: line, Character: char}

		locs, err := m.client.FindReferences(ctx, uri, pos, true)
		if err != nil {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("find references failed: %v", err),
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d reference(s):\n", len(locs)))
		for _, loc := range locs {
			sb.WriteString(fmt.Sprintf("- %s:%d:%d\n", URIToPath(loc.URI), loc.Range.Start.Line+1, loc.Range.Start.Character+1))
		}
		return &domain.ToolResult{
			Success: true,
			Output:  sb.String(),
		}, nil

	case fmt.Sprintf("lsp_%s_code_actions", m.serverName):
		filePath, _ := args["file"].(string)
		startLine, _ := getIntArg(args, "start_line")
		startChar, _ := getIntArg(args, "start_char")
		endLine, _ := getIntArg(args, "end_line")
		endChar, _ := getIntArg(args, "end_char")

		if filePath == "" {
			return &domain.ToolResult{
				Success: false,
				Error:   "file is required",
			}, nil
		}

		uri := PathToURI(filePath)
		rng := Range{
			Start: Position{Line: startLine, Character: startChar},
			End:   Position{Line: endLine, Character: endChar},
		}

		actions, err := m.client.CodeActions(ctx, uri, rng, CodeActionContext{})
		if err != nil {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("code actions failed: %v", err),
			}, nil
		}

		if len(actions) == 0 {
			return &domain.ToolResult{
				Success: true,
				Output:  "No code actions available for the specified range.",
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Available code action(s) (%d):\n", len(actions)))
		for _, act := range actions {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", act.Kind, act.Title))
		}
		return &domain.ToolResult{
			Success: true,
			Output:  sb.String(),
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

func getIntArg(args map[string]interface{}, key string) (int, bool) {
	val, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}
