package core_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchitecture_CoreIsolation ensures internal/core does not import outer layers:
// adapters, agent, commands, modules, tools, gateway, review, server, or database/sql drivers.
func TestArchitecture_CoreIsolation(t *testing.T) {
	forbiddenSubstrings := []string{
		"gaia/internal/adapters",
		"gaia/internal/agent",
		"gaia/internal/commands",
		"gaia/internal/modules",
		"gaia/internal/tools",
		"gaia/internal/gateway",
		"gaia/internal/review",
		"gaia/internal/server",
		"gaia/internal/subagents",
		"gaia/internal/mneme",
		"gaia/cmd",
		"database/sql",
	}

	coreDir := "."
	fset := token.NewFileSet()

	err := filepath.Walk(coreDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("failed to parse imports for %s: %v", path, parseErr)
		}

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenSubstrings {
				if strings.HasPrefix(importPath, forbidden) {
					t.Errorf("Architecture violation in %s: imports forbidden package %q", path, importPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to scan internal/core: %v", err)
	}
}
