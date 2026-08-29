// Package lsp provides an LSP (Language Server Protocol) client for GAIA.
// It connects to language servers (gopls, pylsp, etc.) via stdio transport
// and exposes diagnostics, completions, and refactoring as agent tools.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// RefactoringClient defines refactoring capabilities supported by the LSP client.
type RefactoringClient interface {
	RenameSymbol(ctx context.Context, uri string, pos Position, newName string) (*WorkspaceEdit, error)
	FindReferences(ctx context.Context, uri string, pos Position, includeDecl bool) ([]Location, error)
	CodeActions(ctx context.Context, uri string, rng Range, actionCtx CodeActionContext) ([]CodeAction, error)
	ApplyWorkspaceEdit(ctx context.Context, edit *WorkspaceEdit) (*ApplyResult, error)
}

// ServerConfig defines settings for an LSP server connection.
type ServerConfig struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Workspace string   `json:"workspace"`
}

// Client connects to an LSP server via stdio and provides
// diagnostic and analysis capabilities.
type Client struct {
	cfg     ServerConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	reader  *bufio.Reader
	mu      sync.Mutex
	nextID  int
}

// NewClient creates a new LSP client for the given server config.
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		cfg:    cfg,
		nextID: 1,
	}
}

// NewClientWithIO creates an LSP client with injected io streams for testing or custom transports.
func NewClientWithIO(stdin io.WriteCloser, stdout io.ReadCloser, cfg ServerConfig) *Client {
	c := &Client{
		cfg:    cfg,
		stdin:  stdin,
		stdout: stdout,
		nextID: 1,
		reader: bufio.NewReader(stdout),
	}
	return c
}

// Connect starts the LSP server process and performs the initialize handshake.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cmd = exec.CommandContext(ctx, c.cfg.Command, c.cfg.Args...)
	if c.cfg.Workspace != "" {
		c.cmd.Dir = c.cfg.Workspace
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("lsp stdin: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("lsp stdout: %w", err)
	}

	c.reader = bufio.NewReader(c.stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("lsp start: %w", err)
	}

	// Initialize handshake.
	initReq := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"processId":    nil,
			"rootUri":      fmt.Sprintf("file://%s", c.cfg.Workspace),
			"capabilities": map[string]interface{}{},
		},
	}
	c.nextID++

	_, err = c.sendRequest(initReq)
	if err != nil {
		return fmt.Errorf("lsp initialize: %w", err)
	}

	// Send initialized notification.
	c.sendNotification("initialized", map[string]interface{}{})

	return nil
}

// Diagnostics requests diagnostics for the workspace and returns them parsed.
func (c *Client) Diagnostics(ctx context.Context) ([]Diagnostic, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "workspace/diagnostic",
		Params: map[string]interface{}{
			"identifier": "workspace",
		},
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("lsp diagnostics: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("lsp diagnostics error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	return parseDiagnostics(resp.Result), nil
}

// RenameSymbol sends a textDocument/rename request to rename the symbol at pos.
func (c *Client) RenameSymbol(ctx context.Context, uri string, pos Position, newName string) (*WorkspaceEdit, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
		NewName:      newName,
	}

	req := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "textDocument/rename",
		Params:  params,
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("lsp rename: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("lsp rename error (%d): %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result == nil {
		return &WorkspaceEdit{Changes: make(map[string][]TextEdit)}, nil
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("lsp rename marshal result: %w", err)
	}

	var edit WorkspaceEdit
	if err := json.Unmarshal(data, &edit); err != nil {
		return nil, fmt.Errorf("lsp rename unmarshal edit: %w", err)
	}

	return &edit, nil
}

// FindReferences sends a textDocument/references request to find all references to the symbol at pos.
func (c *Client) FindReferences(ctx context.Context, uri string, pos Position, includeDecl bool) ([]Location, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
		Context:      ReferenceContext{IncludeDeclaration: includeDecl},
	}

	req := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "textDocument/references",
		Params:  params,
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("lsp references: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("lsp references error (%d): %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result == nil {
		return []Location{}, nil
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("lsp references marshal result: %w", err)
	}

	var locs []Location
	if err := json.Unmarshal(data, &locs); err != nil {
		return nil, fmt.Errorf("lsp references unmarshal locations: %w", err)
	}

	return locs, nil
}

// CodeActions sends a textDocument/codeAction request to retrieve code actions for range and context.
func (c *Client) CodeActions(ctx context.Context, uri string, rng Range, actionCtx CodeActionContext) ([]CodeAction, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	params := CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Context:      actionCtx,
	}

	req := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "textDocument/codeAction",
		Params:  params,
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("lsp code actions: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("lsp code actions error (%d): %s", resp.Error.Code, resp.Error.Message)
	}
	if resp.Result == nil {
		return []CodeAction{}, nil
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("lsp code actions marshal result: %w", err)
	}

	var actions []CodeAction
	if err := json.Unmarshal(data, &actions); err != nil {
		return nil, fmt.Errorf("lsp code actions unmarshal: %w", err)
	}

	return actions, nil
}

// ApplyWorkspaceEdit safely applies a WorkspaceEdit using the WorkspaceEditApplier.
func (c *Client) ApplyWorkspaceEdit(ctx context.Context, edit *WorkspaceEdit) (*ApplyResult, error) {
	applier := NewWorkspaceEditApplier()
	return applier.Apply(edit)
}

// Close terminates the LSP server process.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.sendNotification("shutdown", nil)
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// sendRequest writes a JSON-RPC request and reads the Content-Length delimited response.
func (c *Client) sendRequest(req lspRequest) (*lspResponse, error) {
	return c.send(req)
}

// sendNotification sends an LSP notification (no response expected).
func (c *Client) sendNotification(method string, params interface{}) error {
	if c.stdin == nil {
		return fmt.Errorf("lsp client not connected")
	}

	notif := lspRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(reqBytes), string(reqBytes))
	_, err = c.stdin.Write([]byte(frame))
	return err
}

// send writes an LSP message using Content-Length framing and reads the response.
func (c *Client) send(msg lspRequest) (*lspResponse, error) {
	if c.stdin == nil || c.stdout == nil {
		return nil, fmt.Errorf("lsp client not connected")
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// LSP framing: Content-Length: N\r\n\r\n<json>
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(reqBytes), string(reqBytes))
	if _, err := c.stdin.Write([]byte(frame)); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read headers and body using Content-Length framing.
	var contentLength int
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("lsp read header: %w", err)
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
		return nil, fmt.Errorf("lsp: missing or invalid Content-Length")
	}

	// Read body using the same buffered reader.
	body := make([]byte, contentLength)
	n, err := io.ReadFull(c.reader, body)
	if err != nil {
		return nil, fmt.Errorf("lsp: read body: expected %d bytes, got %d: %w", contentLength, n, err)
	}

	// Skip responses without an ID (notifications/responses to unsent requests).
	if msg.ID == 0 {
		return &lspResponse{}, nil
	}

	var resp lspResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("lsp: unmarshal response: %w", err)
	}

	return &resp, nil
}

// lspRequest is a JSON-RPC 2.0 request.
type lspRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// lspResponse is a JSON-RPC 2.0 response.
type lspResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *lspError   `json:"error,omitempty"`
}

// lspError represents a JSON-RPC error.
type lspError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
