package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gaia/internal/core"
	"gopkg.in/yaml.v3"
)

// handlePolicyCLI implements "gaia policy <subcommand>".
// Usage: gaia policy init [--tier=read|sandbox|full] [--global]
func handlePolicyCLI(args []string) {
	fs := flag.NewFlagSet("policy", flag.ExitOnError)
	tier := fs.String("tier", "sandbox", "Default tier: read, sandbox, or full")
	isGlobal := fs.Bool("global", false, "Create global policy instead of project-level")

	fs.Parse(args)

	sub := fs.Arg(0)
	switch sub {
	case "init":
		handlePolicyInit(*tier, *isGlobal)
	default:
		fmt.Fprintln(os.Stderr, "Usage: gaia policy init [--tier=read|sandbox|full] [--global]")
		os.Exit(1)
	}
}

func handlePolicyInit(tier string, isGlobal bool) {
	// Validate tier
	validTiers := map[string]bool{"read": true, "sandbox": true, "full": true}
	if !validTiers[tier] {
		fmt.Fprintf(os.Stderr, "Error: invalid tier %q. Use: read, sandbox, or full\n", tier)
		os.Exit(1)
	}

	var targetDir string
	var scopeName string

	if isGlobal {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot find home directory: %v\n", err)
			os.Exit(1)
		}
		targetDir = filepath.Join(home, ".config", "gaia")
		scopeName = "global"
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot get current directory: %v\n", err)
			os.Exit(1)
		}
		targetDir = filepath.Join(cwd, ".gaia")
		scopeName = "project"
	}

	targetFile := filepath.Join(targetDir, "policy.yaml")

	// Check if exists
	if _, err := os.Stat(targetFile); err == nil {
		data, _ := os.ReadFile(targetFile)
		fmt.Printf("Policy file already exists at %s:\n\n%s\n", targetFile, string(data))
		overwrite := readYesNo("Overwrite? (y/N): ")
		if !overwrite {
			fmt.Println("Cancelled.")
			return
		}
	}

	// Create directory
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create directory %s: %v\n", targetDir, err)
		os.Exit(1)
	}

	cfg := &core.PolicyConfig{
		Tier: core.PolicyTier(tier),
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot marshal policy: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(targetFile, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot write %s: %v\n", targetFile, err)
		os.Exit(1)
	}

	fmt.Printf("Created %s policy: %s\n", scopeName, targetFile)
	fmt.Printf("  Tier: %s\n", tier)
	fmt.Println()
	fmt.Println("Edit this file to add tool overrides or deny rules.")
	fmt.Println("Use /permisos inside GAIA to view and change policies at runtime.")
}

// readYesNo prompts for yes/no input and returns true for y/Y.
func readYesNo(prompt string) bool {
	fmt.Print(prompt)
	var resp string
	fmt.Scanln(&resp)
	return resp == "y" || resp == "Y"
}
