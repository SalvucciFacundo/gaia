package core

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
func (b *Brain) BackgroundTask(ctx context.Context, prompt string) error {
	if prompt == "" {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /background <prompt> — run a task in the background. Use /tasks to track progress."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	asyncPort, isAsync := b.subagentPort.(ports.AsyncSpawner)
	if !isAsync {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Background tasks not available — async subagent system not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
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

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Background task started (ID: %s). Use /tasks to check progress.", taskID[:8]),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotHelp shows usage for /snapshot.
func (b *Brain) SnapshotHelp(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Snapshot commands:\n" +
			"  /snapshot save <name>   Save current conversation\n" +
			"  /snapshot load <name>   Load a saved conversation\n" +
			"  /snap                   Alias for /snapshot",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotSave saves the current conversation to a snapshot file.
func (b *Brain) SnapshotSave(ctx context.Context, name string) error {
	if name == "" {
		return b.SnapshotHelp(ctx)
	}

	history, err := b.repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("snapshot save: get history: %w", err)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	os.MkdirAll(snapDir, 0700)
	snapPath := filepath.Join(snapDir, name+".json")

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot save: marshal: %w", err)
	}
	if err := os.WriteFile(snapPath, data, 0600); err != nil {
		return fmt.Errorf("snapshot save: write: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot saved: %s (%d messages, %s)", name, len(history), snapPath),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotLoad loads a snapshot and displays it.
func (b *Brain) SnapshotLoad(ctx context.Context, name string) error {
	if name == "" {
		return b.SnapshotHelp(ctx)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	snapPath := filepath.Join(snapDir, name+".json")

	data, err := os.ReadFile(snapPath)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Snapshot %q not found. Available snapshots: %s", name, snapDir),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var history []domain.Message
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("snapshot load: parse: %w", err)
	}

	// Restore messages to repo
	for i := range history {
		b.repo.SaveMessage(ctx, history[i])
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot loaded: %s (%d messages restored). Use /history to view.", name, len(history)),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BranchFork saves the current conversation as a named branch point.
// The user can continue from here, and later list/load branches via /branches and /snapshot load.
func (b *Brain) BranchFork(ctx context.Context, name string) error {
	history, err := b.repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("branch: get history: %w", err)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	os.MkdirAll(snapDir, 0700)

	// Save with branch- prefix
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BranchList lists all saved branch points.
func (b *Brain) BranchList(ctx context.Context) error {
	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No branches found. Use /branch <name> to create one.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
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
		sb.WriteString(fmt.Sprintf("\nUse /snapshot load branch-<name> to switch to a branch.\n"))
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueuePush adds a message to the processing queue.
func (b *Brain) QueuePush(ctx context.Context, prompt string) error {
	if prompt == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /queue <prompt> — queue a message for later processing. Use /q as alias.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	b.messageQueue = append(b.messageQueue, prompt)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queued: %s (%d items in queue). It will be processed after the current task completes.", prompt, len(b.messageQueue)),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueueShow displays the current message queue.
func (b *Brain) QueueShow(ctx context.Context) error {
	if len(b.messageQueue) == 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Queue is empty. Use /queue <prompt> to add items.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Message queue (%d items):\n", len(b.messageQueue)))
	for i, item := range b.messageQueue {
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueueClear empties the message queue.
func (b *Brain) QueueClear(ctx context.Context) error {
	count := len(b.messageQueue)
	b.messageQueue = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queue cleared (%d items removed).", count),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HandoffHelp shows available platforms for handoff.
func (b *Brain) HandoffHelp(ctx context.Context) error {
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HandoffTo saves the current session and shows resume instructions for the platform.
func (b *Brain) HandoffTo(ctx context.Context, platform string) error {
	platform = strings.ToLower(platform)

	// First save the session
	name := fmt.Sprintf("handoff-%s-%s", platform, time.Now().Format("2006-01-02-15-04-05"))
	id, err := b.repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("handoff: save session: %w", err)
	}
	b.currentSessionID = id

	var instructions string
	switch platform {
	case "telegram":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Telegram:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Telegram bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "discord":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Discord:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Discord channel\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "slack":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Slack:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Slack channel\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "whatsapp":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on WhatsApp:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Message your WhatsApp bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "signal":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Signal:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Message your Signal bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "cli":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume later:\n"+
				"  gaia session restore %s\n"+
				"  Or use /resume %s inside GAIA.",
			name, id[:12], id[:12], id[:12])
	default:
		return b.HandoffHelp(ctx)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: instructions,
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// Steer injects a guidance message that the agent will see before its next action.
// It sends to a buffered channel (cap 1) that the main loop checks between iterations.
// If the channel is full (previous steer not yet consumed), it drops the old and keeps the new.
func (b *Brain) Steer(ctx context.Context, guidance string) error {
	select {
	case <-b.steerCh: // drain old steer if present
	default:
	}
	select {
	case b.steerCh <- guidance:
	default:
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Steer queued: %s\nIt will be applied after the current tool completes.", guidance),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalSet sets a persistent goal that the agent will work toward across turns.
func (b *Brain) GoalSet(ctx context.Context, text string) error {
	if text == "" {
		return b.GoalShow(ctx)
	}
	b.goal = text
	b.subgoals = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Goal set: %s\n\nThe agent will continue working on this until it's complete. Use /subgoal to add criteria, /goals to check status, /goal clear to cancel.", text),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalShow displays the current goal and subgoals.
func (b *Brain) GoalShow(ctx context.Context) error {
	if b.goal == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Use /goal <text> to set one.\n\nExample: /goal refactor the auth module to use JWT",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active goal: %s\n", b.goal))
	if len(b.subgoals) > 0 {
		sb.WriteString("Subgoals:\n")
		for i, sg := range b.subgoals {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, sg))
		}
	}
	sb.WriteString("\nUse /subgoal <text> to add criteria.")
	sb.WriteString("\nUse /goal clear to cancel.")

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalClear removes the active goal and subgoals.
func (b *Brain) GoalClear(ctx context.Context) error {
	b.goal = ""
	b.subgoals = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Goal cleared. The agent will stop auto-continuing.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SubgoalAdd appends a criterion to the active goal.
func (b *Brain) SubgoalAdd(ctx context.Context, text string) error {
	if text == "" {
		return b.GoalShow(ctx)
	}
	if b.goal == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Set one with /goal first, then add /subgoal criteria.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	b.subgoals = append(b.subgoals, text)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Subgoal added: %s", text),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// checkGoalAfterTurn evaluates goal completion and auto-continues if needed.
// Called after ProcessMessage completes (not for goal commands themselves).
func (b *Brain) checkGoalAfterTurn(ctx context.Context) {
	if b.goal == "" {
		return
	}

	// Get recent history for evaluation
	history, err := b.getHistory(ctx, 20)
	if err != nil || len(history) == 0 {
		return
	}

	// Build evaluation prompt
	var evalPrompt strings.Builder
	evalPrompt.WriteString("Evaluate whether the following goal is COMPLETE or INCOMPLETE.\n")
	evalPrompt.WriteString("Answer with exactly one word: YES or NO.\n\n")
	evalPrompt.WriteString(fmt.Sprintf("Goal: %s\n", b.goal))
	if len(b.subgoals) > 0 {
		evalPrompt.WriteString("Subgoals:\n")
		for _, sg := range b.subgoals {
			evalPrompt.WriteString(fmt.Sprintf("  - %s\n", sg))
		}
	}
	evalPrompt.WriteString("\nRecent conversation:\n")
	for _, m := range history {
		preview := m.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		evalPrompt.WriteString(fmt.Sprintf("[%s] %s\n", strings.ToUpper(string(m.Role[:1])), preview))
	}
	evalPrompt.WriteString("\nIs the goal complete? Answer YES or NO:")

	evalMsg, err := b.provider.Chat(ctx, []domain.Message{
		{Role: domain.RoleSystem, Content: "You are a goal evaluation judge. Be strict — only YES if the goal is genuinely accomplished."},
		{Role: domain.RoleUser, Content: evalPrompt.String()},
	})
	if err != nil || evalMsg == nil {
		return
	}

	answer := strings.TrimSpace(strings.ToUpper(evalMsg.Content))
	if strings.HasPrefix(answer, "YES") {
		b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Goal complete: %s ✓", b.goal),
		})
		b.goal = ""
		b.subgoals = nil
		return
	}

	// Goal is incomplete — auto-continue
	contPrompt := fmt.Sprintf("Continue working on the goal: %s", b.goal)
	if len(b.subgoals) > 0 {
		contPrompt += "\nRequirements:\n"
		for _, sg := range b.subgoals {
			contPrompt += fmt.Sprintf("- %s\n", sg)
		}
	}
	b.ProcessMessage(ctx, contPrompt)
}

// processQueuedMessages checks the queue and processes the next item.
// Called at the end of ProcessMessage if the current message was not a queue command.
func (b *Brain) processQueuedMessages(ctx context.Context) {
	if b.processingQueue || len(b.messageQueue) == 0 {
		return
	}
	b.processingQueue = true
	defer func() { b.processingQueue = false }()

	// Process one item at a time
	prompt := b.messageQueue[0]
	b.messageQueue = b.messageQueue[1:]

	b.repo.SaveMessage(ctx, domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Processing queued message (%d remaining): %s", len(b.messageQueue), prompt),
	})
	b.ProcessMessage(ctx, prompt)
}

