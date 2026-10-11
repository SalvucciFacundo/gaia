package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// onboardStep holds a step in the guided SDD walkthrough.
type onboardStep struct {
	title       string
	description string
	action      string
}

// handleOnboard implements the "gaia onboard" guided SDD walkthrough.
// It walks users through a real example: explore → propose → spec → design → tasks → apply → verify.
// Usage: gaia onboard
func handleOnboard(args []string) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║        Welcome to GAIA SDD Onboarding!                  ║")
	fmt.Println("║  Guided walkthrough of the Spec-Driven Development      ║")
	fmt.Println("║  workflow with a real example.                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	steps := []onboardStep{
		{
			title:       "1. Explore (sdd-explore)",
			description: "Explore the problem space before committing to a change. The explorer subagent analyzes the request, checks existing code, and determines feasibility.",
			action:      `Send a message like: "I want to add dark mode support" — GAIA detects this as an SDD trigger and starts the Explorer subagent.`,
		},
		{
			title:       "2. Propose (sdd-propose)",
			description: "Create a change proposal with intent, scope, approach, and rollback plan. The proposer subagent writes the proposal to the SDD directory.",
			action:      fmt.Sprintf(`The proposal is saved to openspec/changes/<change-name>/proposal.md in your project.`),
		},
		{
			title:       "3. Spec (sdd-spec)",
			description: "Write delta specs with Given/When/Then scenarios and RFC 2119 keywords (MUST, SHALL, SHOULD). Specs are your acceptance criteria.",
			action:      `Specs go to openspec/changes/<change-name>/specs/<capability>/spec.md. Each spec defines requirements and scenarios.`,
		},
		{
			title:       "4. Design (sdd-design)",
			description: "Create the technical design with architecture decisions, data flow diagrams, and file change plans. The designer subagent creates openspec/changes/<change-name>/design.md.",
			action:      `The design documents WHY decisions were made (rationale and alternatives).`,
		},
		{
			title:       "5. Tasks (sdd-tasks)",
			description: "Break the change into implementation tasks grouped by phase. The planner subagent creates openspec/changes/<change-name>/tasks.md.",
			action:      `Tasks are numbered hierarchically (e.g., 1.1, 1.2) and sized to be completable in one session.`,
		},
		{
			title:       "6. Apply (sdd-apply)",
			description: "Implement the tasks following the specs and design. The implementer subagent writes real code, matching existing project patterns.",
			action:      `Run "gaia exec 'implement X'" or type "+/sdd implement X" in the TUI to start implementation.`,
		},
		{
			title:       "7. Verify (sdd-verify)",
			description: "Execute tests and prove the implementation matches specs, design, and tasks. The verifier subagent runs test suites and validates acceptance criteria.",
			action:      `Verification runs "go test ./..." and checks all acceptance criteria from the specs.`,
		},
	}

	for _, step := range steps {
		fmt.Printf("━━━ %s ━━━\n", step.title)
		fmt.Println()
		fmt.Println(step.description)
		fmt.Println()
		fmt.Printf("  💡 %s\n", step.action)
		fmt.Println()
	}

	// Show the directory structure
	fmt.Println("━━━ Artifact Directory Structure ━━━")
	fmt.Println()
	fmt.Println("  openspec/")
	fmt.Println("  ├── config.yaml          # SDD configuration")
	fmt.Println("  ├── specs/               # Baseline specs (post-archive)")
	fmt.Println("  │   └── <capability>/")
	fmt.Println("  │       └── spec.md")
	fmt.Println("  └── changes/")
	fmt.Println("      ├── archive/         # Completed changes")
	fmt.Println("      └── <change-name>/   # Active change")
	fmt.Println("          ├── proposal.md")
	fmt.Println("          ├── specs/")
	fmt.Println("          │   └── <capability>/")
	fmt.Println("          │       └── spec.md")
	fmt.Println("          ├── design.md")
	fmt.Println("          └── tasks.md")
	fmt.Println()

	// Show trigger examples
	fmt.Println("━━━ SDD Triggers ━━━")
	fmt.Println()
	fmt.Println("  GAIA detects SDD intent from natural language messages:")
	fmt.Println("  • \"I need to add authentication\"   → Explorer")
	fmt.Println("  • \"Create a new endpoint for...\"   → Explorer")
	fmt.Println("  • \"Fix the bug in...\"               → Explorer")
	fmt.Println("  • \"+/sdd <task>\"                    → Force SDD pipeline")
	fmt.Println("  • \"+/direct <message>\"              → Bypass SDD (direct mode)")
	fmt.Println()

	// Show current project SDD status
	fmt.Println("━━━ Current Project Status ━━━")
	fmt.Println()

	projectRoot, err := os.Getwd()
	if err != nil {
		projectRoot = "."
	}

	// Check for existing artifacts
	changesDir := filepath.Join(projectRoot, "openspec", "changes")
	hasChanges := false
	if entries, err := os.ReadDir(changesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != "archive" {
				if !hasChanges {
					fmt.Println("  Active changes:")
					hasChanges = true
				}
				fmt.Printf("    📂 %s\n", e.Name())
			}
		}
	}

	if !hasChanges {
		fmt.Println("  No active changes yet. Start by sending a task to GAIA!")
	}

	archiveDir := filepath.Join(projectRoot, "openspec", "changes", "archive")
	if entries, err := os.ReadDir(archiveDir); err == nil && len(entries) > 0 {
		fmt.Printf("  📦 %d archived change(s)\n", len(entries))
	}

	fmt.Println()

	// Show ready state
	fmt.Println("━━━ Ready to Begin? ━━━")
	fmt.Println()
	fmt.Println("  Launch the TUI:       gaia")
	fmt.Println("  Headless execution:   gaia exec \"your task\"")
	fmt.Println("  Run diagnostics:      gaia doctor")
	fmt.Println("  Schedule tasks:       gaia cron create \"0 2 * * *\" \"run backup\"")
	fmt.Println("  Desktop mode:          gaia desktop")
	fmt.Println()
	fmt.Printf("  Generated at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Check if --demo flag was passed for interactive demo
	demoMode := false
	for _, arg := range args {
		if arg == "--demo" {
			demoMode = true
			break
		}
	}

	if demoMode {
		runDemoWalkthrough(projectRoot)
	}
}

