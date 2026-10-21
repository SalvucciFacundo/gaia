package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gaia/internal/mneme"
	"gaia/internal/scan"
)

// ExportFormat represents supported export serialization formats.
type ExportFormat string

const (
	ExportFormatJSON     ExportFormat = "json"
	ExportFormatMarkdown ExportFormat = "markdown"
	ExportFormatTXT      ExportFormat = "txt"
)

// SessionExport represents the structured export payload for JSON formatting.
type SessionExport struct {
	Conversation *mneme.Conversation `json:"conversation"`
	Messages     []mneme.Message     `json:"messages"`
}

// Manager owns the active session id, multi-channel mapping, and session operations.
// It is surface-agnostic: TUI, gateway, and subagents share the same manager.
type Manager struct {
	repo     mneme.Store
	closer   mneme.SessionCloser
	logger   *slog.Logger
	activeID string

	mu            sync.RWMutex
	channelRoutes map[string]string // channelKey (e.g. "whatsapp:+123456") -> convID
}

// New returns an initialized session Manager. closer may be nil.
func New(repo mneme.Store, closer mneme.SessionCloser, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		repo:          repo,
		closer:        closer,
		logger:        logger,
		channelRoutes: make(map[string]string),
	}
}

// ActiveID returns the current active conversation id, or empty if none.
func (m *Manager) ActiveID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeID
}

// SetActive switches the active conversation.
func (m *Manager) SetActive(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeID = id
}

// ChannelRoute resolves or creates a conversation ID for an external chat channel (WhatsApp, Telegram, etc).
func (m *Manager) ChannelRoute(ctx context.Context, platform, channelID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s:%s", strings.ToLower(platform), channelID)

	m.mu.RLock()
	convID, exists := m.channelRoutes[key]
	m.mu.RUnlock()

	if exists && convID != "" {
		if _, err := m.repo.GetConversation(ctx, convID); err == nil {
			return convID, nil
		}
	}

	// Create a new conversation dedicated to this channel
	title := fmt.Sprintf("[%s] %s", strings.ToUpper(platform), channelID)
	conv, err := m.repo.CreateConversation(ctx, title)
	if err != nil {
		return "", fmt.Errorf("creating conversation for channel %s: %w", key, err)
	}

	m.mu.Lock()
	m.channelRoutes[key] = conv.ID
	m.mu.Unlock()

	return conv.ID, nil
}

// NewSession creates a fresh conversation and makes it active.
func (m *Manager) NewSession(ctx context.Context) (*mneme.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conv, err := m.repo.CreateConversation(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	m.SetActive(conv.ID)
	return conv, nil
}

// Save ensures the active session exists; messages already persist incrementally.
func (m *Manager) Save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	actID := m.ActiveID()
	if actID == "" {
		conv, err := m.repo.CreateConversation(ctx, "")
		if err != nil {
			return err
		}
		m.SetActive(conv.ID)
		return nil
	}
	if _, err := m.repo.GetConversation(ctx, actID); err != nil {
		return err
	}
	return nil
}

// List returns recent conversations ordered updated_at DESC, id DESC.
func (m *Manager) List(ctx context.Context, limit int) ([]mneme.Conversation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.repo.ListConversations(ctx, limit, 0)
}

// Restore validates and switches to an existing conversation.
func (m *Manager) Restore(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := m.repo.GetConversation(ctx, id); err != nil {
		return err
	}
	m.SetActive(id)
	return nil
}

// Rename updates a conversation title after scanning for injection.
func (m *Manager) Rename(ctx context.Context, id, title string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	clean, dropped := scan.Lines(title)
	if dropped > 0 {
		m.logger.Warn("session: dropped injected lines from title", "dropped", dropped)
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		clean = "New conversation"
	}
	return m.repo.RenameConversation(ctx, id, clean)
}

// Compress triggers close-time summarization for the given conversation id.
func (m *Manager) Compress(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.closer == nil {
		return nil
	}
	msgs, err := m.repo.Messages(ctx, id, 0)
	if err != nil {
		return fmt.Errorf("fetching messages to compress: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}
	return m.closer.Close(ctx, id, msgs)
}

// Delete permanently removes a conversation and its messages.
func (m *Manager) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.repo.DeleteConversation(ctx, id); err != nil {
		return err
	}
	if m.ActiveID() == id {
		m.SetActive("")
	}
	return nil
}

// Snapshot creates a frozen point-in-time snapshot of the session.
func (m *Manager) Snapshot(ctx context.Context, id string) (*mneme.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.repo.CreateSnapshot(ctx, id)
}

// Export serializes the session messages into the requested format (json, markdown, txt).
func (m *Manager) Export(ctx context.Context, id string, format ExportFormat) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	conv, err := m.repo.GetConversation(ctx, id)
	if err != nil {
		return "", fmt.Errorf("fetching conversation: %w", err)
	}
	msgs, err := m.repo.Messages(ctx, id, 0)
	if err != nil {
		return "", fmt.Errorf("fetching messages: %w", err)
	}

	switch format {
	case ExportFormatJSON:
		payload := SessionExport{
			Conversation: conv,
			Messages:     msgs,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshaling json export: %w", err)
		}
		return string(data), nil

	case ExportFormatMarkdown:
		var b strings.Builder
		title := conv.Title
		if title == "" {
			title = "Conversation " + conv.ID
		}
		b.WriteString("# " + title + "\n\n")
		for _, msg := range msgs {
			role := strings.ToUpper(string(msg.Role))
			b.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", role, msg.Content))
		}
		return strings.TrimRight(b.String(), "\n"), nil

	case ExportFormatTXT:
		var b strings.Builder
		for _, msg := range msgs {
			b.WriteString(fmt.Sprintf("[%s]: %s\n\n", string(msg.Role), msg.Content))
		}
		return strings.TrimRight(b.String(), "\n"), nil

	default:
		return "", fmt.Errorf("unsupported export format: %q", format)
	}
}
