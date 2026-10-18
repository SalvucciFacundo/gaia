package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"context"
	"log/slog"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/adapters/tui"
	"gaia/internal/agent"
	"gaia/internal/agent/memory"
	agtops "gaia/internal/agent/ops"
	agtsdd "gaia/internal/agent/sdd"
	"gaia/internal/blackboard"
	"gaia/internal/browser"
	"gaia/internal/codegraph"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/cron"
	"gaia/internal/gateway"
	blackboardmod "gaia/internal/modules/blackboard"
	"gaia/internal/modules/codemode"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/multimodal"
	"gaia/internal/modules/shell"
	"gaia/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 0. CLI dispatch: handle subcommands before launching TUI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "skill", "skills":
			handleSkillCLI(os.Args[2:])
			return
		case "exec":
			handleExec(os.Args[2:])
			return
		case "review":
			handleReviewCLI(os.Args[2:])
			return
		case "desktop":
			handleDesktop(os.Args[2:])
			return
		case "cron":
			handleCronCLI(os.Args[2:])
			return
		case "doctor":
			handleDoctor(os.Args[2:])
			return
		case "onboard":
			handleOnboard(os.Args[2:])
			return
		case "gateway":
			handleGatewayCLI(os.Args[2:])
			return
		case "plugin":
			handlePluginCLI(os.Args[2:])
			return
		case "webhook":
			handleWebhookCLI(os.Args[2:])
			return
		case "lsp":
			handleLSPCLI(os.Args[2:])
			return
		case "serve", "api":
			handleServeCLI(os.Args[2:])
			return
		case "session":
			handleSessionCLI(os.Args[2:])
			return
		case "tracker":
			handleTrackerCLI(os.Args[2:])
			return
		case "policy":
			handlePolicyCLI(os.Args[2:])
			return
		case "delivery":
			handleDeliveryCLI(os.Args[2:])
			return
		case "profile", "profiles":
			handleProfileCLI(os.Args[2:])
			return
		case "backup":
			handleBackupCLI(os.Args[2:])
			return
		case "restore":
			handleRestoreCLI(os.Args[2:])
			return
		case "update":
			handleUpdateCLI(os.Args[2:])
			return
		case "blackboard":
			handleBlackboardCLI(os.Args[2:])
			return
		case "overnight":
			handleOvernightCLI(os.Args[2:])
			return
		}
	}

	// 1. Root lifecycle context with SIGINT / SIGTERM handling
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1.1 Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 1.2 Support -p / --profile runtime override
	for i := 1; i < len(os.Args); i++ {
		if (os.Args[i] == "-p" || os.Args[i] == "--profile") && i+1 < len(os.Args) {
			cfg.CurrentProfile = os.Args[i+1]
			break
		}
	}

	// 1a. Resolve project root early (needed by wizard and hub).
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error resolving working directory: %v", err)
	}
	projectName := filepath.Base(projectRoot)

	// 1b. Resolve home directory for user skills.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error resolving home directory: %v", err)
	}
	userSkillsDir := filepath.Join(homeDir, ".gaia", "skills")

	// 1c. Initialize Skills Hub.
	bundledDir := filepath.Join(projectRoot, "skills")
	skillsHub := skills.NewHub(bundledDir, userSkillsDir)

	// 2. Check if we need to run the Wizard (first-run / no API keys).
	if cfg.APIKeys["copilot"] == "" && cfg.APIKeys["openai"] == "" && cfg.APIKeys["anthropic"] == "" {
		fmt.Println("No AI configuration detected. Starting Setup Wizard...")
		wizard := tui.NewWizard(projectRoot)
		wizard.SetHub(skillsHub)
		wizard.SetContext(rootCtx)
		p := tea.NewProgram(wizard)
		if _, err := p.Run(); err != nil {
			log.Fatalf("Wizard error: %v", err)
		}

		provider, apiKey, model, language, securityOn, selectedSkills, selectedProfile := wizard.GetResults()
		if provider == "" {
			log.Fatal("Wizard cancelled or failed.")
		}

		if cfg.APIKeys == nil {
			cfg.APIKeys = make(map[string]string)
		}
		if apiKey != "" {
			cfg.APIKeys[provider] = apiKey
		}
		cfg.LLM.Provider = provider
		if model != "" {
			cfg.LLM.Model = model
		}
		if language != "" {
			cfg.System.Language = language
		}
		if securityOn {
			cfg.Policy.Tier = "sandbox"
		}
		if selectedProfile != "" {
			cfg.CurrentProfile = selectedProfile
			if cfg.Profiles == nil {
				cfg.Profiles = make(map[string]domain.ProfileConfig)
			}
			// Enable selected profile
			cfg.Profiles[selectedProfile] = domain.ProfileConfig{Enabled: true}
		}
		if err := config.Save(cfg); err != nil {
			log.Fatalf("Error saving config: %v", err)
		}

		fmt.Printf("Configuration saved successfully! Selected provider: %s, Profile: %s.", provider, cfg.CurrentProfile)
		if len(selectedSkills) > 0 {
			fmt.Printf(" Installed %d skills.", len(selectedSkills))
		}
		if securityOn {
			fmt.Printf(" Security mode enabled.")
		}
		fmt.Println()
	}

	// 3. Build provider chain from config
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
			log.Printf("Warning: unknown provider %q, skipping", name)
			continue
		}
		p, err := constructor(cfg)
		if err != nil {
			log.Printf("Warning: could not initialize %q: %v", name, err)
			continue
		}
		providers = append(providers, p)
	}

	if len(providers) == 0 {
		log.Fatal("No LLM providers available. Check your config.yaml")
	}

	router := llm.NewRouter(providers)

	// 4. Initialize SQLite Repository
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	// 5. Initialize TUI
	ui := tui.NewTUI()
	ui.SetContext(rootCtx)

	// 6. Build ConfirmGuard
	trustMode := domain.TrustMode(cfg.LLM.TrustMode)
	if trustMode == "" {
		trustMode = domain.TrustAlways
	}
	guard := core.NewConfirmGuard(trustMode, false)

	// 7. Initialize Brain with model info for usage tracking
	brain := core.NewBrain(router, repo, ui, guard, cfg.Budget,
		core.WithTokenCallback(func(token string) {
			ui.AppendToken(token)
		}),
		core.WithModelInfo(cfg.LLM.Provider, cfg.LLM.Model),
		core.WithCostTracker(core.NewCostTracker()),
	)

	// 7b. Initialize Engram namespace manager for per-subagent memory isolation
	namespaceMgr := memory.NewNamespaceManager(projectName)
	sessionName := fmt.Sprintf("session-%s", time.Now().Format("2006-01-02-15-04"))
	sessionID, err := repo.CreateSession(rootCtx, sessionName)
	if err == nil && sessionID != "" {
		repo.SetSessionID(sessionID)
		namespaceMgr.SetSessionID(sessionID)
	}

	// 7b1. Initialize PolicyGuard with active profile and memory namespace isolation (RBAC)
	tier := core.PolicyTier(cfg.Policy.Tier)
	if tier == "" {
		tier = core.TierFull
	}
	overrides := make(map[string]core.OverridePolicy)
	for k, v := range cfg.Policy.Overrides {
		overrides[k] = core.OverridePolicy(v)
	}
	policyGuard := core.NewPolicyGuard(tier, overrides, nil)
	activeProfile := cfg.CurrentProfile
	if activeProfile == "" {
		activeProfile = "dev"
	}
	policyGuard.SetActiveProfile(activeProfile)
	policyGuard.SetMemoryValidator(namespaceMgr.ValidateMemoryAccess)
	brain.SetPolicyGuard(policyGuard)
	ui.SetPolicyGuard(policyGuard)

	// 7b2. Initialize Knowledge Graph store for cross-domain facts.
	kgStore := db.NewKnowledgeGraph(repo.DB())
	brain.SetKnowledgeGraphStore(kgStore)

	// 7c. Initialize async TaskManager with SQLite persistence.
	taskRepo := db.NewTaskRepo(repo.DB())
	taskManager, err := agent.NewTaskManagerWithRepo(rootCtx, taskRepo)
	if err != nil {
		log.Fatalf("Error initializing task manager: %v", err)
	}

	// 7d. Initialize MoA providers (all available, for per-subagent MoA fan-out).
	moaProviders := make(map[string]ports.LLMProvider)
	for name, constructor := range llm.Registry {
		p, err := constructor(cfg)
		if err == nil {
			moaProviders[name] = p
		}
	}

	// 7e. Initialize Blackboard working memory (L1 RAM with artifacts spillover).
	blackboardDir := filepath.Join(projectRoot, ".gaia", "blackboard")
	bb := blackboard.New(blackboardDir)

	// 7f. Wire subagent system with memory namespace, async task tracking, MoA providers, and Blackboard.
	subagentRegistry := agent.NewRegistry()
	subagentSpawner := agent.NewSpawner(agent.SpawnerConfig{
		Provider:     router,
		Tools:        brain.Registry(),
		Budget:       cfg.Budget,
		Namespace:    namespaceMgr,
		TaskManager:  taskManager,
		MoAProviders: moaProviders,
		Policy:       policyGuard,
		Blackboard:   bb,
	}, subagentRegistry)
	subagentSpawner.SetOnLearningThreshold(autoCreateSkill)

	// Register SDD subagents (M2: 5 phases; M3: +3 new phases)
	register := func(name string, factory agent.SubagentFactory) {
		if err := subagentRegistry.Register(name, factory); err != nil {
			log.Printf("Warning: could not register %s subagent: %v", name, err)
		}
	}
	register("explorer", agtsdd.NewExplorer)
	register("proposer", agtsdd.NewProposer)
	register("specifier", agtsdd.NewSpecifier)
	register("designer", agtsdd.NewDesigner)
	register("planner", agtsdd.NewPlanner)
	register("implementer", agtsdd.NewImplementer)
	register("verifier", agtsdd.NewVerifier)
	register("archiver", agtsdd.NewArchiver)

	// Register ops (non-SDD) subagents (M3: reviewer, debugger, researcher, learner)
	register("reviewer", agtops.NewReviewer)
	register("debugger", agtops.NewDebugger)
	register("researcher", agtops.NewResearcher)
	register("learner", agtops.NewLearner)

	brain.SetSubagentPort(subagentSpawner)

	// 7f. Wire MoA providers into Brain for /moa command.
	brain.SetMoAProviders(moaProviders)

	// 7g. Wire Skills Hub to Brain for intent-driven progressive skill loading & context protection.
	brain.SetSkillLister(func() []string {
		active := skillsHub.ListActive()
		names := make([]string, len(active))
		for i, s := range active {
			names[i] = s.Name
		}
		return names
	})
	brain.SetSkillPathResolver(func(skillNames []string) []string {
		return skillsHub.ResolveSkillPaths(skillNames)
	})
	brain.SetSkillMatcher(func(taskDesc string, files []string) []string {
		return skillsHub.MatchSkills(taskDesc, files)
	})

	// 8. Register tool modules
	brain.RegisterModule(shell.NewModule(projectRoot))
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))
	brain.RegisterModule(multimodal.NewModule(cfg.Multimodal, router))
	brain.RegisterModule(codemode.NewModule(projectRoot, brain.Registry()))
	bbMod := blackboardmod.NewModule(bb)
	bbMod.SetStore(repo, sessionID, activeProfile)
	brain.RegisterModule(bbMod)

	// Register CodeGraph AST indexer & semantic query module
	codegraphDB := filepath.Join(projectRoot, ".gaia", "codegraph.db")
	if cgMod, cgErr := codegraph.NewModule(codegraphDB); cgErr == nil {
		brain.RegisterModule(cgMod)
	}
	// Register browser module if configured
	if cfg.Gateway.BrowserTools.Enabled {
		browserMod, berr := browser.NewModule(cfg.Gateway.BrowserTools)
		if berr == nil {
			brain.RegisterModule(browserMod)
			fmt.Println("Browser automation registered")
		}
	}

	// Wire brain into the TUI so Enter key dispatches ProcessMessage.
	ui.SetBrain(brain)

	// Wire TaskManager into the TUI for async task display and control.
	ui.SetTaskManager(taskManager)

	// Wire dynamic subagent loader for /create-agent command.
	defRepo := db.NewDefRepo(repo.DB())
	dynamicLoader := agent.NewDynamicLoader(defRepo, subagentRegistry, subagentSpawner, namespaceMgr)

	// Configure tool validation against the brain's tool registry.
	availableTools := brain.Registry().Tools()
	availableToolSet := make(map[string]struct{}, len(availableTools))
	for _, a := range availableTools {
		availableToolSet[a] = struct{}{}
	}
	dynamicLoader.SetValidator(func(allowed []string) error {
		for _, t := range allowed {
			if _, ok := availableToolSet[t]; !ok {
				return fmt.Errorf("unknown tool: %q (available: %v)", t, availableTools)
			}
		}
		return nil
	})

	// Load persisted dynamic subagents on startup.
	if err := dynamicLoader.LoadAll(rootCtx); err != nil {
		log.Printf("Warning: could not load dynamic subagents: %v", err)
	}

	// Wire the TUI for /create-agent interview and dynamic creation.
	ui.SetDynamicCreator(func(def agent.SubagentDef) error {
		return dynamicLoader.CreateFromDef(rootCtx, def)
	})
	ui.SetToolNames(availableTools)

	// 9. Start gateway adapters if configured (unified mode).
	// Gateway adapters run in the same process as the TUI, sharing the same Brain.
	// Enable with --unify flag or when gateway tokens are configured in config.yaml.
	gatewayCtx, gatewayCancel := context.WithCancel(rootCtx)
	if hasGatewayConfig(cfg) {
		if gw := startUnifiedGateway(gatewayCtx, cfg, repo, projectRoot, brain, ui); gw != nil {
			defer func() {
				fmt.Println("Stopping gateway adapters...")
				gw.Stop()
				gatewayCancel()
			}()
			fmt.Println("Gateway adapters started alongside TUI.")
		}
	}

	// 10. Run main Chat UI
	if err := ui.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}

	// 11. Graceful Shutdown & Session Close
	// Persist a clean session close summary so the next session starts with full context.
	if sessionID != "" {
		summaryText := memory.BuildSessionSummaryText(memory.SessionSummaryInput{
			Goal: fmt.Sprintf("GAIA interactive session (%s)", sessionName),
			Accomplished: []string{
				fmt.Sprintf("Completed session in workspace %s", projectName),
			},
		})
		summaryMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("SESSION_CLOSE_SUMMARY:\n%s", summaryText),
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = repo.SaveMessage(closeCtx, summaryMsg)
		closeCancel()
	}
}

