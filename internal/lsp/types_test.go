package lsp

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceEdit_NormalizedChanges(t *testing.T) {
	we := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			"file:///a.go": {
				{Range: Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 5}}, NewText: "editA"},
			},
		},
		DocumentChanges: []TextDocumentEdit{
			{
				TextDocument: OptionalVersionedTextDocumentIdentifier{URI: "file:///b.go"},
				Edits: []TextEdit{
					{Range: Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 2, Character: 5}}, NewText: "editB"},
				},
			},
		},
	}

	norm := we.NormalizedChanges()
	if len(norm) != 2 {
		t.Fatalf("expected 2 files in normalized changes, got %d", len(norm))
	}
	if len(norm["file:///a.go"]) != 1 || norm["file:///a.go"][0].NewText != "editA" {
		t.Errorf("unexpected edit for a.go: %+v", norm["file:///a.go"])
	}
	if len(norm["file:///b.go"]) != 1 || norm["file:///b.go"][0].NewText != "editB" {
		t.Errorf("unexpected edit for b.go: %+v", norm["file:///b.go"])
	}
}

func TestPosition_JSON(t *testing.T) {
	pos := Position{Line: 10, Character: 5}
	data, err := json.Marshal(pos)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Position
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded != pos {
		t.Errorf("got %+v, want %+v", decoded, pos)
	}
}

func TestRange_JSON(t *testing.T) {
	rng := Range{
		Start: Position{Line: 1, Character: 0},
		End:   Position{Line: 1, Character: 10},
	}
	data, err := json.Marshal(rng)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Range
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded != rng {
		t.Errorf("got %+v, want %+v", decoded, rng)
	}
}

func TestLocation_JSON(t *testing.T) {
	loc := Location{
		URI: "file:///path/to/file.go",
		Range: Range{
			Start: Position{Line: 2, Character: 4},
			End:   Position{Line: 2, Character: 14},
		},
	}
	data, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Location
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded != loc {
		t.Errorf("got %+v, want %+v", decoded, loc)
	}
}

func TestTextEdit_JSON(t *testing.T) {
	edit := TextEdit{
		Range: Range{
			Start: Position{Line: 5, Character: 2},
			End:   Position{Line: 5, Character: 8},
		},
		NewText: "replacement",
	}
	data, err := json.Marshal(edit)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded TextEdit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded != edit {
		t.Errorf("got %+v, want %+v", decoded, edit)
	}
}

func TestWorkspaceEdit_JSON(t *testing.T) {
	we := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			"file:///a.go": {
				{
					Range: Range{
						Start: Position{Line: 0, Character: 0},
						End:   Position{Line: 0, Character: 5},
					},
					NewText: "pkgA",
				},
			},
		},
	}
	data, err := json.Marshal(we)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded WorkspaceEdit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(decoded.Changes, we.Changes) {
		t.Errorf("got %+v, want %+v", decoded.Changes, we.Changes)
	}
}

func TestCodeActionContext_JSON(t *testing.T) {
	ctx := CodeActionContext{
		Diagnostics: []Diagnostic{
			{File: "main.go", Line: 1, Column: 1, Severity: "error", Message: "err"},
		},
		Only: []CodeActionKind{CodeActionKindQuickFix, CodeActionKindRefactor},
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CodeActionContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded.Diagnostics) != 1 || len(decoded.Only) != 2 {
		t.Fatalf("unexpected unmarshal result: %+v", decoded)
	}
}

func TestCodeAction_JSON(t *testing.T) {
	action := CodeAction{
		Title: "Refactor function",
		Kind:  CodeActionKindRefactorExtract,
		Edit: &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				"file:///main.go": {
					{
						Range: Range{
							Start: Position{Line: 10, Character: 0},
							End:   Position{Line: 12, Character: 0},
						},
						NewText: "extracted()",
					},
				},
			},
		},
		Command: &Command{
			Title:   "Format",
			Command: "editor.action.formatDocument",
		},
	}

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CodeAction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Title != action.Title || decoded.Kind != action.Kind {
		t.Errorf("mismatch: %+v vs %+v", decoded, action)
	}
	if decoded.Edit == nil || len(decoded.Edit.Changes) != 1 {
		t.Errorf("edit mismatch: %+v", decoded.Edit)
	}
	if decoded.Command == nil || decoded.Command.Command != action.Command.Command {
		t.Errorf("command mismatch: %+v", decoded.Command)
	}
}

func TestURIHelpers(t *testing.T) {
	path := "/home/user/project/main.go"
	uri := PathToURI(path)
	if uri != "file:///home/user/project/main.go" {
		t.Errorf("PathToURI(%q) = %q, want %q", path, uri, "file:///home/user/project/main.go")
	}

	back := URIToPath(uri)
	if back != path {
		t.Errorf("URIToPath(%q) = %q, want %q", uri, back, path)
	}

	// Relative path converts to absolute file:// URI
	relPath := "main.go"
	relURI := PathToURI(relPath)
	if !strings.HasPrefix(relURI, "file://") {
		t.Errorf("expected file:// scheme, got %s", relURI)
	}
	resolved := URIToPath(relURI)
	if !filepath.IsAbs(resolved) {
		t.Errorf("expected absolute path from URI, got %s", resolved)
	}
}
