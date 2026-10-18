package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"
)

// UndoLastTurn removes the last user message and everything the AI generated
// in response, effectively rewinding the conversation by one turn.
func (b *Brain) UndoLastTurn(ctx context.Context) error {
	lastMsgs, err := b.repo.GetLastMessages(ctx, 2)
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
		b.repo.SaveMessage(ctx, undoMsg)
		return b.ui.Display(undoMsg)
	}

	if err := b.repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("undo: delete messages: %w", err)
	}

	// Reset compaction state since history changed
	b.compactedTo = 0

	undoMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Last turn undone.",
	}
	b.repo.SaveMessage(ctx, undoMsg)
	return b.ui.Display(undoMsg)
}

// RetryLastTurn removes the last AI response and re-runs the last user message
// through the full agent loop.
func (b *Brain) RetryLastTurn(ctx context.Context) error {
	lastMsgs, err := b.repo.GetLastMessages(ctx, 2)
	if err != nil {
		return fmt.Errorf("retry: get last messages: %w", err)
	}

	// Find the last user message content
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
		b.repo.SaveMessage(ctx, errMsg)
		return b.ui.Display(errMsg)
	}

	// Delete everything after the last user message
	if err := b.repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("retry: delete messages: %w", err)
	}

	// Reset compaction state
	b.compactedTo = 0

	// Re-process the user message
	return b.ProcessMessage(ctx, lastUserContent)
}

// NewSession clears the conversation and starts a fresh session.
func (b *Brain) NewSession(ctx context.Context) error {
	if err := b.repo.ClearMessages(ctx); err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	b.compactedTo = 0
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Started a fresh conversation. Previous session cleared.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SaveSession explicitly saves the current conversation with an optional name.
func (b *Brain) SaveSession(ctx context.Context, name string) error {
	if name == "" {
		name = fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04"))
	}
	id, err := b.repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	b.currentSessionID = id
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session saved: %s (ID: %s)", name, id[:12]),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ManualCompress forces context compaction immediately.
func (b *Brain) ManualCompress(ctx context.Context) error {
	if b.budget.CompactionThreshold <= 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Compaction is disabled in config. Enable it by setting compaction_threshold.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	err := b.compactHistory(ctx)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Context compacted. Older messages have been summarized.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ShowHistory displays the full conversation history.
func (b *Brain) ShowHistory(ctx context.Context) error {
	count, err := b.repo.GetMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("history count: %w", err)
	}
	history, err := b.repo.GetHistory(ctx, 100)
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
	b.repo.SaveMessage(ctx, response)
	return b.ui.Display(response)
}

// ListSessionsCmd shows all saved sessions.
func (b *Brain) ListSessionsCmd(ctx context.Context) error {
	sessions, err := b.repo.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	var sb strings.Builder
	if len(sessions) == 0 {
		sb.WriteString("No saved sessions.")
	} else {
		sb.WriteString(fmt.Sprintf("Saved sessions (%d):\n", len(sessions)))
		for _, s := range sessions {
			sb.WriteString(fmt.Sprintf("  %s  %-30s %s\n", s.ID[:12], s.Name, s.CreatedAt.Format("2006-01-02 15:04")))
		}
		sb.WriteString("\nUse /resume <id> to load a session.")
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// TitleSession renames the current session.
func (b *Brain) TitleSession(ctx context.Context, name string) error {
	if name == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /title <name> — sets the current session name.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// If we have a saved session ID, rename it
	if b.currentSessionID != "" {
		if err := b.repo.RenameSession(ctx, b.currentSessionID, name); err != nil {
			// Non-fatal: session might not exist yet
			msg := domain.Message{
				Role:    domain.RoleSystem,
				Content: fmt.Sprintf("Note: session not yet saved. Name '%s' will be used when saved.", name),
			}
			b.repo.SaveMessage(ctx, msg)
			return b.ui.Display(msg)
		}
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session renamed to: %s", name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ResumeSession loads and displays a saved session's messages.
func (b *Brain) ResumeSession(ctx context.Context, idOrPrefix string) error {
	if idOrPrefix == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /resume <session-id> — load a saved session. Use /sessions to list available sessions.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Try to find session by ID prefix
	sessions, err := b.repo.ListSessions(ctx)
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
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msgs, err := b.repo.GetMessages(ctx, matched.ID, 100)
	if err != nil {
		return fmt.Errorf("resume: get messages: %w", err)
	}

	b.currentSessionID = matched.ID

	// Also set the repo's active session so subsequent messages use this session
	if repo, ok := b.repo.(interface{ SetSessionID(string) }); ok {
		repo.SetSessionID(matched.ID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Resumed session: %s (ID: %s, %d messages)\n\n", matched.Name, matched.ID[:12], len(msgs)))
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}
