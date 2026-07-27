package llm

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// compile-time check that we can access the anthropic package.
var _ anthropic.ContentBlockParamUnion

func TestAnthropic_BuildParams_WithImages(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "What is in this image?",
			Images: []domain.ImageContent{
				{MIMEType: "image/png", Data: "aGVsbG8="},
			},
		},
	}

	params := adapter.buildParams(messages, nil)

	if len(params.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(params.Messages))
	}

	msgParam := params.Messages[0]
	if string(msgParam.Role) != "user" {
		t.Errorf("expected role 'user', got %q", msgParam.Role)
	}

	if len(msgParam.Content) != 2 {
		t.Fatalf("expected 2 content blocks (text + image), got %d", len(msgParam.Content))
	}

	// Check text block via OfText field.
	tb := msgParam.Content[0].OfText
	if tb == nil {
		t.Fatal("expected first block to be text")
	}
	if tb.Text != "What is in this image?" {
		t.Errorf("unexpected text: %q", tb.Text)
	}

	// Check image block via OfImage field.
	ib := msgParam.Content[1].OfImage
	if ib == nil {
		t.Fatal("expected second block to be image")
	}
	if ib.Source.OfBase64 == nil {
		t.Fatal("expected base64 image source")
	}
	if string(ib.Source.OfBase64.MediaType) != "image/png" {
		t.Errorf("expected media_type 'image/png', got %q", ib.Source.OfBase64.MediaType)
	}
	if ib.Source.OfBase64.Data != "aGVsbG8=" {
		t.Errorf("expected data 'aGVsbG8=', got %q", ib.Source.OfBase64.Data)
	}
}

func TestAnthropic_BuildParams_NoImages(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Hello",
		},
	}

	params := adapter.buildParams(messages, nil)
	if len(params.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(params.Messages))
	}

	msgParam := params.Messages[0]
	if len(msgParam.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msgParam.Content))
	}

	if msgParam.Content[0].OfText == nil {
		t.Error("expected text block")
	}
}

func TestAnthropic_BuildParams_ImageOnly(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{
			Role: domain.RoleUser,
			Images: []domain.ImageContent{
				{MIMEType: "image/jpeg", Data: "dGVzdA=="},
			},
		},
	}

	params := adapter.buildParams(messages, nil)
	msgParam := params.Messages[0]

	if len(msgParam.Content) != 1 {
		t.Fatalf("expected 1 content block (image only), got %d", len(msgParam.Content))
	}

	if msgParam.Content[0].OfImage == nil {
		t.Error("expected image block")
	}
	if msgParam.Content[0].OfImage.Source.OfBase64 == nil {
		t.Fatal("expected base64 source")
	}
	if string(msgParam.Content[0].OfImage.Source.OfBase64.MediaType) != "image/jpeg" {
		t.Errorf("expected media_type 'image/jpeg', got %q",
			msgParam.Content[0].OfImage.Source.OfBase64.MediaType)
	}
}

func TestAnthropic_BuildParams_SystemPrompt(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{Role: domain.RoleSystem, Content: "You are helpful."},
		{Role: domain.RoleUser, Content: "Hello"},
	}

	params := adapter.buildParams(messages, nil)

	if len(params.System) == 0 {
		t.Fatal("expected system prompt, got none")
	}
	if params.System[0].Text != "You are helpful.\n" {
		t.Errorf("unexpected system text: %q", params.System[0].Text)
	}
}

func TestAnthropic_BuildParams_ToolCalls(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleAssistant,
			Content: "Using tool",
			ToolCalls: []domain.ToolCall{
				{
					ID:        "tool-1",
					Name:      "search",
					Arguments: map[string]interface{}{"q": "test"},
				},
			},
		},
	}

	params := adapter.buildParams(messages, nil)
	msgParam := params.Messages[0]

	if string(msgParam.Role) != "assistant" {
		t.Errorf("expected assistant role, got %q", msgParam.Role)
	}

	if len(msgParam.Content) != 2 {
		t.Fatalf("expected 2 blocks (text + tool_use), got %d", len(msgParam.Content))
	}

	if msgParam.Content[0].OfText == nil {
		t.Error("expected first block to be text")
	}
	if msgParam.Content[1].OfToolUse == nil {
		t.Error("expected second block to be tool_use")
	}
	if msgParam.Content[1].OfToolUse.ID != "tool-1" {
		t.Errorf("expected tool id 'tool-1', got %q", msgParam.Content[1].OfToolUse.ID)
	}
}

func TestAnthropic_BuildParams_OptionsApplied(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{Role: domain.RoleUser, Content: "hi"},
	}

	params := adapter.buildParams(messages, []ports.ChatOpt{
		ports.WithMaxTokens(512),
	})

	if params.MaxTokens != 512 {
		t.Errorf("expected max_tokens 512, got %d", params.MaxTokens)
	}
}

func TestAnthropic_BuildParams_JSONShape(t *testing.T) {
	adapter := &AnthropicAdapter{
		model: "claude-sonnet-4-5-20250929",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Describe this image",
			Images: []domain.ImageContent{
				{MIMEType: "image/png", Data: "aGVsbG8="},
			},
		},
	}

	params := adapter.buildParams(messages, nil)

	// Serialize to JSON and verify the exact shape.
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	var raw struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text,omitempty"`
				Source *struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal params JSON: %v", err)
	}

	if len(raw.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(raw.Messages))
	}
	msg := raw.Messages[0]
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(msg.Content))
	}

	// Text block.
	if msg.Content[0].Type != "text" {
		t.Errorf("expected text block type, got %q", msg.Content[0].Type)
	}
	if msg.Content[0].Text != "Describe this image" {
		t.Errorf("unexpected text: %q", msg.Content[0].Text)
	}

	// Image block.
	if msg.Content[1].Type != "image" {
		t.Errorf("expected image block type, got %q", msg.Content[1].Type)
	}
	if msg.Content[1].Source == nil {
		t.Fatal("expected source in image block")
	}
	if msg.Content[1].Source.Type != "base64" {
		t.Errorf("expected base64 source type, got %q", msg.Content[1].Source.Type)
	}
	if msg.Content[1].Source.MediaType != "image/png" {
		t.Errorf("expected image/png media_type, got %q", msg.Content[1].Source.MediaType)
	}
	if msg.Content[1].Source.Data != "aGVsbG8=" {
		t.Errorf("expected data 'aGVsbG8=', got %q", msg.Content[1].Source.Data)
	}
}
