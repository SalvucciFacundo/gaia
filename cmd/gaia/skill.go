package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"gaia/internal/config"
	"gaia/internal/skills"
)

// handleSkillCLI is the entrypoint called by main.go for `gaia skill` and `gaia skills`.
func handleSkillCLI(args []string) {
	code := RunSkillCLI(args, os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}

// RunSkillCLI executes the skill subcommand CLI.
func RunSkillCLI(args []string, stdout, stderr io.Writer) int {
	return RunSkillCLIWithIn(args, os.Stdin, stdout, stderr)
}

// RunSkillCLIWithIn executes the skill subcommand CLI with an explicit stdin reader.
func RunSkillCLIWithIn(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSkillUsage(stdout)
		return 0
	}

	subcommand := args[0]
	if subcommand == "-h" || subcommand == "--help" || subcommand == "-help" || subcommand == "help" {
		printSkillUsage(stdout)
		return 0
	}

	subArgs := args[1:]

	switch subcommand {
	case "list", "ls":
		return runSkillList(subArgs, stdout, stderr)
	case "create", "new", "add":
		return runSkillCreate(subArgs, stdout, stderr)
	case "show", "get", "cat", "view":
		return runSkillShow(subArgs, stdout, stderr)
	case "delete", "remove", "rm":
		return runSkillDelete(subArgs, stdin, stdout, stderr)
	case "search":
		return runSkillSearch(subArgs, stdout, stderr)
	case "activate":
		return runSkillActivate(subArgs, stdout, stderr)
	case "deactivate":
		return runSkillDeactivate(subArgs, stdout, stderr)
	case "install":
		return runSkillInstall(subArgs, stdout, stderr)
	case "add-tap":
		return runSkillAddTap(subArgs, stdout, stderr)
	case "remove-tap":
		return runSkillRemoveTap(subArgs, stdout, stderr)
	case "list-taps":
		return runSkillListTaps(subArgs, stdout, stderr)
	case "stats":
		return runSkillStats(subArgs, stdout, stderr)
	case "audit":
		return runSkillAudit(subArgs, stdout, stderr)
	case "search-hub":
		query := ""
		if len(subArgs) > 0 {
			query = subArgs[0]
		}
		searchSkillsHub(query)
		return 0
	default:
		fmt.Fprintf(stderr, "gaia skill: unknown subcommand '%s'\n", subcommand)
		printSkillUsage(stderr)
		return 2
	}
}

func initSkillHub(configPath string, stderr io.Writer) (*skills.Hub, string, string, error) {
	homeDir := config.BaseHome()
	if configPath != "" {
		// If custom config path is provided, use its directory or GAIA_HOME
		if dir := filepath.Dir(configPath); dir != "." && dir != "" {
			homeDir = dir
		}
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		projectRoot = "."
	}

	bundledDir := filepath.Join(projectRoot, "skills")
	userSkillsDir := filepath.Join(homeDir, "skills")

	hub := skills.NewHub(bundledDir, userSkillsDir)
	return hub, bundledDir, userSkillsDir, nil
}

