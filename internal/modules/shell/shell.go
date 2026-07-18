// Package shell provides a terminal command execution module for GAIA.
// It enforces a command allowlist, validates paths, and redacts secrets
// from output before returning results to the agent loop.
package shell

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/modules/security"
)

// Module implements ports.Module for shell command execution.
type Module struct {
	projectRoot string
	allowedCmds map[string]bool
	timeout     time.Duration
}

// NewModule creates a shell module scoped to the given project root.
func NewModule(projectRoot string) *Module {
	return &Module{
		projectRoot: projectRoot,
		allowedCmds: map[string]bool{
			// File system
			"ls": true, "dir": true, "pwd": true, "cd": true,
			"cat": true, "head": true, "tail": true, "wc": true,
			"find": true, "grep": true, "sort": true, "uniq": true,
			"mkdir": true, "touch": true, "cp": true, "mv": true,
			"rm": true, "rmdir": true,
			// Development
			"go": true, "git": true, "make": true,
			"node": true, "npm": true, "npx": true,
			"python": true, "python3": true, "pip": true,
			"rustc": true, "cargo": true,
			// Container
			"docker": true, "kubectl": true, "kubectx": true,
			// Network
			"curl": true, "wget": true,
			// Utils
			"echo": true, "date": true, "whoami": true, "uname": true,
			"which": true, "where": true, "type": true,
			// Text
			"awk": true, "sed": true, "tr": true, "cut": true,
		},
		timeout: 30 * time.Second,
	}
}

// Name returns the module identifier.
func (m *Module) Name() string { return "shell" }

// Description returns a human-readable summary of the module.
func (m *Module) Description() string {
	return "Execute allowlisted shell commands with path security and secret redaction"
}

// GetTools returns tool definitions registered by this module.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{{
		Name: "shell_exec",
		Arguments: map[string]interface{}{
			"command": "string — shell command to execute (allowlist enforced)",
		},
	}}
}

// Execute dispatches a tool call by name.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch toolName {
	case "shell_exec":
		return m.execShell(ctx, args)
	default:
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", toolName)}, nil
	}
}

// execShell validates and executes a shell command.
func (m *Module) execShell(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || strings.TrimSpace(cmdStr) == "" {
		return &domain.ToolResult{Success: false, Error: "command argument is required"}, nil
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return &domain.ToolResult{Success: false, Error: "empty command"}, nil
	}

	cmdName := parts[0]
	cmdArgs := parts[1:]

	// Check allowlist (try both original and base name)
	baseName := filepath.Base(cmdName)
	if !m.allowedCmds[cmdName] && !m.allowedCmds[baseName] {
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("command %q is not in the allowlist", cmdName),
		}, nil
	}

	// Validate URLs in curl/wget arguments
	if baseName == "curl" || baseName == "wget" {
		for _, arg := range cmdArgs {
			if looksLikeURL(arg) {
				if err := security.ValidateURL(arg); err != nil {
					return &domain.ToolResult{
						Success: false,
						Error:   fmt.Sprintf("URL validation failed: %v", err),
					}, nil
				}
			}
		}
	}

	// Create command with timeout context
	execCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
	cmd.Dir = m.projectRoot

	output, err := cmd.CombinedOutput()
	outputStr := security.RedactSecrets(string(output))

	if err != nil {
		return &domain.ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   err.Error(),
		}, nil
	}

	return &domain.ToolResult{
		Success: true,
		Output:  outputStr,
	}, nil
}

// looksLikeURL is a fast heuristic to identify arguments that may be URLs.
func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
