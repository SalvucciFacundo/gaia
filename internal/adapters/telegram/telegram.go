package telegram

import (
	"fmt"
	"time"

	"gopkg.in/telebot.v3"
)

type TelegramAdapter struct {
	bot            *telebot.Bot
	allowedUserIDs []int64
}

func NewTelegramAdapter(token string, allowedIDs []int64) (*TelegramAdapter, error) {
	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("could not create telegram bot: %w", err)
	}

	return &TelegramAdapter{
		bot:            b,
		allowedUserIDs: allowedIDs,
	}, nil
}

func (a *TelegramAdapter) Start() error {
	a.bot.Handle(telebot.OnText, func(c telebot.Context) error {
		// Check whitelist
		if !a.isAuthorized(c.Sender().ID) {
			return c.Send("No estás autorizado para usar GAIA, hermano.")
		}

		// Process message
		return c.Send(fmt.Sprintf("Recibido: %s. Pronto tendré cerebro, paciencia.", c.Text()))
	})

	fmt.Println("Telegram Bot iniciado...")
	go a.bot.Start()
	return nil
}

func (a *TelegramAdapter) SendMessage(chatID int64, text string) error {
	_, err := a.bot.Send(&telebot.User{ID: chatID}, text)
	return err
}

func (a *TelegramAdapter) isAuthorized(userID int64) bool {
	for _, id := range a.allowedUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// Ensure interface compliance
// var _ ports.MessagingService = (*TelegramAdapter)(nil)
