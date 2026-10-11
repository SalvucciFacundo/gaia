package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"gaia/internal/adapters/db"
	"gaia/internal/adapters/llm"
	"gaia/internal/codegraph"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/modules/fileops"
	"gaia/internal/modules/gitops"
	"gaia/internal/modules/multimodal"
	"gaia/internal/modules/shell"
	"gaia/internal/server"
)

type lazyFallbackProvider struct {
	name  string
	model string
}

func (p *lazyFallbackProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	return nil, fmt.Errorf("no LLM provider configured (provider: %s, model: %s)", p.name, p.model)
}

func (p *lazyFallbackProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, fmt.Errorf("no LLM provider configured (provider: %s, model: %s)", p.name, p.model)
}

func (p *lazyFallbackProvider) Tools() []domain.ToolDef { return nil }
func (p *lazyFallbackProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{p.model}, nil
}

// RunServeCLI runs the `gaia serve` / `gaia api` subcommand router.
func RunServeCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runServeWithContext(ctx, args, stdout, stderr)
}

func handleServeCLI(args []string) {
	code := RunServeCLI(args, os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}

func handleServe(args []string) {
	handleServeCLI(args)
}

func runServeWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $GAIA_HOME/config.yaml or ~/.config/gaia/config.yaml)")
	profileFlag := fs.String("profile", "", "active configuration profile name")
	hostFlag := fs.String("host", "127.0.0.1", "HTTP host to bind (overrides config)")
	portFlag := fs.Int("port", 8080, "HTTP port to bind (overrides config)")
	apiKeyFlag := fs.String("api-key", "", "Bearer token for API authentication (overrides config)")
	corsFlag := fs.String("cors", "", "comma-separated allowed CORS origins (overrides config)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia serve [flags]\n       gaia api [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	var flagArgs []string
	for _, a := range args {
		if a == "run" {
			continue
		}
		flagArgs = append(flagArgs, a)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia serve: %v\n", err)
		return 2
	}

	var cfg *domain.Config
	var err error
	if *configPath != "" {
		data, readErr := os.ReadFile(*configPath)
		if readErr != nil {
			fmt.Fprintf(stderr, "gaia serve: loading config: %v\n", readErr)
			return 1
		}
		var fileCfg domain.Config
		if unmarshalErr := yaml.Unmarshal(data, &fileCfg); unmarshalErr != nil {
			fmt.Fprintf(stderr, "gaia serve: loading config: %v\n", unmarshalErr)
			return 1
		}
		cfg = &fileCfg
	} else {
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(stderr, "gaia serve: loading config: %v\n", err)
			return 1
		}
	}

	if *profileFlag != "" {
		cfg.CurrentProfile = *profileFlag
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = "assistant"
	}

	// Apply flag overrides
	if isFlagSet(fs, "host") {
		cfg.Server.Host = *hostFlag
	} else if cfg.Server.Host == "" {
		cfg.Server.Host = *hostFlag
	}

	if isFlagSet(fs, "port") {
		cfg.Server.Port = *portFlag
	} else if cfg.Server.Port == 0 && !isFlagSet(fs, "port") {
		cfg.Server.Port = *portFlag
	}

	if isFlagSet(fs, "api-key") || *apiKeyFlag != "" {
		cfg.Server.APIKey = *apiKeyFlag
	}

	if isFlagSet(fs, "cors") || *corsFlag != "" {
		parts := strings.Split(*corsFlag, ",")
		var origins []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		cfg.Server.CORSOrigins = origins
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	var repo *db.SQLiteRepo
	if cfg.DB.Path != "" {
		repo, err = db.NewSQLiteRepoWithPath(cfg.DB.Path)
	} else {
		repo, err = db.NewSQLiteRepo()
	}
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "gaia serve: opening database: %v\n", err)
		return 1
	}

	provider := initProvider(cfg)
	if provider == nil {
		provider = &lazyFallbackProvider{
			name:  cfg.LLM.Provider,
			model: cfg.LLM.Model,
		}
	}

	guard := core.NewConfirmGuard(domain.TrustAlways, false)
	brain := core.NewBrain(provider, repo, nil, guard, cfg.Budget,
		core.WithModelInfo(cfg.LLM.Provider, cfg.LLM.Model),
		core.WithCostTracker(core.NewCostTracker()),
	)

	projectRoot, _ := os.Getwd()
	brain.RegisterModule(fileops.NewModule(projectRoot))
	brain.RegisterModule(gitops.NewModule(projectRoot))
	brain.RegisterModule(shell.NewModule(projectRoot))
	brain.RegisterModule(multimodal.NewModule(cfg.Multimodal, provider))

	codegraphDB := filepath.Join(projectRoot, ".gaia", "codegraph.db")
	if cgMod, cgErr := codegraph.NewModule(codegraphDB); cgErr == nil {
		brain.RegisterModule(cgMod)
	}

	activeModel := cfg.LLM.Model
	if prof, ok := cfg.Profiles[cfg.CurrentProfile]; ok && prof.Model != "" {
		activeModel = prof.Model
	}

	srvOpts := server.Options{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		APIKey:       cfg.Server.APIKey,
		CORSOrigins:  cfg.Server.CORSOrigins,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		Brain:        brain,
		Logger:       logger,
		Profile:      cfg.CurrentProfile,
		Version:      "1.0.0",
		Provider:     cfg.LLM.Provider,
		Model:        activeModel,
	}

	srv := server.New(srvOpts)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	fmt.Fprintf(stdout, "GAIA REST API Server listening on %s (profile: %s, model: %s/%s)\n",
		srv.Addr(), cfg.CurrentProfile, cfg.LLM.Provider, activeModel)

	select {
	case <-ctx.Done():
		fmt.Fprintf(stdout, "Shutting down API Server gracefully...\n")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(stderr, "gaia serve: shutdown error: %v\n", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "gaia serve: server error: %v\n", err)
			return 1
		}
		return 0
	}
}

func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
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
