package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"gaia/internal/core/domain"
	"gaia/internal/server"
)

type mockBrain struct {
	mu          sync.Mutex
	events      []string
	err         error
	delay       time.Duration
	lastSession string
	lastPrompt  string
	lastAtts    []domain.Attachment
	messages    map[string][]domain.Message
}

func newMockBrain() *mockBrain {
	return &mockBrain{
		messages: make(map[string][]domain.Message),
	}
}

func (m *mockBrain) Step(ctx context.Context, input string) error {
	return m.StepWithSessionAndAttachments(ctx, "", input, nil, nil)
}

func (m *mockBrain) StepWithSessionAndAttachments(ctx context.Context, sessionID string, input string, attachments []domain.Attachment, sink func(string)) error {
	m.mu.Lock()
	err := m.err
	events := m.events
	delay := m.delay
	m.lastSession = sessionID
	m.lastPrompt = input
	m.lastAtts = attachments

	convKey := "conv-" + sessionID
	if sessionID == "" {
		convKey = "conv-default"
	}

	m.messages[convKey] = append(m.messages[convKey], domain.Message{
		Role:        domain.RoleUser,
		Content:     input,
		Attachments: attachments,
	})
	m.mu.Unlock()

	if err != nil {
		return err
	}

	var replyAccum strings.Builder
	for _, ev := range events {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if sink != nil {
			sink(ev)
		}
		replyAccum.WriteString(ev)
	}

	m.mu.Lock()
	m.messages[convKey] = append(m.messages[convKey], domain.Message{
		Role:    domain.RoleAssistant,
		Content: replyAccum.String(),
	})
	m.mu.Unlock()

	return nil
}

func TestChatCompletions_NonStreaming(t *testing.T) {
	brain := newMockBrain()
	brain.events = []string{"Hello", " from ", "GAIA!"}

	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		Brain:    brain,
		Provider: "ollama",
		Model:    "llama3.2",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "llama3.2",
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hi there"},
		},
		User: "session-user-42",
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", "custom-session-uuid")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp server.ChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("resp.Object = %q, want 'chat.completion'", resp.Object)
	}
	if !strings.HasPrefix(resp.ID, "chatcmpl-") {
		t.Errorf("resp.ID = %q, want prefix 'chatcmpl-'", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("got %d choices, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello from GAIA!" {
		t.Errorf("choice content = %q, want 'Hello from GAIA!'", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("choice finish_reason = %q, want 'stop'", resp.Choices[0].FinishReason)
	}

	// Verify conversation session persistence
	brain.mu.Lock()
	msgs := brain.messages["conv-custom-session-uuid"]
	brain.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("persisted %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "Hi there" {
		t.Errorf("user message = %q, want 'Hi there'", msgs[0].Content)
	}
	if msgs[1].Content != "Hello from GAIA!" {
		t.Errorf("assistant message = %q, want 'Hello from GAIA!'", msgs[1].Content)
	}
}

func TestChatCompletions_MultimodalContent(t *testing.T) {
	brain := newMockBrain()
	brain.events = []string{"I see an image with text."}

	opts := server.Options{
		Brain:    brain,
		Provider: "openai",
		Model:    "gpt-4o",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []server.ChatCompletionMessage{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "Describe this photo:"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/photo.jpg"}},
				},
			},
		},
		User: "user-session-123",
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp server.ChatCompletionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Choices[0].Message.Content != "I see an image with text." {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}

	// Verify conversation session resolution from req.User
	brain.mu.Lock()
	msgs := brain.messages["conv-user-session-123"]
	brain.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("persisted %d messages, want 2", len(msgs))
	}
	if len(msgs[0].Attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(msgs[0].Attachments))
	}
	if msgs[0].Attachments[0].URL != "https://example.com/photo.jpg" {
		t.Errorf("attachment URL = %q", msgs[0].Attachments[0].URL)
	}
}

func TestChatCompletions_MissingBrain(t *testing.T) {
	opts := server.Options{
		Brain: nil,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletions_ProviderError(t *testing.T) {
	brain := newMockBrain()
	brain.err = errors.New("upstream provider failure")

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
}

func TestChatCompletions_StreamingSSE(t *testing.T) {
	brain := newMockBrain()
	brain.events = []string{"Token1 ", "Token2"}

	opts := server.Options{
		Host:     "127.0.0.1",
		Port:     8080,
		Brain:    brain,
		Provider: "ollama",
		Model:    "llama3.2",
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Model: "llama3.2",
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Stream me"},
		},
		Stream: true,
	}
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Content-Type = %q, want containing 'text/event-stream'", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("body missing SSE data prefix: %s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]") {
		t.Errorf("body missing terminal 'data: [DONE]': %s", body)
	}

	// Verify chunks parsing
	lines := strings.Split(body, "\n")
	var tokenAccum strings.Builder
	var sawStopReason bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}

		var chunk server.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			t.Fatalf("failed to unmarshal chunk %q: %v", data, err)
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("chunk.Object = %q, want 'chat.completion.chunk'", chunk.Object)
		}
		if len(chunk.Choices) > 0 {
			tokenAccum.WriteString(chunk.Choices[0].Delta.Content)
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
				sawStopReason = true
			}
		}
	}

	if tokenAccum.String() != "Token1 Token2" {
		t.Errorf("streamed text = %q, want 'Token1 Token2'", tokenAccum.String())
	}
	if !sawStopReason {
		t.Error("expected finish_reason 'stop' chunk before [DONE]")
	}
}

func TestChatCompletions_ErrorHandling(t *testing.T) {
	brain := newMockBrain()

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "method not allowed GET",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid json body",
			method:     http.MethodPost,
			body:       `{"messages": invalid}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request_error",
		},
		{
			name:       "empty messages list",
			method:     http.MethodPost,
			body:       `{"messages": []}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "missing_required_parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/v1/chat/completions", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestChatCompletions_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	brain := newMockBrain()
	brain.events = []string{"chunk 1", "chunk 2"}
	brain.delay = 50 * time.Millisecond

	opts := server.Options{
		Brain: brain,
	}
	srv := server.New(opts)

	reqBody := server.ChatCompletionRequest{
		Messages: []server.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}
	payload, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Cancel context after a small delay
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	srv.Handler().ServeHTTP(rec, req)
	// Give goroutines time to exit before goleak verification
	time.Sleep(100 * time.Millisecond)
}
