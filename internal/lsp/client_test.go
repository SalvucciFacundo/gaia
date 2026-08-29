package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMockClientServer sets up a Client connected to an in-memory mock LSP server.
func setupMockClientServer(t *testing.T, handler func(req lspRequest) (interface{}, *lspError)) (*Client, func()) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()

	client := NewClientWithIO(clientToServerW, serverToClientR, ServerConfig{
		Name:      "mocklsp",
		Workspace: "/workspace",
	})

	serverReader := bufio.NewReader(clientToServerR)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			var contentLength int
			for {
				line, err := serverReader.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					_, _ = fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				}
			}

			if contentLength <= 0 {
				return
			}

			body := make([]byte, contentLength)
			if _, err := io.ReadFull(serverReader, body); err != nil {
				return
			}

			var req lspRequest
			if err := json.Unmarshal(body, &req); err != nil {
				continue
			}

			if req.ID == 0 {
				continue // notification
			}

			res, lspErr := handler(req)
			resp := lspResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  res,
				Error:   lspErr,
			}
			respBytes, _ := json.Marshal(resp)
			frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(respBytes), string(respBytes))
			if _, err := serverToClientW.Write([]byte(frame)); err != nil {
				return
			}
		}
	}()

	cleanup := func() {
		_ = client.Close()
		_ = clientToServerR.Close()
		_ = clientToServerW.Close()
		_ = serverToClientR.Close()
		_ = serverToClientW.Close()
		<-done
	}

	return client, cleanup
}

func TestClient_RenameSymbol(t *testing.T) {
	client, cleanup := setupMockClientServer(t, func(req lspRequest) (interface{}, *lspError) {
		if req.Method != "textDocument/rename" {
			return nil, &lspError{Code: -32601, Message: "method not found"}
		}

		paramsBytes, _ := json.Marshal(req.Params)
		var params RenameParams
		_ = json.Unmarshal(paramsBytes, &params)

		if params.NewName == "invalid" {
			return nil, &lspError{Code: 1, Message: "cannot rename to keyword"}
		}

		return WorkspaceEdit{
			Changes: map[string][]TextEdit{
				params.TextDocument.URI: {
					{
						Range: Range{
							Start: Position{Line: 10, Character: 5},
							End:   Position{Line: 10, Character: 12},
						},
						NewText: params.NewName,
					},
				},
			},
		}, nil
	})
	defer cleanup()

	ctx := context.Background()
	pos := Position{Line: 10, Character: 5}
	edit, err := client.RenameSymbol(ctx, "file:///workspace/main.go", pos, "RenamedSymbol")
	if err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}

	if edit == nil || len(edit.Changes) != 1 {
		t.Fatalf("unexpected edit response: %+v", edit)
	}
	edits := edit.Changes["file:///workspace/main.go"]
	if len(edits) != 1 || edits[0].NewText != "RenamedSymbol" {
		t.Errorf("unexpected edit content: %+v", edits)
	}

	// Test error scenario
	_, err = client.RenameSymbol(ctx, "file:///workspace/main.go", pos, "invalid")
	if err == nil {
		t.Error("expected error for invalid rename, got nil")
	}
}

func TestClient_FindReferences(t *testing.T) {
	client, cleanup := setupMockClientServer(t, func(req lspRequest) (interface{}, *lspError) {
		if req.Method != "textDocument/references" {
			return nil, &lspError{Code: -32601, Message: "method not found"}
		}

		return []Location{
			{
				URI: "file:///workspace/main.go",
				Range: Range{
					Start: Position{Line: 5, Character: 1},
					End:   Position{Line: 5, Character: 8},
				},
			},
			{
				URI: "file:///workspace/service.go",
				Range: Range{
					Start: Position{Line: 20, Character: 10},
					End:   Position{Line: 20, Character: 17},
				},
			},
		}, nil
	})
	defer cleanup()

	ctx := context.Background()
	pos := Position{Line: 5, Character: 1}
	locs, err := client.FindReferences(ctx, "file:///workspace/main.go", pos, true)
	if err != nil {
		t.Fatalf("FindReferences failed: %v", err)
	}

	if len(locs) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locs))
	}
	if locs[0].URI != "file:///workspace/main.go" || locs[1].URI != "file:///workspace/service.go" {
		t.Errorf("unexpected locations: %+v", locs)
	}
}

func TestClient_CodeActions(t *testing.T) {
	client, cleanup := setupMockClientServer(t, func(req lspRequest) (interface{}, *lspError) {
		if req.Method != "textDocument/codeAction" {
			return nil, &lspError{Code: -32601, Message: "method not found"}
		}

		return []CodeAction{
			{
				Title: "Remove unused import",
				Kind:  CodeActionKindQuickFix,
				Edit: &WorkspaceEdit{
					Changes: map[string][]TextEdit{
						"file:///workspace/main.go": {
							{
								Range: Range{
									Start: Position{Line: 3, Character: 0},
									End:   Position{Line: 4, Character: 0},
								},
								NewText: "",
							},
						},
					},
				},
			},
		}, nil
	})
	defer cleanup()

	ctx := context.Background()
	rng := Range{
		Start: Position{Line: 3, Character: 0},
		End:   Position{Line: 3, Character: 10},
	}
	actionCtx := CodeActionContext{
		Diagnostics: []Diagnostic{
			{File: "/workspace/main.go", Line: 4, Column: 1, Severity: "warning", Message: "unused import"},
		},
	}

	actions, err := client.CodeActions(ctx, "file:///workspace/main.go", rng, actionCtx)
	if err != nil {
		t.Fatalf("CodeActions failed: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Title != "Remove unused import" || actions[0].Kind != CodeActionKindQuickFix {
		t.Errorf("unexpected action: %+v", actions[0])
	}
}

func TestClient_ApplyWorkspaceEdit(t *testing.T) {
	client := NewClient(ServerConfig{Name: "test"})
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.go")
	_ = os.WriteFile(f, []byte("func Old() {}"), 0644)

	we := &WorkspaceEdit{
		Changes: map[string][]TextEdit{
			PathToURI(f): {
				{
					Range: Range{
						Start: Position{Line: 0, Character: 5},
						End:   Position{Line: 0, Character: 8},
					},
					NewText: "New",
				},
			},
		},
	}

	res, err := client.ApplyWorkspaceEdit(context.Background(), we)
	if err != nil {
		t.Fatalf("ApplyWorkspaceEdit failed: %v", err)
	}
	if res.TotalEdits != 1 {
		t.Errorf("expected 1 edit, got %d", res.TotalEdits)
	}

	content, _ := os.ReadFile(f)
	if string(content) != "func New() {}" {
		t.Errorf("got %q, want 'func New() {}'", string(content))
	}
}
