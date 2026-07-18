package core

import (
	"context"
	"fmt"
	"io"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// stubProvider records calls and returns canned responses.
type stubProvider struct {
	chatCalls   int
	streamCalls int
	resp        *domain.Message
	chatErr     error
	streamErr   error
}

func (s *stubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	s.chatCalls++
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *stubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	s.streamCalls++
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprintf(pw, `{"content":"%s","done":true}`+"\n", s.resp.Content)
	}()
	return pr, nil
}

func (s *stubProvider) Tools() []domain.ToolDef { return nil }

// stubRepo is a no-op repository for tests.
type stubRepo struct{}

func (r *stubRepo) SaveMessage(ctx context.Context, msg domain.Message) error    { return nil }
func (r *stubRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}
func (r *stubRepo) CreateSession(ctx context.Context, name string) (string, error) {
	return "test-session", nil
}
func (r *stubRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) {
	return []domain.Message{}, nil
}

// stubUI records Display calls.
type stubUI struct {
	displayed []domain.Message
}

func (u *stubUI) Display(msg domain.Message) error {
	u.displayed = append(u.displayed, msg)
	return nil
}
func (u *stubUI) AppendToken(content string) error { return nil }
func (u *stubUI) PromptConfirmation(prompt string) (bool, error)   { return true, nil }
func (u *stubUI) Run() error                                        { return nil }

func TestBrain_BudgetExhausted(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll call a tool",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "fake_tool", Arguments: map[string]interface{}{}},
			},
		},
		// Fail streaming so the brain falls back to Chat (which returns tool calls).
		streamErr: io.EOF,
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 3}
	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 3 iterations and tool calls every time, we should hit budget.
	if len(ui.displayed) == 0 {
		t.Fatal("expected at least one displayed message")
	}
	last := ui.displayed[len(ui.displayed)-1]
	if last.Content != "Iteration budget exhausted (3 iterations). Stopping." {
		t.Errorf("expected budget exhausted message, got %q", last.Content)
	}
}

func TestBrain_SimpleChat(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Hello, how can I help?",
		},
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 25}
	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.displayed) != 1 {
		t.Fatalf("expected 1 displayed msg, got %d", len(ui.displayed))
	}
	if ui.displayed[0].Content != "Hello, how can I help?" {
		t.Errorf("unexpected response: %q", ui.displayed[0].Content)
	}
}
