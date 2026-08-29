package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Position in a text document expressed as zero-based line and character offset.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document expressed as (zero-based) start and end positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource, such as a line inside a text file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextEdit represents a text edit applicable to a text document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// OptionalVersionedTextDocumentIdentifier is an identifier to denote a specific version of a text document.
type OptionalVersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version *int   `json:"version,omitempty"`
}

// TextDocumentEdit describes textual changes on a single text document.
type TextDocumentEdit struct {
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                              `json:"edits"`
}

// WorkspaceEdit represents changes to many resources managed in the workspace.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []TextDocumentEdit    `json:"documentChanges,omitempty"`
}

// NormalizedChanges returns a map of document URI to TextEdits, combining
// both Changes and DocumentChanges into a single map.
func (we *WorkspaceEdit) NormalizedChanges() map[string][]TextEdit {
	if we == nil {
		return nil
	}
	res := make(map[string][]TextEdit)
	for uri, edits := range we.Changes {
		res[uri] = append(res[uri], edits...)
	}
	for _, docEdit := range we.DocumentChanges {
		uri := docEdit.TextDocument.URI
		res[uri] = append(res[uri], docEdit.Edits...)
	}
	return res
}

// CodeActionKind represents the kind of a code action.
type CodeActionKind string

const (
	CodeActionKindEmpty                 CodeActionKind = ""
	CodeActionKindQuickFix              CodeActionKind = "quickfix"
	CodeActionKindRefactor              CodeActionKind = "refactor"
	CodeActionKindRefactorExtract       CodeActionKind = "refactor.extract"
	CodeActionKindRefactorInline        CodeActionKind = "refactor.inline"
	CodeActionKindRefactorRewrite       CodeActionKind = "refactor.rewrite"
	CodeActionKindSource                CodeActionKind = "source"
	CodeActionKindSourceOrganizeImports CodeActionKind = "source.organizeImports"
	CodeActionKindSourceFixAll          CodeActionKind = "source.fixAll"
)

// CodeActionContext contains additional diagnostic information about the context in which
// a code action is run.
type CodeActionContext struct {
	Diagnostics []Diagnostic     `json:"diagnostics"`
	Only        []CodeActionKind `json:"only,omitempty"`
	TriggerKind int              `json:"triggerKind,omitempty"`
}

// Command represents a reference to a command.
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// CodeActionDisabled indicates that the code action is currently disabled.
type CodeActionDisabled struct {
	Reason string `json:"reason"`
}

// CodeAction represents a change and/or command to be executed.
type CodeAction struct {
	Title       string              `json:"title"`
	Kind        CodeActionKind      `json:"kind,omitempty"`
	Diagnostics []Diagnostic        `json:"diagnostics,omitempty"`
	IsPreferred *bool               `json:"isPreferred,omitempty"`
	Disabled    *CodeActionDisabled `json:"disabled,omitempty"`
	Edit        *WorkspaceEdit      `json:"edit,omitempty"`
	Command     *Command            `json:"command,omitempty"`
	Data        interface{}         `json:"data,omitempty"`
}

// TextDocumentIdentifier identifies a text document using a URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// RenameParams are the parameters sent for a textDocument/rename request.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// ReferenceContext defines context for a textDocument/references request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams are the parameters sent for a textDocument/references request.
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

// CodeActionParams are the parameters sent for a textDocument/codeAction request.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// PathToURI converts a filesystem path to an LSP file:// URI.
func PathToURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// Normalize path separators to forward slashes for URI formatting
	slashPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	u := url.URL{
		Scheme: "file",
		Path:   slashPath,
	}
	return u.String()
}

// URIToPath converts an LSP file:// URI to a filesystem path.
func URIToPath(rawURI string) string {
	if !strings.HasPrefix(rawURI, "file://") {
		return rawURI
	}
	u, err := url.Parse(rawURI)
	if err != nil {
		return strings.TrimPrefix(rawURI, "file://")
	}
	path := u.Path
	// On Windows, /C:/path -> C:/path
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}
