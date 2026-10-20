package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompleteWord_SlashCommands(t *testing.T) {
	completed, ok := CompleteWord("/di", nil)
	if !ok || completed != "/diff " {
		t.Errorf("expected /diff , got %q (ok=%v)", completed, ok)
	}

	completed, ok = CompleteWord("/pas", nil)
	if !ok || completed != "/paste " {
		t.Errorf("expected /paste , got %q (ok=%v)", completed, ok)
	}

	completed, ok = CompleteWord("/nonexistentxyz", nil)
	if ok {
		t.Errorf("expected false for unknown command, got %q", completed)
	}
}

func TestCompleteWord_Subagents(t *testing.T) {
	subagents := []string{"implementer", "reviewer", "architect", "debugger"}

	completed, ok := CompleteWord("analyze with @rev", subagents)
	if !ok || completed != "analyze with @reviewer " {
		t.Errorf("expected 'analyze with @reviewer ', got %q (ok=%v)", completed, ok)
	}

	completed, ok = CompleteWord("@impl", subagents)
	if !ok || completed != "@implementer " {
		t.Errorf("expected '@implementer ', got %q (ok=%v)", completed, ok)
	}
}

func TestCompleteWord_FilePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-completer-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Create test files
	_ = os.WriteFile("service.go", []byte("package main"), 0644)
	_ = os.WriteFile("server.go", []byte("package main"), 0644)
	_ = os.Mkdir("internal", 0755)
	_ = os.WriteFile(filepath.Join("internal", "config.go"), []byte("package internal"), 0644)

	// Test single file prefix
	completed, ok := CompleteWord("check @servi", nil)
	if !ok || completed != "check @service.go" {
		t.Errorf("expected 'check @service.go', got %q", completed)
	}

	// Test dir prefix
	completed, ok = CompleteWord("@intern", nil)
	if !ok || completed != "@internal/" {
		t.Errorf("expected '@internal/', got %q", completed)
	}
}
