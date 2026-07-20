package gateway

import (
	"context"
	"fmt"
	"sync"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"

	"github.com/bwmarrin/discordgo"
)

// DiscordAdapter wraps the discordgo library to implement ports.GatewayAdapter.
// It connects to Discord via WebSocket Gateway and supports receiving messages
// from channels and DMs using a bot token.
type DiscordAdapter struct {
	session  *discordgo.Session
	handler  ports.MessageHandler
	stopOnce sync.Once
}

// NewDiscordAdapter creates a Discord gateway adapter using a bot token.
func NewDiscordAdapter(cfg domain.DiscordGatewayConfig) (*DiscordAdapter, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord gateway: token is required")
	}

	// discordgo expects "Bot <token>" prefix
	token := cfg.Token
	if len(token) < 5 || token[:4] != "Bot " {
		token = "Bot " + token
	}

	session, err := discordgo.New(token)
	if err != nil {
		return nil, fmt.Errorf("discord: create session: %w", err)
	}

	return &DiscordAdapter{
		session: session,
	}, nil
}

// Name returns the platform identifier.
func (a *DiscordAdapter) Name() string {
	return "discord"
}

// Start connects to Discord and registers the message handler.
func (a *DiscordAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	a.session.AddHandler(a.handleMessage)
	a.session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	if err := a.session.Open(); err != nil {
		return fmt.Errorf("discord: open session: %w", err)
	}

	return nil
}

// Stop disconnects from Discord.
func (a *DiscordAdapter) Stop() error {
	var err error
	a.stopOnce.Do(func() {
		err = a.session.Close()
	})
	return err
}

// Send sends a message to a Discord channel via the webhook/send API.
func (a *DiscordAdapter) Send(ctx context.Context, target string, content string) error {
	_, err := a.session.ChannelMessageSend(target, content)
	if err != nil {
		return fmt.Errorf("discord send: %w", err)
	}
	return nil
}

// handleMessage processes incoming Discord messages.
func (a *DiscordAdapter) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if a.handler == nil {
		return
	}

	// Skip bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}
	if m.Author.Bot {
		return
	}
	if m.Content == "" {
		return
	}

	chatID := m.ChannelID
	if m.GuildID != "" {
		chatID = m.ChannelID // Guild channel — use channel ID
	} else {
		chatID = m.ChannelID // DM — channel IS the conversation
	}

	msg := ports.IncomingMessage{
		Platform:   "discord",
		SenderID:   m.Author.ID,
		SenderName: m.Author.Username,
		Content:    m.Content,
		ChatID:     chatID,
	}
	a.handler(context.Background(), msg)
}
