package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// mockProvider implements ports.LLMProvider for tests.
type mockProvider struct {
	name     string
	chatFn   func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error)
	streamFn func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error)
	toolsFn  func() []domain.ToolDef
}

func (m *mockProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, msgs, opts...)
	}
	return &domain.Message{Role: domain.RoleAssistant, Content: "mock " + m.name}, nil
}

func (m *mockProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, msgs, opts...)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprintf(pw, `{"content":"mock %s","done":true}`+"\n", m.name)
	}()
	return pr, nil
}

func (m *mockProvider) Tools() []domain.ToolDef {
	if m.toolsFn != nil {
		return m.toolsFn()
	}
	return nil
}

func TestRouter_PrimarySucceeds(t *testing.T) {
	primary := &mockProvider{name: "primary"}
	fallback := &mockProvider{name: "fallback"}
	router := NewRouter([]ports.LLMProvider{primary, fallback})

	resp, err := router.Chat(context.Background(), []domain.Message{{Role: domain.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock primary" {
		t.Errorf("expected primary response, got %q", resp.Content)
	}
}

func TestRouter_FallbackOnError(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		chatFn: func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
			return nil, errors.New("primary down")
		},
	}
	fallback := &mockProvider{name: "fallback"}
	router := NewRouter([]ports.LLMProvider{primary, fallback})

	resp, err := router.Chat(context.Background(), []domain.Message{{Role: domain.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock fallback" {
		t.Errorf("expected fallback response, got %q", resp.Content)
	}
}

func TestRouter_AllFail(t *testing.T) {
	errPrimary := &mockProvider{
		name: "primary",
		chatFn: func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
			return nil, errors.New("primary down")
		},
	}
	errFallback := &mockProvider{
		name: "fallback",
		chatFn: func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
			return nil, errors.New("fallback down")
		},
	}
	router := NewRouter([]ports.LLMProvider{errPrimary, errFallback})

	_, err := router.Chat(context.Background(), []domain.Message{{Role: domain.RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestRouter_ToolsDelegatesToPrimary(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		toolsFn: func() []domain.ToolDef {
			return []domain.ToolDef{{Name: "test_tool"}}
		},
	}
	router := NewRouter([]ports.LLMProvider{primary})

	tools := router.Tools()
	if len(tools) != 1 || tools[0].Name != "test_tool" {
		t.Errorf("expected [test_tool], got %v", tools)
	}
}

func TestRouter_StreamFallback(t *testing.T) {
	primary := &mockProvider{
		name: "primary",
		streamFn: func(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
			return nil, errors.New("primary stream down")
		},
	}
	fallback := &mockProvider{name: "fallback"}
	router := NewRouter([]ports.LLMProvider{primary, fallback})

	stream, err := router.Stream(context.Background(), []domain.Message{{Role: domain.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Close()

	data, _ := io.ReadAll(stream)
	if len(data) == 0 {
		t.Error("expected stream data, got empty")
	}
}

func TestRouter_EmptyChain(t *testing.T) {
	router := NewRouter(nil)
	_, err := router.Chat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error with empty provider chain")
	}
}
