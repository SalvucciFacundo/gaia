package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gaia/internal/plugins"
)

// handlePluginCLI implements the "gaia plugin" subcommand family.
func handlePluginCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gaia plugin <command> [args]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  list           List all installed plugins")
		fmt.Println("  enable <name>  Enable a plugin")
		fmt.Println("  disable <name> Disable a plugin")
		fmt.Println("  install <path> Install a plugin from a directory")
		fmt.Println("  remove <name>  Remove a plugin")
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving home directory: %v\n", err)
		os.Exit(1)
	}

	pluginsDir := filepath.Join(homeDir, ".gaia", "plugins")
	mgr := plugins.NewManager(pluginsDir)
	if err := mgr.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading plugins: %v\n", err)
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list":
		all := mgr.List()
		if len(all) == 0 {
			fmt.Println("No plugins installed.")
			return
		}
		fmt.Printf("Plugins (%d):\n", len(all))
		for _, p := range all {
			status := "disabled"
			if p.Enabled {
				status = "enabled"
			}
			fmt.Printf("  %-20s v%-10s %-10s %s\n", p.Manifest.Name, p.Manifest.Version, status, p.Manifest.Description)
			if len(p.Manifest.Tools) > 0 {
				fmt.Printf("    Tools: %v\n", p.Manifest.Tools)
			}
		}

	case "enable":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia plugin enable <name>")
			return
		}
		if err := mgr.Enable(cmdArgs[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin %q enabled.\n", cmdArgs[0])

	case "disable":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia plugin disable <name>")
			return
		}
		if err := mgr.Disable(cmdArgs[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin %q disabled.\n", cmdArgs[0])

	case "install":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia plugin install <path>")
			return
		}
		srcDir := cmdArgs[0]
		name := filepath.Base(srcDir)
		if err := mgr.Install(name, srcDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin %q installed.\n", name)

	case "remove":
		if len(cmdArgs) == 0 {
			fmt.Println("Usage: gaia plugin remove <name>")
			return
		}
		if err := mgr.Remove(cmdArgs[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin %q removed.\n", cmdArgs[0])

	default:
		fmt.Printf("Unknown plugin command: %s\n", cmd)
		fmt.Println("Run 'gaia plugin' for usage.")
	}
}
