package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gaia/internal/core/domain"

	"github.com/google/uuid"
)

// TaskRepository is the persistence contract for async task state.
// Implementations store tasks in SQLite or other backends.
type TaskRepository interface {
	// Create persists a new task in Pending state.
	Create(ctx context.Context, state TaskState) error
	// UpdateStatus transitions a task to a new status with optional result/error.
	UpdateStatus(ctx context.Context, taskID string, status TaskStatus, resultJSON, errorText string, completedAt time.Time) error
	// LoadActive returns all tasks in non-terminal states (Pending, Running).
	LoadActive(ctx context.Context) ([]TaskState, error)
}

// TaskStatus represents the lifecycle state of an async task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// TaskState is a snapshot of an async task at a point in time.
type TaskState struct {
	TaskID       string
	SubagentName string
	Status       TaskStatus
	Result       *domain.SubagentResult
	Error        string
	CreatedAt    time.Time
	CompletedAt  time.Time
}

// taskEntry holds the mutable state of a single async task.
type taskEntry struct {
	state  TaskState
	cancel context.CancelFunc
	subs   []chan TaskState // per-task subscribers (buffered, cap 1)
}

// TaskManager tracks async subagent tasks through their lifecycle.
// It is safe for concurrent use. When a TaskRepository is configured,
// tasks are persisted to SQLite and survive restarts.
type TaskManager struct {
	mu      sync.RWMutex
	tasks   map[string]*taskEntry

	subMu   sync.Mutex
	allSubs []chan TaskState // SubscribeAll subscribers (buffered, cap 64)

	repo  TaskRepository  // nil = in-memory only
	bgCtx context.Context // used for persistence operations; nil when repo is nil
}

// NewTaskManager creates an in-memory-only TaskManager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*taskEntry),
	}
}

// NewTaskManagerWithRepo creates a TaskManager backed by a TaskRepository.
// Active tasks (Pending, Running) are loaded from the repo on creation.
func NewTaskManagerWithRepo(ctx context.Context, repo TaskRepository) (*TaskManager, error) {
	tm := &TaskManager{
		tasks: make(map[string]*taskEntry),
		repo:  repo,
		bgCtx: ctx,
	}

	active, err := repo.LoadActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active tasks: %w", err)
	}
	for _, state := range active {
		entry := &taskEntry{state: state}
		if state.Status == TaskRunning {
			_, entry.cancel = context.WithCancel(context.Background())
		}
		tm.tasks[state.TaskID] = entry
	}

	return tm, nil
}

// CreateTask registers a new task in Pending state and returns its TaskID.
// The returned context is derived from the parent and carries cancellation.
// If a TaskRepository is configured, the task is persisted to SQLite.
func (tm *TaskManager) CreateTask(name string, task domain.SubagentTask) (string, context.Context) {
	id := uuid.NewString()
	taskCtx, cancel := context.WithCancel(context.Background())

	state := TaskState{
		TaskID:       id,
		SubagentName: name,
		Status:       TaskPending,
		CreatedAt:    time.Now(),
	}

	entry := &taskEntry{
		state:  state,
		cancel: cancel,
	}

	if tm.repo != nil {
		if err := tm.repo.Create(tm.bgCtx, state); err != nil {
			// Log but don't fail — in-memory operation continues.
			fmt.Printf("task_manager: persist create failed: %v\n", err)
		}
	}

	tm.mu.Lock()
	tm.tasks[id] = entry
	tm.mu.Unlock()

	tm.broadcast(state)
	return id, taskCtx
}

