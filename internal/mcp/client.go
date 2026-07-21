package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"gaia/internal/core/domain"
)

// Client connects to an MCP server via stdio transport and provides
// tool discovery and invocation.
type Client struct {
	config     domain.MCPServerConfig
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	scanner    *bufio.Scanner
	mu         sync.Mutex
	nextID     int
	serverInfo *ServerInfo
	connected  bool
}

// NewClient creates an MCP client for a given server config.
// The connection is established lazily on first Connect() call.
func NewClient(config domain.MCPServerConfig) *Client {
	return &Client{
		config: config,
		nextID: 1,
	}
}

// Connect starts the MCP server process and performs the initialize handshake.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Start the MCP server process via stdio
	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Set environment variables (inherit parent + custom env)
	env := os.Environ()
	if c.config.Env != nil {
		for k, v := range c.config.Env {
			env = append(env, k+"="+v)
		}
	}
	// Inject OAuth access token for authenticated MCP servers
	if c.config.AccessToken != "" {
		env = append(env, "MCP_ACCESS_TOKEN="+c.config.AccessToken)
		env = append(env, "ACCESS_TOKEN="+c.config.AccessToken)
	}
	if c.config.TokenURL != "" {
		env = append(env, "MCP_TOKEN_URL="+c.config.TokenURL)
	}
	c.cmd.Env = env

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp start: %w", err)
	}

	c.scanner = bufio.NewScanner(c.stdout)

	// Initialize handshake
	initReq := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "0.1.0",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "gaia",
				"version": "1.0.0",
			},
		},
	}
	c.nextID++

	resp, err := c.sendRequest(initReq)
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}

	// Parse server info
	if resultBytes, err := json.Marshal(resp.Result); err == nil {
		var info ServerInfo
		if err := json.Unmarshal(resultBytes, &info); err == nil {
			c.serverInfo = &info
		}
	}

	c.connected = true
	return nil
}

// DiscoverTools calls tools/list and returns available MCP tools.
func (c *Client) DiscoverTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("mcp: not connected")
	}

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}

	var toolsResult GetToolsResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal tools result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &toolsResult); err != nil {
		return nil, fmt.Errorf("unmarshal tools result: %w", err)
	}

	return toolsResult.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPCallResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("mcp: not connected")
	}

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  "tools/call",
		Params: MCPCallParams{
			Name:      name,
			Arguments: args,
		},
	}
	c.nextID++

	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/call: %w", err)
	}

	if resp.Error != nil {
		return &MCPCallResult{
			IsError: true,
			Content: []MCPContent{{Type: "text", Text: resp.Error.Message}},
		}, nil
	}

	var callResult MCPCallResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal call result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		return nil, fmt.Errorf("unmarshal call result: %w", err)
	}

	return &callResult, nil
}

// Close terminates the MCP server process.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// ServerName returns the configured server name.
func (c *Client) ServerName() string {
	return c.config.Name
}

// sendRequest writes a JSON-RPC request and reads the response.
func (c *Client) sendRequest(req MCPJSONRPCRequest) (*MCPJSONRPCResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write request with newline delimiter (JSON-RPC over stdio)
	if _, err := c.stdin.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("mcp: unexpected EOF")
	}

	var resp MCPJSONRPCResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}


