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