func parseFlagAndPositionalArgs(args []string, valueFlags []string) ([]string, []string) {
	var positional []string
	var flagArgs []string

	isValFlag := func(arg string) bool {
		for _, vf := range valueFlags {
			if arg == "-"+vf || arg == "--"+vf {
				return true
			}
		}
		return false
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if isValFlag(arg) && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return positional, flagArgs
}

func runSkillList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill list", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	jsonOutput := fs.Bool("json", false, "output skills in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia skill list [flags]\n\n")
		fmt.Fprintf(stdout, "List all installed skills.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia skill list: %v\n", err)
		return 2
	}

	hub, _, _, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}

	list := hub.List()
	if list == nil {
		list = []skills.SkillMeta{}
	}

	type skillListItem struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Trigger     string `json:"trigger"`
		Source      string `json:"source"`
		Status      string `json:"status"`
	}

	items := make([]skillListItem, 0, len(list))
	for _, s := range list {
		status := "inactive"
		if hub.IsActive(s.Name) {
			status = "active"
		}
		trigger := s.Trigger
		if trigger == "" && len(s.Triggers) > 0 {
			trigger = strings.Join(s.Triggers, ", ")
		}
		items = append(items, skillListItem{
			Name:        s.Name,
			Description: s.Description,
			Trigger:     trigger,
			Source:      s.Source,
			Status:      status,
		})
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "gaia skill list: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if len(list) == 0 {
		fmt.Fprintln(stdout, "No skills found. Use 'gaia skill create <name>' to create one.")
		return 0
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tTRIGGER\tSOURCE\tSTATUS\tDESCRIPTION")
	for _, item := range items {
		desc := item.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		trig := item.Trigger
		if trig == "" {
			trig = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", item.Name, trig, item.Source, item.Status, desc)
	}
	_ = w.Flush()
	return 0
}

func runSkillCreate(args []string, stdout, stderr io.Writer) int {
	positionalArgs, flagArgs := parseFlagAndPositionalArgs(args, []string{"desc", "trigger", "body", "config"})

	fs := flag.NewFlagSet("skill create", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	desc := fs.String("desc", "", "short description of the skill")
	trigger := fs.String("trigger", "", "trigger keywords for the skill")
	body := fs.String("body", "", "markdown body instructions for the skill")
	force := fs.Bool("force", false, "overwrite existing skill if present")
	overwrite := fs.Bool("overwrite", false, "alias for -force")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia skill create <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Scaffold a new agentskills.io-compliant skill.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia skill create: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "gaia skill create: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	if err := skills.ValidateSkillName(name); err != nil {
		fmt.Fprintf(stderr, "gaia skill create: %v\n", err)
		return 2
	}

	hub, _, userSkillsDir, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}

	canOverwrite := *force || *overwrite

	// Check if already exists
	if !canOverwrite {
		if _, exists := hub.GetSkill(name); exists {
			fmt.Fprintf(stderr, "gaia skill create: skill %q already exists (use --overwrite to replace)\n", name)
			return 1
		}
	}

	skillDir := filepath.Join(userSkillsDir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "gaia skill create: creating directory: %v\n", err)
		return 1
	}

	description := *desc
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Procedural instructions for %s", name)
	}

	bodyContent := strings.TrimSpace(*body)
	var fullContent string

	if bodyContent != "" {
		if strings.HasPrefix(bodyContent, "---") {
			fullContent = bodyContent + "\n"
		} else {
			fullContent = fmt.Sprintf(`---
name: %s
description: %s
trigger: %s
license: Apache-2.0
metadata:
  author: user
  version: "1.0"
---

%s
`, name, description, *trigger, bodyContent)
		}
	} else {
		fullContent = fmt.Sprintf(`---
name: %s
description: %s
trigger: %s
license: Apache-2.0
metadata:
  author: user
  version: "1.0"
---

## When to Use
Describe when this skill should be applied by the agent.

## Critical Rules
1. Follow standard operational guidelines.

## Workflow
1. Step 1: Initial preparation.
2. Step 2: Main execution procedure.

## Examples
Provide concrete examples of input scenarios and expected outcomes.
`, name, description, *trigger)
	}

	filePath := filepath.Join(skillDir, "SKILL.md")
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(fullContent), 0o600); err != nil {
		fmt.Fprintf(stderr, "gaia skill create: writing file: %v\n", err)
		return 1
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(stderr, "gaia skill create: finalizing file: %v\n", err)
		return 1
	}

	_ = hub.Reload()
	_ = hub.Activate(name)

	fmt.Fprintf(stdout, "Skill %q created successfully at %s\n", name, filePath)
	return 0
}

