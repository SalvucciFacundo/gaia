package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gaia/internal/tools"
)

func TestLoadToolRunner_Contract(t *testing.T) {
	runner := tools.NewLoadToolRunner(sampleCatalog())

	if runner.Name() != "load_tool" {
		t.Errorf("Name() = %q, want 'load_tool'", runner.Name())
	}
	if runner.Backend() != "internal" {
		t.Errorf("Backend() = %q, want 'internal'", runner.Backend())
	}
	if !strings.Contains(runner.Description(), "Load the full schema") {
		t.Errorf("Description() = %q, want description mentioning loading schema", runner.Description())
	}
}

func TestLoadToolRunner_Run(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errSubstr   string
		wantName    string
		wantDescSub string
		checkSchema bool
	}{
		{
			name:        "load existing tool returns loaded confirmation",
			input:       `{"name": "mcp_github_create_issue"}`,
			wantName:    "mcp_github_create_issue",
			wantDescSub: "GitHub repository",
		},
		{
			name:        "load existing web tool with parameters",
			input:       `{"name": "web_search"}`,
			wantName:    "web_search",
			wantDescSub: "DuckDuckGo",
			checkSchema: true,
		},
		{
			name:      "load nonexistent tool returns error",
			input:     `{"name": "nonexistent_tool"}`,
			wantErr:   true,
			errSubstr: `tool "nonexistent_tool" not found`,
		},
		{
			name:      "load with empty name returns error",
			input:     `{"name": ""}`,
			wantErr:   true,
			errSubstr: "tool name is required",
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
			runner := tools.NewLoadToolRunner(sampleCatalog())
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

			var resp tools.LoadToolOutput
			if err := json.Unmarshal([]byte(out), &resp); err != nil {
				t.Fatalf("failed to unmarshal output %q: %v", out, err)
			}

			if resp.Status != "loaded" {
				t.Errorf("Status = %q, want 'loaded'", resp.Status)
			}
			if resp.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", resp.Name, tt.wantName)
			}
			if !strings.Contains(resp.Description, tt.wantDescSub) {
				t.Errorf("Description %q does not contain %q", resp.Description, tt.wantDescSub)
			}
			if tt.checkSchema && resp.Parameters == nil {
				t.Errorf("expected non-nil Parameters in response for %s", tt.wantName)
			}
		})
	}
}
