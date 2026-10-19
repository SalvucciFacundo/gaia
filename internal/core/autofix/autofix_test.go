package autofix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsMutationTool(t *testing.T) {
	mutationTools := []string{"file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch"}
	for _, tool := range mutationTools {
		if !IsMutationTool(tool) {
			t.Errorf("expected %q to be mutation tool", tool)
		}
	}

	nonMutation := []string{"file_read", "file_tree", "search", "web_fetch", "run_command"}
	for _, tool := range nonMutation {
		if IsMutationTool(tool) {
			t.Errorf("expected %q to NOT be mutation tool", tool)
		}
	}
}

func TestExtractModifiedPaths(t *testing.T) {
	if p := ExtractModifiedPaths(map[string]interface{}{"path": "main.go"}); p != "main.go" {
		t.Errorf("expected main.go, got %q", p)
	}
	if p := ExtractModifiedPaths(map[string]interface{}{"file": "pkg/calc.go"}); p != "pkg/calc.go" {
		t.Errorf("expected pkg/calc.go, got %q", p)
	}
	if p := ExtractModifiedPaths(map[string]interface{}{"cmd": "ls"}); p != "" {
		t.Errorf("expected empty string, got %q", p)
	}
}

func TestDetectLanguageGo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-autofix-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmod\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tempDir, "pkg", "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	lang, root := DetectLanguage(subDir)
	if lang != LangGo {
		t.Errorf("expected LangGo, got %q", lang)
	}
	if root != tempDir {
		t.Errorf("expected root %q, got %q", tempDir, root)
	}
}

func TestCheckerWithValidGoProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-autofix-valid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module validmod\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mainGoPath := filepath.Join(tempDir, "main.go")
	mainContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(mainGoPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	checker := NewChecker()
	res, err := checker.Check(context.Background(), mainGoPath)
	if err != nil {
		t.Fatalf("checker error: %v", err)
	}
	if !res.Clean {
		t.Errorf("expected clean build, got errors: %s", res.Output)
	}
	if res.Language != LangGo {
		t.Errorf("expected LangGo, got %v", res.Language)
	}
}

func TestFormatAutoRepairPrompt(t *testing.T) {
	cleanRes := &CheckResult{Clean: true}
	if prompt := FormatAutoRepairPrompt(cleanRes); prompt != "" {
		t.Errorf("expected empty prompt for clean result, got %q", prompt)
	}

	brokenRes := &CheckResult{
		Language: LangGo,
		Clean:    false,
		Output:   "main.go:12:2: undefined: fmt.Printlnx",
	}
	prompt := FormatAutoRepairPrompt(brokenRes)
	if prompt == "" {
		t.Fatal("expected non-empty prompt for broken result")
	}
	if !strings.Contains(prompt, "undefined: fmt.Printlnx") {
		t.Errorf("prompt missing compiler error: %s", prompt)
	}
}

func TestCheckerWithBrokenGoProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-autofix-broken-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module brokenmod\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mainGoPath := filepath.Join(tempDir, "main.go")
	// Syntax / compile error: undefined identifier
	mainContent := "package main\n\nfunc main() {\n\tundefinedFunctionCall()\n}\n"
	if err := os.WriteFile(mainGoPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	checker := NewChecker()
	res, err := checker.Check(context.Background(), mainGoPath)
	if err != nil {
		t.Fatalf("checker error: %v", err)
	}
	if res.Clean {
		t.Error("expected broken build for undefined function call, got clean")
	}
	if !strings.Contains(res.Output, "undefinedFunctionCall") && !strings.Contains(res.Output, "undefined") {
		t.Errorf("expected error output to mention undefined function, got: %s", res.Output)
	}
}
