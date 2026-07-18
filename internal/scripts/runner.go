// Package scripts provides pre-agent script injection for GAIA.
// Scripts defined in config (scripts.pre_run) are executed with the
// project's working directory. Their stdout is captured and injected
// as a system context message before the user message.
//
// The [SILENT] prefix in stdout suppresses notification to the user.
package scripts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config holds script injection settings.
type Config struct {
	PreRun  []string `yaml:"pre_run"`  // script paths relative to project root
	Timeout int      `yaml:"timeout"`  // seconds, default 30
}

// Runner executes pre-agent scripts and captures their output.
type Runner struct {
	cfg     Config
	workDir string
}

// NewRunner creates a script runner with the given work directory.
func NewRunner(cfg Config, workDir string) *Runner {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}
	return &Runner{
		cfg:     cfg,
		workDir: workDir,
	}
}

// ContextResult holds the output of running pre-run scripts.
type ContextResult struct {
	SystemMessage string   // stdout content to inject as system context
	Warnings      []string // non-fatal warnings
}

// Run executes all configured pre-run scripts and returns the aggregated
// context. Scripts that fail are logged as warnings, not errors — pre-run
// scripts are advisory.
func (r *Runner) Run(ctx context.Context) (*ContextResult, error) {
	if len(r.cfg.PreRun) == 0 {
		return &ContextResult{}, nil
	}

	result := &ContextResult{}
	var messages []string

	for _, scriptPath := range r.cfg.PreRun {
		// Resolve and validate the script path.
		resolved, err := r.validatePath(scriptPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: %v", scriptPath, err))
			continue
		}

		if !r.isExecutable(resolved) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: not executable", scriptPath))
			continue
		}

		// Run with timeout.
		timeout := time.Duration(r.cfg.Timeout) * time.Second
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, resolved)
		cmd.Dir = r.workDir
		output, err := cmd.CombinedOutput()

		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("script %s: %v", scriptPath, err))
			if len(output) > 0 {
				messages = append(messages, string(output))
			}
			continue
		}

		messages = append(messages, string(output))
	}

	result.SystemMessage = r.formatContext(messages)
	return result, nil
}

// validatePath checks that the script path is safe and exists.
// Rejects: directories, paths with traversal, non-allowed extensions.
func (r *Runner) validatePath(path string) (string, error) {
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal detected")
	}

	// Resolve to absolute path within workDir.
	fullPath := filepath.Join(r.workDir, path)
	cleaned := filepath.Clean(fullPath)

	if !strings.HasPrefix(cleaned, filepath.Clean(r.workDir)) {
		return "", fmt.Errorf("path escapes working directory")
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("path not found: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a script")
	}

	return cleaned, nil
}

// isExecutable checks if a file has an allowed extension for execution.
// Allowed: .sh, .py, .go, .bat, .ps1
func (r *Runner) isExecutable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".sh", ".py", ".go", ".bat", ".ps1", ".cmd":
		return true
	}
	return false
}

// formatContext builds the system context message from script outputs.
// Lines starting with [SILENT] are excluded from the output.
func (r *Runner) formatContext(outputs []string) string {
	var parts []string
	for _, out := range outputs {
		lines := strings.Split(out, "\n")
		var filtered []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "[SILENT]") {
				continue // Suppressed.
			}
			filtered = append(filtered, line)
		}
		if len(filtered) > 0 {
			parts = append(parts, strings.Join(filtered, "\n"))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "[Pre-agent scripts output]\n" + strings.Join(parts, "\n---\n")
}
