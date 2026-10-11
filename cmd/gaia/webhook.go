package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"gaia/internal/webhook"
)

// handleWebhookCLI implements the "gaia webhook" subcommand family.
func handleWebhookCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia webhook <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  start          Start the webhook listener")
		fmt.Println("  list-subs      List configured webhook subscriptions")
		return
	}

	cmd := args[0]
	switch cmd {
	case "start":
		handleWebhookStart()
	case "list-subs":
		handleWebhookList()
	default:
		fmt.Printf("Unknown webhook command: %s\n", cmd)
		fmt.Println("Run 'gaia webhook' for usage.")
	}
}

func handleWebhookStart() {
	addr := os.Getenv("GAIA_WEBHOOK_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	listener := webhook.NewListener(addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down webhook listener...")
		cancel()
	}()

	fmt.Printf("Starting webhook listener on %s...\n", addr)
	if err := listener.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting webhook listener: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Webhook listener running. Press Ctrl+C to stop.")
	<-ctx.Done()

	if err := listener.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping webhook listener: %v\n", err)
	}
	fmt.Println("Webhook listener stopped.")
}

func handleWebhookList() {
	addr := os.Getenv("GAIA_WEBHOOK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	listener := webhook.NewListener(addr)

	subs := listener.ListSubscriptions()
	if len(subs) == 0 {
		fmt.Println("No webhook subscriptions configured.")
		fmt.Println("Add subscriptions in config.yaml under webhook.subscriptions.")
		return
	}

	fmt.Printf("Webhook subscriptions (%d):\n", len(subs))
	for _, s := range subs {
		status := "disabled"
		if s.Enabled {
			status = "enabled"
		}
		fmt.Printf("  %s: %s (%s) → %s\n", s.ID, s.Name, status, s.Action)
		fmt.Printf("    Events: %v\n", s.Events)
	}
}
