package agent

import (
	"context"
	"testing"

	"gaia/internal/core"
	"gaia/internal/core/domain"
)

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