// runDemoWalkthrough executes a dry-run demonstration of an SDD cycle
// using a simple example: adding a "hello" command to GAIA.
func runDemoWalkthrough(projectRoot string) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║              DEMO: Adding a 'hello' command             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	demoRoot := filepath.Join(projectRoot, "openspec", "changes", "demo-hello-command")
	fmt.Printf("  Artifact root: %s\n", demoRoot)
	fmt.Println()

	steps := []string{
		"Explorer analyzes: 'Add a hello command to GAIA'",
		"Proposer writes:  demo-hello-command/proposal.md",
		"Specifier writes: demo-hello-command/specs/hello-cli/spec.md",
		"Designer writes:  demo-hello-command/design.md",
		"Planner writes:   demo-hello-command/tasks.md",
		"Implementer adds: cmd/gaia/hello.go (new file)",
		"Verifier runs:    go test ./... → all pass",
	}

	phases := []string{"Explore", "Propose", "Spec", "Design", "Tasks", "Apply", "Verify"}
	for i, step := range steps {
		fmt.Printf("  [%s] %s\n", phases[i], step)
		// Simulate delay for readability
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("  ✅ Demo complete! The actual SDD pipeline would:")
	fmt.Println("     1. Create real artifact files")
	fmt.Println("     2. Write production code")
	fmt.Println("     3. Run real tests")
	fmt.Println("     4. Commit and archive the change")
	fmt.Println()

	fmt.Printf("  Artifact tree created at: %s\n", demoRoot)
	fmt.Println("  " + strings.Repeat("─", 55))
	fmt.Println("  demo-hello-command/")
	fmt.Println("  ├── proposal.md      # Intent, scope, approach, rollback")
	fmt.Println("  ├── specs/")
	fmt.Println("  │   └── hello-cli/")
	fmt.Println("  │       └── spec.md  # Requirements with Given/When/Then")
	fmt.Println("  ├── design.md        # Architecture decisions and data flow")
	fmt.Println("  └── tasks.md         # Implementation tasks (1.1, 1.2...)")
	fmt.Println()

	fmt.Println("  Ready to try for real? Send GAIA an SDD-triggering message!")
	fmt.Println()
}
