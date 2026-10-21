package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core"
	"gaia/internal/core/ports"
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

// AutofixVerifier verifies if mutations build cleanly or introduce compiler/linter errors.
// It implements ports.MutationVerifier.
type AutofixVerifier struct {
	Timeout time.Duration
}

var _ ports.MutationVerifier = (*AutofixVerifier)(nil)

// NewAutofixVerifier creates an AutofixVerifier with default sensible timeouts.
func NewAutofixVerifier() *AutofixVerifier {
	return &AutofixVerifier{
		Timeout: 15 * time.Second,
	}
}

// IsMutationTool returns true if the tool modifies source files.
func (v *AutofixVerifier) IsMutationTool(toolName string) bool {
	switch strings.ToLower(toolName) {
	case "file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch":
		return true
	default:
		return false
	}
}

// ExtractModifiedPaths extracts file paths from arguments of mutation tools.
func (v *AutofixVerifier) ExtractModifiedPaths(args map[string]interface{}) string {
	if p, ok := args["path"].(string); ok && p != "" {
		return p
	}
	if f, ok := args["file"].(string); ok && f != "" {
		return f
	}
	return ""
}

// DetectProjectLanguage inspects the directory hierarchy upwards from startPath using core.DetectLanguage.
func DetectProjectLanguage(startPath string) (Language, string) {
	if startPath == "" {
		startPath = "."
	}
	abs, err := filepath.Abs(startPath)
	if err != nil {
		abs = startPath
	}

	fi, err := os.Stat(abs)
	if err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}

	curr := abs
	for {
		langStr := core.DetectLanguage(curr)
		if langStr != "" {
			return Language(langStr), curr
		}

		parent := filepath.Dir(curr)
		if parent == curr || parent == "" {
			break
		}
		curr = parent
	}

	return LangUnknown, abs
}

// Check runs the compiler/linter check for the project located at or above targetPath.
func (v *AutofixVerifier) Check(ctx context.Context, targetPath string) (*CheckResult, error) {
	lang, root := DetectProjectLanguage(targetPath)
	if lang == LangUnknown {
		return &CheckResult{Language: LangUnknown, Clean: true}, nil
	}

	timeout := v.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	var cmd *exec.Cmd

	switch lang {
	case LangGo:
		cmd = exec.CommandContext(ctx, "go", "vet", "./...")
	case LangRust:
		cmd = exec.CommandContext(ctx, "cargo", "check", "--message-format=short")
	case LangTypeScript:
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

// VerifyMutations checks modified paths across all unique roots and returns a repair prompt if any check fails.
func (v *AutofixVerifier) VerifyMutations(ctx context.Context, modifiedPaths []string) (string, error) {
	checkedRoots := make(map[string]bool)
	var prompts []string

	for _, path := range modifiedPaths {
		_, root := DetectProjectLanguage(path)
		if checkedRoots[root] {
			continue
		}
		checkedRoots[root] = true

		res, err := v.Check(ctx, path)
		if err == nil && res != nil && !res.Clean {
			if prompt := FormatAutoRepairPrompt(res); prompt != "" {
				prompts = append(prompts, prompt)
			}
		}
	}

	if len(prompts) == 0 {
		return "", nil
	}
	return strings.Join(prompts, "\n\n"), nil
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
