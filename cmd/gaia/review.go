package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/adapters/tui"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/review/gates"
)

// handleReviewCLI implements the "gaia review" subcommand family.
// Usage: gaia review <command> [flags]
func handleReviewCLI(args []string) {
	if len(args) == 0 {
		printReviewUsage()
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "start":
		handleReviewStart(cmdArgs)
	case "status":
		handleReviewStatus()
	case "validate":
		handleReviewValidate(cmdArgs)
	case "list":
		handleReviewList()
	case "install-hooks":
		handleReviewInstallHooks()
	default:
		fmt.Fprintf(os.Stderr, "Unknown review command: %s\n", cmd)
		printReviewUsage()
		os.Exit(1)
	}
}

func printReviewUsage() {
	fmt.Println("Usage: gaia review <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start           Start a review of staged files")
	fmt.Println("  status          Show current review status")
	fmt.Println("  validate        Validate a review gate (used by git hooks)")
	fmt.Println("  list            List all reviews")
	fmt.Println("  install-hooks   Install git hook scripts (pre-commit, pre-push)")
	fmt.Println()
	fmt.Println("Start flags:")
	fmt.Println("  --files <list>   Comma-separated file list to review")
	fmt.Println("  --lens <name>    Force a specific lens (risk, resilience, readability, reliability)")
	fmt.Println("  --judgment-day   Run Judgment Day adversarial review")
	fmt.Println()
	fmt.Println("Validate flags:")
	fmt.Println("  --gate <name>    Gate to validate: pre-commit, pre-push, pre-pr")
	fmt.Println("  --files <list>   Comma-separated file list to validate")
	fmt.Println()
	fmt.Println("Status flags:")
	fmt.Println("  --change <name>  Specific change name to check")
	fmt.Println()
	fmt.Println("List flags:")
	fmt.Println("  --state <state>  Filter by state (approved, escalated, invalidated)")
}

// handleReviewStart starts a new review for the given files.
// Usage: gaia review start [--files <list>] [--lens <name>] [--judgment-day]
func handleReviewStart(args []string) {
	fs := flag.NewFlagSet("review-start", flag.ExitOnError)
	filesFlag := fs.String("files", "", "Comma-separated list of files to review")
	lensFlag := fs.String("lens", "", "Force a specific lens (overrides auto-selection)")
	judgmentDay := fs.Bool("judgment-day", false, "Run Judgment Day adversarial review")
	fs.Parse(args)

	var files []string
	if *filesFlag != "" {
		for _, f := range strings.Split(*filesFlag, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				files = append(files, f)
			}
		}
	} else {
		// Default: get staged files.
		var err error
		files, err = getStagedFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting staged files: %v\n", err)
			os.Exit(1)
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No files to review. Stage files with 'git add' first.")
		os.Exit(1)
	}

	fmt.Printf("Starting review of %d file(s)...\n", len(files))
	for _, f := range files {
		fmt.Printf("  - %s\n", f)
	}

	// Build engine and LLM provider.
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, _ := config.Load()
	provider, err := buildLLMProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing LLM: %v\n", err)
		os.Exit(1)
	}

	// Use agent-based review via the reviewer subagent.
	_ = judgmentDay // TODO: PR 3 wires Judgment Day
	_ = lensFlag
	_ = provider
	_ = projectRoot

	fmt.Println()
	fmt.Println("Review engine initialized. To run a full review, use the agent:")
	fmt.Println("  gaia exec 'review these files: " + strings.Join(files, ", ") + "'")
	fmt.Println()
	fmt.Println("Tip: Use 'gaia review validate --gate pre-commit' to check review status.")
}

// handleReviewStatus shows the current review status.
// Usage: gaia review status [--change <name>]
func handleReviewStatus() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store := gates.NewFSReceiptStore(projectRoot)

	// Try to find the latest receipt.
	summaries, err := store.ListReceipts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing receipts: %v\n", err)
		os.Exit(1)
	}

	if len(summaries) == 0 {
		fmt.Println("No review receipts found. Run 'gaia review start' to begin a review.")
		return
	}

	fmt.Printf("Found %d receipt(s):\n\n", len(summaries))
	for _, s := range summaries {
		stateIcon := "?"
		switch s.State {
		case "approved":
			stateIcon = "✓"
		case "escalated":
			stateIcon = "⚠"
		case "invalidated":
			stateIcon = "✗"
		default:
			stateIcon = "…"
		}
		fmt.Printf("  %s  %-30s  state=%-12s  risk=%s  date=%s\n",
			stateIcon, s.ChangeName, s.State, s.RiskLevel, s.CreatedAt.Format("2006-01-02 15:04"))
	}
}

