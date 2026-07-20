package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestTaskManager_Lifecycle(t *testing.T) {
	tm := NewTaskManager()

	task := domain.SubagentTask{
		ID:          "test-001",
		Description: "test task",
		Mode:        "plan",
	}

	id, _ := tm.CreateTask("explorer", task)
	if id == "" {
		t.Fatal("expected non-empty TaskID")
	}

	// Initial state: Pending
	states := tm.ListTasks()
	if len(states) != 1 {
		t.Fatalf("expected 1 task, got %d", len(states))
	}
	if states[0].Status != TaskPending {
		t.Errorf("expected Pending, got %s", states[0].Status)
	}
	if states[0].SubagentName != "explorer" {
		t.Errorf("expected SubagentName explorer, got %s", states[0].SubagentName)
	}

	// Transition to Running
	tm.UpdateStatus(id, TaskRunning, nil, nil)
	states = tm.ListTasks()
	if states[0].Status != TaskRunning {
		t.Errorf("expected Running, got %s", states[0].Status)
	}

	// Transition to Completed
	result := &domain.SubagentResult{
		Status:  domain.SubagentSuccess,
		Summary: "all done",
	}
	tm.UpdateStatus(id, TaskCompleted, result, nil)
	states = tm.ListTasks()
	if states[0].Status != TaskCompleted {
		t.Errorf("expected Completed, got %s", states[0].Status)
	}
	if states[0].Result == nil {
		t.Fatal("expected non-nil Result")
	}
	if states[0].Result.Summary != "all done" {
		t.Errorf("expected 'all done', got %q", states[0].Result.Summary)
	}
	if states[0].CompletedAt.IsZero() {
		t.Error("expected non-zero CompletedAt")
	}
}

func TestTaskManager_Failed(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("debugger", domain.SubagentTask{ID: "test-002", Description: "debug"})

	tm.UpdateStatus(id, TaskRunning, nil, nil)
	tm.UpdateStatus(id, TaskFailed, nil, fmt.Errorf("something went wrong"))

	states := tm.ListTasks()
	if states[0].Status != TaskFailed {
		t.Errorf("expected Failed, got %s", states[0].Status)
	}
	if states[0].Error != "something went wrong" {
		t.Errorf("expected error message, got %q", states[0].Error)
	}
}

func TestTaskManager_CancelTask(t *testing.T) {
	tm := NewTaskManager()

	id, taskCtx := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-003", Description: "long task"})
	tm.UpdateStatus(id, TaskRunning, nil, nil)

	// Verify context is alive
	select {
	case <-taskCtx.Done():
		t.Fatal("context should not be cancelled yet")
	default:
	}

	// Cancel the task
	if err := tm.CancelTask(id); err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	// Verify context was cancelled
	select {
	case <-taskCtx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be cancelled after CancelTask")
	}

	// Verify status
	states := tm.ListTasks()
	if states[0].Status != TaskCancelled {
		t.Errorf("expected Cancelled, got %s", states[0].Status)
	}

	// Cancelling again should error
	if err := tm.CancelTask(id); err == nil {
		t.Error("expected error on second cancel")
	}
}

func TestTaskManager_CancelUnknown(t *testing.T) {
	tm := NewTaskManager()
	if err := tm.CancelTask("nonexistent"); err == nil {
		t.Error("expected error for unknown task")
	}
}

func TestTaskManager_SubscribeTask(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-004", Description: "subscribe test"})
	tm.UpdateStatus(id, TaskRunning, nil, nil)

	// Subscribe before completion
	sub := tm.SubscribeTask(id)

	// Complete the task
	result := &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "done"}
	tm.UpdateStatus(id, TaskCompleted, result, nil)

	// Should receive the final state
	select {
	case state := <-sub:
		if state.Status != TaskCompleted {
			t.Errorf("expected Completed, got %s", state.Status)
		}
		if state.Result.Summary != "done" {
			t.Errorf("expected 'done', got %q", state.Result.Summary)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for task update")
	}

	// Channel should be closed
	_, ok := <-sub
	if ok {
		t.Error("channel should be closed after terminal state")
	}
}

