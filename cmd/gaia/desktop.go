package main

import (
	"fmt"
	"os"

	"gaia/internal/adapters/desktop"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/core/ports"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/shell"
)

// handleDesktop starts GAIA in desktop (Wails-compatible) mode.
// Usage: gaia desktop
func handleDesktop(args []string) {
	fmt.Println("GAIA Desktop Mode")
	fmt.Println("Starting desktop adapter...")

	// 1. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 2. Resolve project root.
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	// 3. Build LLM provider chain.
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

	// 4. Initialize SQLite repository.
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	// 5. Create DesktopUI adapter.
	ui := desktop.NewDesktopUI()

	// 6. Build confirmation guard (desktop mode uses TrustAlways since user is present).
	trustMode := domain.TrustMode(cfg.LLM.TrustMode)
	if trustMode == "" {
		trustMode = domain.TrustAlways
	}
	guard := core.NewConfirmGuard(trustMode, false)

	// 7. Initialize Brain.
	brain := core.NewBrain(router, repo, ui, guard, cfg.Budget,
		core.WithTokenCallback(func(token string) {
			ui.AppendToken(token)
		}),
	)

	// 8. Register tool modules.
	brain.RegisterModule(shell.NewModuleWithConfig(projectRoot, &cfg.Terminal))
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))

	// 9. Create Wails binding API (created — frontend connects via Wails service layer).
	_ = desktop.NewBindingAPI(ui, brain)

	fmt.Println("Desktop adapter ready. Connect your Wails v3 frontend to the BindingAPI.")
	fmt.Printf("BindingAPI ready with %d modules registered.\n", 3)
	fmt.Println("The Wails application lifecycle is managed by the frontend.")
	fmt.Println("Exiting desktop adapter (ready state).")
}