// autoCreateSkill generates a SKILL.md for a subagent that reached the learning threshold.
func autoCreateSkill(subagentName string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	skillsDir := filepath.Join(home, ".gaia", "skills")
	skillName := subagentName + "-patterns"
	skillDir := filepath.Join(skillsDir, skillName)

	if _, err := os.Stat(skillDir); err == nil {
		return
	}

	content := fmt.Sprintf(`---
name: %s
description: "Auto-generated patterns from %s subagent"
tags: [auto-generated, learned, %s]
category: learned
---

# %s Patterns

Auto-generated after %d executions of the %s subagent.

## Purpose

Capture recurring patterns from the %s subagent in this project.

## Usage

Load when working with the %s subagent or reviewing its output.

## Patterns

- (Edit this file to add specific patterns)
`, skillName, subagentName, subagentName, subagentName, 5, subagentName, subagentName, subagentName)

	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
	fmt.Printf("[learn] Created skill: %s\n", skillDir)
}

// hasGatewayConfig checks if any gateway adapters are configured.
func hasGatewayConfig(cfg *domain.Config) bool {
	return cfg.Telegram.Token != "" ||
		cfg.Discord.Token != "" ||
		cfg.Slack.Token != "" ||
		cfg.WhatsApp.Command != "" ||
		cfg.Signal.Command != ""
}

