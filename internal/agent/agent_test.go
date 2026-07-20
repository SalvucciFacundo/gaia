package agent

import (
	"context"
	"fmt"
	"io"
	"testing"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// --- Stubs ---

// stubProvider records calls and returns canned responses for subagent tests.
type stubProvider struct {
	chatCalls int
	resp      *domain.Message
	chatErr   error
	streamErr error
}

func (s *stubProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	s.chatCalls++
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.resp, nil
}

func (s *stubProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
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

// --- Test helpers ---

func newTestSpawner(prov ports.LLMProvider) *Spawner {
	cfg := SpawnerConfig{
		Provider: prov,
		Tools:    core.NewToolRegistry(),
		Budget:   domain.BudgetConfig{MaxIterations: 5},
	}
	return NewSpawner(cfg, NewRegistry())
}

// newTask creates a minimal test task.
func newTask() domain.SubagentTask {
	return domain.SubagentTask{
		ID:          "test-1",
		Description: "Investigate the codebase structure",
		Mode:        "plan",
	}
}

// --- 1.9 Tests ---

// testSub is a minimal Subagent for interface contract testing.
type testSub struct {
	name string
	desc string
	res  *domain.SubagentResult
}

func (s *testSub) Name() string        { return s.name }
func (s *testSub) Description() string { return s.desc }
func (s *testSub) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	return s.res
}

// TestSubagentContract verifies a Subagent implementation satisfies the interface.
func TestSubagentContract(t *testing.T) {
	// Compile-time: testSub implements Subagent
	var _ Subagent = (*testSub)(nil)

	sa := &testSub{name: "test", desc: "A test subagent"}
	if sa.Name() != "test" {
		t.Errorf("expected name 'test', got %q", sa.Name())
	}
	if sa.Description() != "A test subagent" {
		t.Errorf("expected description, got %q", sa.Description())
	}
}

// stubExecuteSubagent is a full Subagent implementation for tests.
type stubExecuteSubagent struct {
	name string
	desc string
	res  *domain.SubagentResult
}

func (s *stubExecuteSubagent) Name() string        { return s.name }
func (s *stubExecuteSubagent) Description() string { return s.desc }
func (s *stubExecuteSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	return s.res
}

var _ Subagent = (*stubExecuteSubagent)(nil)

// TestSpawnerIsolation verifies that the Spawner creates isolated contexts.
func TestSpawnerIsolation(t *testing.T) {
	expected := &domain.SubagentResult{
		Status:          domain.SubagentSuccess,
		Summary:         "Codebase structure analyzed.",
		Artifacts:       []string{"exploration-report"},
		NextRecommended: "sdd-propose",
		Risks:           nil,
		SkillResolution: "none",
	}

	reg := NewRegistry()
	sa := &stubExecuteSubagent{name: "explorer", desc: "test explorer", res: expected}
	reg.Register("explorer", func(spawner *Spawner) Subagent { return sa })

	spawner := NewSpawner(SpawnerConfig{
		Provider: &stubProvider{},
		Tools:    core.NewToolRegistry(),
		Budget:   domain.DefaultBudget(),
	}, reg)

	task := newTask()
	result, err := spawner.Spawn(context.Background(), "explorer", task)
	if err != nil {
		t.Fatalf("unexpected error spawning: %v", err)
	}
	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected status success, got %q", result.Status)
	}
	if result.Summary != expected.Summary {
		t.Errorf("expected summary %q, got %q", expected.Summary, result.Summary)
	}
	if result.NextRecommended != "sdd-propose" {
		t.Errorf("expected next 'sdd-propose', got %q", result.NextRecommended)
	}
}

// TestSpawnerUnknownSubagent verifies error on unknown subagent name.
func TestSpawnerUnknownSubagent(t *testing.T) {
	spawner := newTestSpawner(&stubProvider{})
	_, err := spawner.Spawn(context.Background(), "nonexistent", newTask())
	if err == nil {
		t.Fatal("expected error for unknown subagent")
	}
}

// TestSpawnerNilResult verifies that a nil result is handled gracefully.
func TestSpawnerNilResult(t *testing.T) {
	reg := NewRegistry()
	sa := &stubExecuteSubagent{name: "nil-agent", desc: "returns nil", res: nil}
	reg.Register("nil-agent", func(spawner *Spawner) Subagent { return sa })

	spawner := NewSpawner(SpawnerConfig{
		Provider: &stubProvider{},
		Tools:    core.NewToolRegistry(),
		Budget:   domain.DefaultBudget(),
	}, reg)

	result, err := spawner.Spawn(context.Background(), "nil-agent", newTask())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for nil subagent output")
	}
	if result.Status != domain.SubagentBlocked {
		t.Errorf("expected blocked status for nil result, got %q", result.Status)
	}
}

// TestRegistryLookup verifies basic registry operations.
func TestRegistryLookup(t *testing.T) {
	reg := NewRegistry()
	if len(reg.Available()) != 0 {
		t.Error("new registry should be empty")
	}

	sa := &stubExecuteSubagent{name: "explorer", desc: "explorer"}
	err := reg.Register("explorer", func(spawner *Spawner) Subagent { return sa })
	if err != nil {
		t.Fatalf("unexpected error registering: %v", err)
	}

	// Duplicate registration should fail
	err = reg.Register("explorer", func(spawner *Spawner) Subagent { return sa })
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}

	// Lookup should succeed
	factory, ok := reg.Get("explorer")
	if !ok {
		t.Fatal("expected to find 'explorer'")
	}
	sub := factory(nil)
	if sub.Name() != "explorer" {
		t.Errorf("expected name 'explorer', got %q", sub.Name())
	}

	// Available should return the name
	avail := reg.Available()
	if len(avail) != 1 || avail[0] != "explorer" {
		t.Errorf("expected ['explorer'], got %v", avail)
	}

	// Missing lookup
	_, ok = reg.Get("missing")
	if ok {
		t.Error("expected false for missing subagent")
	}
}

