package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModule_GetTools(t *testing.T) {
	client := NewClient(ServerConfig{Name: "gopls"})
	mod := NewModule(client)

	tools := mod.GetTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expected := []string{
		"lsp_gopls_diagnostics",
		"lsp_gopls_rename_symbol",
		"lsp_gopls_find_references",
		"lsp_gopls_code_actions",
	}

	for _, exp := range expected {
		if !toolNames[exp] {
			t.Errorf("expected tool %q in GetTools", exp)
		}
	}
}

func TestModule_Execute_Tools(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.go")
	_ = os.WriteFile(filePath, []byte("package main\nfunc OldFoo() {}\n"), 0644)

	client, cleanup := setupMockClientServer(t, func(req lspRequest) (interface{}, *lspError) {
		switch req.Method {
		case "textDocument/rename":
			return WorkspaceEdit{
				Changes: map[string][]TextEdit{
					PathToURI(filePath): {
						{
							Range: Range{
								Start: Position{Line: 1, Character: 5},
								End:   Position{Line: 1, Character: 11},
							},
							NewText: "NewFoo",
						},
					},
				},
			}, nil

		case "textDocument/references":
			return []Location{
				{
					URI: PathToURI(filePath),
					Range: Range{
						Start: Position{Line: 1, Character: 5},
						End:   Position{Line: 1, Character: 11},
					},
				},
			}, nil

		case "textDocument/codeAction":
			return []CodeAction{
				{
					Title: "Organize Imports",
					Kind:  "source.organizeImports",
				},
			}, nil

		default:
			return nil, &lspError{Code: -32601, Message: "not found"}
		}
	})
	defer cleanup()

	mod := NewModule(client)
	ctx := context.Background()

	// 1. Test Rename
	renameRes, err := mod.Execute(ctx, "lsp_mocklsp_rename_symbol", map[string]interface{}{
		"file":      filePath,
		"line":      1,
		"character": 5,
		"new_name":  "NewFoo",
	})
	if err != nil || !renameRes.Success {
		t.Fatalf("rename tool failed: %v / %v", err, renameRes)
	}
	if !strings.Contains(renameRes.Output, "Successfully renamed") {
		t.Errorf("expected success message, got %q", renameRes.Output)
	}

	// 2. Test Find References
	refRes, err := mod.Execute(ctx, "lsp_mocklsp_find_references", map[string]interface{}{
		"file":      filePath,
		"line":      1,
		"character": 5,
	})
	if err != nil || !refRes.Success {
		t.Fatalf("references tool failed: %v / %v", err, refRes)
	}
	if !strings.Contains(refRes.Output, "Found 1 reference") {
		t.Errorf("expected 1 reference, got %q", refRes.Output)
	}

	// 3. Test Code Actions
	actRes, err := mod.Execute(ctx, "lsp_mocklsp_code_actions", map[string]interface{}{
		"file":       filePath,
		"start_line": 0,
		"start_char": 0,
		"end_line":   1,
		"end_char":   10,
	})
	if err != nil || !actRes.Success {
		t.Fatalf("code actions tool failed: %v / %v", err, actRes)
	}
	if !strings.Contains(actRes.Output, "Organize Imports") {
		t.Errorf("expected Organize Imports action, got %q", actRes.Output)
	}
}
