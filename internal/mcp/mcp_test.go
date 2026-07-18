package mcp

import (
	"context"
	"testing"

	"gaia/internal/core/domain"
)

func TestSanitizeToolName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello-world", "hello-world"},
		{"hello world", "hello_world"},
		{"read_file", "read_file"},
		{"file.name", "file_name"},
		{"tool/path", "tool_path"},
		{"valid-name_123", "valid-name_123"},
	}

	for _, tc := range tests {
		result := sanitizeToolName(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeToolName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtractOriginalName(t *testing.T) {
	tests := []struct {
		prefixed   string
		serverName string
		expected   string
	}{
		{"mcp_filesystem_read_file", "filesystem", "read_file"},
		{"mcp_github_create_issue", "github", "create_issue"},
		{"mcp_db_read", "db", "read"},
		{"no_prefix_tool", "server", "no_prefix_tool"}, // fallback: no prefix match
	}

	for _, tc := range tests {
		result := extractOriginalName(tc.prefixed, tc.serverName)
		if result != tc.expected {
			t.Errorf("extractOriginalName(%q, %q) = %q, want %q",
				tc.prefixed, tc.serverName, result, tc.expected)
		}
	}
}

func TestMCPModuleName(t *testing.T) {
	config := testServerConfig("test-server")
	client := NewClient(config)
	mod := NewMCPModule(client)

	if mod.Name() != "mcp_test-server" {
		t.Errorf("expected module name 'mcp_test-server', got %q", mod.Name())
	}
	if mod.Description() != "MCP tools from test-server" {
		t.Errorf("expected description 'MCP tools from test-server', got %q", mod.Description())
	}
}

func TestMCPModuleGetToolsEmpty(t *testing.T) {
	config := testServerConfig("empty-server")
	client := NewClient(config)
	mod := NewMCPModule(client)

	tools := mod.GetTools()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools before Discover, got %d", len(tools))
	}
}

func TestMCPCallResultWrapResult(t *testing.T) {
	// Test success case
	result := &MCPCallResult{
		Content: []MCPContent{
			{Type: "text", Text: "Hello, world!"},
		},
		IsError: false,
	}

	domainResult := result.WrapResult()
	if !domainResult.Success {
		t.Error("expected success")
	}
	if domainResult.Output != "Hello, world!" {
		t.Errorf("expected output 'Hello, world!', got %q", domainResult.Output)
	}

	// Test error case
	errResult := &MCPCallResult{
		Content: []MCPContent{
			{Type: "text", Text: "Something went wrong"},
		},
		IsError: true,
	}

	domainErrResult := errResult.WrapResult()
	if domainErrResult.Success {
		t.Error("expected failure for error result")
	}
	if domainErrResult.Error != "Something went wrong" {
		t.Errorf("expected error message, got %q", domainErrResult.Error)
	}
}

func TestMCPClientNotConnected(t *testing.T) {
	config := testServerConfig("test")
	client := NewClient(config)

	if client.IsConnected() {
		t.Error("expected client to not be connected initially")
	}

	ctx := context.Background()
	_, err := client.DiscoverTools(ctx)
	if err == nil {
		t.Error("expected error when discovering tools without connection")
	}

	_, err = client.CallTool(ctx, "test", nil)
	if err == nil {
		t.Error("expected error when calling tool without connection")
	}
}

// testServerConfig creates a domain.MCPServerConfig for testing.
func testServerConfig(name string) domain.MCPServerConfig {
	return domain.MCPServerConfig{
		Name:    name,
		Command: "nonexistent-mcp-server",
		Args:    []string{},
	}
}
