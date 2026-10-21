package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/diff"
)

// Insights reports conversation metrics and costs.
func (d *Dispatcher) Insights(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msgCount, _ := repo.GetMessageCount(ctx)
	history, _ := repo.GetHistory(ctx, 200)

	var userMsgs, assistantMsgs, toolMsgs, systemMsgs int
	for _, m := range history {
		switch m.Role {
		case domain.RoleUser:
			userMsgs++
		case domain.RoleAssistant:
			assistantMsgs++
		case domain.RoleTool:
			toolMsgs++
		case domain.RoleSystem:
			systemMsgs++
		}
	}

	sessID := d.brain.CurrentSessionID()
	if len(sessID) > 12 {
		sessID = sessID[:12]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session Insights\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Total messages:  %d\n", msgCount))
	sb.WriteString(fmt.Sprintf("  User:          %d\n", userMsgs))
	sb.WriteString(fmt.Sprintf("  AI:            %d\n", assistantMsgs))
	sb.WriteString(fmt.Sprintf("  Tool calls:    %d\n", toolMsgs))
	sb.WriteString(fmt.Sprintf("  System:        %d\n", systemMsgs))
	sb.WriteString(fmt.Sprintf("  Compactions:   %d\n\n", d.brain.CompactedTo()))
	sb.WriteString(fmt.Sprintf("Model:           %s\n", d.brain.ProviderName()))
	sb.WriteString(fmt.Sprintf("Session ID:      %s\n", sessID))
	sb.WriteString(fmt.Sprintf("Budget used:     %d/%d iterations\n", len(history), d.brain.Budget().MaxIterations))

	if ct := d.brain.CostTracker(); ct != nil {
		summary := ct.GetSummary()
		sb.WriteString(fmt.Sprintf("\nCost:           $%.2f (%.1fK in / %.1fK out)\n", summary.TotalCost, float64(summary.TotalInput)/1000, float64(summary.TotalOutput)/1000))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// DebugInfo collects system information for debugging.
func (d *Dispatcher) DebugInfo(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "gaia", "config.yaml")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GAIA Debug Report\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Go version:     %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("Provider:       %s\n", d.brain.ProviderName()))
	sb.WriteString(fmt.Sprintf("Model:          %s\n", d.brain.ModelName()))
	sb.WriteString(fmt.Sprintf("Config:         %s\n", configPath))
	sb.WriteString(fmt.Sprintf("Session Msgs:   %d\n", func() int { n, _ := repo.GetMessageCount(ctx); return n }()))
	sb.WriteString(fmt.Sprintf("Budget:         %d max iterations\n", d.brain.Budget().MaxIterations))
	sb.WriteString(fmt.Sprintf("Compaction:     %d threshold, %d compacted\n", d.brain.Budget().CompactionThreshold, d.brain.CompactedTo()))
	sb.WriteString(fmt.Sprintf("Policy tier:    %s\n", func() string {
		if p := d.brain.Policy(); p != nil {
			return string(p.Tier())
		}
		return "not configured"
	}()))
	if goal := d.brain.Goal(); len(goal) > 0 {
		sb.WriteString(fmt.Sprintf("Active goal:    %s\n", goal))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// CreditsInfo shows credit/usage balance information.
func (d *Dispatcher) CreditsInfo(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Credit/Billing Info\n" + strings.Repeat("─", 40) + "\n\n" +
			"GAIA doesn't have built-in billing. Credit information depends on your LLM provider:\n\n" +
			"  • OpenRouter: https://openrouter.ai/credits\n" +
			"  • OpenAI:     https://platform.openai.com/usage\n" +
			"  • Anthropic:  https://console.anthropic.com/settings/usage\n" +
			"  • Copilot:    Included in GitHub subscription\n\n" +
			"For session-level cost tracking, enable cost_tracker in config.yaml\n" +
			"and use /insights to see per-session costs.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// BillingInfo shows billing management information.
func (d *Dispatcher) BillingInfo(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Billing management is handled by your LLM provider:\n\n" +
			"  • OpenRouter: https://openrouter.ai/billing\n" +
			"  • OpenAI:     https://platform.openai.com/account/billing\n" +
			"  • Anthropic:  https://console.anthropic.com/settings/billing\n" +
			"  • Copilot:    https://github.com/settings/billing\n\n" +
			"Use /insights to track your session-level LLM costs.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// CommandsList delegates to HelpCommands.
func (d *Dispatcher) CommandsList(ctx context.Context) error {
	return d.HelpCommands(ctx)
}

// RestartGateway performs a graceful restart by spawning a new process and exiting.
func (d *Dispatcher) RestartGateway(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Restarting GAIA...",
	}
	_ = repo.SaveMessage(ctx, msg)
	_ = ui.Display(msg)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
	}()

	os.Exit(0)
	return nil
}

type updateReleaseInfo struct {
	Available   bool
	Tag         string
	DownloadURL string
	Size        string
}

func checkLatestRelease() (*updateReleaseInfo, error) {
	resp, err := http.Get("https://api.github.com/repos/SalvucciFacundo/gaia/releases/latest")
	if err != nil {
		return &updateReleaseInfo{
			Available:   false,
			Tag:         "",
			DownloadURL: "https://github.com/SalvucciFacundo/gaia/releases",
			Size:        "",
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int    `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if release.TagName == "" || release.TagName == core.GAIAVersion {
		return &updateReleaseInfo{Available: false}, nil
	}

	arch := runtime.GOOS + "-" + runtime.GOARCH
	var downloadURL string
	var size int
	for _, a := range release.Assets {
		if strings.Contains(a.Name, arch) || strings.Contains(a.Name, runtime.GOOS) {
			downloadURL = a.BrowserDownloadURL
			size = a.Size
			break
		}
	}
	if downloadURL == "" && len(release.Assets) > 0 {
		downloadURL = release.Assets[0].BrowserDownloadURL
		size = release.Assets[0].Size
	}

	sizeStr := fmt.Sprintf("%.1f MB", float64(size)/1_000_000)
	return &updateReleaseInfo{
		Available:   true,
		Tag:         release.TagName,
		DownloadURL: downloadURL,
		Size:        sizeStr,
	}, nil
}

func downloadAndReplace(execPath, downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("no download URL available")
	}

	tmpPath := execPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write download: %w", err)
	}
	out.Close()

	oldPath := execPath + ".old"
	os.Remove(oldPath)
	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Rename(oldPath, execPath)
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	os.Chmod(execPath, 0755)
	return nil
}

// UpdateGAIA checks for updates and applies them automatically.
func (d *Dispatcher) UpdateGAIA(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: "Checking for updates..."}
	_ = repo.SaveMessage(ctx, msg)
	_ = ui.Display(msg)

	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	latest, err := checkLatestRelease()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update check failed: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if !latest.Available {
		msg := domain.Message{Role: domain.RoleSystem, Content: "You're on the latest version. No update needed.\nCheck manually: https://github.com/SalvucciFacundo/gaia/releases"}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Downloading %s (%s)...", latest.Tag, latest.Size)}
	_ = repo.SaveMessage(ctx, msg)
	_ = ui.Display(msg)

	if err := downloadAndReplace(execPath, latest.DownloadURL); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update failed: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Updated to %s! Restarting...", latest.Tag)}
	_ = repo.SaveMessage(ctx, msg)
	_ = ui.Display(msg)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
	}()
	os.Exit(0)
	return nil
}

// TopicHelp displays usage for multi-session DM topics.
func (d *Dispatcher) TopicHelp(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Multi-session DM mode (Telegram-specific):\n\n" +
			"  /topic list           — Show active topics\n" +
			"  /topic new <name>     — Start a new topic\n" +
			"  /topic switch <name>  — Switch to a topic\n" +
			"  /topic close <name>   — Close a topic\n\n" +
			"Each topic has its own conversation history.\n" +
			"Useful for parallel conversations in one chat.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// TopicSession switches to a named topic/session.
func (d *Dispatcher) TopicSession(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Switched to topic: %s\nAll messages will be scoped to this topic until you switch back.", name),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// HelpCommands is the shared implementation for both /help and /commands.
func (d *Dispatcher) HelpCommands(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	help := `Available Commands:

  Session:     /new, /clear, /history, /save, /resume, /sessions, /title, /compress, /undo, /retry
  State:       /branch, /branches, /snapshot save, /snapshot load
  Goals:       /goal, /subgoal, /goals, /goal clear
  Queue/Steer: /queue, /q, /steer
  Background:  /background, /moa, /tasks, /cancel
  Handoff:     /handoff telegram, /handoff discord, /handoff cli
  Config:      /model, /reasoning, /personality, /yolo, /verbose, /timestamps, /statusbar, /footer, /indicator, /skin
  Permissions: /permisos, /trust
  Skills:      /skills, /learn, /suggestions, /blueprint, /curator
  Cron:        /cron list, /cron add, /cron remove
  Memory:      /memory pending, /memory approve, /memory reject
  Info:        /help, /version, /platforms, /copy, /insights, /debug, /credits, /billing
  Media:       /image, /paste
  Messaging:   /sethome, /approve, /deny, /commands, /restart, /update, /topic

For details: gaia help or check the README.`
	msg := domain.Message{Role: domain.RoleSystem, Content: help}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// DiffView renders the current git diff in unified format or reports clean tree.
func (d *Dispatcher) DiffView(ctx context.Context, staged bool) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	args := []string{"diff"}
	targetName := "working tree"
	if staged {
		args = append(args, "--staged")
		targetName = "staging area"
	}
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Error running git diff: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	files, err := diff.ParseUnifiedDiff(string(output))
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Error parsing git diff: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if len(files) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Working tree clean — no modifications in %s.", targetName)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Git Diff (%s):\n\n", targetName))
	for _, f := range files {
		sb.WriteString(diff.RenderDiffFile(f, 80, -1))
		sb.WriteString("\n")
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// AuditLog displays the policy audit trail.
func (d *Dispatcher) AuditLog(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	logs := d.brain.AuditEntries()
	if len(logs) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log is empty.\n\nSet a tool override to 'audit' via /permisos to start logging.\nExample: set shell_exec → audit to log all shell commands."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Policy Audit Log (%d entries)\n%s\n\n", len(logs), strings.Repeat("─", 40)))
	start := 0
	if len(logs) > 50 {
		start = len(logs) - 50
		sb.WriteString(fmt.Sprintf("Showing last 50 of %d entries.\n\n", len(logs)))
	}
	for i := start; i < len(logs); i++ {
		e := logs[i]
		status := "ALLOW"
		if !e.Allowed {
			status = "DENY"
		}
		preview := e.Args
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %-20s %s\n", e.Time.Format("15:04:05"), status, e.Tool, preview))
	}
	sb.WriteString("\nKey: [ALLOW] audited and permitted  [DENY] audited and blocked\n")
	sb.WriteString("Use /audit clear to clear the log.")

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// AuditClear clears the policy audit log.
func (d *Dispatcher) AuditClear(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	d.brain.ClearAuditEntries()
	msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log cleared."}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SessionStatus reports current multi-platform session status.
func (d *Dispatcher) SessionStatus(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	sm := d.brain.SessionManager()
	if sm == nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Session manager is not configured. Running in default single-session mode.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sm.Status()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SessionCommand handles /session subcommands.
func (d *Dispatcher) SessionCommand(ctx context.Context, arg string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	sm := d.brain.SessionManager()
	if sm == nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Session manager is not configured. Running in default single-session mode.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	arg = strings.ToLower(strings.TrimSpace(arg))
	switch arg {
	case "unify":
		sm.SetMode(core.SessionUnify)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to **unify**. All platforms now share one conversation."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	case "isolate":
		sm.SetMode(core.SessionIsolate)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to **isolate**. Each platform has its own separate conversation."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	case "ask":
		sm.SetMode(core.SessionAsk)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to **ask**. I will ask how to handle new platforms when they message."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	default:
		return d.SessionStatus(ctx)
	}
}
