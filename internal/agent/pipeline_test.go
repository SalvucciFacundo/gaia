package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

// stubPipelineSubagent is a simple subagent that returns a predefined result.
type stubPipelineSubagent struct {
	name        string
	description string
	result      *domain.SubagentResult
	delay       time.Duration
}

func (s *stubPipelineSubagent) Name() string        { return s.name }
func (s *stubPipelineSubagent) Description() string { return s.description }
func (s *stubPipelineSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return &domain.SubagentResult{Status: domain.SubagentBlocked, Summary: "cancelled"}
		}
	}
	return s.result
}

// setupPipelineSpawner creates a Spawner with TaskManager and registers stub subagents.
func setupPipelineSpawner(t *testing.T) *Spawner {
	t.Helper()

	reg := NewRegistry()
	spawner := NewSpawner(SpawnerConfig{
		TaskManager: NewTaskManager(),
	}, reg)

	// Register stub subagents for pipeline phases
	for _, name := range []string{"explorer", "proposer", "specifier", "designer", "planner", "implementer", "verifier"} {
		s := name // capture
		reg.Register(name, func(sp *Spawner) Subagent {
			return &stubPipelineSubagent{
				name:        s,
				description: "Stub " + s,
				result: &domain.SubagentResult{
					Status:  domain.SubagentSuccess,
					Summary: s + " done",
					Artifacts: []string{s + "-artifact"},
				},
			}
		})
	}

	return spawner
}

func TestRunPipeline_Success(t *testing.T) {
	spawner := setupPipelineSpawner(t)
	phases := domain.SDDPhases("test task")

	results, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	if len(results) != 7 {
		t.Fatalf("expected 7 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Status != domain.SubagentSuccess {
			t.Errorf("phase %d (%s): expected success, got %s", i, phases[i].SubagentName, r.Status)
		}
	}
}

func TestRunPipeline_ArtifactForwarding(t *testing.T) {
	spawner := setupPipelineSpawner(t)
	phases := []domain.PipelinePhase{
		{SubagentName: "explorer", Description: "explore"},
		{SubagentName: "proposer", Description: "propose"},
	}

	results, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
	if err != nil {
		t.Fatalf("RunPipeline failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[1].Status != domain.SubagentSuccess {
		t.Errorf("expected second phase success, got %s", results[1].Status)
	}
}

func TestRunPipeline_PhaseBlocked(t *testing.T) {
	spawner := setupPipelineSpawner(t)

	// Replace implementer with one that returns blocked
	spawner.registry.Unregister("implementer")
	spawner.registry.Register("implementer", func(sp *Spawner) Subagent {
		return &stubPipelineSubagent{
			name:        "implementer",
			description: "Blocked implementer",
			result: &domain.SubagentResult{
				Status:  domain.SubagentBlocked,
				Summary: "implementation blocked",
			},
		}
	})

	phases := domain.SDDPhases("test")
	results, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
	if err != nil {
		t.Fatalf("RunPipeline should not error on blocked phase: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("expected 7 results, got %d", len(results))
	}
	if results[5].Status != domain.SubagentBlocked {
		t.Errorf("expected implementer to be blocked, got %s", results[5].Status)
	}
}

func TestRunPipeline_Cancellation(t *testing.T) {
	spawner := setupPipelineSpawner(t)

	// Replace verifier with slow subagent that gets cancelled
	spawner.registry.Unregister("verifier")
	spawner.registry.Register("verifier", func(sp *Spawner) Subagent {
		return &stubPipelineSubagent{
			name:  "verifier",
			delay: 5 * time.Second,
			result: &domain.SubagentResult{
				Status:  domain.SubagentSuccess,
				Summary: "verifier done",
			},
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	phases := domain.SDDPhases("test")
	_, err := spawner.RunPipeline(ctx, phases, domain.SubagentTask{})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
}

func TestRunPipeline_NoTaskManager(t *testing.T) {
	reg := NewRegistry()
	spawner := NewSpawner(SpawnerConfig{}, reg) // No TaskManager

	phases := domain.SDDPhases("test")
	_, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
	if err == nil {
		t.Fatal("expected error when TaskManager is nil")
	}
}

func TestRunPipeline_ConcurrentSafe(t *testing.T) {
	spawner := setupPipelineSpawner(t)
	phases := domain.SDDPhases("concurrent")

	var wg sync.WaitGroup
	errs := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent RunPipeline failed: %v", err)
		}
	}
}

func TestSDDPhases_NotEmpty(t *testing.T) {
	phases := domain.SDDPhases("test task")
	if len(phases) != 7 {
		t.Errorf("expected 7 phases, got %d", len(phases))
	}

	expected := []string{"explorer", "proposer", "specifier", "designer", "planner", "implementer", "verifier"}
	for i, phase := range phases {
		if phase.SubagentName != expected[i] {
			t.Errorf("phase %d: expected %q, got %q", i, expected[i], phase.SubagentName)
		}
		if phase.Description == "" {
			t.Errorf("phase %d (%s): empty description", i, phase.SubagentName)
		}
	}
}

func TestRunPipeline_PhaseFailed(t *testing.T) {
	spawner := setupPipelineSpawner(t)

	// Make a subagent that panics (simulates real failure)
	spawner.registry.Unregister("proposer")
		spawner.registry.Register("proposer", func(sp *Spawner) Subagent {
		return &panicPipelineSubagent{name: "proposer"}
	})

	phases := []domain.PipelinePhase{
		{SubagentName: "explorer", Description: "explore"},
		{SubagentName: "proposer", Description: "propose"},
	}

	_, err := spawner.RunPipeline(context.Background(), phases, domain.SubagentTask{})
	if err == nil {
		t.Fatal("expected error from panicked phase, got nil")
	}
}

// panicPipelineSubagent panics in Execute to simulate a real crash.
type panicPipelineSubagent struct{ name string }

func (p *panicPipelineSubagent) Name() string                               { return p.name }
func (p *panicPipelineSubagent) Description() string                        { return "panics" }
func (p *panicPipelineSubagent) Execute(ctx context.Context, t domain.SubagentTask) *domain.SubagentResult {
	panic("simulated crash")
}

var _ Subagent = (*stubPipelineSubagent)(nil)
var _ Subagent = (*panicPipelineSubagent)(nil)
