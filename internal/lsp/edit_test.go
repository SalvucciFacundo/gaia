package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSortTextEditsReverse(t *testing.T) {
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 1, Character: 0},
				End:   Position{Line: 1, Character: 5},
			},
			NewText: "first",
		},
		{
			Range: Range{
				Start: Position{Line: 10, Character: 2},
				End:   Position{Line: 10, Character: 8},
			},
			NewText: "second",
		},
		{
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 10},
			},
			NewText: "middle",
		},
		{
			Range: Range{
				Start: Position{Line: 10, Character: 12},
				End:   Position{Line: 10, Character: 20},
			},
			NewText: "third",
		},
	}

	SortTextEditsReverse(edits)

	if edits[0].NewText != "third" {
		t.Errorf("expected edits[0] to be 'third', got %q", edits[0].NewText)
	}
	if edits[1].NewText != "second" {
		t.Errorf("expected edits[1] to be 'second', got %q", edits[1].NewText)
	}
	if edits[2].NewText != "middle" {
		t.Errorf("expected edits[2] to be 'middle', got %q", edits[2].NewText)
	}
	if edits[3].NewText != "first" {
		t.Errorf("expected edits[3] to be 'first', got %q", edits[3].NewText)
	}
}

func TestApplyTextEdits_SingleLine(t *testing.T) {
	content := "hello world from GAIA"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 6},
				End:   Position{Line: 0, Character: 11},
			},
			NewText: "universe",
		},
	}

	result, err := ApplyTextEdits(content, edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "hello universe from GAIA"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApplyTextEdits_MultiLineReverse(t *testing.T) {
	content := "line 0: foo\nline 1: bar\nline 2: baz"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 8},
				End:   Position{Line: 0, Character: 11},
			},
			NewText: "FOO",
		},
		{
			Range: Range{
				Start: Position{Line: 2, Character: 8},
				End:   Position{Line: 2, Character: 11},
			},
			NewText: "BAZ",
		},
	}

	result, err := ApplyTextEdits(content, edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "line 0: FOO\nline 1: bar\nline 2: BAZ"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApplyTextEdits_OverlappingError(t *testing.T) {
	content := "abcdefghij"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 2},
				End:   Position{Line: 0, Character: 6},
			},
			NewText: "123",
		},
		{
			Range: Range{
				Start: Position{Line: 0, Character: 4},
				End:   Position{Line: 0, Character: 8},
			},
			NewText: "456",
		},
	}

	_, err := ApplyTextEdits(content, edits)
	if err == nil {
		t.Fatal("expected error for overlapping edits, got nil")
	}
}

func TestApplyWorkspaceEdit_MultiFileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "fileA.go")
	fileB := filepath.Join(tmpDir, "fileB.go")

	if err := os.WriteFile(fileA, []byte("package main\n\nfunc OldName() {}\n"), 0644); err != nil {
		t.Fatalf("failed to create fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("package main\n\nfunc Call() {\n\tOldName()\n}\n"), 0644); err != nil {
		t.Fatalf("failed to create fileB: %v", err)
	}

	we := &WorkspaceEdit{
		Changes: map[string][]TextEdit{
			PathToURI(fileA): {
				{
					Range: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 12},
					},
					NewText: "NewName",
				},
			},
			PathToURI(fileB): {
				{
					Range: Range{
						Start: Position{Line: 3, Character: 1},
						End:   Position{Line: 3, Character: 8},
					},
					NewText: "NewName",
				},
			},
		},
	}

	applier := NewWorkspaceEditApplier()
	res, err := applier.Apply(we)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if res.TotalEdits != 2 {
		t.Errorf("expected 2 total edits, got %d", res.TotalEdits)
	}
	if len(res.ModifiedFiles) != 2 {
		t.Errorf("expected 2 modified files, got %d", len(res.ModifiedFiles))
	}

	contentA, _ := os.ReadFile(fileA)
	expectedA := "package main\n\nfunc NewName() {}\n"
	if string(contentA) != expectedA {
		t.Errorf("fileA mismatch: got %q, want %q", string(contentA), expectedA)
	}

	contentB, _ := os.ReadFile(fileB)
	expectedB := "package main\n\nfunc Call() {\n\tNewName()\n}\n"
	if string(contentB) != expectedB {
		t.Errorf("fileB mismatch: got %q, want %q", string(contentB), expectedB)
	}
}

func TestApplyTextEdits_InsertAndDelete(t *testing.T) {
	content := "func Foo(a int) int {\n\treturn a\n}\n"
	edits := []TextEdit{
		// Insertion at line 1 col 0: add comment
		{
			Range: Range{
				Start: Position{Line: 1, Character: 0},
				End:   Position{Line: 1, Character: 0},
			},
			NewText: "\t// added comment\n",
		},
		// Replace second "int" at line 0 col 16..19
		{
			Range: Range{
				Start: Position{Line: 0, Character: 16},
				End:   Position{Line: 0, Character: 19},
			},
			NewText: "string",
		},
	}

	result, err := ApplyTextEdits(content, edits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "func Foo(a int) string {\n\t// added comment\n\treturn a\n}\n"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApplyTextEdits_OutOfBounds(t *testing.T) {
	content := "one line"
	// Line out of bounds
	_, err := ApplyTextEdits(content, []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 5, Character: 0},
				End:   Position{Line: 5, Character: 1},
			},
			NewText: "x",
		},
	})
	if err == nil {
		t.Error("expected error for line out of bounds")
	}

	// Character out of bounds
	_, err = ApplyTextEdits(content, []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 50},
				End:   Position{Line: 0, Character: 51},
			},
			NewText: "x",
		},
	})
	if err == nil {
		t.Error("expected error for character out of bounds")
	}
}

func TestApplyWorkspaceEdit_RollbackOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "fileA.go")
	blockerFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockerFile, []byte("i am a file not a directory"), 0644); err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}
	// Trying to write inside blocker as a directory will fail with ENOTDIR
	fileInvalid := filepath.Join(blockerFile, "fileB.go")

	originalA := "package main\n\nfunc KeepMe() {}\n"
	if err := os.WriteFile(fileA, []byte(originalA), 0644); err != nil {
		t.Fatalf("failed to create fileA: %v", err)
	}

	we := &WorkspaceEdit{
		Changes: map[string][]TextEdit{
			PathToURI(fileA): {
				{
					Range: Range{
						Start: Position{Line: 2, Character: 5},
						End:   Position{Line: 2, Character: 11},
					},
					NewText: "Changed",
				},
			},
			PathToURI(fileInvalid): {
				{
					Range: Range{
						Start: Position{Line: 0, Character: 0},
						End:   Position{Line: 0, Character: 0},
					},
					NewText: "new file content",
				},
			},
		},
	}

	applier := NewWorkspaceEditApplier()
	_, err := applier.Apply(we)
	if err == nil {
		t.Fatal("expected error due to invalid file path, got nil")
	}

	// fileA should remain untouched (rolled back)
	contentA, _ := os.ReadFile(fileA)
	if string(contentA) != originalA {
		t.Errorf("fileA was not rolled back: got %q, want %q", string(contentA), originalA)
	}
}