// handleReviewValidate validates a review gate.
// Usage: gaia review validate --gate <pre-commit|pre-push|pre-pr> [--files <list>]
// Exits 0 on pass, 1 on fail.
func handleReviewValidate(args []string) {
	fs := flag.NewFlagSet("review-validate", flag.ExitOnError)
	gateName := fs.String("gate", "", "Gate to validate: pre-commit, pre-push, pre-pr")
	filesFlag := fs.String("files", "", "Comma-separated list of files to validate")
	fs.Parse(args)

	if *gateName == "" {
		fmt.Fprintln(os.Stderr, "Error: --gate flag is required (pre-commit, pre-push, pre-pr)")
		os.Exit(1)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var files []string
	if *filesFlag != "" {
		for _, f := range strings.Split(*filesFlag, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				files = append(files, f)
			}
		}
	} else {
		files, err = getStagedFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting files: %v\n", err)
			os.Exit(1)
		}
	}

	store := gates.NewFSReceiptStore(projectRoot)

	var gate *gates.Gate
	switch gates.GateName(*gateName) {
	case gates.PreCommitGate:
		gate = gates.GatePreCommit
	case gates.PrePushGate:
		gate = gates.GatePrePush
	case gates.PrePRGate:
		gate = gates.GatePrePR
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown gate %q (use pre-commit, pre-push, or pre-pr)\n", *gateName)
		os.Exit(1)
	}

	result, err := gate.Validate(projectRoot, files, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if result.Passed {
		fmt.Printf("[GAIA] %s gate PASSED: %s\n", *gateName, result.Reason)
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "[GAIA] %s gate FAILED: %s\n", *gateName, result.Reason)
	os.Exit(1)
}

// handleReviewList lists all review receipts.
// Usage: gaia review list [--state <state>]
func handleReviewList() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store := gates.NewFSReceiptStore(projectRoot)
	summaries, err := store.ListReceipts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing receipts: %v\n", err)
		os.Exit(1)
	}

	if len(summaries) == 0 {
		fmt.Println("No review receipts found.")
		return
	}

	fmt.Printf("Review Receipts (%d):\n\n", len(summaries))
	fmt.Printf("%-30s  %-12s  %-8s  %s\n", "CHANGE", "STATE", "RISK", "DATE")
	fmt.Println(strings.Repeat("-", 80))
	for _, s := range summaries {
		fmt.Printf("%-30s  %-12s  %-8s  %s\n",
			truncate(s.ChangeName, 30), s.State, s.RiskLevel,
			s.CreatedAt.Format("2006-01-02 15:04"))
	}
}

// handleReviewInstallHooks installs git hook scripts.
// Usage: gaia review install-hooks
func handleReviewInstallHooks() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := gates.WriteHooks(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing hooks: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Git hooks installed successfully:")
	fmt.Println("  .git/hooks/pre-commit  →  gaia review validate --gate pre-commit")
	fmt.Println("  .git/hooks/pre-push    →  gaia review validate --gate pre-push")
	fmt.Println()
	fmt.Println("Tip: To uninstall, remove the GAIA block from the hook scripts manually.")
}

// getStagedFiles returns the list of staged files from git.
func getStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACM")
	out, err := cmd.Output()
	if err != nil {
		// If git isn't available, return empty.
		return nil, nil
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// buildLLMProvider creates an LLM provider from configuration.
func buildLLMProvider(cfg *domain.Config) (ports.LLMProvider, error) {
	primary := cfg.LLM.Provider
	if primary == "" {
		primary = "copilot"
	}

	constructor, ok := llm.Registry[primary]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", primary)
	}
	return constructor(cfg)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Ensure unused imports are handled.
var _ = context.Background
var _ = db.NewSQLiteRepo
var _ = tui.NewNullUI
var _ = core.NewBrain
var _ = domain.ReviewReceipt{}