// startUnifiedGateway creates gateway adapters and starts them with the existing Brain.
// Returns the Multiplexer instance so the caller can stop it on shutdown.
func startUnifiedGateway(ctx context.Context, cfg *domain.Config, repo *db.SQLiteRepo, projectRoot string, brain *core.Brain, tuiUI *tui.Model) *gateway.Multiplexer {
	logger := slog.Default()
	muxOpts := []gateway.MultiplexerOption{
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerLogger(logger),
	}
	mux := gateway.NewMultiplexer(muxOpts...)

	var count int
	// Register Telegram adapter if configured.
	if cfg.Telegram.Token != "" || cfg.Gateway.Telegram.Enabled {
		tgCfg := cfg.Gateway.Telegram
		if tgCfg.Token == "" {
			tgCfg.Token = cfg.Telegram.Token
		}
		if len(tgCfg.Allowlist) == 0 && len(cfg.Telegram.AllowedUserIDs) > 0 {
			for _, id := range cfg.Telegram.AllowedUserIDs {
				tgCfg.Allowlist = append(tgCfg.Allowlist, strconv.FormatInt(id, 10))
			}
		}
		adapter := gateway.NewTelegramAdapter(tgCfg,
			gateway.WithTelegramHandler(mux.HandleEvent),
			gateway.WithTelegramLogger(logger),
		)
		mux.RegisterAdapter(adapter)
		count++
		fmt.Println("Telegram adapter registered.")
	}

	// Register Discord adapter if configured.
	if cfg.Discord.Token != "" || cfg.Gateway.Discord.Enabled {
		dCfg := cfg.Gateway.Discord
		if dCfg.Token == "" {
			dCfg.Token = cfg.Discord.Token
		}
		adapter := gateway.NewDiscordAdapter(dCfg,
			gateway.WithDiscordHandler(mux.HandleEvent),
			gateway.WithDiscordLogger(logger),
		)
		mux.RegisterAdapter(adapter)
		count++
		fmt.Println("Discord adapter registered.")
	}

	// Register Slack adapter if configured.
	if cfg.Slack.Token != "" || cfg.Gateway.Slack.Enabled {
		sCfg := cfg.Gateway.Slack
		if sCfg.BotToken == "" {
			sCfg.BotToken = cfg.Slack.Token
		}
		adapter := gateway.NewSlackAdapter(sCfg,
			gateway.WithSlackHandler(mux.HandleEvent),
			gateway.WithSlackLogger(logger),
		)
		mux.RegisterAdapter(adapter)
		count++
		fmt.Println("Slack adapter registered.")
	}

	// Register WhatsApp adapter if configured.
	if cfg.Gateway.WhatsApp.Enabled {
		adapter := gateway.NewWhatsAppAdapter(cfg.Gateway.WhatsApp,
			gateway.WithWhatsAppHandler(mux.HandleEvent),
			gateway.WithWhatsAppLogger(logger),
		)
		mux.RegisterAdapter(adapter)
		count++
		fmt.Println("WhatsApp adapter registered.")
	}

	if count == 0 {
		return nil
	}

	// Set up delivery service for cron jobs.
	delivery := cron.NewDeliveryService()
	delivery.SetGatewaySender(mux.Send)

	// Start gateway in background
	go func() {
		if err := mux.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("Gateway error: %v", err)
		}
	}()

	return mux
}