// TestRegistryMultiple verifies multiple registrations.
func TestRegistryMultiple(t *testing.T) {
	reg := NewRegistry()
	sa1 := &stubExecuteSubagent{name: "alpha", desc: "a"}
	sa2 := &stubExecuteSubagent{name: "beta", desc: "b"}

	reg.Register("alpha", func(spawner *Spawner) Subagent { return sa1 })
	reg.Register("beta", func(spawner *Spawner) Subagent { return sa2 })

	avail := reg.Available()
	if len(avail) != 2 {
		t.Errorf("expected 2 available, got %d", len(avail))
	}
}

// TestSpawnerAvailable verifies that Spawner.Available delegates to registry.
func TestSpawnerAvailable(t *testing.T) {
	spawner := newTestSpawner(&stubProvider{})
	if len(spawner.Available()) != 0 {
		t.Error("empty spawner should report no agents")
	}

	spawner.registry.Register("explorer", func(sp *Spawner) Subagent {
		return &stubExecuteSubagent{name: "explorer", desc: "e"}
	})

	avail := spawner.Available()
	if len(avail) != 1 || avail[0] != "explorer" {
		t.Errorf("expected ['explorer'], got %v", avail)
	}
}

// TestSubagentTaskFields verifies that SubagentTask carries all expected fields.
func TestSubagentTaskFields(t *testing.T) {
	task := domain.SubagentTask{
		ID:           "task-42",
		Description:  "Explore the auth module",
		KGContext:    []string{"Auth uses JWT", "Token expires in 24h"},
		Skills:       []string{"go-testing", "code-search"},
		AllowedTools: []string{"file_read", "grep", "glob"},
		Mode:         "plan",
	}

	if task.ID != "task-42" {
		t.Errorf("expected ID 'task-42', got %q", task.ID)
	}
	if len(task.AllowedTools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(task.AllowedTools))
	}
	if task.Mode != "plan" {
		t.Errorf("expected mode 'plan', got %q", task.Mode)
	}
}

// TestSubagentResultFields verifies SubagentResult carries all expected fields.
func TestSubagentResultFields(t *testing.T) {
	result := domain.SubagentResult{
		Status:          domain.SubagentSuccess,
		Summary:         "All done.",
		Artifacts:       []string{"spec/foo", "design/foo"},
		NextRecommended: "sdd-design",
		Risks:           []string{"Large scope change"},
		SkillResolution: "paths-injected",
	}

	if result.Status != domain.SubagentSuccess {
		t.Errorf("expected success, got %q", result.Status)
	}
	if result.NextRecommended != "sdd-design" {
		t.Errorf("expected next 'sdd-design', got %q", result.NextRecommended)
	}
	if len(result.Risks) != 1 {
		t.Errorf("expected 1 risk, got %d", len(result.Risks))
	}
}

// TestBuildSystemPrompt verifies system prompt generation from task context.
func TestBuildSystemPrompt(t *testing.T) {
	task := domain.SubagentTask{
		ID:          "explore-001",
		Description: "Find auth patterns",
		KGContext:   []string{"JWT used for API auth"},
		Skills:      []string{"go-testing", "code-search"},
		Mode:        "plan",
	}

	prompt := BuildSystemPrompt("explorer", "Investigates code", task)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(prompt, "explorer") {
		t.Error("prompt should mention subagent role")
	}
	if !contains(prompt, "explore-001") {
		t.Error("prompt should contain task ID")
	}
	if !contains(prompt, "JWT used for API auth") {
		t.Error("prompt should contain KG context")
	}
	if !contains(prompt, "go-testing") {
		t.Error("prompt should contain skills")
	}
	if !contains(prompt, "plan") {
		t.Error("prompt should contain mode")
	}
}

// TestRunLoopDirectResponse verifies that RunLoop handles a simple response.
func TestRunLoopDirectResponse(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Found auth patterns in internal/auth/.",
		},
	}

	spawner := newTestSpawner(prov)
	task := newTask()
	prompt := "You are the explorer subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Found auth patterns in internal/auth/." {
		t.Errorf("unexpected response: %q", resp.Content)
	}
	if prov.chatCalls != 1 {
		t.Errorf("expected 1 chat call, got %d", prov.chatCalls)
	}
}

// TestRunLoopBudgetExhausted verifies that the agent loop stops on budget.
func TestRunLoopBudgetExhausted(t *testing.T) {
	// Provider that always returns tool calls — forces iteration loop.
	prov := &stubProvider{
		resp: &domain.Message{
			Role: domain.RoleAssistant,
			Content: "calling tool",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "unknown_tool", Arguments: map[string]interface{}{}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	// Override budget to 2 to keep test fast.
	spawner.cfg.Budget.MaxIterations = 2

	task := newTask()
	prompt := "You are the explorer subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(resp.Content, "budget exhausted") {
		t.Errorf("expected budget exhausted message, got: %q", resp.Content)
	}
}

// contains is a helper to check substring presence.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
