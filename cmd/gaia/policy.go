package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gaia/internal/adapters/tui"
	"gaia/internal/config"
	"gaia/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// handlePolicyCLI implements "gaia policy <subcommand>".
// Usage:
//   gaia policy status [--verbose]
//   gaia policy panel
//   gaia policy init [--tier=read|sandbox|full] [--global]
func handlePolicyCLI(args []string) {
	if len(args) == 0 {
		printPolicyUsage()
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	// Also support flags before subcommand if someone runs `gaia policy --tier=full init`
	if strings.HasPrefix(subcommand, "-") {
		found := false
		for i, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				subcommand = arg
				subArgs = append(args[:i], args[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			printPolicyUsage()
			return
		}
	}

	switch subcommand {
	case "status":
		handlePolicyStatus(subArgs)
	case "panel", "tui":
		handlePolicyPanel(subArgs)
	case "init":
		fs := flag.NewFlagSet("policy init", flag.ExitOnError)
		tier := fs.String("tier", "sandbox", "Default tier: read, sandbox, or full")
		isGlobal := fs.Bool("global", false, "Create global policy instead of project-level")
		fs.Parse(subArgs)
		handlePolicyInit(*tier, *isGlobal)
	default:
		printPolicyUsage()
		os.Exit(1)
	}
}

func printPolicyUsage() {
	fmt.Println("Usage: gaia policy <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  status    Display effective permissions across 4 categories and blast radiuses")
	fmt.Println("  panel     Open the interactive Cyberpunk TUI permissions control panel")
	fmt.Println("  init      Create initial policy.yaml configuration file")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --verbose, -v   Show granular tool states in status command")
	fmt.Println("  --tier=<tier>   Default tier for init: read, sandbox, or full")
	fmt.Println("  --global        Initialize global user policy instead of project policy")
}

func getAllDefaultTools() []string {
	toolSet := make(map[string]bool)
	for _, cat := range core.StandardCategories {
		for _, t := range cat.DefaultTools {
			toolSet[t] = true
		}
	}
	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return tools
}

func loadPolicyGuard() (*core.PolicyGuard, []string) {
	tools := getAllDefaultTools()
	cfg, err := config.Load()
	if err != nil {
		return core.NewPolicyGuard(core.TierSandbox, nil, nil), tools
	}
	tier := core.PolicyTier(cfg.Policy.Tier)
	if tier == "" {
		tier = core.TierSandbox
	}
	overrides := make(map[string]core.OverridePolicy)
	for k, v := range cfg.Policy.Overrides {
		overrides[k] = core.OverridePolicy(v)
	}
	return core.NewPolicyGuard(tier, overrides, nil), tools
}

func handlePolicyStatus(args []string) {
	fs := flag.NewFlagSet("policy status", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed tool breakdown")
	fs.BoolVar(verbose, "v", false, "Show detailed tool breakdown (shorthand)")
	fs.Parse(args)

	guard, tools := loadPolicyGuard()
	tier := "sandbox"
	if guard != nil {
		tier = string(guard.Tier())
	}

	fmt.Println("================================================================================")
	fmt.Printf("🛡️  GAIA Security & Permissions Status (Tier: %s)\n", strings.ToUpper(tier))
	fmt.Println("================================================================================")
	fmt.Printf("%-24s %-12s %-24s %s\n", "CATEGORY", "STATUS", "BLAST RADIUS", "TOOLS")
	fmt.Println("--------------------------------------------------------------------------------")

	summaries := core.GetCategorySummaries(guard, tools)
	for _, s := range summaries {
		var badge string
		switch s.State {
		case core.StateAllow:
			badge = "🟢 ALLOW"
		case core.StateAsk:
			badge = "🟡 ASK"
		case core.StateBlock:
			badge = "🔴 BLOCK"
		}

		fmt.Printf("%-24s %-12s %-24s %d tools\n", s.Name, badge, s.BlastRadius, len(s.Tools))
	}
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("Run 'gaia policy panel' for the interactive TUI control panel.")

	if *verbose {
		fmt.Println()
		fmt.Println("Granular Tool Breakdown:")
		for _, s := range summaries {
			fmt.Printf("\n  📁 %s (💥 %s):\n", s.Name, s.BlastRadius)
			sortedTools := make([]core.ToolState, len(s.Tools))
			copy(sortedTools, s.Tools)
			sort.Slice(sortedTools, func(i, j int) bool {
				return sortedTools[i].Name < sortedTools[j].Name
			})
			for _, t := range sortedTools {
				var dot string
				switch t.State {
				case core.StateAllow:
					dot = "🟢 ALLOW"
				case core.StateAsk:
					dot = "🟡 ASK  "
				case core.StateBlock:
					dot = "🔴 BLOCK"
				}
				fmt.Printf("    • %-20s %s\n", t.Name, dot)
			}
		}
	}
}

func handlePolicyPanel(args []string) {
	guard, tools := loadPolicyGuard()
	model := tui.NewPolicyPanelModel(guard, tools)
	model.SetStandalone(true)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running policy panel: %v\n", err)
		os.Exit(1)
	}
}

func handlePolicyInit(tier string, isGlobal bool) {
	// Validate tier
	validTiers := map[string]bool{"read": true, "sandbox": true, "full": true}
	if !validTiers[tier] {
		fmt.Fprintf(os.Stderr, "Error: invalid tier %q. Use: read, sandbox, or full\n", tier)
		os.Exit(1)
	}

	var targetDir string
	var scopeName string

	if isGlobal {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot find home directory: %v\n", err)
			os.Exit(1)
		}
		targetDir = filepath.Join(home, ".config", "gaia")
		scopeName = "global"
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot get current directory: %v\n", err)
			os.Exit(1)
		}
		targetDir = filepath.Join(cwd, ".gaia")
		scopeName = "project"
	}

	targetFile := filepath.Join(targetDir, "policy.yaml")

	// Check if exists
	if _, err := os.Stat(targetFile); err == nil {
		data, _ := os.ReadFile(targetFile)
		fmt.Printf("Policy file already exists at %s:\n\n%s\n", targetFile, string(data))
		overwrite := readYesNo("Overwrite? (y/N): ")
		if !overwrite {
			fmt.Println("Cancelled.")
			return
		}
	}

	// Create directory
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", targetDir, err)
		os.Exit(1)
	}

	cfg := &core.PolicyConfig{
		Tier: core.PolicyTier(tier),
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot marshal policy: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(targetFile, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", targetFile, err)
		os.Exit(1)
	}

	fmt.Printf("Created %s policy: %s\n", scopeName, targetFile)
	fmt.Printf("  Tier: %s\n", tier)
	fmt.Println()
	fmt.Println("Edit this file to add tool overrides or deny rules.")
	fmt.Println("Use /permisos inside GAIA to view and change policies at runtime.")
}

// readYesNo prompts for yes/no input and returns true for y/Y.
func readYesNo(prompt string) bool {
	fmt.Print(prompt)
	var resp string
	fmt.Scanln(&resp)
	return resp == "y" || resp == "Y"
}
