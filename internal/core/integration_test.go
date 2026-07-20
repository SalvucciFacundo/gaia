package core

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// loopMockProvider returns tool calls for N iterations, then a final text response.
type loopMockProvider struct {
	chatCalls    int
	streamCalls  int
	toolCallName string
	toolArgs     map[string]interface{}
	toolCycles   int // how many iterations to return tool calls
	streamErr    error
}

func (p *loopMockProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	p.chatCalls++
	if p.chatCalls <= p.toolCycles {
		return &domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Calling tool (iteration %d)", p.chatCalls),
			ToolCalls: []domain.ToolCall{
				{ID: fmt.Sprintf("tc-%d", p.chatCalls), Name: p.toolCallName, Arguments: p.toolArgs},
			},
		}, nil
	}
	return &domain.Message{
		Role:    domain.RoleAssistant,
		Content: "All done!",
	}, nil
}

func (p *loopMockProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	p.streamCalls++
	if p.streamErr != nil {
		return nil, p.streamErr
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if p.chatCalls+1 <= p.toolCycles {
			fmt.Fprintf(pw, `{"content":"Calling tool","done":true}`+"\n")
		} else {
			fmt.Fprintf(pw, `{"content":"All done!","done":true}`+"\n")
		}
	}()
	return pr, nil
}

func (p *loopMockProvider) Tools() []domain.ToolDef { return nil }

// echoModule is a simple module that echoes back its arguments.
type echoModule struct{}

func (m *echoModule) Name() string        { return "echo" }
func (m *echoModule) Description() string { return "Echo module for integration testing" }
func (m *echoModule) GetTools() []domain.ToolCall {
	return []domain.ToolCall{{
		Name: "echo_tool",
		Arguments: map[string]interface{}{
			"message": "string — message to echo",
		},
	}}
}

func (m *echoModule) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	if toolName != "echo_tool" {
		return &domain.ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", toolName)}, nil
	}
	msg, _ := args["message"].(string)
	if msg == "" {
		msg = "no message provided"
	}
	return &domain.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("echo: %s", msg),
	}, nil
}

// TestIntegration_ToolCallLoopHaltsAtBudget verifies that the agent loop
// stops executing when the iteration budget is exhausted, even when the
// LLM keeps returning tool calls.
func TestIntegration_ToolCallLoopHaltsAtBudget(t *testing.T) {
	prov := &loopMockProvider{
		toolCallName: "echo_tool",
		toolArgs:     map[string]interface{}{"message": "hello"},
		toolCycles:   100, // infinite tool calls
		streamErr:    io.EOF,
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 3}

	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)
	brain.RegisterModule(&echoModule{})

	err := brain.ProcessMessage(context.Background(), "do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.displayed) == 0 {
		t.Fatal("expected at least one displayed message")
	}

	last := ui.displayed[len(ui.displayed)-1]
	if !strings.Contains(last.Content, "budget exhausted") {
		t.Errorf("expected budget exhausted message, got %q", last.Content)
	}

	// Confirm the provider was called exactly budget.MaxIterations times.
	if prov.chatCalls != budget.MaxIterations {
		t.Errorf("expected %d LLM calls, got %d", budget.MaxIterations, prov.chatCalls)
	}
}

// TestIntegration_ToolCallLoopCompletesBeforeBudget verifies normal
// completion when the LLM returns a text response before budget is hit.
func TestIntegration_ToolCallLoopCompletesBeforeBudget(t *testing.T) {
	prov := &loopMockProvider{
		toolCallName: "echo_tool",
		toolArgs:     map[string]interface{}{"message": "work"},
		toolCycles:   2, // return tool calls for 2 iterations, then text
		streamErr:    io.EOF,
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 10}

	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)
	brain.RegisterModule(&echoModule{})

	err := brain.ProcessMessage(context.Background(), "do some work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ui.displayed) == 0 {
		t.Fatal("expected at least one displayed message")
	}

	last := ui.displayed[len(ui.displayed)-1]
	if last.Content != "All done!" {
		t.Errorf("expected 'All done!', got %q", last.Content)
	}
	if prov.chatCalls != 3 {
		t.Errorf("expected 3 LLM calls (2 tool + 1 final), got %d", prov.chatCalls)
	}
}

// TestIntegration_UnknownToolReturnsError verifies the registry returns
// an error result when a tool is not registered, without crashing the loop.
func TestIntegration_UnknownToolReturnsError(t *testing.T) {
	prov := &loopMockProvider{
		toolCallName: "nonexistent_tool",
		toolArgs:     map[string]interface{}{},
		toolCycles:   1,
		streamErr:    io.EOF,
	}
	ui := &stubUI{}
	budget := domain.BudgetConfig{MaxIterations: 3}

	brain := NewBrain(prov, &stubRepo{}, ui, nil, budget)

	err := brain.ProcessMessage(context.Background(), "call unknown tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still complete (not panic) and return the text response.
	last := ui.displayed[len(ui.displayed)-1]
	if last.Content != "All done!" {
		t.Errorf("expected 'All done!', got %q", last.Content)
	}
}
