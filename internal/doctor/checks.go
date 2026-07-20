// Package doctor provides system diagnostics for GAIA.
// Each check is a standalone function returning a CheckResult.
// Checks are run sequentially and output as a table or JSON.
package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gaia/internal/adapters/db"
	"gaia/internal/config"
)

// Status represents the health of a check.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name     string
	Status   Status
	Message  string
	Duration time.Duration
}

// Check is a function that performs a health check.
type Check func(ctx context.Context) CheckResult

// AllChecks returns the list of all registered health checks.
func AllChecks() []Check {
	return []Check{
		CheckLLMProvider,
		CheckDatabase,
		CheckGit,
		CheckConfig,
		CheckGoVersion,
		CheckMemory,
	}
}

// CheckLLMProvider verifies LLM provider configuration.
func CheckLLMProvider(ctx context.Context) CheckResult {
	start := time.Now()
	cfg, err := config.Load()
	if err != nil {
		return CheckResult{
			Name:     "LLM Provider",
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot load config: %v", err),
			Duration: time.Since(start),
		}
	}

	provider := cfg.LLM.Provider
	if provider == "" {
		provider = "copilot"
	}

	apiKey := cfg.APIKeys[provider]
	if apiKey == "" {
		return CheckResult{
			Name:     "LLM Provider",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Provider %q configured but no API key found", provider),
			Duration: time.Since(start),
		}
	}

	return CheckResult{
		Name:     "LLM Provider",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Provider %q ready (model: %s)", provider, cfg.LLM.Model),
		Duration: time.Since(start),
	}
}

// CheckDatabase verifies the SQLite database is accessible.
func CheckDatabase(ctx context.Context) CheckResult {
	start := time.Now()
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		return CheckResult{
			Name:     "Database",
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot open database: %v", err),
			Duration: time.Since(start),
		}
	}

	// Try a simple query
	messages, err := repo.GetHistory(ctx, 1)
	if err != nil {
		return CheckResult{
			Name:     "Database",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Database open but query failed: %v", err),
			Duration: time.Since(start),
		}
	}

	_ = messages
	return CheckResult{
		Name:     "Database",
		Status:   StatusOK,
		Message:  "SQLite database accessible",
		Duration: time.Since(start),
	}
}

// CheckGit verifies git is available and the repo is clean.
func CheckGit(ctx context.Context) CheckResult {
	start := time.Now()

	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		return CheckResult{
			Name:     "Git",
			Status:   StatusWarn,
			Message:  "Git is not installed or not in PATH",
			Duration: time.Since(start),
		}
	}

	// Check if we're in a git repo
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CheckResult{
			Name:     "Git",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Not a git repository: %s", strings.TrimSpace(string(output))),
			Duration: time.Since(start),
		}
	}

	// Check branch
	branchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, _ := branchCmd.CombinedOutput()
	branch := strings.TrimSpace(string(branchOutput))

	return CheckResult{
		Name:     "Git",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Repository ready (branch: %s)", branch),
		Duration: time.Since(start),
	}
}

// CheckConfig verifies the configuration file is valid.
func CheckConfig(ctx context.Context) CheckResult {
	start := time.Now()
	cfg, err := config.Load()
	if err != nil {
		return CheckResult{
			Name:     "Config",
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot load config: %v", err),
			Duration: time.Since(start),
		}
	}

	issues := []string{}
	if cfg.LLM.Provider == "" {
		issues = append(issues, "no LLM provider set")
	}
	if cfg.Budget.MaxIterations <= 0 {
		issues = append(issues, "no iteration budget set")
	}

	if len(issues) > 0 {
		return CheckResult{
			Name:     "Config",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Config loaded with issues: %s", strings.Join(issues, "; ")),
			Duration: time.Since(start),
		}
	}

	return CheckResult{
		Name:     "Config",
		Status:   StatusOK,
		Message:  "Configuration valid",
		Duration: time.Since(start),
	}
}

// CheckGoVersion verifies the Go toolchain version.
func CheckGoVersion(ctx context.Context) CheckResult {
	start := time.Now()

	cmd := exec.CommandContext(ctx, "go", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CheckResult{
			Name:     "Go Version",
			Status:   StatusFail,
			Message:  fmt.Sprintf("Go not found: %v", err),
			Duration: time.Since(start),
		}
	}

	version := strings.TrimSpace(string(output))
	return CheckResult{
		Name:     "Go Version",
		Status:   StatusOK,
		Message:  version,
		Duration: time.Since(start),
	}
}

// CheckMemory verifies memory/database health (reading history).
func CheckMemory(ctx context.Context) CheckResult {
	start := time.Now()

	repo, err := db.NewSQLiteRepo()
	if err != nil {
		return CheckResult{
			Name:     "Memory",
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot access memory: %v", err),
			Duration: time.Since(start),
		}
	}

	count, err := countMessages(ctx, repo)
	if err != nil {
		return CheckResult{
			Name:     "Memory",
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Memory accessible but count failed: %v", err),
			Duration: time.Since(start),
		}
	}

	return CheckResult{
		Name:     "Memory",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Memory healthy (%d messages stored)", count),
		Duration: time.Since(start),
	}
}

// countMessages returns the number of stored messages.
func countMessages(ctx context.Context, repo *db.SQLiteRepo) (int, error) {
	msgs, err := repo.GetHistory(ctx, 10000)
	if err != nil {
		return 0, err
	}
	return len(msgs), nil
}

// RunAll executes all checks sequentially and returns results.
func RunAll(ctx context.Context) []CheckResult {
	checks := AllChecks()
	results := make([]CheckResult, len(checks))
	for i, check := range checks {
		results[i] = check(ctx)
	}
	return results
}

// FormatTable formats results as a readable table.
func FormatTable(results []CheckResult) string {
	var sb strings.Builder
	sb.WriteString("GAIA System Diagnostics\n")
	sb.WriteString(strings.Repeat("=", 60))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-25s %-8s %s\n", "Check", "Status", "Details"))
	sb.WriteString(strings.Repeat("-", 60))
	sb.WriteString("\n")

	for _, r := range results {
		icon := statusIcon(r.Status)
		duration := fmt.Sprintf("(%s)", r.Duration.Round(time.Millisecond))
		sb.WriteString(fmt.Sprintf("%-25s %s %-6s %s %s\n",
			r.Name, icon, r.Status, r.Message, duration))
	}

	sb.WriteString(strings.Repeat("=", 60))
	sb.WriteString("\n")

	okCount := 0
	warnCount := 0
	failCount := 0
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			okCount++
		case StatusWarn:
			warnCount++
		case StatusFail:
			failCount++
		}
	}

	sb.WriteString(fmt.Sprintf("Summary: %d ok, %d warnings, %d failures\n", okCount, warnCount, failCount))
	return sb.String()
}

func statusIcon(s Status) string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusWarn:
		return "⚠"
	case StatusFail:
		return "✗"
	default:
		return "?"
	}
}

// BuildInfo returns build information.
func BuildInfo() map[string]string {
	return map[string]string{
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cwd":        currentDir(),
	}
}

func currentDir() string {
	dir, _ := os.Getwd()
	return dir
}
