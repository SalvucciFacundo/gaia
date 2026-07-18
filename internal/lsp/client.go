// Package lsp provides an LSP (Language Server Protocol) client for GAIA.
// It connects to language servers (gopls, pylsp, etc.) via stdio transport
// and exposes diagnostics, completions, and hover as agent tools.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

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
	scanner *bufio.Scanner
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

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("lsp start: %w", err)
	}

	c.scanner = bufio.NewScanner(c.stdout)
	c.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1 MB buffer for large responses.

	// Initialize handshake.
	initReq := lspRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"processId":         nil,
			"rootUri":           fmt.Sprintf("file://%s", c.cfg.Workspace),
			"capabilities":      map[string]interface{}{},
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

	// Send workspace/diagnostic refresh request.
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

	return parseDiagnostics(resp.Result), nil
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
	notif := lspRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	_, err := c.send(notif)
	return err
}

// send writes an LSP message using Content-Length framing and reads the response.
func (c *Client) send(msg lspRequest) (*lspResponse, error) {
	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// LSP framing: Content-Length: N\r\n\r\n<json>
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(reqBytes), string(reqBytes))
	if _, err := c.stdin.Write([]byte(frame)); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read Content-Length header.
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("lsp: unexpected EOF reading header")
	}

	header := c.scanner.Text()
	var contentLength int
	if _, err := fmt.Sscanf(header, "Content-Length: %d", &contentLength); err != nil {
		return nil, fmt.Errorf("lsp: bad header: %s", header)
	}

	// Read blank line separator.
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("lsp: unexpected EOF after header")
	}

	// Read body.
	body := make([]byte, contentLength)
	n, err := io.ReadFull(c.stdout, body)
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
