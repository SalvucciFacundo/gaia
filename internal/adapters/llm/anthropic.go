package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// AnthropicAdapter wraps the anthropic-sdk-go client.
type AnthropicAdapter struct {
	client anthropic.Client
	model  string
}

// NewAnthropic creates an Anthropic adapter from config.
func NewAnthropic(cfg *domain.Config) (ports.LLMProvider, error) {
	key := cfg.APIKeys["anthropic"]
	if key == "" {
		return nil, fmt.Errorf("anthropic: missing api key")
	}
	model := cfg.LLM.Model
	if model == "" {
		model = anthropic.ModelClaudeSonnet4_5_20250929
	}
	return &AnthropicAdapter{
		client: anthropic.NewClient(option.WithAPIKey(key)),
		model:  model,
	}, nil
}

// Chat sends a non-streaming request.
func (a *AnthropicAdapter) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	params := a.buildParams(messages, opts)
	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}
	return a.toDomainMessage(resp)
}

// Stream sends a streaming request; returns an io.ReadCloser of normalized tokens.
func (a *AnthropicAdapter) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	params := a.buildParams(messages, opts)

	stream := a.client.Messages.NewStreaming(ctx, params)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for stream.Next() {
			event := stream.Current()
			switch evt := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if evt.Delta.Text != "" {
					data, _ := json.Marshal(domain.TokenChunk{Content: evt.Delta.Text})
					data = append(data, '\n')
					pw.Write(data)
				}
			}
		}
		if err := stream.Err(); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	return pr, nil
}

// Tools returns the provider's tool definitions.
func (a *AnthropicAdapter) Tools() []domain.ToolDef {
	return nil
}

func (a *AnthropicAdapter) buildParams(messages []domain.Message, opts []ports.ChatOpt) anthropic.MessageNewParams {
	msgParams := make([]anthropic.MessageParam, 0, len(messages))
	var systemPrompt string

	for _, m := range messages {
		switch m.Role {
		case domain.RoleSystem:
			systemPrompt += m.Content + "\n"
		case domain.RoleUser:
			msgParams = append(msgParams, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case domain.RoleAssistant:
			var content []anthropic.ContentBlockParamUnion
			if m.Content != "" {
				content = append(content, anthropic.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				content = append(content, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
			}
			msgParams = append(msgParams, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: content,
			})
		case domain.RoleTool:
			msgParams = append(msgParams, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("", m.Content, false),
			))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: 4096,
		Messages:  msgParams,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}

	// Apply functional options
	o := &ports.ChatOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.MaxTokens > 0 {
		params.MaxTokens = int64(o.MaxTokens)
	}

	return params
}

func (a *AnthropicAdapter) toDomainMessage(resp *anthropic.Message) (*domain.Message, error) {
	msg := &domain.Message{
		Role: domain.RoleAssistant,
	}
	for _, block := range resp.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			msg.Content += variant.Text
		case anthropic.ToolUseBlock:
			var args map[string]interface{}
			json.Unmarshal(variant.Input, &args)
			msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{
				ID:        variant.ID,
				Name:      variant.Name,
				Arguments: args,
			})
		}
	}
	return msg, nil
}
