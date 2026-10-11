package main

import (
	"context"
	"fmt"
	"os"

	"gaia/internal/lsp"
)

// handleLSPCLI implements the "gaia lsp" subcommand family.
func handleLSPCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia lsp <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  list              List configured LSP servers")
		fmt.Println("  diagnostics <name> Get diagnostics from an LSP server")
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list":
		handleLSPList()
	case "diagnostics":
		handleLSPDiagnostics(cmdArgs)
	default:
		fmt.Printf("Unknown lsp command: %s\n", cmd)
		fmt.Println("Run 'gaia lsp' for usage.")
	}
}

func handleLSPList() {
	fmt.Println("Configured LSP servers:")
	fmt.Println("  Add servers in config.yaml under lsp.servers.")
	fmt.Println("  Example:")
	fmt.Println("    lsp:")
	fmt.Println("      servers:")
	fmt.Println("        - name: gopls")
	fmt.Println("          command: gopls")
	fmt.Println("          args: [\"serve\"]")
	fmt.Println("          workspace: .")
}

func handleLSPDiagnostics(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia lsp diagnostics <name>")
		return
	}

	name := args[0]
	cfg := lsp.ServerConfig{
		Name:    name,
		Command: name, // Default: command name matches server name (e.g., "gopls").
		Args:    nil,
	}

	// Try common configurations.
	switch name {
	case "gopls":
		cfg.Command = "gopls"
		cfg.Args = []string{"serve"}
	case "pylsp":
		cfg.Command = "pylsp"
	}

	workspace, _ := os.Getwd()
	cfg.Workspace = workspace

	client := lsp.NewClient(cfg)
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %v\n", name, err)
		os.Exit(1)
	}
	defer client.Close()

	diags, err := client.Diagnostics(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting diagnostics: %v\n", err)
		os.Exit(1)
	}

	output := lsp.FormatDiagnostics(diags)
	fmt.Println(output)
}
