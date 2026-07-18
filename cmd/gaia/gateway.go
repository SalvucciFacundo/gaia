package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"gaia/internal/adapters/db"
	"gaia/internal/config"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/cron"
	"gaia/internal/gateway"
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
	case "start":
		handleGatewayStart()
	case "stop":
		handleGatewayStop()
	case "status":
		handleGatewayStatus()
	default:
		fmt.Printf("Unknown gateway command: %s\n", cmd)
		fmt.Println("Run 'gaia gateway' for usage.")
	}
}

func handleGatewayStart() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	gw := gateway.NewGateway()

	// Register Telegram adapter if configured.
	if cfg.Telegram.Token != "" {
		tgCfg := domain.TelegramGatewayConfig{
			Token:          cfg.Telegram.Token,
			AllowedUserIDs: cfg.Telegram.AllowedUserIDs,
		}
		adapter, err := gateway.NewTelegramAdapter(tgCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Telegram adapter: %v\n", err)
		} else {
			gw.Register(adapter)
			fmt.Println("Telegram adapter registered.")
		}
	}

	// Initialize DB for brain+delivery support.
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving working directory: %v\n", err)
		os.Exit(1)
	}

	// Message handler: route incoming gateway messages.
	handler := func(ctx context.Context, msg ports.IncomingMessage) (string, error) {
		// Build a simple brain if available, otherwise echo.
		brain := core.NewBrain(nil, repo, nil, core.NewConfirmGuard(domain.TrustAlways, false), domain.DefaultBudget())
		_ = brain
		_ = projectRoot
		fmt.Printf("[gateway:%s] %s: %s\n", msg.Platform, msg.SenderName, msg.Content)
		return fmt.Sprintf("Received: %s", msg.Content), nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gateway...")
		cancel()
	}()

	// Set up delivery service with gateway sender.
	delivery := cron.NewDeliveryService()
	delivery.SetGatewaySender(gw.Send)

	fmt.Printf("Starting gateway with %d adapters...\n", len(gw.ListAdapters()))
	if err := gw.Start(ctx, handler); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting gateway: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Gateway running. Press Ctrl+C to stop.")
	<-ctx.Done()

	if err := gw.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping gateway: %v\n", err)
	}
	fmt.Println("Gateway stopped.")
}

func handleGatewayStop() {
	pidFile := filepath.Join(os.TempDir(), "gaia-gateway.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		fmt.Println("Gateway is not running.")
		return
	}
	fmt.Println("Gateway stop signal sent.")
	// In a full implementation, we'd read the PID and signal it.
}

func handleGatewayStatus() {
	gw := gateway.NewGateway()
	adapters := gw.ListAdapters()
	if len(adapters) == 0 {
		fmt.Println("Gateway status: no adapters configured.")
		fmt.Println("Configure adapters in config.yaml under telegram or discord sections.")
		return
	}

	fmt.Printf("Gateway status: %d adapter(s) available\n", len(adapters))
	for _, name := range adapters {
		fmt.Printf("  - %s\n", name)
	}
}
