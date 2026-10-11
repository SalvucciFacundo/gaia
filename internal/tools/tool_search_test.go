package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/tools"
)

func sampleCatalog() *tools.ToolCatalog {
	cat := tools.NewToolCatalog()
	cat.Register(tools.CatalogEntry{
		Name:        "web_search",
		Category:    "web",
		Description: "Search the web using DuckDuckGo, Brave, Tavily, or SearXNG.",
		Definition: domain.ToolDef{
			Name:        "web_search",
			Description: "Search the web using DuckDuckGo, Brave, Tavily, or SearXNG.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
	})
	cat.Register(tools.CatalogEntry{
		Name:        "web_fetch",
		Category:    "web",
		Description: "Extract readable text content and markdown from a webpage URL.",
		Definition: domain.ToolDef{
			Name:        "web_fetch",
			Description: "Extract readable text content and markdown from a webpage URL.",
		},
	})
	cat.Register(tools.CatalogEntry{
		Name:        "shell-local",
		Category:    "shell",
		Description: "Run a shell command on the local backend.",
		Definition: domain.ToolDef{
			Name:        "shell-local",
			Description: "Run a shell command on the local backend.",
		},
	})
	cat.Register(tools.CatalogEntry{
		Name:        "mcp_github_create_issue",
		Category:    "mcp",
		Description: "Create a new issue on a GitHub repository.",
		Definition: domain.ToolDef{
			Name:        "mcp_github_create_issue",
			Description: "Create a new issue on a GitHub repository.",
		},
	})
	cat.Register(tools.CatalogEntry{
		Name:        "mcp_slack_post_message",
		Category:    "mcp",
		Description: "Post a message to a Slack channel.",
		Definition: domain.ToolDef{
			Name:        "mcp_slack_post_message",
			Description: "Post a message to a Slack channel.",
		},
	})
	cat.Register(tools.CatalogEntry{
		Name:        "delegate_task",
		Category:    "subagents",
		Description: "Delegate a subtask to an isolated worker subagent.",
		Definition: domain.ToolDef{
			Name:        "delegate_task",
			Description: "Delegate a subtask to an isolated worker subagent.",
		},
	})
	return cat
}

func TestToolSearchRunner_Contract(t *testing.T) {
	runner := tools.NewToolSearchRunner(sampleCatalog())

	if runner.Name() != "tool_search" {
		t.Errorf("Name() = %q, want 'tool_search'", runner.Name())
	}
	if runner.Backend() != "internal" {
		t.Errorf("Backend() = %q, want 'internal'", runner.Backend())
	}
	if !strings.Contains(runner.Description(), "Search for available tools") {
		t.Errorf("Description() = %q, want description mentioning searching tools", runner.Description())
	}
}

func TestToolSearchRunner_Search(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errSubstr   string
		wantMatches []string
	}{
		{
			name:        "search by keyword matches name",
			input:       `{"query": "github"}`,
			wantMatches: []string{"mcp_github_create_issue"},
		},
		{
			name:        "search by keyword matches description",
			input:       `{"query": "markdown"}`,
			wantMatches: []string{"web_fetch"},
		},
		{
			name:        "search by category web",
			input:       `{"category": "web"}`,
			wantMatches: []string{"web_fetch", "web_search"},
		},
		{
			name:        "search by category mcp",
			input:       `{"category": "mcp"}`,
			wantMatches: []string{"mcp_github_create_issue", "mcp_slack_post_message"},
		},
		{
			name:        "search by query and category filter",
			input:       `{"query": "slack", "category": "mcp"}`,
			wantMatches: []string{"mcp_slack_post_message"},
		},
		{
			name:        "search with no matches returns empty json array",
			input:       `{"query": "nonexistent_tool_query"}`,
			wantMatches: []string{},
		},
		{
			name:      "empty query and category returns error",
			input:     `{}`,
			wantErr:   true,
			errSubstr: "at least one search parameter",
		},
		{
			name:      "invalid json input returns error",
			input:     `{not json`,
			wantErr:   true,
			errSubstr: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := tools.NewToolSearchRunner(sampleCatalog())
			out, err := runner.Run(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Run(%s) expected error, got nil", tt.input)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Run(%s) unexpected error = %v", tt.input, err)
			}

			var results []tools.ToolMetadata
			if err := json.Unmarshal([]byte(out), &results); err != nil {
				t.Fatalf("failed to unmarshal output %q: %v", out, err)
			}

			if len(results) != len(tt.wantMatches) {
				t.Fatalf("got %d matches, want %d (results: %+v)", len(results), len(tt.wantMatches), results)
			}

			for i, wantName := range tt.wantMatches {
				if results[i].Name != wantName {
					t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, wantName)
				}
			}
		})
	}
}
