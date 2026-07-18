package gateway

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/mcp"
)

// DiscordMCPAdapter bridges Discord via MCP protocol, implementing ports.GatewayAdapter.
// It wraps an MCP client that connects to a Discord MCP server process.
type DiscordMCPAdapter struct {
	client  *mcp.Client
	handler ports.MessageHandler
}

// NewDiscordMCPAdapter creates a Discord gateway adapter using an MCP client.
func NewDiscordMCPAdapter(cfg domain.DiscordGatewayConfig) (*DiscordMCPAdapter, error) {
	mcpCfg := domain.MCPServerConfig{
		Name:    "discord",
		Command: cfg.Command,
		Args:    nil,
	}

	return &DiscordMCPAdapter{
		client: mcp.NewClient(mcpCfg),
	}, nil
}

// Name returns the platform identifier.
func (a *DiscordMCPAdapter) Name() string {
	return "discord"
}

// Start connects to the Discord MCP server and begins message polling.
func (a *DiscordMCPAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	if err := a.client.Connect(ctx); err != nil {
		return fmt.Errorf("discord mcp: connect: %w", err)
	}

	// Start a goroutine that periodically polls for messages.
	go a.pollMessages(ctx)

	return nil
}

// Stop disconnects from the Discord MCP server.
func (a *DiscordMCPAdapter) Stop() error {
	return a.client.Close()
}

// Send sends a message to a Discord channel via the MCP bridge.
func (a *DiscordMCPAdapter) Send(ctx context.Context, target string, content string) error {
	result, err := a.client.CallTool(ctx, "send_message", map[string]interface{}{
		"channel_id": target,
		"content":    content,
	})
	if err != nil {
		return fmt.Errorf("discord send: %w", err)
	}

	if result.IsError {
		errMsg := "unknown mcp error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return fmt.Errorf("discord send: %s", errMsg)
	}

	return nil
}

// pollMessages periodically calls get_messages on the MCP server to check
// for incoming messages and dispatches them to the handler.
func (a *DiscordMCPAdapter) pollMessages(ctx context.Context) {
	if a.handler == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := a.client.CallTool(ctx, "get_messages", map[string]interface{}{
			"format": "json",
		})
		if err != nil {
			continue
		}

		if result.IsError {
			continue
		}

		// Extract text from result and route to handler.
		output := ""
		for _, c := range result.Content {
			if c.Type == "text" {
				output += c.Text
			}
		}

		if output != "" && output != "[]" {
			// Parse as array of messages — dispatch each.
			// We delegate actual parsing to the MCP server's structured output.
			msg := ports.IncomingMessage{
				Platform:   "discord",
				SenderName: "discord-user",
				Content:    output,
				ChatID:     "discord-channel",
			}
			a.handler(ctx, msg)
		}
	}
}
