package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"
)

// UndoLastTurn removes the last user message and everything the AI generated
// in response, effectively rewinding the conversation by one turn.
func (d *Dispatcher) UndoLastTurn(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	lastMsgs, err := repo.GetLastMessages(ctx, 2)
	if err != nil {
		return fmt.Errorf("undo: get last messages: %w", err)
	}

	// Find the last user message to know what to delete
	var lastUserID string
	for _, msg := range lastMsgs {
		if msg.Role == domain.RoleUser {
			lastUserID = msg.ID
			break
		}
	}
	if lastUserID == "" {
		undoMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Nothing to undo — no user message found.",
		}
		_ = repo.SaveMessage(ctx, undoMsg)
		return ui.Display(undoMsg)
	}

	if err := repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("undo: delete messages: %w", err)
	}

	// Reset compaction state since history changed
	d.brain.SetCompactedTo(0)

	undoMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Last turn undone.",
	}
	_ = repo.SaveMessage(ctx, undoMsg)
	return ui.Display(undoMsg)
}

// RetryLastTurn removes the last AI response and re-runs the last user message
// through the full agent loop.
func (d *Dispatcher) RetryLastTurn(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	lastMsgs, err := repo.GetLastMessages(ctx, 2)
	if err != nil {
		return fmt.Errorf("retry: get last messages: %w", err)
	}

	var lastUserContent string
	var lastUserID string
	for _, msg := range lastMsgs {
		if msg.Role == domain.RoleUser {
			lastUserContent = msg.Content
			lastUserID = msg.ID
			break
		}
	}
	if lastUserContent == "" {
		errMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Nothing to retry — no previous user message.",
		}
		_ = repo.SaveMessage(ctx, errMsg)
		return ui.Display(errMsg)
	}

	if err := repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("retry: delete messages: %w", err)
	}

	d.brain.SetCompactedTo(0)
	return d.brain.ProcessMessage(ctx, lastUserContent)
}

// NewSession clears the conversation and starts a fresh session.
func (d *Dispatcher) NewSession(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if err := repo.ClearMessages(ctx); err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	d.brain.SetCompactedTo(0)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Started a fresh conversation. Previous session cleared.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SaveSession explicitly saves the current conversation with an optional name.
func (d *Dispatcher) SaveSession(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if name == "" {
		name = fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04"))
	}
	id, err := repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	d.brain.SetActiveConversation(id)
	shortID := id
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session saved: %s (ID: %s)", name, shortID),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// ManualCompress forces context compaction immediately.
func (d *Dispatcher) ManualCompress(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if d.brain.Budget().CompactionThreshold <= 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Compaction is disabled in config. Enable it by setting compaction_threshold.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	err := d.brain.CompactHistory(ctx)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Context compacted. Older messages have been summarized.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// ShowHistory displays the full conversation history.
func (d *Dispatcher) ShowHistory(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	count, err := repo.GetMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("history count: %w", err)
	}
	history, err := repo.GetHistory(ctx, 100)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Conversation history (%d messages, %d total):\n\n", len(history), count))
	for _, msg := range history {
		preview := msg.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(string(msg.Role[:1])), preview))
	}

	response := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, response)
	return ui.Display(response)
}

// ListSessionsCmd shows all saved sessions.
func (d *Dispatcher) ListSessionsCmd(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	sessions, err := repo.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	var sb strings.Builder
	if len(sessions) == 0 {
		sb.WriteString("No saved sessions.")
	} else {
		sb.WriteString(fmt.Sprintf("Saved sessions (%d):\n", len(sessions)))
		for _, s := range sessions {
			shortID := s.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			sb.WriteString(fmt.Sprintf("  %s  %-30s %s\n", shortID, s.Name, s.CreatedAt.Format("2006-01-02 15:04")))
		}
		sb.WriteString("\nUse /resume <id> to load a session.")
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// TitleSession renames the current session.
func (d *Dispatcher) TitleSession(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if name == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /title <name> — sets the current session name.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	currentID := d.brain.CurrentSessionID()
	if currentID != "" {
		if err := repo.RenameSession(ctx, currentID, name); err != nil {
			msg := domain.Message{
				Role:    domain.RoleSystem,
				Content: fmt.Sprintf("Note: session not yet saved. Name '%s' will be used when saved.", name),
			}
			_ = repo.SaveMessage(ctx, msg)
			return ui.Display(msg)
		}
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session renamed to: %s", name),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// ResumeSession loads and displays a saved session's messages.
func (d *Dispatcher) ResumeSession(ctx context.Context, idOrPrefix string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if idOrPrefix == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /resume <session-id> — load a saved session. Use /sessions to list available sessions.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	sessions, err := repo.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	var matched *domain.SessionInfo
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, idOrPrefix) {
			matched = &s
			break
		}
	}
	if matched == nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("No session found matching %q. Use /sessions to list available sessions.", idOrPrefix),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msgs, err := repo.GetMessages(ctx, matched.ID, 100)
	if err != nil {
		return fmt.Errorf("resume: get messages: %w", err)
	}

	d.brain.SetActiveConversation(matched.ID)

	if setter, ok := repo.(interface{ SetSessionID(string) }); ok {
		setter.SetSessionID(matched.ID)
	}

	shortID := matched.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Resumed session: %s (ID: %s, %d messages)\n\n", matched.Name, shortID, len(msgs)))
	for _, m := range msgs {
		preview := m.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(string(m.Role[:1])), preview))
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}
