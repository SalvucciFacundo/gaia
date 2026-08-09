package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/adapters/web"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/shell"
)

type serveMsg struct {
	Content string `json:"content"`
}

func handleServe(args []string) {
	port := "8080"
	if len(args) > 0 {
		port = args[0]
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	provider := initProvider(cfg)
	if provider == nil {
		log.Fatal("No LLM provider available")
	}

	repo, err := db.NewSQLiteRepo()
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}

	guard := core.NewConfirmGuard(domain.TrustNever, false)
	brain := core.NewBrain(provider, repo, nil, guard, cfg.Budget,
		core.WithModelInfo(cfg.LLM.Provider, cfg.LLM.Model),
		core.WithCostTracker(core.NewCostTracker()),
	)

	projectRoot, _ := os.Getwd()
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))
	brain.RegisterModule(shell.NewModule(projectRoot))

	webServer := web.NewServer(brain, port, cfg.LLM.Provider, cfg.LLM.Model)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		<-stop
		cancel()
	}()

	if err := webServer.Start(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func initProvider(cfg *domain.Config) ports.LLMProvider {
	primary := cfg.LLM.Provider
	if primary == "" {
		primary = "copilot"
	}
	names := []string{primary}
	for _, f := range cfg.LLM.FallbackChain {
		if f != primary {
			names = append(names, f)
		}
	}
	var providers []ports.LLMProvider
	for _, name := range names {
		c, ok := llm.Registry[name]
		if !ok {
			continue
		}
		p, err := c(cfg)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		return nil
	}
	return llm.NewRouter(providers)
}