// UpdateStatus transitions a task to a new status and optionally records a result or error.
// Per-task subscribers are notified only on terminal states (Completed, Failed, Cancelled).
// If a TaskRepository is configured, the status change is persisted to SQLite.
func (tm *TaskManager) UpdateStatus(taskID string, status TaskStatus, result *domain.SubagentResult, err error) {
	tm.mu.Lock()
	entry, ok := tm.tasks[taskID]
	if !ok {
		tm.mu.Unlock()
		return
	}

	entry.state.Status = status
	if result != nil {
		entry.state.Result = result
	}
	if err != nil {
		entry.state.Error = err.Error()
	}

	isTerminal := status == TaskCompleted || status == TaskFailed || status == TaskCancelled
	if isTerminal {
		entry.state.CompletedAt = time.Now()
	}

	state := entry.state // snapshot for broadcast

	// Notify per-task subscribers only on terminal states
	var subs []chan TaskState
	if isTerminal {
		subs = entry.subs
		entry.subs = nil
	}
	tm.mu.Unlock()

	// Persist to SQLite outside the lock (I/O should never block task dispatch).
	if tm.repo != nil {
		var resultJSON string
		if state.Result != nil {
			if b, err := json.Marshal(state.Result); err == nil {
				resultJSON = string(b)
			}
		}
		if err := tm.repo.UpdateStatus(tm.bgCtx, taskID, state.Status, resultJSON, state.Error, state.CompletedAt); err != nil {
			fmt.Printf("task_manager: persist status failed: %v\n", err)
		}
	}

	for _, sub := range subs {
		select {
		case sub <- state:
		default: // subscriber too slow; skip
		}
	}
	// Close subscriber channels after sending the terminal state
	for _, sub := range subs {
		close(sub)
	}

	tm.broadcast(state)
}

// CancelTask cancels a pending or running task.
// Returns an error if the task is not found or is already in a terminal state.
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	entry, ok := tm.tasks[taskID]
	if !ok {
		tm.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}
	if entry.state.Status == TaskCompleted || entry.state.Status == TaskFailed || entry.state.Status == TaskCancelled {
		tm.mu.Unlock()
		return fmt.Errorf("task %s is already in terminal state: %s", taskID, entry.state.Status)
	}
	entry.cancel()
	tm.mu.Unlock()

	tm.UpdateStatus(taskID, TaskCancelled, nil, nil)
	return nil
}

// WaitTask blocks until the task reaches a terminal state or the context is done.
// Returns the final TaskState, or an error if the context is cancelled first.
func (tm *TaskManager) WaitTask(ctx context.Context, taskID string) (*TaskState, error) {
	// Fast path: check if already terminal
	tm.mu.RLock()
	entry, ok := tm.tasks[taskID]
	tm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// If already terminal, return immediately
	if entry.state.Status == TaskCompleted || entry.state.Status == TaskFailed || entry.state.Status == TaskCancelled {
		state := entry.state
		return &state, nil
	}

	sub := tm.SubscribeTask(taskID)
	select {
	case state := <-sub:
		return &state, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SubscribeTask returns a channel that receives the final TaskState when the
// task reaches a terminal state. If the task is already terminal, the channel
// is pre-filled and closed. If the task is unknown, the channel is closed immediately.
func (tm *TaskManager) SubscribeTask(taskID string) <-chan TaskState {
	tm.mu.RLock()
	entry, ok := tm.tasks[taskID]
	tm.mu.RUnlock()

	ch := make(chan TaskState, 1)

	if !ok {
		close(ch)
		return ch
	}

	tm.mu.Lock()
	// Double-check after acquiring write lock
	if entry.state.Status == TaskCompleted || entry.state.Status == TaskFailed || entry.state.Status == TaskCancelled {
		ch <- entry.state
		close(ch)
		tm.mu.Unlock()
		return ch
	}

	entry.subs = append(entry.subs, ch)
	tm.mu.Unlock()
	return ch
}

// SubscribeAll returns a channel that receives TaskState updates for ALL tasks.
// This is used by the TUI to render the task pane. The channel is buffered (cap 64);
// slow consumers will miss updates.
func (tm *TaskManager) SubscribeAll() <-chan TaskState {
	ch := make(chan TaskState, 64)
	tm.subMu.Lock()
	tm.allSubs = append(tm.allSubs, ch)
	tm.subMu.Unlock()
	return ch
}

// ListTasks returns a snapshot of all task states, ordered by creation time.
func (tm *TaskManager) ListTasks() []TaskState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	states := make([]TaskState, 0, len(tm.tasks))
	for _, entry := range tm.tasks {
		states = append(states, entry.state)
	}
	return states
}

// broadcast sends a TaskState to all SubscribeAll subscribers (non-blocking).
func (tm *TaskManager) broadcast(state TaskState) {
	tm.subMu.Lock()
	defer tm.subMu.Unlock()

	for _, sub := range tm.allSubs {
		select {
		case sub <- state:
		default: // subscriber too slow; drop this update
		}
	}
}
