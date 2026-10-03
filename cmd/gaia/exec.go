package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/adapters/output"
	"gaia/internal/adapters/tui"
	"gaia/internal/agent"
	"gaia/internal/agent/ops"
	"gaia/internal/agent/sdd"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/shell"
)

// handleExec implements the "gaia exec" headless subcommand.
// Usage: gaia exec "task" [--json] [--quiet] [--verbose] [--dry-run] [--yes] [--policy-tier=<tier>]
func handleExec(args []string) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output as structured JSON")
	quiet := fs.Bool("quiet", false, "Suppress non-essential output")
	verbose := fs.Bool("verbose", false, "Show detailed execution trace")
	dryRun := fs.Bool("dry-run", false, "Plan only, don't execute tool calls")
	yes := fs.Bool("yes", false, "Auto-confirm all tool executions")
	policyTier := fs.String("policy-tier", "", "Policy tier: read, sandbox, or full (default: full)")

	reordered := reorderExecArgs(args)
	fs.Parse(reordered)

	task := fs.Arg(0)
	if task == "" {
		fmt.Fprintln(os.Stderr, "Error: no task provided. Usage: gaia exec \"task description\" [flags]")
		os.Exit(1)
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check trust mode: if "always" and no --yes, block execution.
	trustMode := domain.TrustMode(cfg.LLM.TrustMode)
	if trustMode == "" {
		trustMode = domain.TrustAlways
	}
	if trustMode == domain.TrustAlways && !*yes {
		fmt.Fprintln(os.Stderr,
			"Error: confirmation mode is 'always'. Use --yes to auto-confirm in headless mode,",
			"or change trust_mode in your config (~/.config/gaia/config.yaml).")
		os.Exit(1)
	}

	// Build provider chain.
	primary := cfg.LLM.Provider
	if primary == "" {
		primary = "copilot"
	}

	providerNames := []string{primary}
	for _, f := range cfg.LLM.FallbackChain {
		if f != primary {
			providerNames = append(providerNames, f)
		}
	}

	var providers []ports.LLMProvider
	for _, name := range providerNames {
		constructor, ok := llm.Registry[name]
		if !ok {
			continue
		}
		p, err := constructor(cfg)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}

	if len(providers) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no LLM providers available. Check your config.yaml.")
		os.Exit(1)
	}

	router := llm.NewRouter(providers)

	// Initialize repository.
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	// Create NullUI. In dry-run mode, autoApprove=false so tools are denied.
	// In normal mode, autoApprove=true so PromptConfirmation auto-approves.
	autoApprove := !*dryRun
	nullUI := tui.NewNullUI(autoApprove)

	// Create confirmation guard.
	// When --yes is passed or trust mode is not "always", headless mode
	// uses TrustNever (no interactive prompts possible).
	headless := *yes || trustMode != domain.TrustAlways
	guard := core.NewConfirmGuard(trustMode, headless)

	// Determine policy tier from flags and config.
	// Precedence: --policy-tier flag > --dry-run/--yes mappings > config.
	var tier core.PolicyTier
	switch {
	case *policyTier != "":
		tier = core.PolicyTier(*policyTier)
	case *dryRun:
		tier = core.TierRead
	case *yes:
		tier = core.TierFull
	case cfg.Policy.Tier != "":
		tier = core.PolicyTier(cfg.Policy.Tier)
	default:
		tier = core.TierFull
	}

	// Build overrides from domain config.
	overrides := make(map[string]core.OverridePolicy)
	for k, v := range cfg.Policy.Overrides {
		overrides[k] = core.OverridePolicy(v)
	}

	// Create PolicyGuard.
	policyGuard := core.NewPolicyGuard(tier, overrides, nil)
	// If there are deny rules from config, apply them (stored in store if needed)
	if len(cfg.Policy.DenyRules) > 0 {
		// Apply deny rules via store (create a transient store)
		store := core.NewYAMLPolicyStore(".")
		policyGuard.SetStore(store)
	}

	// Build Brain without token streaming callback.
	brain := core.NewBrain(router, repo, nullUI, guard, cfg.Budget)
	brain.SetPolicyGuard(policyGuard)

	// Register core execution modules
	projectRoot, _ := os.Getwd()
	brain.RegisterModule(shell.NewModule(projectRoot))
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))

	// Wire Subagent Spawner for ODD dynamic delegations
	subagentRegistry := agent.NewRegistry()
	_ = subagentRegistry.Register("explorer", sdd.NewExplorer)
	_ = subagentRegistry.Register("implementer", sdd.NewImplementer)
	_ = subagentRegistry.Register("verifier", sdd.NewVerifier)
	_ = subagentRegistry.Register("reviewer", ops.NewReviewer)

	spawner := agent.NewSpawner(agent.SpawnerConfig{
		Provider: router,
		Tools:    brain.Registry(),
		Budget:   cfg.Budget,
		Policy:   policyGuard,
	}, subagentRegistry)
	brain.SetSubagentPort(spawner)

	// Process the user's message.
	ctx := context.Background()
	execErr := brain.ProcessMessage(ctx, task)

	// Build structured output envelope.
	var result *output.ExecResult
	if execErr != nil {
		result = output.NewErrorResult(execErr.Error())
	} else {
		result = output.NewSuccessResult(nullUI.LastOutput(), nil, nil)
	}

	// Format and write output.
	switch {
	case *jsonOut:
		jsonStr, err := result.FormatJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	default:
		fmt.Println(result.FormatText(*quiet, *verbose))
	}
}

// reorderExecArgs reorders CLI arguments so that flags appearing after
// positional task arguments (e.g., gaia exec "task" --json) are placed before
// positional arguments for standard flag parsing.
func reorderExecArgs(args []string) []string {
	var flagArgs []string
	var posArgs []string
	stringFlags := map[string]bool{
		"policy-tier": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			posArgs = append(posArgs, args[i:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if idx := strings.Index(name, "="); idx != -1 {
				name = name[:idx]
			} else if stringFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}
	return append(flagArgs, posArgs...)
}
