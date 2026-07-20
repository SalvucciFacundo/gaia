package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// OpenAIAdapter wraps the go-openai client.
type OpenAIAdapter struct {
	client *openai.Client
	model  string
}

// NewOpenAI creates an OpenAI adapter from config.
func NewOpenAI(cfg *domain.Config) (ports.LLMProvider, error) {
	key := cfg.APIKeys["openai"]
	if key == "" {
		return nil, fmt.Errorf("openai: missing api key")
	}
	model := cfg.LLM.Model
	if model == "" {
		model = openai.GPT4o
	}
	return &OpenAIAdapter{
		client: openai.NewClient(key),
		model:  model,
	}, nil
}

// Chat sends a non-streaming request.
func (a *OpenAIAdapter) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	req := a.buildRequest(messages, opts, false)
	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	return a.toDomainMessage(resp), nil
}

// Stream sends a streaming request; returns an io.ReadCloser of normalized tokens.
func (a *OpenAIAdapter) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	req := a.buildRequest(messages, opts, true)
	stream, err := a.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer stream.Close()
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				_ = pw.CloseWithError(err)
				return
			}
			if len(resp.Choices) == 0 {
				continue
			}
			delta := resp.Choices[0].Delta
			if delta.Content != "" {
				data, _ := json.Marshal(domain.TokenChunk{Content: delta.Content})
				data = append(data, '\n')
				pw.Write(data)
			}
		}
	}()

	return pr, nil
}

// Tools returns the provider's tool definitions in domain format.
func (a *OpenAIAdapter) Tools() []domain.ToolDef {
	return nil
}

func (a *OpenAIAdapter) buildRequest(messages []domain.Message, opts []ports.ChatOpt, stream bool) openai.ChatCompletionRequest {
	msgs := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		omsg := openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
		for _, tc := range m.ToolCalls {
			args, _ := json.Marshal(tc.Arguments)
			omsg.ToolCalls = append(omsg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: string(args),
				},
			})
		}
		msgs = append(msgs, omsg)
	}

	req := openai.ChatCompletionRequest{
		Model:    a.model,
		Messages: msgs,
		Stream:   stream,
	}

	// Apply functional options
	o := &ports.ChatOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.Temperature != 0 {
		req.Temperature = float32(o.Temperature)
	}
	if o.MaxTokens != 0 {
		req.MaxTokens = o.MaxTokens
	}

	return req
}

func (a *OpenAIAdapter) toDomainMessage(resp openai.ChatCompletionResponse) *domain.Message {
	if len(resp.Choices) == 0 {
		return &domain.Message{Role: domain.RoleAssistant, Content: ""}
	}
	choice := resp.Choices[0]
	msg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: choice.Message.Content,
	}
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]interface{}
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}
	return msg
}


