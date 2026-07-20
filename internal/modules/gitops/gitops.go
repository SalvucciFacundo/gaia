// Package gitops provides read-only git operations for GAIA.
// All commands run in the project root directory; arbitrary -C flags
// and path escapes are blocked.
package gitops

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/modules/security"
)

// Module implements ports.Module for read-only git operations.
type Module struct {
	projectRoot string
	timeout     time.Duration
}

// NewModule creates a gitops module scoped to the given project root.
func NewModule(projectRoot string) *Module {
	return &Module{
		projectRoot: projectRoot,
		timeout:     15 * time.Second,
	}
}

// Name returns the module identifier.
func (m *Module) Name() string { return "gitops" }

// Description returns a human-readable summary of the module.
func (m *Module) Description() string {
	return "Read-only git operations (status, log, diff) with path security"
}

// GetTools returns tool definitions registered by this module.
func (m *Module) GetTools() []domain.ToolCall {
	return []domain.ToolCall{
		{
			Name:      "git_status",
			Arguments: map[string]interface{}{},
		},
		{
			Name: "git_log",
			Arguments: map[string]interface{}{
				"count": "number — commits to show (default 10)",
			},
		},
		{
			Name: "git_diff",
			Arguments: map[string]interface{}{
				"staged": "bool — show staged diff instead of unstaged",
			},
		},
	}
}

// Execute dispatches a tool call by name.
func (m *Module) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	switch toolName {
	case "git_status":
		return m.gitStatus(ctx)
	case "git_log":
		return m.gitLog(ctx, args)
	case "git_diff":
		return m.gitDiff(ctx, args)
	default:
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", toolName)}, nil
	}
}

// gitStatus runs `git status --porcelain`.
func (m *Module) gitStatus(ctx context.Context) (*domain.ToolResult, error) {
	return m.runGit(ctx, "status", "--porcelain")
}

// gitLog runs `git log --oneline -n <count>`.
func (m *Module) gitLog(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
	count := 10
	if v, ok := args["count"]; ok {
		switch n := v.(type) {
		case float64:
			count = int(n)
		case int:
			count = n
		case string:
			if parsed, err := strconv.Atoi(n); err == nil {
				count = parsed
			}
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	return m.runGit(ctx, "log", "--oneline", "-n", strconv.Itoa(count))
}

// gitDiff runs `git diff` (or `git diff --staged`).
func (m *Module) gitDiff(ctx context.Context, args map[string]interface{}) (*domain.ToolResult, error) {
	gitArgs := []string{"diff"}
	if staged, _ := args["staged"].(bool); staged {
		gitArgs = append(gitArgs, "--staged")
	}
	return m.runGit(ctx, gitArgs...)
}

// runGit executes a git command with the project root as working directory.
// It rejects any argument that attempts to change the git directory (-C) or
// escape the project root via path traversal.
func (m *Module) runGit(ctx context.Context, args ...string) (*domain.ToolResult, error) {
	// Block -C flag (changes git directory)
	for i, a := range args {
		if a == "-C" || strings.HasPrefix(a, "-C=") {
			return &domain.ToolResult{
				Success: false,
				Error:   "git -C flag is blocked for security",
			}, nil
		}
		// Block --git-dir and --work-tree overrides
		if a == "--git-dir" || a == "--work-tree" {
			// Also block the next argument (the path value)
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git %s flag is blocked for security", a),
			}, nil
		}
		if strings.HasPrefix(a, "--git-dir=") || strings.HasPrefix(a, "--work-tree=") {
			return &domain.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git %s flag is blocked for security", a),
			}, nil
		}
		// Validate any path arguments against traversal
		if looksLikePath(a) {
			if _, err := security.ValidatePath(m.projectRoot, a); err != nil {
				return &domain.ToolResult{
					Success: false,
					Error:   fmt.Sprintf("path security check: %v", err),
				}, nil
			}
		}
		_ = i // suppress unused warning
	}

	execCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "git", args...)
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

// looksLikePath is a heuristic to detect path-like arguments.
func looksLikePath(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return strings.Contains(s, string(filepath.Separator)) ||
		strings.Contains(s, "/") ||
		strings.HasSuffix(s, ".go") ||
		strings.HasSuffix(s, ".md") ||
		strings.HasSuffix(s, ".txt")
}
