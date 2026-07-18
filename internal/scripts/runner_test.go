package scripts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunnerEmptyConfig(t *testing.T) {
	r := NewRunner(Config{}, t.TempDir())
	result, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SystemMessage != "" {
		t.Errorf("expected empty system message, got %q", result.SystemMessage)
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	r := NewRunner(Config{}, t.TempDir())
	_, err := r.validatePath("../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidatePath_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRunner(Config{}, tmpDir)
	_, err := r.validatePath(".")
	if err == nil {
		t.Error("expected error for directory path")
	}
}

func TestIsExecutable(t *testing.T) {
	r := NewRunner(Config{}, t.TempDir())

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"shell script", "script.sh", true},
		{"python script", "script.py", true},
		{"go file", "script.go", true},
		{"batch file", "script.bat", true},
		{"powershell", "script.ps1", true},
		{"markdown (rejected)", "README.md", false},
		{"no extension", "justfile", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.isExecutable(tt.path)
			if got != tt.want {
				t.Errorf("isExecutable(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRunnerExecutesScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script execution test uses shell scripts, not available on Windows by default")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")

	script := `#!/bin/sh
echo "Hello from script"
echo "[SILENT] This should be suppressed"
echo "Normal output"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		PreRun:  []string{"test.sh"},
		Timeout: 10,
	}
	r := NewRunner(cfg, tmpDir)

	result, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SystemMessage == "" {
		t.Error("expected non-empty system message")
	}

	// Check that SILENT lines are suppressed.
	if contains(result.SystemMessage, "SILENT") {
		t.Errorf("SILENT lines should be suppressed, got: %s", result.SystemMessage)
	}
}

func TestFormatContext_SilentPattern(t *testing.T) {
	r := NewRunner(Config{}, t.TempDir())

	outputs := []string{
		"Visible line\n[SILENT] Hidden line\nAlso visible",
	}

	result := r.formatContext(outputs)
	if result == "" {
		t.Error("expected non-empty formatted context")
	}
	if contains(result, "[SILENT]") {
		t.Error("SILENT lines should be stripped")
	}
	if !contains(result, "Visible line") {
		t.Error("visible lines should be preserved")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
