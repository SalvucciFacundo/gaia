package agent

import (
	"context"
	"strings"
	"testing"

	"gaia/internal/blackboard"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type capturingProvider struct {
	lastMessages []domain.Message
	responseFunc func(iter int) *domain.Message
	calls        int
}

func (c *capturingProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	c.calls++
	c.lastMessages = make([]domain.Message, len(msgs))
	copy(c.lastMessages, msgs)

	if c.responseFunc != nil {
		return c.responseFunc(c.calls), nil
	}
	return &domain.Message{Role: domain.RoleAssistant, Content: "ok"}, nil
}

func (c *capturingProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, nil
}
func (c *capturingProvider) Tools() []domain.ToolDef                     { return nil }
func (c *capturingProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

func TestRunLoop_BlackboardInputInjection(t *testing.T) {
	bb := blackboard.New(t.TempDir())
	_, err := bb.Set("", "global.shared_fact", []byte("AuthTokenV2Enabled"))
	if err != nil {
		t.Fatalf("failed to set blackboard: %v", err)
	}

	provider := &capturingProvider{}
	spawner := NewSpawner(SpawnerConfig{
		Provider:   provider,
		Tools:      core.NewToolRegistry(),
		Blackboard: bb,
		Budget:     domain.BudgetConfig{MaxIterations: 5},
	}, NewRegistry())

	task := domain.SubagentTask{
		ID:          "task-123",
		Description: "Verify auth token config",
		InputKeys:   []string{"global.shared_fact"},
		Ephemeral:   true,
	}

	resp, err := spawner.RunLoop(context.Background(), task, "Base system prompt")
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("unexpected response: %s", resp.Content)
	}

	// Verify that the blackboard input was injected into the prompt
	if len(provider.lastMessages) == 0 {
		t.Fatal("expected messages received by provider")
	}
	systemContent := provider.lastMessages[0].Content
	if !strings.Contains(systemContent, "AuthTokenV2Enabled") || !strings.Contains(systemContent, "global.shared_fact") {
		t.Errorf("expected blackboard fact in system prompt, got: %s", systemContent)
	}

	// Verify that ephemeral output was flushed to blackboard
	entry, err := bb.Get("tasks.task-123.output")
	if err != nil {
		t.Fatalf("expected output key in blackboard: %v", err)
	}
	payload, _ := blackboard.ReadPayload(entry)
	if string(payload) != "ok" {
		t.Errorf("expected blackboard output 'ok', got %s", string(payload))
	}
}

func TestRunLoop_EphemeralBoundedTurns(t *testing.T) {
	bb := blackboard.New(t.TempDir())
	reg := core.NewToolRegistry()
	reg.Register(&dummyEchoModule{})

	provider := &capturingProvider{
		responseFunc: func(iter int) *domain.Message {
			// Always return a tool call to simulate loop
			return &domain.Message{
				Role: domain.RoleAssistant,
				ToolCalls: []domain.ToolCall{
					{
						ID:   "call-1",
						Name: "echo",
						Arguments: map[string]interface{}{
							"msg": "ping",
						},
					},
				},
			}
		},
	}

	spawner := NewSpawner(SpawnerConfig{
		Provider:   provider,
		Tools:      reg,
		Blackboard: bb,
		Budget:     domain.BudgetConfig{MaxIterations: 10}, // Even with 10, ephemeral limits to 3
	}, NewRegistry())

	task := domain.SubagentTask{
		ID:           "bounded-task",
		Description:  "Infinite tool caller",
		AllowedTools: []string{"echo"},
		Ephemeral:    true,
	}

	resp, err := spawner.RunLoop(context.Background(), task, "You are a runner")
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	// Ephemeral bound must terminate after exactly 3 turns
	if provider.calls != 3 {
		t.Errorf("expected 3 iterations for ephemeral task, got %d", provider.calls)
	}
	if !strings.Contains(resp.Content, "budget exhausted after 3 iterations") {
		t.Errorf("expected budget exhausted message, got: %s", resp.Content)
	}

	// Verify output stored in Blackboard
	entry, err := bb.Get("tasks.bounded-task.output")
	if err != nil {
		t.Fatalf("expected output stored in blackboard: %v", err)
	}
	payload, _ := blackboard.ReadPayload(entry)
	if !strings.Contains(string(payload), "budget exhausted after 3 iterations") {
		t.Errorf("unexpected output payload: %s", string(payload))
	}
}

type dummyEchoModule struct{}

func (d *dummyEchoModule) Name() string        { return "echo" }
func (d *dummyEchoModule) Description() string { return "echo" }
func (d *dummyEchoModule) GetTools() []domain.ToolCall {
	return []domain.ToolCall{{Name: "echo"}}
}
func (d *dummyEchoModule) Tools() []domain.ToolDef {
	return []domain.ToolDef{{Name: "echo"}}
}
func (d *dummyEchoModule) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error) {
	return &domain.ToolResult{Success: true, Output: "pong"}, nil
}