func runSkillShow(args []string, stdout, stderr io.Writer) int {
	positionalArgs, flagArgs := parseFlagAndPositionalArgs(args, []string{"config"})

	fs := flag.NewFlagSet("skill show", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	raw := fs.Bool("raw", false, "display raw skill file with frontmatter")
	jsonOutput := fs.Bool("json", false, "output skill details in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia skill show <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Show detailed information and instructions for a skill.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia skill show: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "gaia skill show: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	hub, bundledDir, userSkillsDir, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}

	skill, ok := hub.GetSkill(name)
	if !ok {
		fmt.Fprintf(stderr, "gaia skill show: skill %q not found\n", name)
		return 1
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(skill, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "gaia skill show: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if *raw {
		paths := []string{
			filepath.Join(userSkillsDir, name, "SKILL.md"),
			filepath.Join(bundledDir, name, "SKILL.md"),
		}
		var rawBytes []byte
		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil {
				rawBytes = data
				break
			}
		}
		if len(rawBytes) > 0 {
			fmt.Fprint(stdout, string(rawBytes))
		} else {
			trig := skill.Meta.Trigger
			if trig == "" && len(skill.Meta.Triggers) > 0 {
				trig = strings.Join(skill.Meta.Triggers, ", ")
			}
			fmt.Fprintf(stdout, "---\nname: %s\ndescription: %s\ntrigger: %s\n---\n\n%s\n",
				skill.Meta.Name, skill.Meta.Description, trig, skill.Content)
		}
		return 0
	}

	trig := skill.Meta.Trigger
	if trig == "" && len(skill.Meta.Triggers) > 0 {
		trig = strings.Join(skill.Meta.Triggers, ", ")
	}

	status := "inactive"
	if hub.IsActive(skill.Meta.Name) {
		status = "active"
	}

	fmt.Fprintf(stdout, "Skill:       %s\n", skill.Meta.Name)
	fmt.Fprintf(stdout, "Description: %s\n", skill.Meta.Description)
	if trig != "" {
		fmt.Fprintf(stdout, "Trigger:     %s\n", trig)
	}
	fmt.Fprintf(stdout, "Source:      %s\n", skill.Meta.Source)
	fmt.Fprintf(stdout, "Status:      %s\n", status)
	fmt.Fprintf(stdout, "\n%s\n", skill.Content)
	return 0
}

func runSkillDelete(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	positionalArgs, flagArgs := parseFlagAndPositionalArgs(args, []string{"config"})

	fs := flag.NewFlagSet("skill delete", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	yes := fs.Bool("yes", false, "confirm deletion without interactive prompt")
	force := fs.Bool("force", false, "alias for -yes")
	fShort := fs.Bool("f", false, "alias for -yes")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: gaia skill delete <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Delete an installed skill from disk.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "gaia skill delete: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "gaia skill delete: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	hub, _, userSkillsDir, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}

	skill, ok := hub.GetSkill(name)
	if !ok {
		fmt.Fprintf(stderr, "gaia skill delete: skill %q not found\n", name)
		return 1
	}

	if skill.Meta.Source == "bundled" {
		fmt.Fprintf(stderr, "gaia skill delete: cannot delete bundled skill %q (bundled skills are read-only)\n", name)
		return 1
	}

	confirmed := *yes || *force || *fShort
	if !confirmed {
		fmt.Fprintf(stdout, "Are you sure you want to delete skill %q? [y/N]: ", name)
		reader := bufio.NewReader(stdin)
		input, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(stderr, "gaia skill delete: reading confirmation: %v\n", err)
			return 1
		}
		trimmed := strings.ToLower(strings.TrimSpace(input))
		if trimmed != "y" && trimmed != "yes" {
			fmt.Fprintln(stdout, "Deletion canceled.")
			return 0
		}
	}

	nestedDir := filepath.Join(userSkillsDir, name)
	flatFile := filepath.Join(userSkillsDir, name+".md")
	_ = os.RemoveAll(nestedDir)
	_ = os.Remove(flatFile)

	_ = hub.Remove(name)
	_ = hub.Reload()

	fmt.Fprintf(stdout, "Skill %q deleted successfully.\n", name)
	return 0
}

