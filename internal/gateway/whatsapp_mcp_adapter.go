package gateway

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/mcp"
)

// WhatsAppMCPAdapter bridges WhatsApp via MCP protocol, implementing ports.GatewayAdapter.
// It wraps an MCP client that connects to a WhatsApp MCP server process.
type WhatsAppMCPAdapter struct {
	client  *mcp.Client
	handler ports.MessageHandler
}

// NewWhatsAppMCPAdapter creates a WhatsApp gateway adapter using an MCP client.
func NewWhatsAppMCPAdapter(cfg domain.MCPGatewayConfig) (*WhatsAppMCPAdapter, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("whatsapp gateway: command is required")
	}

	mcpCfg := domain.MCPServerConfig{
		Name:    "whatsapp",
		Command: cfg.Command,
		Args:    nil,
	}

	return &WhatsAppMCPAdapter{
		client: mcp.NewClient(mcpCfg),
	}, nil
}

// Name returns the platform identifier.
func (a *WhatsAppMCPAdapter) Name() string {
	return "whatsapp"
}

// Start connects to the WhatsApp MCP server and begins message polling.
func (a *WhatsAppMCPAdapter) Start(ctx context.Context, handler ports.MessageHandler) error {
	a.handler = handler

	if err := a.client.Connect(ctx); err != nil {
		return fmt.Errorf("whatsapp mcp: connect: %w", err)
	}

	go a.pollMessages(ctx)
	return nil
}

// Stop disconnects from the WhatsApp MCP server.
func (a *WhatsAppMCPAdapter) Stop() error {
	return a.client.Close()
}

// Send sends a message to a WhatsApp chat via the MCP bridge.
func (a *WhatsAppMCPAdapter) Send(ctx context.Context, target string, content string) error {
	result, err := a.client.CallTool(ctx, "send_message", map[string]interface{}{
		"to":      target,
		"message": content,
	})
	if err != nil {
		return fmt.Errorf("whatsapp send: %w", err)
	}

	if result.IsError {
		errMsg := "unknown mcp error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return fmt.Errorf("whatsapp send: %s", errMsg)
	}

	return nil
}

// pollMessages periodically calls get_messages on the MCP server to check
// for incoming messages and dispatches them to the handler.
func (a *WhatsAppMCPAdapter) pollMessages(ctx context.Context) {
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
				Platform:   "whatsapp",
				SenderName: "whatsapp-user",
				Content:    output,
				ChatID:     "whatsapp-chat",
			}
			a.handler(ctx, msg)
		}
	}
}
