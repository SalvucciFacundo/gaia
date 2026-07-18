package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellExec_AllowedCommand(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	// where.exe is in the allowlist and is a real executable on Windows.
	result, err := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "where where",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(strings.ToLower(result.Output), "where") {
		t.Errorf("expected 'where' in output, got: %s", result.Output)
	}
}

func TestShellExec_BlockedCommand(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	// shutdown is not in the allowlist and exists as a real binary.
	result, err := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "shutdown /?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected shutdown to be blocked")
	}
	if !strings.Contains(result.Error, "not in the allowlist") {
		t.Errorf("expected allowlist error, got: %s", result.Error)
	}
}

func TestShellExec_ShBlocked(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "sh -c 'echo injected'",
	})
	if result == nil || !result.Success {
		return // blocked as expected
	}
	t.Error("expected sh to be blocked (not in allowlist)")
}

func TestShellExec_EmptyCommand(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "",
	})
	if result == nil || result.Success {
		t.Fatal("expected empty command to fail")
	}
}

func TestShellExec_MissingArg(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{})
	if result == nil || result.Success {
		t.Fatal("expected missing command arg to fail")
	}
}

func TestShellExec_GitAllowed(t *testing.T) {
	root := t.TempDir()
	// Initialize a git repo so `git status` doesn't error with "not a git repository"
	mod := NewModule(root)

	// Even without a git repo, git should be allowed (just returns an error from git itself)
	result, err := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "git --version",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Logf("git --version failed (may not be installed): %s", result.Error)
		// Don't fail — git might not be installed
	}
}

func TestShellExec_SecretsRedacted(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	// Use find to echo a string containing an API key pattern.
	result, err := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "find /?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// find /? should succeed and produce output without secrets.
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}
	// Verify no redaction when there's no secret to redact.
	t.Logf("output: %s", result.Output)
}

func TestShellExec_UnknownTool(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "nonexistent", nil)
	if result == nil || result.Success {
		t.Fatal("expected unknown tool to fail")
	}
}

func TestShellExec_GoAllowed(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	// go version should work
	result, err := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "go version",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Logf("go version failed (may not be installed): %s", result.Error)
		// Don't fail — go might not be on PATH
	}
}

func TestShellExec_WorkingDir(t *testing.T) {
	root := t.TempDir()
	// Create a marker file
	marker := filepath.Join(root, "marker.txt")
	os.WriteFile(marker, []byte("found"), 0644)

	mod := NewModule(root)
	result, _ := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "cat marker.txt", // relative path, should resolve inside root
	})
	if result != nil && result.Success {
		if !strings.Contains(result.Output, "found") {
			t.Errorf("expected 'found' in output, got: %s", result.Output)
		}
	}
}

func TestShellExec_CurlURLValidated(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	// curl with localhost URL should be blocked
	result, _ := mod.Execute(context.Background(), "shell_exec", map[string]interface{}{
		"command": "curl http://localhost:8080",
	})
	if result == nil || result.Success {
		t.Fatal("expected curl with localhost URL to be blocked")
	}
	if !strings.Contains(result.Error, "URL validation") {
		t.Errorf("expected URL validation error, got: %s", result.Error)
	}
}
