package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"net/url"
	"strings"

	"context"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/adapters/tui"
	"gaia/internal/agent"
	"gaia/internal/agent/memory"
	agtsdd "gaia/internal/agent/sdd"
	agtops "gaia/internal/agent/ops"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/shell"
	"gaia/internal/skills"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 0. CLI dispatch: handle subcommands before launching TUI.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "skills":
			handleSkillsCLI(os.Args[2:])
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
		case "serve":
			handleServe(os.Args[2:])
			return
		case "session":
			handleSessionCLI(os.Args[2:])
			return
		case "tracker":
			handleTrackerCLI(os.Args[2:])
			return
		}
	}

	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
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
		p := tea.NewProgram(wizard)
		if _, err := p.Run(); err != nil {
			log.Fatalf("Wizard error: %v", err)
		}

		token, model, language, selectedSkills := wizard.GetResults()
		if token == "" {
			log.Fatal("Wizard cancelled or failed.")
		}

		cfg.APIKeys["copilot"] = token
		cfg.LLM.Provider = "copilot"
		cfg.LLM.Model = model
		if language != "" {
			cfg.System.Language = language
		}
		if err := config.Save(cfg); err != nil {
			log.Fatalf("Error saving config: %v", err)
		}

		fmt.Printf("Configuration saved successfully!")
		if len(selectedSkills) > 0 {
			fmt.Printf(" Installed %d skills.", len(selectedSkills))
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

	// 7b2. Initialize Knowledge Graph store for cross-domain facts.
	kgStore := db.NewKnowledgeGraph(repo.DB())
	brain.SetKnowledgeGraphStore(kgStore)

	// 7c. Initialize async TaskManager with SQLite persistence.
	taskRepo := db.NewTaskRepo(repo.DB())
	taskManager, err := agent.NewTaskManagerWithRepo(context.Background(), taskRepo)
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

	// 7e. Wire subagent system with memory namespace, async task tracking, and MoA providers.
	subagentRegistry := agent.NewRegistry()
	subagentSpawner := agent.NewSpawner(agent.SpawnerConfig{
		Provider:     router,
		Tools:        brain.Registry(),
		Budget:       cfg.Budget,
		Namespace:    namespaceMgr,
		TaskManager:  taskManager,
		MoAProviders: moaProviders,
	}, subagentRegistry)

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

	// 8. Register tool modules
	brain.RegisterModule(shell.NewModule(projectRoot))
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))

	// Wire brain into the TUI so Enter key dispatches ProcessMessage.
	ui.SetBrain(brain)

	// Wire TaskManager into the TUI for async task display and control.
	ui.SetTaskManager(taskManager)

	// Wire dynamic subagent loader for /create-agent command.
	defRepo := db.NewDefRepo(repo.DB())
	dynamicLoader := agent.NewDynamicLoader(defRepo, subagentRegistry, subagentSpawner, namespaceMgr)

	// Configure tool validation against the brain's tool registry.
	availableTools := brain.Registry().Tools()
	dynamicLoader.SetValidator(func(allowed []string) error {
		for _, t := range allowed {
			found := false
			for _, a := range availableTools {
				if a == t {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown tool: %q (available: %v)", t, availableTools)
			}
		}
		return nil
	})

	// Load persisted dynamic subagents on startup.
	ctx := context.Background()
	if err := dynamicLoader.LoadAll(ctx); err != nil {
		log.Printf("Warning: could not load dynamic subagents: %v", err)
	}

	// Wire the TUI for /create-agent interview and dynamic creation.
	ui.SetDynamicCreator(func(def agent.SubagentDef) error {
		return dynamicLoader.CreateFromDef(ctx, def)
	})
	ui.SetToolNames(availableTools)

	// 9. Run main Chat UI
	if err := ui.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
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
				Name        string `json:"name"`
				Owner       struct{ Login string } `json:"owner"`
				Description string `json:"description"`
				URL         string `json:"url"`
				UpdatedAt   string `json:"updatedAt"`
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


func handleSkillsCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia skills <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  search <query>   Search skills by name, description, or tags")
		fmt.Println("  install <name>   Install a bundled skill")
		fmt.Println("  list             List all available skills")
		fmt.Println("  activate <name>  Activate a skill for prompt injection")
		fmt.Println("  deactivate <name> Deactivate a skill")
		fmt.Println("  remove <name>    Remove an installed skill")
		fmt.Println("  add-tap <url>    Add a community tap (GitHub repo or owner/repo)")
		fmt.Println("  remove-tap <url> Remove a community tap")
		fmt.Println("  list-taps        List installed taps")
		fmt.Println("  search-hub <q>   Search for skills on GitHub")
	fmt.Println("  stats             Show skill usage statistics")
	fmt.Println("  audit             Security audit all skills")
		return
	}

	// Resolve paths needed by the hub.
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error resolving working directory: %v", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error resolving home directory: %v", err)
	}

	bundledDir := filepath.Join(projectRoot, "skills")
	userSkillsDir := filepath.Join(homeDir, ".gaia", "skills")
	hub := skills.NewHub(bundledDir, userSkillsDir)

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "search":
		query := ""
		if len(cmdArgs) > 0 {
			query = cmdArgs[0]
		}
		results := hub.Search(query)
		if len(results) == 0 {
			fmt.Println("No skills found.")
			return
		}
		fmt.Printf("Found %d matching skill(s):\n", len(results))
		for _, s := range results {
			fmt.Printf("  %-30s [%s] %s\n", s.Name, s.Source, s.Description)
		}
		// Show active status.
		fmt.Println()
		for _, s := range results {
			status := "inactive"
			if hub.IsActive(s.Name) {
				status = "active"
			}
			fmt.Printf("  %-30s → %s\n", s.Name, status)
		}

	case "install":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills install <name>")
			return
		}
		name := cmdArgs[0]
		if err := hub.Install(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Skill %q installed and activated.\n", name)

	case "list":
		all := hub.List()
		if len(all) == 0 {
			fmt.Println("No skills available.")
			return
		}
		fmt.Printf("Available skills (%d):\n", len(all))
		for _, s := range all {
			status := "inactive"
			if hub.IsActive(s.Name) {
				status = "active"
			}
			fmt.Printf("  %-30s [%s] %s → %s\n", s.Name, s.Source, status, s.Description)
		}

	case "activate":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills activate <name>")
			return
		}
		name := cmdArgs[0]
		if err := hub.Activate(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Skill %q activated.\n", name)

	case "deactivate":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills deactivate <name>")
			return
		}
		name := cmdArgs[0]
		if err := hub.Deactivate(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Skill %q deactivated.\n", name)

	case "remove":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills remove <name>")
			return
		}
		name := cmdArgs[0]
		if err := hub.Remove(name); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Skill %q removed.\n", name)

	case "add-tap":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills add-tap <url> [--branch <branch>]")
			return
		}
		url := cmdArgs[0]
		branch := "main"
		if len(cmdArgs) > 2 && cmdArgs[1] == "--branch" {
			branch = cmdArgs[2]
		}
		if err := hub.AddTap(url, branch); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Tap added: %s\n", url)

	case "remove-tap":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia skills remove-tap <url>")
			return
		}
		if err := hub.RemoveTap(cmdArgs[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Tap removed: %s\n", cmdArgs[0])

	case "list-taps":
		taps, err := hub.ListTaps()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if len(taps) == 0 {
			fmt.Println("No taps installed.")
			return
		}
		fmt.Printf("Installed taps (%d):\n", len(taps))
		for _, t := range taps {
			url := t.URL
			if url == "" {
				url = t.InstalledPath
			}
			fmt.Printf("  %-50s %d skills\n", url, t.SkillCount)
		}

	case "stats":
		stats := hub.UsageStats()
		if len(stats) == 0 {
			fmt.Println("No skills installed.")
			return
		}
		fmt.Printf("Skill usage stats (%d total):\n", len(stats))
		fmt.Printf("%-30s %-8s %-7s %-8s %s\n", "Name", "Loads", "Active", "Source", "Last Used")
		fmt.Println(strings.Repeat("-", 80))
		for _, s := range stats {
			active := "inactive"
			if s.Active {
				active = "active"
			}
			lastUsed := "-"
			if !s.LastUsed.IsZero() {
				lastUsed = s.LastUsed.Format("15:04 02/01")
			}
			fmt.Printf("%-30s %-8d %-7s %-8s %s\n", s.Name, s.LoadCount, active, s.Source, lastUsed)
		}

	case "audit":
		results := hub.AuditAll()
		fmt.Print(skills.FormatAuditResults(results))

	case "search-hub":
		query := ""
		if len(cmdArgs) > 0 {
			query = cmdArgs[0]
		}
		searchSkillsHub(query)
		return

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Run 'gaia skills' for usage.")
	}
}