func TestTaskManager_SubscribeTask_AlreadyTerminal(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-005", Description: "done task"})
	tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess}, nil)

	// Subscribe after completion — should receive immediately
	sub := tm.SubscribeTask(id)

	select {
	case state := <-sub:
		if state.Status != TaskCompleted {
			t.Errorf("expected Completed, got %s", state.Status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("should have received immediately")
	}
}

func TestTaskManager_SubscribeTask_Unknown(t *testing.T) {
	tm := NewTaskManager()

	sub := tm.SubscribeTask("nonexistent")

	// Unknown task — channel should be closed immediately
	select {
	case _, ok := <-sub:
		if ok {
			t.Error("channel should be closed for unknown task")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel should be closed immediately")
	}
}

func TestTaskManager_SubscribeAll(t *testing.T) {
	tm := NewTaskManager()

	sub := tm.SubscribeAll()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-006", Description: "fan-out test"})

	// Should receive CreateTask event
	select {
	case state := <-sub:
		if state.TaskID != id {
			t.Errorf("expected TaskID %s, got %s", id, state.TaskID)
		}
		if state.Status != TaskPending {
			t.Errorf("expected Pending, got %s", state.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for SubscribeAll event")
	}

	// Update status and verify
	tm.UpdateStatus(id, TaskRunning, nil, nil)
	select {
	case state := <-sub:
		if state.Status != TaskRunning {
			t.Errorf("expected Running, got %s", state.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for status update")
	}

	// Complete and verify
	tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess}, nil)
	select {
	case state := <-sub:
		if state.Status != TaskCompleted {
			t.Errorf("expected Completed, got %s", state.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for completion event")
	}
}

func TestTaskManager_SubscribeAll_MultipleSubscribers(t *testing.T) {
	tm := NewTaskManager()

	sub1 := tm.SubscribeAll()
	sub2 := tm.SubscribeAll()

	id, _ := tm.CreateTask("debugger", domain.SubagentTask{ID: "test-007", Description: "multi-sub"})

	// Both should receive the same state
	var state1, state2 TaskState
	select {
	case state1 = <-sub1:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sub1 timed out")
	}
	select {
	case state2 = <-sub2:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sub2 timed out")
	}

	if state1.TaskID != id || state2.TaskID != id {
		t.Errorf("both subscribers should receive task %s: got %s / %s", id, state1.TaskID, state2.TaskID)
	}
}

func TestTaskManager_ConcurrentAccess(t *testing.T) {
	tm := NewTaskManager()
	const numGoroutines = 20

	var wg sync.WaitGroup

	// Concurrent task creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task := domain.SubagentTask{
				ID:          fmt.Sprintf("concurrent-%d", idx),
				Description: fmt.Sprintf("task %d", idx),
			}
			id, _ := tm.CreateTask("explorer", task)
			tm.UpdateStatus(id, TaskRunning, nil, nil)
			time.Sleep(time.Millisecond)
			tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess}, nil)
		}(i)
	}

	wg.Wait()

	states := tm.ListTasks()
	if len(states) != numGoroutines {
		t.Errorf("expected %d tasks, got %d", numGoroutines, len(states))
	}

	for _, s := range states {
		if s.Status != TaskCompleted {
			t.Errorf("task %s: expected Completed, got %s", s.TaskID, s.Status)
		}
	}
}

func TestTaskManager_ConcurrentSubscribeAndUpdate(t *testing.T) {
	tm := NewTaskManager()
	const numTasks = 10

	// Create tasks and subscribers concurrently
	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			task := domain.SubagentTask{ID: fmt.Sprintf("cs-%d", idx), Description: "concurrent sub"}
			id, _ := tm.CreateTask("explorer", task)

			// Subscribe concurrently with update
			sub := tm.SubscribeTask(id)

			// Simulate work
			time.Sleep(time.Millisecond)
			tm.UpdateStatus(id, TaskRunning, nil, nil)
			time.Sleep(time.Millisecond)
			tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess}, nil)

			// Should receive the final state
			select {
			case state := <-sub:
				if state.Status != TaskCompleted {
					t.Errorf("task %d: expected Completed, got %s", idx, state.Status)
				}
			case <-time.After(time.Second):
				t.Errorf("task %d: timed out", idx)
			}
		}(i)
	}

	wg.Wait()
}

func TestTaskManager_WaitTask(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-008", Description: "wait test"})
	tm.UpdateStatus(id, TaskRunning, nil, nil)

	// Complete in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "waited"}, nil)
	}()

	ctx := context.Background()
	state, err := tm.WaitTask(ctx, id)
	if err != nil {
		t.Fatalf("WaitTask failed: %v", err)
	}
	if state.Status != TaskCompleted {
		t.Errorf("expected Completed, got %s", state.Status)
	}
	if state.Result.Summary != "waited" {
		t.Errorf("expected 'waited', got %q", state.Result.Summary)
	}
}

