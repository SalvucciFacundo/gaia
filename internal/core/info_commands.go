package core

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

	"gaia/internal/core/domain"
	"gaia/internal/diff"
)

func (b *Brain) Insights(ctx context.Context) error {
	msgCount, _ := b.repo.GetMessageCount(ctx)
	history, _ := b.repo.GetHistory(ctx, 200)

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

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session Insights\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Total messages:  %d\n", msgCount))
	sb.WriteString(fmt.Sprintf("  User:          %d\n", userMsgs))
	sb.WriteString(fmt.Sprintf("  AI:            %d\n", assistantMsgs))
	sb.WriteString(fmt.Sprintf("  Tool calls:    %d\n", toolMsgs))
	sb.WriteString(fmt.Sprintf("  System:        %d\n", systemMsgs))
	sb.WriteString(fmt.Sprintf("  Compactions:   %d\n\n", b.compactedTo))
	sb.WriteString(fmt.Sprintf("Model:           %s\n", b.providerName))
	sb.WriteString(fmt.Sprintf("Session ID:      %s\n", b.currentSessionID[:min(12, len(b.currentSessionID))]))
	sb.WriteString(fmt.Sprintf("Budget used:     %d/%d iterations\n", len(history), b.budget.MaxIterations))

	if b.costTracker != nil {
		summary := b.costTracker.GetSummary()
		sb.WriteString(fmt.Sprintf("\nCost:           $%.2f (%.1fK in / %.1fK out)\n", summary.TotalCost, float64(summary.TotalInput)/1000, float64(summary.TotalOutput)/1000))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// DebugInfo collects system information for debugging.
func (b *Brain) DebugInfo(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "gaia", "config.yaml")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GAIA Debug Report\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Go version:     %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("Provider:       %s\n", b.providerName))
	sb.WriteString(fmt.Sprintf("Model:          %s\n", b.modelName))
	sb.WriteString(fmt.Sprintf("Config:         %s\n", configPath))
	sb.WriteString(fmt.Sprintf("Session Msgs:   %d\n", func() int { n, _ := b.repo.GetMessageCount(ctx); return n }()))
	sb.WriteString(fmt.Sprintf("Budget:         %d max iterations\n", b.budget.MaxIterations))
	sb.WriteString(fmt.Sprintf("Compaction:     %d threshold, %d compacted\n", b.budget.CompactionThreshold, b.compactedTo))
	sb.WriteString(fmt.Sprintf("Policy tier:    %s\n", func() string {
		if b.policy != nil {
			return string(b.policy.Tier())
		}
		return "not configured"
	}()))
	if len(b.goal) > 0 {
		sb.WriteString(fmt.Sprintf("Active goal:    %s\n", b.goal))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// CreditsInfo shows credit/usage balance information.
func (b *Brain) CreditsInfo(ctx context.Context) error {
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BillingInfo shows billing management information.
func (b *Brain) BillingInfo(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Billing management is handled by your LLM provider:\n\n" +
			"  • OpenRouter: https://openrouter.ai/billing\n" +
			"  • OpenAI:     https://platform.openai.com/account/billing\n" +
			"  • Anthropic:  https://console.anthropic.com/settings/billing\n" +
			"  • Copilot:    https://github.com/settings/billing\n\n" +
			"Use /insights to track your session-level LLM costs.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}
func (b *Brain) CommandsList(ctx context.Context) error {
	// Delegate to the help system
	return b.HelpCommands(ctx)
}

// RestartGateway performs a graceful restart by spawning a new process and exiting.
func (b *Brain) RestartGateway(ctx context.Context) error {
	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Restarting GAIA...",
	}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	// Spawn a new instance after a brief delay (allows this message to be sent)
	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
	}()

	os.Exit(0)
	return nil
}

// updateReleaseInfo holds info about a GitHub release.
type updateReleaseInfo struct {
	Available   bool
	Tag         string
	DownloadURL string
	Size        string
}

// checkLatestRelease queries GitHub for the latest GAIA release.
func checkLatestRelease() (*updateReleaseInfo, error) {
	// GitHub API endpoint for the latest release
	resp, err := http.Get("https://api.github.com/repos/SalvucciFacundo/gaia/releases/latest")
	if err != nil {
		return &updateReleaseInfo{
			Available:   false,
			Tag:         "",
			DownloadURL: "https://github.com/SalvucciFacundo/gaia/releases",
			Size:        "",
		}, nil // non-fatal — show manual link
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

	if release.TagName == "" || release.TagName == GAIAVersion {
		return &updateReleaseInfo{Available: false}, nil
	}

	// Find the right asset for this platform
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

// downloadAndReplace downloads a release binary and replaces the current executable atomically.
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

	// Atomically replace: rename current to .old, move new in place
	oldPath := execPath + ".old"
	os.Remove(oldPath)
	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Rename(oldPath, execPath) // restore backup
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	os.Chmod(execPath, 0755)

	return nil
}

// UpdateGAIA checks for updates and applies them automatically.
func (b *Brain) UpdateGAIA(ctx context.Context) error {
	msg := domain.Message{Role: domain.RoleSystem, Content: "Checking for updates..."}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	latest, err := checkLatestRelease()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update check failed: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if !latest.Available {
		msg := domain.Message{Role: domain.RoleSystem, Content: "You're on the latest version. No update needed.\nCheck manually: https://github.com/SalvucciFacundo/gaia/releases"}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Downloading %s (%s)...", latest.Tag, latest.Size)}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	if err := downloadAndReplace(execPath, latest.DownloadURL); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update failed: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Updated to %s! Restarting...", latest.Tag)}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
	}()
	os.Exit(0)
	return nil
}
func (b *Brain) TopicHelp(ctx context.Context) error {
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// TopicSession switches to a named topic/session.
func (b *Brain) TopicSession(ctx context.Context, name string) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Switched to topic: %s\nAll messages will be scoped to this topic until you switch back.", name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HelpCommands is the shared implementation for both /help and /commands.
func (b *Brain) HelpCommands(ctx context.Context) error {
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// DiffView renders the current git diff in unified format or reports clean tree.
func (b *Brain) DiffView(ctx context.Context, staged bool) error {
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
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	files, err := diff.ParseUnifiedDiff(string(output))
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Error parsing git diff: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if len(files) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Working tree clean — no modifications in %s.", targetName)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Git Diff (%s):\n\n", targetName))
	for _, f := range files {
		sb.WriteString(diff.RenderDiffFile(f, 80, -1))
		sb.WriteString("\n")
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// AuditLog displays the policy audit trail.
func (b *Brain) AuditLog(ctx context.Context) error {
	if len(b.auditLog) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log is empty.\n\nSet a tool override to 'audit' via /permisos to start logging.\nExample: set shell_exec → audit to log all shell commands."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Policy Audit Log (%d entries)\n%s\n\n", len(b.auditLog), strings.Repeat("─", 40)))
	start := 0
	if len(b.auditLog) > 50 {
		start = len(b.auditLog) - 50
		sb.WriteString(fmt.Sprintf("Showing last 50 of %d entries.\n\n", len(b.auditLog)))
	}
	for i := start; i < len(b.auditLog); i++ {
		e := b.auditLog[i]
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// AuditClear clears the policy audit log.
func (b *Brain) AuditClear(ctx context.Context) error {
	b.auditLog = nil
	msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log cleared."}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// logAudit records a tool execution in the audit trail if the tool has an "audit" override.
func (b *Brain) logAudit(toolName string, args map[string]interface{}, success bool) {
	if b.policy == nil {
		return
	}
	ov, ok := b.policy.Override(toolName)
	if !ok || ov != OverrideAudit {
		return
	}
	var argStr string
	if len(args) > 0 {
		parts := make([]string, 0, len(args))
		for k, v := range args {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		argStr = strings.Join(parts, " ")
	}
	b.auditLog = append(b.auditLog, AuditEntry{
		Time:    time.Now(),
		Tool:    toolName,
		Args:    argStr,
		Tier:    string(b.policy.Tier()),
		Allowed: success,
	})
}
