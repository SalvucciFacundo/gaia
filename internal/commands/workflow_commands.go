package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// BackgroundTask spawns a subagent in the background and returns immediately.
func (d *Dispatcher) BackgroundTask(ctx context.Context, prompt string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if prompt == "" {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /background <prompt> — run a task in the background. Use /tasks to track progress."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	asyncPort, isAsync := d.brain.SubagentPort().(ports.AsyncSpawner)
	if !isAsync {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Background tasks not available — async subagent system not configured."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	task := domain.SubagentTask{
		ID:          fmt.Sprintf("bg-%d", time.Now().UnixNano()),
		Description: prompt,
		Mode:        "build",
	}

	taskID, err := asyncPort.SpawnAsync(ctx, "explorer", task)
	if err != nil {
		return fmt.Errorf("background task: %w", err)
	}

	shortID := taskID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Background task started (ID: %s). Use /tasks to check progress.", shortID),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SnapshotHelp shows usage for /snapshot.
func (d *Dispatcher) SnapshotHelp(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Snapshot commands:\n" +
			"  /snapshot save <name>   Save current conversation\n" +
			"  /snapshot load <name>   Load a saved conversation\n" +
			"  /snap                   Alias for /snapshot",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SnapshotSave saves the current conversation to a snapshot file.
func (d *Dispatcher) SnapshotSave(ctx context.Context, name string) error {
	if name == "" {
		return d.SnapshotHelp(ctx)
	}

	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	history, err := repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("snapshot save: get history: %w", err)
	}

	safeName := filepath.Base(filepath.Clean(name))
	if safeName == "." || safeName == "/" || safeName == "" || safeName == ".." {
		return fmt.Errorf("invalid snapshot name %q", name)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	_ = os.MkdirAll(snapDir, 0700)
	snapPath := filepath.Join(snapDir, safeName+".json")

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot save: marshal: %w", err)
	}
	if err := os.WriteFile(snapPath, data, 0600); err != nil {
		return fmt.Errorf("snapshot save: write: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot saved: %s (%d messages, %s)", safeName, len(history), snapPath),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SnapshotLoad loads a snapshot and displays it.
func (d *Dispatcher) SnapshotLoad(ctx context.Context, name string) error {
	if name == "" {
		return d.SnapshotHelp(ctx)
	}

	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	safeName := filepath.Base(filepath.Clean(name))
	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	snapPath := filepath.Join(snapDir, safeName+".json")

	data, err := os.ReadFile(snapPath)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Snapshot %q not found. Available snapshots: %s", name, snapDir),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var history []domain.Message
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("snapshot load: parse: %w", err)
	}

	for i := range history {
		_ = repo.SaveMessage(ctx, history[i])
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot loaded: %s (%d messages restored). Use /history to view.", name, len(history)),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// BranchFork saves the current conversation as a named branch point.
func (d *Dispatcher) BranchFork(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	history, err := repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("branch: get history: %w", err)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	_ = os.MkdirAll(snapDir, 0700)

	snapPath := filepath.Join(snapDir, "branch-"+name+".json")
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("branch: marshal: %w", err)
	}
	if err := os.WriteFile(snapPath, data, 0600); err != nil {
		return fmt.Errorf("branch: write: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Branch created: %s (%d messages).\nYou're now on a new branch. Use /branches to list, /snapshot load branch-%s to return.", name, len(history), name),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// BranchList lists all saved branch points.
func (d *Dispatcher) BranchList(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No branches found. Use /branch <name> to create one.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString("Branches:\n\n")
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "branch-") && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "branch-"), ".json")
			sb.WriteString(fmt.Sprintf("  branch-%s\n", name))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("  (none) — use /branch <name> to create one.\n")
	} else {
		sb.WriteString("\nUse /snapshot load branch-<name> to switch to a branch.\n")
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// QueuePush adds a message to the processing queue.
func (d *Dispatcher) QueuePush(ctx context.Context, prompt string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if prompt == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /queue <prompt> — queue a message for later processing. Use /q as alias.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	d.brain.EnqueueMessage(prompt)
	qLen := len(d.brain.MessageQueue())

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queued: %s (%d items in queue). It will be processed after the current task completes.", prompt, qLen),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// QueueShow displays the current message queue.
func (d *Dispatcher) QueueShow(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	queue := d.brain.MessageQueue()
	if len(queue) == 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Queue is empty. Use /queue <prompt> to add items.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Message queue (%d items):\n", len(queue)))
	for i, item := range queue {
		preview := item
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, preview))
	}
	sb.WriteString("\nUse /queue clear to clear the queue.")

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// QueueClear empties the message queue.
func (d *Dispatcher) QueueClear(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	qLen := len(d.brain.MessageQueue())
	d.brain.ClearMessageQueue()
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queue cleared (%d items removed).", qLen),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// HandoffHelp shows available platforms for handoff.
func (d *Dispatcher) HandoffHelp(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Handoff saves the current session and shows how to resume on another platform.\n\n" +
			"Usage: /handoff <platform>\n\n" +
			"Platforms:\n" +
			"  telegram   — Resume on Telegram\n" +
			"  discord    — Resume on Discord\n" +
			"  slack      — Resume on Slack\n" +
			"  whatsapp   — Resume on WhatsApp\n" +
			"  signal     — Resume on Signal\n" +
			"  cli        — Resume on terminal (save only)\n\n" +
			"Requirements:\n" +
			"  The gateway must be running on the target platform:\n" +
			"  gaia gateway start",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// HandoffTo saves the current session and shows resume instructions for the platform.
func (d *Dispatcher) HandoffTo(ctx context.Context, platform string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	platform = strings.ToLower(platform)
	name := fmt.Sprintf("handoff-%s-%s", platform, time.Now().Format("2006-01-02-15-04-05"))
	id, err := repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("handoff: save session: %w", err)
	}
	d.brain.SetActiveConversation(id)

	shortID := id
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	var instructions string
	switch platform {
	case "telegram", "discord", "slack", "whatsapp", "signal":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on %s:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your %s app\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, shortID, strings.Title(platform), strings.Title(platform), shortID)
	case "cli":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume later:\n"+
				"  gaia session restore %s\n"+
				"  Or use /resume %s inside GAIA.",
			name, shortID, shortID, shortID)
	default:
		return d.HandoffHelp(ctx)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: instructions,
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// Steer injects a guidance message mid-execution.
func (d *Dispatcher) Steer(ctx context.Context, guidance string) error {
	ch := d.brain.SteerChannel()
	if ch != nil {
		select {
		case <-ch: // drain old steer if present
		default:
		}
		select {
		case ch <- guidance:
		default:
		}
	}

	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Steer queued: %s\nIt will be applied after the current tool completes.", guidance),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// GoalSet sets a persistent goal across turns.
func (d *Dispatcher) GoalSet(ctx context.Context, text string) error {
	if text == "" {
		return d.GoalShow(ctx)
	}
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	d.brain.SetGoal(text)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Goal set: %s\n\nThe agent will continue working on this until it's complete. Use /subgoal to add criteria, /goals to check status, /goal clear to cancel.", text),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// GoalShow displays the current goal and subgoals.
func (d *Dispatcher) GoalShow(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	goal := d.brain.Goal()
	if goal == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Use /goal <text> to set one.\n\nExample: /goal refactor the auth module to use JWT",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	subgoals := d.brain.Subgoals()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active goal: %s\n", goal))
	if len(subgoals) > 0 {
		sb.WriteString("Subgoals:\n")
		for i, sg := range subgoals {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, sg))
		}
	}
	sb.WriteString("\nUse /subgoal <text> to add criteria.")
	sb.WriteString("\nUse /goal clear to cancel.")

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// GoalClear removes the active goal and subgoals.
func (d *Dispatcher) GoalClear(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	d.brain.ClearGoal()
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Goal cleared. The agent will stop auto-continuing.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SubgoalAdd appends a criterion to the active goal.
func (d *Dispatcher) SubgoalAdd(ctx context.Context, text string) error {
	if text == "" {
		return d.GoalShow(ctx)
	}
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if d.brain.Goal() == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Set one with /goal first, then add /subgoal criteria.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	d.brain.AddSubgoal(text)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Subgoal added: %s", text),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// Moa runs a one-shot Mixture of Agents on the given prompt.
func (d *Dispatcher) Moa(ctx context.Context, prompt string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()

	if prompt == "" {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /moa <prompt> — run prompt through multiple models and synthesize."}
		if repo != nil {
			_ = repo.SaveMessage(ctx, msg)
		}
		if ui != nil {
			return ui.Display(msg)
		}
		return nil
	}

	runner := d.brain.MoARunner()
	if runner == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No additional MoA providers configured. Use subagents config to add models."}
		if repo != nil {
			_ = repo.SaveMessage(ctx, msg)
		}
		if ui != nil {
			return ui.Display(msg)
		}
		return nil
	}

	resp, err := runner.RunPrompt(ctx, prompt)
	if err != nil {
		return err
	}
	if resp == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "All MoA models failed. Try with a simpler prompt."}
		if repo != nil {
			_ = repo.SaveMessage(ctx, msg)
		}
		if ui != nil {
			return ui.Display(msg)
		}
		return nil
	}

	resp.Role = domain.RoleAssistant
	if repo != nil {
		_ = repo.SaveMessage(ctx, *resp)
	}
	if ui != nil {
		return ui.Display(*resp)
	}
	return nil
}