// handleSkillsCLI implements the "gaia skills" subcommand family.
// searchSkillsHub searches GitHub for repositories containing agent skills.
// Uses the gh CLI when available; falls back to a skills.sh URL.
func searchSkillsHub(query string) {
	// Try gh CLI for rich search results.
	ghPath, err := exec.LookPath("gh")
	if err == nil {
		searchArgs := []string{"search", "repos", query, "path:skills/SKILL.md", "--limit", "15",
			"--json", "name,owner,description,url,updatedAt"}
		cmd := exec.Command(ghPath, searchArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			var results []struct {
				Name        string                 `json:"name"`
				Owner       struct{ Login string } `json:"owner"`
				Description string                 `json:"description"`
				URL         string                 `json:"url"`
				UpdatedAt   string                 `json:"updatedAt"`
			}
			if json.Unmarshal(out, &results) == nil && len(results) > 0 {
				fmt.Printf("Found %d repositories with skills matching %q:\n\n", len(results), query)
				for _, r := range results {
					desc := r.Description
					if len(desc) > 70 {
						desc = desc[:70] + "..."
					}
					fmt.Printf("  %s/%s\n", r.Owner.Login, r.Name)
					fmt.Printf("  %s\n", r.URL)
					if desc != "" {
						fmt.Printf("  %s\n", desc)
					}
					fmt.Printf("  Install: gaia skills add-tap %s/%s\n", r.Owner.Login, r.Name)
					fmt.Println()
				}
				fmt.Println("Tip: install any tap with: gaia skills add-tap <owner/repo>")
				return
			}
		}
	}

	// Fallback: point to skills.sh
	fmt.Printf("Search the skills hub online:\n")
	fmt.Printf("  https://skills.sh/search?q=%s\n", url.QueryEscape(query))
	fmt.Println()
	fmt.Println("Install a skill from skills.sh with:")
	fmt.Println("  gaia skills add-tap <owner/repo>")
}
