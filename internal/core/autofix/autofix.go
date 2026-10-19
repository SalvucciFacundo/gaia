package autofix

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Language represents a detected programming language ecosystem.
type Language string

const (
	LangGo         Language = "go"
	LangRust       Language = "rust"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangPython     Language = "python"
	LangUnknown    Language = "unknown"
)

// CheckResult contains the result of a compilation or lint check.
type CheckResult struct {
	Language Language
	Clean    bool
	Output   string
	Duration time.Duration
}

// Checker verifies if the codebase builds cleanly or has syntax/type errors.
type Checker struct {
	Timeout time.Duration
}

// NewChecker creates a Checker with default sensible timeouts.
func NewChecker() *Checker {
	return &Checker{
		Timeout: 15 * time.Second,
	}
}

// IsMutationTool returns true if the tool modifies source files.
func IsMutationTool(toolName string) bool {
	switch strings.ToLower(toolName) {
	case "file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch":
		return true
	default:
		return false
	}
}

// ExtractModifiedPaths extracts file paths from arguments of mutation tools.
func ExtractModifiedPaths(args map[string]interface{}) string {
	if p, ok := args["path"].(string); ok && p != "" {
		return p
	}
	if f, ok := args["file"].(string); ok && f != "" {
		return f
	}
	return ""
}

// DetectLanguage inspects the directory hierarchy upwards from startPath to find project markers.
func DetectLanguage(startPath string) (Language, string) {
	if startPath == "" {
		startPath = "."
	}
	abs, err := filepath.Abs(startPath)
	if err != nil {
		abs = startPath
	}

	// If startPath is a file, take its directory
	fi, err := os.Stat(abs)
	if err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}

	curr := abs
	for {
		if fileExists(filepath.Join(curr, "go.mod")) {
			return LangGo, curr
		}
		if fileExists(filepath.Join(curr, "Cargo.toml")) {
			return LangRust, curr
		}
		if fileExists(filepath.Join(curr, "tsconfig.json")) {
			return LangTypeScript, curr
		}
		if fileExists(filepath.Join(curr, "package.json")) {
			return LangJavaScript, curr
		}
		if fileExists(filepath.Join(curr, "pyproject.toml")) || fileExists(filepath.Join(curr, "requirements.txt")) {
			return LangPython, curr
		}

		parent := filepath.Dir(curr)
		if parent == curr || parent == "" {
			break
		}
		curr = parent
	}

	return LangUnknown, abs
}

// Check runs the appropriate compiler/linter check for the project located at or above path.
func (c *Checker) Check(ctx context.Context, targetPath string) (*CheckResult, error) {
	lang, root := DetectLanguage(targetPath)
	if lang == LangUnknown {
		return &CheckResult{Language: LangUnknown, Clean: true}, nil
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	var cmd *exec.Cmd

	switch lang {
	case LangGo:
		// Fast syntax & type check across packages
		cmd = exec.CommandContext(ctx, "go", "vet", "./...")
	case LangRust:
		cmd = exec.CommandContext(ctx, "cargo", "check", "--message-format=short")
	case LangTypeScript:
		// Check if tsc exists
		if _, err := exec.LookPath("tsc"); err == nil {
			cmd = exec.CommandContext(ctx, "tsc", "--noEmit")
		} else if _, err := exec.LookPath("npx"); err == nil {
			cmd = exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
		} else {
			return &CheckResult{Language: lang, Clean: true}, nil
		}
	case LangJavaScript:
		return &CheckResult{Language: lang, Clean: true}, nil
	case LangPython:
		if _, err := exec.LookPath("ruff"); err == nil {
			cmd = exec.CommandContext(ctx, "ruff", "check", ".")
		} else {
			return &CheckResult{Language: lang, Clean: true}, nil
		}
	default:
		return &CheckResult{Language: lang, Clean: true}, nil
	}

	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	duration := time.Since(start)

	outputStr := strings.TrimSpace(string(out))
	if err != nil {
		// Non-zero exit code means compile / lint error
		return &CheckResult{
			Language: lang,
			Clean:    false,
			Output:   outputStr,
			Duration: duration,
		}, nil
	}

	return &CheckResult{
		Language: lang,
		Clean:    true,
		Output:   outputStr,
		Duration: duration,
	}, nil
}

// FormatAutoRepairPrompt formats compiler error output into an actionable system instruction.
func FormatAutoRepairPrompt(res *CheckResult) string {
	if res == nil || res.Clean {
		return ""
	}

	return fmt.Sprintf(
		"[AUTO-REPAIR FEEDBACK]\n"+
			"The recent file modifications introduced compilation/linter errors in the %s project:\n\n"+
			"%s\n\n"+
			"CRITICAL: Fix these compilation/syntax errors immediately using file_patch or file_write before concluding your response.",
		res.Language,
		res.Output,
	)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
