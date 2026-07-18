package gateway

import (
	"context"
	"fmt"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"

	"gopkg.in/telebot.v3"
)

// TelegramAdapter wraps the telebot library to implement ports.GatewayAdapter.
type TelegramAdapter struct {
	bot            *telebot.Bot
	allowedUserIDs []int64
	handler        ports.MessageHandler
}

// NewTelegramAdapter creates a Telegram gateway adapter.
func NewTelegramAdapter(cfg domain.TelegramGatewayConfig) (*TelegramAdapter, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram gateway: token is required")
	}

	pref := telebot.Settings{
		Token:  cfg.Token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("telegram gateway: create bot: %w", err)
	}

	return &TelegramAdapter{
		bot:            b,
		allowedUserIDs: cfg.AllowedUserIDs,
	}, nil
}

// Name returns the platform identifier.
func (a *TelegramAdapter) Name() string {
	return "telegram"
}

// Start launches the Telegram bot and routes messages through the handler.
func (a *TelegramAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	a.bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if !a.isAuthorized(c.Sender().ID) {
			return c.Send("Access denied.")
		}

		msg := ports.IncomingMessage{
			Platform:   "telegram",
			SenderID:   fmt.Sprintf("%d", c.Sender().ID),
			SenderName: c.Sender().FirstName,
			Content:    c.Text(),
			ChatID:     fmt.Sprintf("%d", c.Chat().ID),
		}

		response, err := handler(ctx, msg)
		if err != nil {
			return c.Send(fmt.Sprintf("Error: %v", err))
		}

		if response != "" {
			return c.Send(response)
		}
		return nil
	})

	go a.bot.Start()
	return nil
}

// Stop stops the Telegram bot.
func (a *TelegramAdapter) Stop() error {
	if a.bot != nil {
		a.bot.Stop()
	}
	return nil
}

// Send sends a message to a Telegram chat.
func (a *TelegramAdapter) Send(ctx context.Context, target string, content string) error {
	// target is the chat ID as string
	var chatID int64
	if _, err := fmt.Sscanf(target, "%d", &chatID); err != nil {
		return fmt.Errorf("telegram send: invalid chat ID %q: %w", target, err)
	}

	_, err := a.bot.Send(&telebot.User{ID: chatID}, content)
	return err
}

// isAuthorized checks whether a user ID is in the allowed list.
// If the allowed list is empty, all users are authorized.
func (a *TelegramAdapter) isAuthorized(userID int64) bool {
	if len(a.allowedUserIDs) == 0 {
		return true
	}
	for _, id := range a.allowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}
