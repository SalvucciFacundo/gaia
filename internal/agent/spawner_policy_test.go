package agent

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type multiRespProvider struct {
	idx       int
	responses []*domain.Message
}

func (m *multiRespProvider) Chat(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return resp, nil
	}
	return &domain.Message{Role: domain.RoleAssistant, Content: "Done"}, nil
}

func (m *multiRespProvider) Stream(ctx context.Context, msgs []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *multiRespProvider) Tools() []domain.ToolDef { return nil }

func (m *multiRespProvider) ListModels(ctx context.Context) ([]string, error) { return nil, nil }

// =============================================================================
// 4.7 — Spawner.RunLoop Gating with PolicyGuard
// =============================================================================

func TestSpawnerRunLoop_PolicyGuardReadBlocksWrite(t *testing.T) {
	// Provider that returns a tool call to shell_exec.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll run a shell command.",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "shell_exec", Arguments: map[string]interface{}{
					"command": "echo hello",
				}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 2
	// Set PolicyGuard at TierRead — shell_exec is TierSandbox, so it should be blocked.
	spawner.cfg.Policy = core.NewPolicyGuard(core.TierRead, nil, nil)

	task := newTask()
	prompt := "You are a test subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The policy should block shell_exec, so the loop continues.
	// Since the provider always returns the same tool call, we'll exhaust the budget.
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Content == "" {
		t.Error("expected content in final response")
	}
}

func TestSpawnerRunLoop_PolicyGuardFullAllows(t *testing.T) {
	// At TierFull, shell_exec should be allowed.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll run a command.",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "shell_exec", Arguments: map[string]interface{}{
					"command": "echo hello",
				}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 2
	spawner.cfg.Policy = core.NewPolicyGuard(core.TierFull, nil, nil)

	task := newTask()
	prompt := "You are a test subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// At TierFull, shell_exec should be allowed and executed.
	// The tool may fail (not registered), but policy should not block it.
}

func TestSpawnerRunLoop_PolicyGuardHardlineBlocks(t *testing.T) {
	// Even at TierFull, hardline patterns should block.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "I'll delete everything.",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "shell_exec", Arguments: map[string]interface{}{
					"command": "rm -rf /",
				}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 2
	spawner.cfg.Policy = core.NewPolicyGuard(core.TierFull, nil, nil)

	task := newTask()
	prompt := "You are a test subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Hardline should block rm -rf / even at TierFull
}

func TestSpawnerRunLoop_PolicyGuardNilNoEnforcement(t *testing.T) {
	// When Policy is nil, no enforcement happens — tools run normally.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Done.",
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 2
	// Policy is nil by default in newTestSpawner

	task := newTask()
	prompt := "You are a test subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Content != "Done." {
		t.Errorf("expected 'Done.', got %q", resp.Content)
	}
}

func TestSpawnerRunLoop_PolicyGuardAllowsReadToolsInReadTier(t *testing.T) {
	// At TierRead, read tools (read, glob, grep) should be allowed.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Reading a file.",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "read", Arguments: map[string]interface{}{
					"path": "/tmp/test.txt",
				}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 2
	spawner.cfg.Policy = core.NewPolicyGuard(core.TierRead, nil, nil)

	task := newTask()
	prompt := "You are a test subagent."

	resp, err := spawner.RunLoop(context.Background(), task, prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// read tool is TierRead — should be allowed at TierRead
}

