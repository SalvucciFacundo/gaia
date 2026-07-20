package gateway

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/mcp"
)

// SignalMCPAdapter bridges Signal via MCP protocol, implementing ports.GatewayAdapter.
// It wraps an MCP client that connects to a Signal MCP server process.
type SignalMCPAdapter struct {
	client  *mcp.Client
	handler ports.MessageHandler
}

// NewSignalMCPAdapter creates a Signal gateway adapter using an MCP client.
func NewSignalMCPAdapter(cfg domain.MCPGatewayConfig) (*SignalMCPAdapter, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("signal gateway: command is required")
	}

	mcpCfg := domain.MCPServerConfig{
		Name:    "signal",
		Command: cfg.Command,
		Args:    nil,
	}

	return &SignalMCPAdapter{
		client: mcp.NewClient(mcpCfg),
	}, nil
}

// Name returns the platform identifier.
func (a *SignalMCPAdapter) Name() string {
	return "signal"
}

// Start connects to the Signal MCP server and begins message polling.
func (a *SignalMCPAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	if err := a.client.Connect(ctx); err != nil {
		return fmt.Errorf("signal mcp: connect: %w", err)
	}

	go a.pollMessages(ctx)
	return nil
}

// Stop disconnects from the Signal MCP server.
func (a *SignalMCPAdapter) Stop() error {
	return a.client.Close()
}

// Send sends a message to a Signal conversation via the MCP bridge.
func (a *SignalMCPAdapter) Send(ctx context.Context, target string, content string) error {
	result, err := a.client.CallTool(ctx, "send_message", map[string]interface{}{
		"to":      target,
		"message": content,
	})
	if err != nil {
		return fmt.Errorf("signal send: %w", err)
	}

	if result.IsError {
		errMsg := "unknown mcp error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return fmt.Errorf("signal send: %s", errMsg)
	}

	return nil
}

// pollMessages periodically calls get_messages on the MCP server to check
// for incoming messages and dispatches them to the handler.
func (a *SignalMCPAdapter) pollMessages(ctx context.Context) {
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

		output := ""
		for _, c := range result.Content {
			if c.Type == "text" {
				output += c.Text
			}
		}

		if output != "" && output != "[]" {
			msg := ports.IncomingMessage{
				Platform:   "signal",
				SenderName: "signal-user",
				Content:    output,
				ChatID:     "signal-chat",
			}
			a.handler(ctx, msg)
		}
	}
}
