package fileops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRead_InsideRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0644)

	mod := NewModule(root)
	result, err := mod.Execute(context.Background(), "file_read", map[string]interface{}{
		"path": "test.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got: %s", result.Output)
	}
}

func TestFileRead_TraversalBlocked(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_read", map[string]interface{}{
		"path": "../../etc/passwd",
	})
	if result == nil || result.Success {
		t.Fatal("expected traversal to be blocked")
	}
}

func TestFileRead_AbsoluteOutsideBlocked(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_read", map[string]interface{}{
		"path": "/etc/passwd",
	})
	if result == nil || result.Success {
		t.Fatal("expected absolute outside path to be blocked")
	}
}

func TestFileRead_MissingFile(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_read", map[string]interface{}{
		"path": "nonexistent.txt",
	})
	if result == nil || result.Success {
		t.Fatal("expected missing file to fail")
	}
}

func TestFileWrite_InsideRoot(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, err := mod.Execute(context.Background(), "file_write", map[string]interface{}{
		"path":    "output.txt",
		"content": "generated content",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}

	// Verify file was written
	data, _ := os.ReadFile(filepath.Join(root, "output.txt"))
	if string(data) != "generated content" {
		t.Errorf("unexpected file content: %s", string(data))
	}
}

func TestFileWrite_TraversalBlocked(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_write", map[string]interface{}{
		"path":    "../outside.txt",
		"content": "danger",
	})
	if result == nil || result.Success {
		t.Fatal("expected traversal to be blocked")
	}
}

func TestFileWrite_NestedDirCreated(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_write", map[string]interface{}{
		"path":    "sub/deep/nested/file.txt",
		"content": "deep",
	})
	if result == nil || !result.Success {
		t.Fatalf("expected success, got error: %v", result)
	}

	data, _ := os.ReadFile(filepath.Join(root, "sub", "deep", "nested", "file.txt"))
	if string(data) != "deep" {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestFileList_EmptyDir(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, err := mod.Execute(context.Background(), "file_list", map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Error)
	}
	// Empty dir should produce empty listing (or just header lines)
	t.Logf("output: %s", result.Output)
}

func TestFileList_WithFiles(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644)
	os.Mkdir(filepath.Join(root, "subdir"), 0755)

	mod := NewModule(root)
	result, _ := mod.Execute(context.Background(), "file_list", map[string]interface{}{
		"path": ".",
	})
	if result == nil || !result.Success {
		t.Fatalf("expected success")
	}
	if !strings.Contains(result.Output, "a.txt") {
		t.Errorf("expected a.txt in listing: %s", result.Output)
	}
	if !strings.Contains(result.Output, "subdir") {
		t.Errorf("expected subdir in listing: %s", result.Output)
	}
}

func TestFileList_TraversalBlocked(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_list", map[string]interface{}{
		"path": "../outside",
	})
	if result == nil || result.Success {
		t.Fatal("expected traversal to be blocked")
	}
}

func TestFileOps_UnknownTool(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_delete", nil)
	if result == nil || result.Success {
		t.Fatal("expected unknown tool to fail")
	}
}

func TestFileOps_MissingArgument(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_read", map[string]interface{}{})
	if result == nil || result.Success {
		t.Fatal("expected missing path argument to fail")
	}
}

func TestFileWrite_EmptyContent(t *testing.T) {
	root := t.TempDir()
	mod := NewModule(root)

	result, _ := mod.Execute(context.Background(), "file_write", map[string]interface{}{
		"path":    "empty.txt",
		"content": "",
	})
	// empty content should fail (argument validation)
	if result == nil || result.Success {
		t.Fatal("expected empty content to fail")
	}
}
