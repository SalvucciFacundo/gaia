package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type mockLLMProvider struct {
	name       string
	response   string
	err        error
	delay      time.Duration
	receivedMs []domain.Message
}

func (m *mockLLMProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	m.receivedMs = msgs
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &domain.Message{
		Role:    domain.RoleAssistant,
		Content: m.response,
	}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}

func (m *mockLLMProvider) Tools() []domain.ToolDef {
	return nil
}

func (m *mockLLMProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{m.name}, nil
}

func TestMoARunner_Empty(t *testing.T) {
	runner := NewOneShotMoARunner(&mockLLMProvider{name: "primary"}, nil, 1*time.Second)
	res, err := runner.RunPrompt(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil for empty providers, got: %v", res)
	}
}

func TestMoARunner_SingleProvider(t *testing.T) {
	primary := &mockLLMProvider{name: "primary", response: "primary answer"}
	p1 := &mockLLMProvider{name: "modelA", response: "modelA answer"}

	providers := map[string]ports.LLMProvider{
		"modelA": p1,
	}

	runner := NewOneShotMoARunner(primary, providers, 1*time.Second)
	res, err := runner.RunPrompt(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Content != "modelA answer" {
		t.Fatalf("expected direct single response `modelA answer`, got: %v", res)
	}
}

func TestMoARunner_MultipleProviders_Synthesizes(t *testing.T) {
	primary := &mockLLMProvider{name: "primary", response: "synthesized best answer"}
	p1 := &mockLLMProvider{name: "modelA", response: "response A"}
	p2 := &mockLLMProvider{name: "modelB", response: "response B"}

	providers := map[string]ports.LLMProvider{
		"modelA": p1,
		"modelB": p2,
	}

	runner := NewOneShotMoARunner(primary, providers, 1*time.Second)
	res, err := runner.RunPrompt(context.Background(), "explain quantum computing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Content != "synthesized best answer" {
		t.Fatalf("expected synthesized answer, got: %v", res)
	}

	// Verify synthesizer received both responses in prompt
	var synthPrompt string
	for _, m := range primary.receivedMs {
		if m.Role == domain.RoleUser {
			synthPrompt = m.Content
		}
	}
	if !strings.Contains(synthPrompt, "response A") || !strings.Contains(synthPrompt, "response B") {
		t.Fatalf("expected synthesis prompt to include responses from both models, got: %s", synthPrompt)
	}
}

func TestMoARunner_SynthesisFailureFallback(t *testing.T) {
	primary := &mockLLMProvider{name: "primary", err: errors.New("synthesis model failed")}
	p1 := &mockLLMProvider{name: "modelA", response: "fallback answer A"}
	p2 := &mockLLMProvider{name: "modelB", response: "fallback answer B"}

	providers := map[string]ports.LLMProvider{
		"modelA": p1,
		"modelB": p2,
	}

	runner := NewOneShotMoARunner(primary, providers, 1*time.Second)
	res, err := runner.RunPrompt(context.Background(), "explain quantum computing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || (res.Content != "fallback answer A" && res.Content != "fallback answer B") {
		t.Fatalf("expected fallback to first available response, got: %v", res)
	}
}