func TestTaskManager_WaitTask_ContextCancel(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-009", Description: "wait cancel"})
	tm.UpdateStatus(id, TaskRunning, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := tm.WaitTask(ctx, id)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestTaskManager_WaitTask_AlreadyTerminal(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-010", Description: "already done"})
	tm.UpdateStatus(id, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "pre-done"}, nil)

	ctx := context.Background()
	state, err := tm.WaitTask(ctx, id)
	if err != nil {
		t.Fatalf("WaitTask should not error for already-terminal: %v", err)
	}
	if state.Status != TaskCompleted {
		t.Errorf("expected Completed, got %s", state.Status)
	}
}

func TestTaskManager_WaitTask_Unknown(t *testing.T) {
	tm := NewTaskManager()

	ctx := context.Background()
	_, err := tm.WaitTask(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestTaskManager_ListTasks(t *testing.T) {
	tm := NewTaskManager()

	// Empty initially
	states := tm.ListTasks()
	if len(states) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(states))
	}

	// Add tasks with different statuses
	id1, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "t1", Description: "task 1"})
	id2, _ := tm.CreateTask("debugger", domain.SubagentTask{ID: "t2", Description: "task 2"})
	tm.UpdateStatus(id1, TaskRunning, nil, nil)
	tm.UpdateStatus(id2, TaskCompleted, &domain.SubagentResult{Status: domain.SubagentSuccess}, nil)

	states = tm.ListTasks()
	if len(states) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(states))
	}

	names := make(map[string]bool)
	for _, s := range states {
		names[s.SubagentName] = true
	}
	if !names["explorer"] || !names["debugger"] {
		t.Errorf("expected both subagent names, got %v", names)
	}
}

func TestTaskManager_UpdateStatus_Unknown(t *testing.T) {
	tm := NewTaskManager()

	// Should not panic
	tm.UpdateStatus("nonexistent", TaskCompleted, nil, nil)
}

func TestTaskManager_MultipleSubscribersPerTask(t *testing.T) {
	tm := NewTaskManager()

	id, _ := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-011", Description: "multi-sub task"})
	tm.UpdateStatus(id, TaskRunning, nil, nil)

	sub1 := tm.SubscribeTask(id)
	sub2 := tm.SubscribeTask(id)

	result := &domain.SubagentResult{Status: domain.SubagentSuccess, Summary: "multi-done"}
	tm.UpdateStatus(id, TaskCompleted, result, nil)

	// Both should receive the state
	select {
	case state := <-sub1:
		if state.Status != TaskCompleted {
			t.Errorf("sub1: expected Completed, got %s", state.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sub1 timed out")
	}

	select {
	case state := <-sub2:
		if state.Status != TaskCompleted {
			t.Errorf("sub2: expected Completed, got %s", state.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sub2 timed out")
	}
}

func TestTaskManager_ContextPropagation(t *testing.T) {
	tm := NewTaskManager()

	id, taskCtx := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-012", Description: "ctx test"})

	// The task context should be derived from background, not cancelled
	select {
	case <-taskCtx.Done():
		t.Fatal("task context should not be cancelled initially")
	default:
	}

	// Cancel should propagate
	tm.CancelTask(id)

	select {
	case <-taskCtx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("task context should be cancelled after CancelTask")
	}

	// Context error should be context.Canceled
	if taskCtx.Err() != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", taskCtx.Err())
	}
}

func TestTaskManager_CreateTask_ReturnsContext(t *testing.T) {
	tm := NewTaskManager()

	_, ctx := tm.CreateTask("explorer", domain.SubagentTask{ID: "test-013", Description: "ctx return"})

	// Context should not be nil
	if ctx == nil {
		t.Fatal("expected non-nil context from CreateTask")
	}
}

// TestTaskManager_PanicRecovery verifies that SpawnAsync recovers from goroutine panics.
// This is an integration-style test that creates a real Spawner with TaskManager.
func TestTaskManager_PanicRecovery(t *testing.T) {
	tm := NewTaskManager()

	// Create a minimal spawner with a TaskManager but no real LLM provider.
	// The subagent's Execute will panic, and SpawnAsync should recover.
	spawner := &Spawner{
		cfg: SpawnerConfig{
			TaskManager: tm,
		},
		registry: NewRegistry(),
	}

	// Register a subagent that panics
	spawner.registry.Register("panicbot", func(s *Spawner) Subagent {
		return &panicSubagent{}
	})

	task := domain.SubagentTask{
		ID:          "panic-001",
		Description: "this will panic",
		Mode:        "plan",
	}

	taskID, err := spawner.SpawnAsync(context.Background(), "panicbot", task)
	if err != nil {
		t.Fatalf("SpawnAsync should not error: %v", err)
	}

	// Wait for the task to fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	state, err := tm.WaitTask(ctx, taskID)
	if err != nil {
		t.Fatalf("WaitTask failed: %v", err)
	}

	if state.Status != TaskFailed {
		t.Errorf("expected TaskFailed, got %s", state.Status)
	}
	if state.Error == "" {
		t.Error("expected non-empty Error field with panic message")
	}
}

// panicSubagent is a subagent that always panics in Execute.
type panicSubagent struct{}

func (p *panicSubagent) Name() string        { return "panicbot" }
func (p *panicSubagent) Description() string  { return "always panics" }
func (p *panicSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	panic("intentional panic for test")
}
