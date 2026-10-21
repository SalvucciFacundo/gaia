package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutofixVerifier_IsMutationTool(t *testing.T) {
	v := NewAutofixVerifier()
	mutationTools := []string{"file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch"}
	for _, tool := range mutationTools {
		if !v.IsMutationTool(tool) {
			t.Errorf("expected %q to be mutation tool", tool)
		}
	}

	nonMutation := []string{"file_read", "file_tree", "search", "web_fetch", "run_command"}
	for _, tool := range nonMutation {
		if v.IsMutationTool(tool) {
			t.Errorf("expected %q to NOT be mutation tool", tool)
		}
	}
}

func TestAutofixVerifier_ExtractModifiedPaths(t *testing.T) {
	v := NewAutofixVerifier()
	if p := v.ExtractModifiedPaths(map[string]interface{}{"path": "main.go"}); p != "main.go" {
		t.Errorf("expected main.go, got %q", p)
	}
	if p := v.ExtractModifiedPaths(map[string]interface{}{"file": "pkg/calc.go"}); p != "pkg/calc.go" {
		t.Errorf("expected pkg/calc.go, got %q", p)
	}
	if p := v.ExtractModifiedPaths(map[string]interface{}{"cmd": "ls"}); p != "" {
		t.Errorf("expected empty string, got %q", p)
	}
}

func TestDetectProjectLanguageGo(t *testing.T) {
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

	lang, root := DetectProjectLanguage(subDir)
	if lang != LangGo {
		t.Errorf("expected LangGo, got %q", lang)
	}
	if root != tempDir {
		t.Errorf("expected root %q, got %q", tempDir, root)
	}
}

func TestAutofixVerifier_ValidGoProject(t *testing.T) {
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

	v := NewAutofixVerifier()
	res, err := v.Check(context.Background(), mainGoPath)
	if err != nil {
		t.Fatalf("checker error: %v", err)
	}
	if !res.Clean {
		t.Errorf("expected clean build, got errors: %s", res.Output)
	}
	if res.Language != LangGo {
		t.Errorf("expected LangGo, got %v", res.Language)
	}

	prompt, err := v.VerifyMutations(context.Background(), []string{mainGoPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt != "" {
		t.Errorf("expected empty prompt for clean project, got %q", prompt)
	}
}

func TestAutofixVerifier_BrokenGoProject(t *testing.T) {
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
	mainContent := "package main\n\nfunc main() {\n\tundefinedFunctionCall()\n}\n"
	if err := os.WriteFile(mainGoPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	v := NewAutofixVerifier()
	res, err := v.Check(context.Background(), mainGoPath)
	if err != nil {
		t.Fatalf("checker error: %v", err)
	}
	if res.Clean {
		t.Error("expected broken build for undefined function call, got clean")
	}
	if !strings.Contains(res.Output, "undefinedFunctionCall") && !strings.Contains(res.Output, "undefined") {
		t.Errorf("expected error output to mention undefined function, got: %s", res.Output)
	}

	prompt, err := v.VerifyMutations(context.Background(), []string{mainGoPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "[AUTO-REPAIR FEEDBACK]") {
		t.Errorf("expected prompt to contain [AUTO-REPAIR FEEDBACK], got: %s", prompt)
	}
}
