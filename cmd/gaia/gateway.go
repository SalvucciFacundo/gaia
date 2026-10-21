package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gaia/internal/adapters/audio"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/gateway"
	"gaia/internal/mneme"
	"gaia/internal/session"
	"gopkg.in/yaml.v3"
)

// handleGatewayCLI implements the "gaia gateway" subcommand family.
func handleGatewayCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia gateway <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  start     Start the messaging gateway")
		fmt.Println("  stop      Stop the messaging gateway")
		fmt.Println("  status    Show gateway status and adapters")
		return
	}

	cmd := args[0]
	switch cmd {
	case "start", "run":
		code := RunGatewayCLI(args, os.Stdout, os.Stderr)
		if code != 0 {
			os.Exit(code)
		}
	case "stop":
		handleGatewayStop()
	case "status":
		handleGatewayStatus()
	default:
		if strings.HasPrefix(cmd, "-") {
			code := RunGatewayCLI(args, os.Stdout, os.Stderr)
			if code != 0 {
				os.Exit(code)
			}
			return
		}
		fmt.Printf("Unknown gateway command: %s\n", cmd)
		fmt.Println("Run 'gaia gateway' for usage.")
	}
}

// RunGatewayCLI runs the `gaia gateway` subcommand daemon.
func RunGatewayCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runGatewayWithContext(ctx, args, stdout, stderr)
}

func runGatewayWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $GAIA_HOME/config.yaml or ~/.config/gaia/config.yaml)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia gateway [run] [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	// Filter out optional "start" or "run" subcommand argument
	var flagArgs []string
	for _, a := range args {
		if a == "run" || a == "start" {
			continue
		}
		flagArgs = append(flagArgs, a)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia gateway: %v\n", err)
		return 2
	}

	var cfg *domain.Config
	var err error
	if *configPath != "" {
		data, readErr := os.ReadFile(*configPath)
		if readErr != nil {
			fmt.Fprintf(stderr, "gaia gateway: loading config: %v\n", readErr)
			return 1
		}
		cfg = &domain.Config{}
		if unmarshalErr := yaml.Unmarshal(data, cfg); unmarshalErr != nil {
			fmt.Fprintf(stderr, "gaia gateway: loading config: %v\n", unmarshalErr)
			return 1
		}
	} else {
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(stderr, "gaia gateway: loading config: %v\n", err)
			return 1
		}
	}

	if !cfg.Gateway.Enabled && !cfg.Gateway.Telegram.Enabled && !cfg.Gateway.Discord.Enabled && !cfg.Gateway.Slack.Enabled && !cfg.Gateway.WhatsApp.Enabled &&
		cfg.Telegram.Token == "" && cfg.Discord.Token == "" && cfg.Slack.Token == "" {
		fmt.Fprintf(stderr, "gaia gateway: gateway is disabled in configuration\n")
		return 1
	}

	hasAny := cfg.Gateway.Telegram.Enabled || cfg.Gateway.Discord.Enabled || cfg.Gateway.Slack.Enabled || cfg.Gateway.WhatsApp.Enabled ||
		cfg.Telegram.Token != "" || cfg.Discord.Token != "" || cfg.Slack.Token != ""
	if !hasAny {
		fmt.Fprintf(stderr, "gaia gateway: no chat adapters (telegram, discord, slack, whatsapp) enabled\n")
		return 1
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	dbPath := cfg.DB.Path
	if dbPath == "" {
		dbPath = filepath.Join(config.GaiaHome(), "gaia.db")
	}

	repo, err := mneme.NewRepository(ctx, dbPath)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "gaia gateway: opening database: %v\n", err)
		return 1
	}
	defer repo.Close()

	brain := core.NewBrain(nil, nil, nil, core.NewConfirmGuard(domain.TrustAlways, false), domain.DefaultBudget())
	sessionManager := session.New(repo, nil, logger)

	var apiKey string
	if cfg.APIKeys != nil {
		apiKey = cfg.APIKeys["openai"]
	}
	if apiKey == "" {
		apiKey = cfg.Multimodal.APIKey
	}

	var transcriber ports.Transcriber
	if cfg.Multimodal.Audio.Enabled {
		transcriber = audio.NewTranscriber(cfg.Multimodal.Audio, apiKey)
		if transcriber != nil {
			logger.Info("multimodal: audio transcriber initialized", "provider", cfg.Multimodal.Audio.Provider, "model", cfg.Multimodal.Audio.Model)
		}
	}

	var synthesizer ports.Synthesizer
	ttsCfg := cfg.TTS
	if !ttsCfg.Enabled && cfg.Multimodal.TTS.Enabled {
		ttsCfg = cfg.Multimodal.TTS
	}
	if ttsCfg.Enabled {
		synthesizer = audio.NewSynthesizer(ttsCfg, apiKey)
		if synthesizer != nil {
			logger.Info("multimodal: outbound speech synthesizer initialized", "provider", ttsCfg.Provider, "model", ttsCfg.Model)
		}
	}

	muxOpts := []gateway.MultiplexerOption{
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
		gateway.WithMultiplexerSessionManager(sessionManager),
		gateway.WithMultiplexerLogger(logger),
	}
	if synthesizer != nil {
		muxOpts = append(muxOpts, gateway.WithMultiplexerSynthesizer(synthesizer))
	}

	mux := gateway.NewMultiplexer(muxOpts...)

	if cfg.Gateway.Telegram.Enabled || cfg.Telegram.Token != "" {
		tgCfg := cfg.Gateway.Telegram
		if tgCfg.Token == "" {
			tgCfg.Token = cfg.Telegram.Token
		}
		if len(tgCfg.Allowlist) == 0 && len(cfg.Telegram.AllowedUserIDs) > 0 {
			for _, id := range cfg.Telegram.AllowedUserIDs {
				tgCfg.Allowlist = append(tgCfg.Allowlist, strconv.FormatInt(id, 10))
			}
		}

		var tgOpts []gateway.TelegramOption
		tgOpts = append(tgOpts,
			gateway.WithTelegramHandler(mux.HandleEvent),
			gateway.WithTelegramLogger(logger),
		)
		if transcriber != nil {
			tgOpts = append(tgOpts, gateway.WithTelegramTranscriber(transcriber))
		}
		if cfg.Multimodal.Vision.MaxImageSizeMB > 0 {
			tgOpts = append(tgOpts, gateway.WithTelegramMaxImageSize(int64(cfg.Multimodal.Vision.MaxImageSizeMB)*1024*1024))
		}
		if cfg.Multimodal.Audio.MaxAudioSizeMB > 0 {
			tgOpts = append(tgOpts, gateway.WithTelegramMaxAudioSize(int64(cfg.Multimodal.Audio.MaxAudioSizeMB)*1024*1024))
		}

		tg := gateway.NewTelegramAdapter(tgCfg, tgOpts...)
		mux.RegisterAdapter(tg)
	}

	if cfg.Gateway.Discord.Enabled || cfg.Discord.Token != "" {
		dcCfg := cfg.Gateway.Discord
		if dcCfg.Token == "" {
			dcCfg.Token = cfg.Discord.Token
		}

		var dcOpts []gateway.DiscordOption
		dcOpts = append(dcOpts,
			gateway.WithDiscordHandler(mux.HandleEvent),
			gateway.WithDiscordLogger(logger),
		)
		if transcriber != nil {
			dcOpts = append(dcOpts, gateway.WithDiscordTranscriber(transcriber))
		}
		if cfg.Multimodal.Vision.MaxImageSizeMB > 0 {
			dcOpts = append(dcOpts, gateway.WithDiscordMaxImageSize(int64(cfg.Multimodal.Vision.MaxImageSizeMB)*1024*1024))
		}
		if cfg.Multimodal.Audio.MaxAudioSizeMB > 0 {
			dcOpts = append(dcOpts, gateway.WithDiscordMaxAudioSize(int64(cfg.Multimodal.Audio.MaxAudioSizeMB)*1024*1024))
		}

		dc := gateway.NewDiscordAdapter(dcCfg, dcOpts...)
		mux.RegisterAdapter(dc)
	}

	if cfg.Gateway.Slack.Enabled || cfg.Slack.Token != "" {
		slCfg := cfg.Gateway.Slack
		if slCfg.BotToken == "" {
			slCfg.BotToken = cfg.Slack.Token
		}

		var slOpts []gateway.SlackOption
		slOpts = append(slOpts,
			gateway.WithSlackHandler(mux.HandleEvent),
			gateway.WithSlackLogger(logger),
		)

		sl := gateway.NewSlackAdapter(slCfg, slOpts...)
		mux.RegisterAdapter(sl)
	}

	if cfg.Gateway.WhatsApp.Enabled {
		var waOpts []gateway.WhatsAppOption
		waOpts = append(waOpts,
			gateway.WithWhatsAppHandler(mux.HandleEvent),
			gateway.WithWhatsAppLogger(logger),
		)
		if transcriber != nil {
			waOpts = append(waOpts, gateway.WithWhatsAppTranscriber(transcriber))
		}
		if cfg.Multimodal.Audio.MaxAudioSizeMB > 0 {
			waOpts = append(waOpts, gateway.WithWhatsAppMaxAudioSize(int64(cfg.Multimodal.Audio.MaxAudioSizeMB)*1024*1024))
		}

		wa := gateway.NewWhatsAppAdapter(cfg.Gateway.WhatsApp, waOpts...)
		mux.RegisterAdapter(wa)
	}

	if err := mux.Start(ctx); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "gaia gateway: starting multiplexer: %v\n", err)
		return 1
	}

	pidFile := filepath.Join(os.TempDir(), "gaia-gateway.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		fmt.Fprintf(stderr, "gaia gateway: warning: could not write pid file: %v\n", err)
	}
	defer os.Remove(pidFile)

	fmt.Fprintf(stdout, "gaia gateway: multiplexer daemon running (PID: %d). Press Ctrl+C to terminate.\n", os.Getpid())

	<-ctx.Done()

	fmt.Fprintf(stdout, "gaia gateway: shutting down gracefully...\n")
	if err := mux.Stop(); err != nil {
		fmt.Fprintf(stderr, "gaia gateway: stop error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "gaia gateway: stopped successfully.\n")
	return 0
}

func handleGatewayStop() {
	pidFile := filepath.Join(os.TempDir(), "gaia-gateway.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Gateway is not running (no PID file found).")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Printf("Invalid PID in %s: %v\n", pidFile, err)
		os.Remove(pidFile)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Could not find process %d: %v\n", pid, err)
		os.Remove(pidFile)
		return
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to send stop signal to process %d: %v\n", pid, err)
		return
	}

	fmt.Printf("Gateway stop signal sent to PID %d.\n", pid)
}

func handleGatewayStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return
	}

	var adapters []string
	if cfg.Gateway.Telegram.Enabled || cfg.Telegram.Token != "" {
		adapters = append(adapters, "telegram")
	}
	if cfg.Gateway.Discord.Enabled || cfg.Discord.Token != "" {
		adapters = append(adapters, "discord")
	}
	if cfg.Gateway.Slack.Enabled || cfg.Slack.Token != "" {
		adapters = append(adapters, "slack")
	}
	if cfg.Gateway.WhatsApp.Enabled {
		adapters = append(adapters, "whatsapp")
	}

	if len(adapters) == 0 {
		fmt.Println("Gateway status: no adapters configured.")
		fmt.Println("Configure adapters in config.yaml under gateway section.")
		return
	}

	fmt.Printf("Gateway status: %d adapter(s) configured\n", len(adapters))
	for _, name := range adapters {
		fmt.Printf("  - %s\n", name)
	}
}
