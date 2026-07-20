package agentsmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	// Create a temporary AGENTS.md file.
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	content := `---
rules:
  - id: no-fmt-println
    pattern: "fmt\\.Println"
    severity: error
    message: "Use structured logging instead of fmt.Println"
  - id: no-panics
    pattern: "panic\\("
    severity: error
    message: "Don't panic in production code"
forbidden:
  - "reflect package in hot paths"
conventions:
  - "Use early returns"
  - "Prefer table-driven tests"
---
# Coding Guidelines

Always write tests before implementation. Follow Go conventions.
Use context.Context as the first parameter.`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	std, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// Verify rules.
	if len(std.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(std.Rules))
	}
	if std.Rules[0].ID != "no-fmt-println" {
		t.Errorf("rule[0].ID = %q, want %q", std.Rules[0].ID, "no-fmt-println")
	}
	if std.Rules[0].Severity != "error" {
		t.Errorf("rule[0].Severity = %q, want %q", std.Rules[0].Severity, "error")
	}

	// Verify forbidden.
	if len(std.Forbidden) != 1 {
		t.Errorf("expected 1 forbidden, got %d", len(std.Forbidden))
	}

	// Verify conventions.
	if len(std.Conventions) != 2 {
		t.Errorf("expected 2 conventions, got %d", len(std.Conventions))
	}

	// Verify prose (should not include YAML frontmatter).
	if strings.Contains(std.Prose, "rules:") {
		t.Error("prose should not contain YAML frontmatter")
	}
	if !strings.Contains(std.Prose, "Always write tests") {
		t.Error("prose should contain markdown body")
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	content := `# Code Review Guidelines

This project follows these guidelines:
1. Write tests first.
2. Use context.Context as first param.
3. Don't panic in production.`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	std, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if len(std.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(std.Rules))
	}
	if len(std.Forbidden) != 0 {
		t.Errorf("expected 0 forbidden, got %d", len(std.Forbidden))
	}
	if len(std.Conventions) != 0 {
		t.Errorf("expected 0 conventions, got %d", len(std.Conventions))
	}
	if std.Prose != content {
		t.Errorf("prose should contain full content when no frontmatter")
	}
}

func TestParseEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	std, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() failed on empty file: %v", err)
	}
	if std.Prose != "" {
		t.Errorf("prose should be empty for empty file, got %q", std.Prose)
	}
}

func TestParseMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	content := `---
rules:
  - this is: not a list
    broken: [unclosed
---
# Prose after broken YAML`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	std, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() should not fail on malformed YAML: %v", err)
	}
	// Malformed YAML should result in all prose.
	if !strings.Contains(std.Prose, "Prose after broken YAML") {
		t.Error("prose should contain the markdown body even after YAML failure")
	}
}

func TestInjectIntoPrompt(t *testing.T) {
	std := &Standards{
		Rules: []Rule{
			{ID: "no-panic", Pattern: `panic\(`, Severity: "error", Message: "No panics in production code"},
		},
		Forbidden:   []string{"reflect in hot paths"},
		Conventions: []string{"Use early returns"},
		Prose:       "## Guidelines\nAlways write tests.",
	}

	original := "You are a reviewer."
	result := std.InjectIntoPrompt(original)

	if !strings.Contains(result, "Team Standards") {
		t.Error("InjectIntoPrompt should add team standards section")
	}
	if !strings.Contains(result, "no-panic") {
		t.Error("InjectIntoPrompt should include rules")
	}
	if !strings.Contains(result, "reflect in hot paths") {
		t.Error("InjectIntoPrompt should include forbidden patterns")
	}
	if !strings.Contains(result, "Use early returns") {
		t.Error("InjectIntoPrompt should include conventions")
	}
	if !strings.Contains(result, "Always write tests") {
		t.Error("InjectIntoPrompt should include prose")
	}
}

func TestInjectIntoPromptNil(t *testing.T) {
	var std *Standards
	result := std.InjectIntoPrompt("original prompt")
	if result != "original prompt" {
		t.Errorf("InjectIntoPrompt on nil Standards should return unchanged: got %q", result)
	}
}

func TestFindAndParse(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md in a parent directory.
	parentDir := filepath.Join(dir, "parent")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
conventions:
  - "Use idiomatic Go"
---
# Parent standards`
	if err := os.WriteFile(filepath.Join(parentDir, "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Look from a child directory.
	childDir := filepath.Join(parentDir, "child", "grandchild")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}

	std, err := FindAndParse(childDir)
	if err != nil {
		t.Fatalf("FindAndParse() failed: %v", err)
	}
	if len(std.Conventions) != 1 {
		t.Errorf("expected 1 convention, got %d", len(std.Conventions))
	}
}

func TestFindAndParseNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindAndParse(dir)
	if err == nil {
		t.Error("FindAndParse() should return error when AGENTS.md not found")
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/AGENTS.md")
	if err == nil {
		t.Error("Parse() should return error for missing file")
	}
}

func TestInjectIntoPromptEmptyStandards(t *testing.T) {
	std := &Standards{}
	result := std.InjectIntoPrompt("Hello")
	if !strings.Contains(result, "Team Standards") {
		t.Error("InjectIntoPrompt should add standards header even for empty standards")
	}
}