func runSkillSearch(args []string, stdout, stderr io.Writer) int {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	results := hub.Search(query)
	if len(results) == 0 {
		fmt.Fprintln(stdout, "No skills found.")
		return 0
	}
	fmt.Fprintf(stdout, "Found %d matching skill(s):\n", len(results))
	for _, s := range results {
		status := "inactive"
		if hub.IsActive(s.Name) {
			status = "active"
		}
		fmt.Fprintf(stdout, "  %-30s [%s] → %s: %s\n", s.Name, s.Source, status, s.Description)
	}
	return 0
}

func runSkillActivate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: gaia skill activate <name>")
		return 2
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	name := args[0]
	if err := hub.Activate(name); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Skill %q activated.\n", name)
	return 0
}

func runSkillDeactivate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: gaia skill deactivate <name>")
		return 2
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	name := args[0]
	if err := hub.Deactivate(name); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Skill %q deactivated.\n", name)
	return 0
}

func runSkillInstall(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: gaia skill install <name>")
		return 2
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	name := args[0]
	if err := hub.Install(name); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Skill %q installed and activated.\n", name)
	return 0
}

func runSkillAddTap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: gaia skill add-tap <url> [--branch <branch>]")
		return 2
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	url := args[0]
	branch := "main"
	if len(args) > 2 && args[1] == "--branch" {
		branch = args[2]
	}
	if err := hub.AddTap(url, branch); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Tap added: %s\n", url)
	return 0
}

func runSkillRemoveTap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: gaia skill remove-tap <url>")
		return 2
	}
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	if err := hub.RemoveTap(args[0]); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Tap removed: %s\n", args[0])
	return 0
}

func runSkillListTaps(args []string, stdout, stderr io.Writer) int {
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	taps, err := hub.ListTaps()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if len(taps) == 0 {
		fmt.Fprintln(stdout, "No taps installed.")
		return 0
	}
	fmt.Fprintf(stdout, "Installed taps (%d):\n", len(taps))
	for _, t := range taps {
		url := t.URL
		if url == "" {
			url = t.InstalledPath
		}
		fmt.Fprintf(stdout, "  %-50s %d skills\n", url, t.SkillCount)
	}
	return 0
}

func runSkillStats(args []string, stdout, stderr io.Writer) int {
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	stats := hub.UsageStats()
	if len(stats) == 0 {
		fmt.Fprintln(stdout, "No skills installed.")
		return 0
	}
	fmt.Fprintf(stdout, "Skill usage stats (%d total):\n", len(stats))
	fmt.Fprintf(stdout, "%-30s %-8s %-7s %-8s %s\n", "Name", "Loads", "Active", "Source", "Last Used")
	fmt.Fprintln(stdout, strings.Repeat("-", 80))
	for _, s := range stats {
		active := "inactive"
		if s.Active {
			active = "active"
		}
		lastUsed := "-"
		if !s.LastUsed.IsZero() {
			lastUsed = s.LastUsed.Format("15:04 02/01")
		}
		fmt.Fprintf(stdout, "%-30s %-8d %-7s %-8s %s\n", s.Name, s.LoadCount, active, s.Source, lastUsed)
	}
	return 0
}

func runSkillAudit(args []string, stdout, stderr io.Writer) int {
	hub, _, _, err := initSkillHub("", stderr)
	if err != nil {
		return 1
	}
	results := hub.AuditAll()
	fmt.Fprint(stdout, skills.FormatAuditResults(results))
	return 0
}

func printSkillUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: gaia skill <command> [flags]\n\n")
	fmt.Fprintf(w, "Manage GAIA skills, discovery, and agentskills.io conformance.\n\n")
	fmt.Fprintf(w, "Commands:\n")
	fmt.Fprintf(w, "  list     List all installed skills\n")
	fmt.Fprintf(w, "  create   Scaffold a new skill\n")
	fmt.Fprintf(w, "  show     Show skill details and instructions\n")
	fmt.Fprintf(w, "  delete   Delete an installed skill\n\n")
	fmt.Fprintf(w, "Use 'gaia skill <command> --help' for more information about a command.\n")
}
