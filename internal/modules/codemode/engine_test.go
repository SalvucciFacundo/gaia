package codemode

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEngine_BasicExecution(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(tmpDir, nil)

	code := `
		msg := "Hello from Go Code Mode"
		gaia.Print(strings.ToUpper(msg))
	`

	out, err := eng.Execute(context.Background(), code)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	want := "HELLO FROM GO CODE MODE"
	if out != want {
		t.Errorf("got output %q, want %q", out, want)
	}
}

func TestEngine_FileOpsAndGrep(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(tmpDir, nil)

	code := `
		gaia.WriteFile("notes/doc.txt", "line 1\nTargetKeyword found here\nline 3")
		files := gaia.Find("doc.txt")
		if len(files) == 0 {
			gaia.Print("not found")
			return
		}
		matches := gaia.Grep("TargetKeyword", files[0])
		for _, m := range matches {
			gaia.Print(m)
		}
	`

	out, err := eng.Execute(context.Background(), code)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}

	if !strings.Contains(out, "TargetKeyword found here") {
		t.Errorf("expected grep output to contain match, got %q", out)
	}
}

func TestEngine_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(tmpDir, nil)
	eng.SetTimeout(50 * time.Millisecond)

	code := `
		for {
			time.Sleep(5 * time.Millisecond)
		}
	`

	_, err := eng.Execute(context.Background(), code)
	if err == nil {
		t.Fatal("expected timeout error for infinite loop, got nil")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention 'timed out', got %v", err)
	}
}

func TestEngine_PanicRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(tmpDir, nil)

	code := `
		panic("custom intentional panic")
	`

	_, err := eng.Execute(context.Background(), code)
	if err == nil {
		t.Fatal("expected panic error, got nil")
	}

	if !strings.Contains(err.Error(), "custom intentional panic") {
		t.Errorf("expected error to contain panic message, got %v", err)
	}
}

func TestModule_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	mod := NewModule(tmpDir, nil)

	res, err := mod.Execute(context.Background(), "execute_code", map[string]interface{}{
		"code": `gaia.Print("module executed")`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
	if res.Output != "module executed" {
		t.Errorf("got %q, want 'module executed'", res.Output)
	}
}

func TestGaiaSDK_FindError(t *testing.T) {
	// Pointing to a nonexistent directory should cause Find to panic, safely caught by engine
	eng := NewEngine("/nonexistent_path_for_testing_gaia_find_error", nil)

	code := `
		files := gaia.Find("*.go")
		gaia.Print(len(files))
	`

	_, err := eng.Execute(context.Background(), code)
	if err == nil {
		t.Fatal("expected error when walking nonexistent directory, got nil")
	}
	if !strings.Contains(err.Error(), "Find failed for pattern") {
		t.Errorf("expected error to contain 'Find failed for pattern', got %v", err)
	}
}

func TestGaiaSDK_PathTraversalJail(t *testing.T) {
	tmpDir := t.TempDir()
	eng := NewEngine(tmpDir, nil)

	// Attempt to read outside project root
	codeRead := `
		gaia.ReadFile("../../../etc/passwd")
	`
	_, err := eng.Execute(context.Background(), codeRead)
	if err == nil {
		t.Fatal("expected error on path traversal in ReadFile, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("expected path traversal error, got: %v", err)
	}

	// Attempt to write outside project root
	codeWrite := `
		gaia.WriteFile("/tmp/malicious_escape.txt", "payload")
	`
	_, err = eng.Execute(context.Background(), codeWrite)
	if err == nil {
		t.Fatal("expected error on path traversal in WriteFile, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}
