package gateway

import (
	"context"
	"fmt"
	"sync"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SlackAdapter wraps the slack-go library to implement ports.GatewayAdapter.
// It connects via Socket Mode (no public HTTP endpoint needed) and supports
// receiving messages from channels and DMs.
type SlackAdapter struct {
	client   *slack.Client
	socket   *socketmode.Client
	handler  ports.MessageHandler
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewSlackAdapter creates a Slack gateway adapter using a bot token.
func NewSlackAdapter(cfg domain.SlackGatewayConfig) (*SlackAdapter, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("slack gateway: token is required")
	}

	cli := slack.New(cfg.Token, slack.OptionAppLevelToken(cfg.Token))
	socketCli := socketmode.New(cli)

	return &SlackAdapter{
		client: cli,
		socket: socketCli,
	}, nil
}

// Name returns the platform identifier.
func (a *SlackAdapter) Name() string {
	return "slack"
}

// Start connects to Slack via Socket Mode and begins processing events.
func (a *SlackAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	a.wg.Add(2)
	go func() {
		defer a.wg.Done()
		a.socket.RunContext(ctx)
	}()
	go a.handleEvents(ctx)

	return nil
}

// Stop disconnects from Slack.
func (a *SlackAdapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	return nil
}

// Send sends a message to a Slack channel or DM.
func (a *SlackAdapter) Send(ctx context.Context, target string, content string) error {
	_, _, err := a.client.PostMessageContext(ctx, target,
		slack.MsgOptionText(content, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return fmt.Errorf("slack send: %w", err)
	}
	return nil
}

// handleEvents processes incoming Slack events from the socket mode client.
func (a *SlackAdapter) handleEvents(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-a.socket.Events:
			if !ok {
				return
			}
			a.processEvent(ctx, event)
		}
	}
}

// processEvent routes a single Slack event to the message handler.
func (a *SlackAdapter) processEvent(ctx context.Context, event socketmode.Event) {
	switch event.Type {
	case socketmode.EventTypeEventsAPI:
		payload, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		a.socket.Ack(*event.Request)

		innerEvent := payload.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			if ev.BotID != "" || ev.SubType == "bot_message" {
				return
			}
			if ev.Text == "" {
				return
			}

			chatID := ev.Channel
			if ev.ChannelType == "im" {
				chatID = ev.User
			}

			msg := ports.IncomingMessage{
				Platform:   "slack",
				SenderID:   ev.User,
				SenderName: ev.User,
				Content:    ev.Text,
				ChatID:     chatID,
			}
			a.handler(ctx, msg)
		}
	}
}