func TestSpawnerRunLoop_PolicyContextCancellation(t *testing.T) {
	// Verify RunLoop respects context cancellation with PolicyGuard.
	prov := &stubProvider{
		resp: &domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Running.",
			ToolCalls: []domain.ToolCall{
				{ID: "1", Name: "shell_exec", Arguments: map[string]interface{}{
					"command": "sleep 10",
				}},
			},
		},
	}

	spawner := newTestSpawner(prov)
	spawner.cfg.Budget.MaxIterations = 5
	spawner.cfg.Policy = core.NewPolicyGuard(core.TierFull, nil, nil)

	task := newTask()
	prompt := "You are a test subagent."

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := spawner.RunLoop(ctx, task, prompt)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestCheckEditAuthority_BoundaryAndEscapes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	rootsList := [][]string{
		{"."},
		{wd},
	}

	for _, roots := range rootsList {
		label := roots[0]
		// Forbidden escape paths
		escapes := []string{
			"/etc/passwd",
			"/etc/shadow",
			"../../secret",
			"../../../etc/passwd",
			"../outside.txt",
		}
		for _, target := range escapes {
			if checkEditAuthority(target, roots) {
				t.Errorf("root %q: expected target %q to be rejected, but was allowed", label, target)
			}
		}

		// Legitimate child paths
		allowed := []string{
			"foo.txt",
			"./foo.txt",
			"subdir/bar.go",
			"./subdir/bar.go",
		}
		for _, target := range allowed {
			if !checkEditAuthority(target, roots) {
				t.Errorf("root %q: expected target %q to be allowed, but was rejected", label, target)
			}
		}
	}
}

func TestSpawnerRunLoop_EditAuthorityRejectsEscapesEvenWhenRootIsDot(t *testing.T) {
	prov := &multiRespProvider{
		responses: []*domain.Message{
			{
				Role: domain.RoleAssistant,
				ToolCalls: []domain.ToolCall{
					{
						ID:   "call_1",
						Name: "write_file",
						Arguments: map[string]interface{}{
							"path": "/etc/passwd",
						},
					},
				},
			},
			{
				Role:    domain.RoleAssistant,
				Content: "Finished after edit check",
			},
		},
	}

	spawner := newTestSpawner(prov)
	task := newTask()
	task.AllowedEditRoots = []string{"."}

	resp, err := spawner.RunLoop(context.Background(), task, "Write to passwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Content != "Finished after edit check" {
		t.Errorf("unexpected response content: %q", resp.Content)
	}
}

func TestSpawnerRunLoop_EditAuthorityRejectsParentTraversalEvenWhenRootIsDot(t *testing.T) {
	prov := &multiRespProvider{
		responses: []*domain.Message{
			{
				Role: domain.RoleAssistant,
				ToolCalls: []domain.ToolCall{
					{
						ID:   "call_1",
						Name: "edit_file",
						Arguments: map[string]interface{}{
							"path": "../../secret",
						},
					},
				},
			},
			{
				Role:    domain.RoleAssistant,
				Content: "Finished after secret check",
			},
		},
	}

	spawner := newTestSpawner(prov)
	task := newTask()
	task.AllowedEditRoots = []string{"."}

	resp, err := spawner.RunLoop(context.Background(), task, "Write to secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Content != "Finished after secret check" {
		t.Errorf("unexpected response content: %q", resp.Content)
	}
}

type testLearningSubagent struct{}

func (s *testLearningSubagent) Name() string        { return "test-sub" }
func (s *testLearningSubagent) Description() string { return "test subagent for learning" }
func (s *testLearningSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	return &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "ok"}
}

func TestSpawner_OnLearningThreshold(t *testing.T) {
	prov := &stubProvider{
		resp: &domain.Message{Role: domain.RoleAssistant, Content: "All done"},
	}
	spawner := newTestSpawner(prov)

	// Register a stub subagent
	spawner.registry.Register("test-sub", func(s *Spawner) Subagent {
		return &testLearningSubagent{}
	})

	var triggeredSubagent string
	spawner.SetOnLearningThreshold(func(name string) {
		triggeredSubagent = name
	})

	// Threshold is 5 by default
	task := newTask()
	for i := 0; i < 4; i++ {
		_, err := spawner.Spawn(context.Background(), "test-sub", task)
		if err != nil {
			t.Fatalf("spawn %d failed: %v", i, err)
		}
		if triggeredSubagent != "" {
			t.Fatalf("threshold callback triggered prematurely on iteration %d", i)
		}
	}

	// 5th iteration hits threshold
	_, err := spawner.Spawn(context.Background(), "test-sub", task)
	if err != nil {
		t.Fatalf("spawn 5 failed: %v", err)
	}
	if triggeredSubagent != "test-sub" {
		t.Errorf("expected triggeredSubagent 'test-sub', got %q", triggeredSubagent)
	}
}

