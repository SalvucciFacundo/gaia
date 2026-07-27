package llm

import (
	"encoding/json"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

func TestOpenAI_BuildRequest_WithImages(t *testing.T) {
	adapter := &OpenAIAdapter{
		model: "gpt-4o",
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

	req := adapter.buildRequest(messages, nil, false)

	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}

	msg := req.Messages[0]
	if len(msg.MultiContent) != 2 {
		t.Fatalf("expected 2 content parts (text + image), got %d", len(msg.MultiContent))
	}

	// First part should be text.
	if msg.MultiContent[0].Type != openai.ChatMessagePartTypeText {
		t.Errorf("expected first part type 'text', got %q", msg.MultiContent[0].Type)
	}
	if msg.MultiContent[0].Text != "What is in this image?" {
		t.Errorf("unexpected text: %q", msg.MultiContent[0].Text)
	}

	// Second part should be image.
	if msg.MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
		t.Errorf("expected second part type 'image_url', got %q", msg.MultiContent[1].Type)
	}
	if msg.MultiContent[1].ImageURL == nil {
		t.Fatal("expected ImageURL to be non-nil")
	}
	if msg.MultiContent[1].ImageURL.URL != "data:image/png;base64,aGVsbG8=" {
		t.Errorf("unexpected image URL: %q", msg.MultiContent[1].ImageURL.URL)
	}

	// Verify the full JSON shape matches spec.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var raw struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text,omitempty"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(reqJSON, &raw); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if len(raw.Messages) == 0 {
		t.Fatal("expected messages in JSON")
	}
	parts := raw.Messages[0].Content
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts in JSON, got %d", len(parts))
	}
	if parts[0].Type != "text" {
		t.Errorf("expected text type, got %q", parts[0].Type)
	}
	if parts[1].Type != "image_url" {
		t.Errorf("expected image_url type, got %q", parts[1].Type)
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,aGVsbG8=" {
		t.Errorf("unexpected image_url: %+v", parts[1].ImageURL)
	}
}

func TestOpenAI_BuildRequest_NoImages(t *testing.T) {
	adapter := &OpenAIAdapter{
		model: "gpt-4o",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Hello",
		},
	}

	req := adapter.buildRequest(messages, nil, false)
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}

	msg := req.Messages[0]
	if msg.Content != "Hello" {
		t.Errorf("expected Content 'Hello', got %q", msg.Content)
	}
	if len(msg.MultiContent) != 0 {
		t.Errorf("expected no MultiContent, got %d parts", len(msg.MultiContent))
	}
}

func TestOpenAI_BuildRequest_ImageOnly(t *testing.T) {
	adapter := &OpenAIAdapter{
		model: "gpt-4o",
	}

	messages := []domain.Message{
		{
			Role: domain.RoleUser,
			Images: []domain.ImageContent{
				{MIMEType: "image/jpeg", Data: "dGVzdA=="},
			},
		},
	}

	req := adapter.buildRequest(messages, nil, false)
	msg := req.Messages[0]

	if len(msg.MultiContent) != 1 {
		t.Fatalf("expected 1 content part (image only), got %d", len(msg.MultiContent))
	}
	if msg.MultiContent[0].Type != openai.ChatMessagePartTypeImageURL {
		t.Errorf("expected image_url type, got %q", msg.MultiContent[0].Type)
	}
}

func TestOpenAI_BuildRequest_ToolCalls(t *testing.T) {
	adapter := &OpenAIAdapter{
		model: "gpt-4o",
	}

	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Use this tool",
			Images: []domain.ImageContent{
				{MIMEType: "image/png", Data: "aGVsbG8="},
			},
			ToolCalls: []domain.ToolCall{
				{
					ID:        "call-1",
					Name:      "search",
					Arguments: map[string]interface{}{"query": "test"},
				},
			},
		},
	}

	req := adapter.buildRequest(messages, nil, false)
	msg := req.Messages[0]

	// Images are on user role → MultiContent should be used.
	if len(msg.MultiContent) == 0 {
		t.Fatal("expected MultiContent for user message with images")
	}
	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 ToolCall, got %d", len(msg.ToolCalls))
	}
}

func TestOpenAI_BuildRequest_OptionsApplied(t *testing.T) {
	adapter := &OpenAIAdapter{
		model: "gpt-4o",
	}

	messages := []domain.Message{
		{Role: domain.RoleUser, Content: "hi"},
	}

	req := adapter.buildRequest(messages, []ports.ChatOpt{
		ports.WithTemperature(0.7),
		ports.WithMaxTokens(100),
	}, false)

	if req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", req.Temperature)
	}
	if req.MaxTokens != 100 {
		t.Errorf("expected max_tokens 100, got %d", req.MaxTokens)
	}
}
