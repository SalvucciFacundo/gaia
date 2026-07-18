# Proposal: Async Delegation + Direct Subagent Chat

## Intent

GAIA's current `Spawner.Spawn` is synchronous — the TUI blocks until a subagent finishes. This prevents parallel work, makes long SDD phases freeze the UI, and forces the orchestrator to act as the only chat entry point. Users cannot address a specific subagent directly, nor can they continue interacting while a task runs in the background.

## Scope

### In Scope
- `TaskManager` with `TaskID`, `TaskStatus` lifecycle (Pending → Running → Completed/Failed/Cancelled)
- `Spawner.SpawnAsync(ctx, name, task) (TaskID, error)` — runs subagent in goroutine, returns immediately
- Completion channel (`<-chan TaskResult`) per task, fan-out to subscribers
- TUI model updates: running-task list, completion toasts, cancellation keybinding
- Direct routing syntax: `@<name> <prompt>` parsed by Brain, bypassing orchestrator fan-out
- Subagent scope enforcement: reject tool calls outside the subagent's declared set
- SDD pipeline phases (`sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`) become async-delegatable

### Out of Scope
- Distributed/multi-process execution (single-process goroutines only)
- Persistent task queue across restarts (v1 is in-memory; persistence is a follow-up)
- Subagent-to-subagent direct messaging (still routed via Spawner)

## Capabilities

### New Capabilities
- `async-task-manager`: TaskID generation, status lifecycle, cancellation, completion fan-out
- `direct-subagent-routing`: `@name` parser, Brain-level routing, scope enforcement

### Modified Capabilities
- `agent-loop`: Spawner gains `SpawnAsync`; loop must tolerate background completions
- `tool-engine`: tool execution must respect per-task cancellation context

## Approach

Extend `Spawner` with a `SpawnAsync` method that wraps `Spawn` in a goroutine keyed by a UUID `TaskID`. A new `TaskManager` port (`internal/core/ports`) owns the task map (sync.RWMutex-protected) and a per-task `chan TaskResult`. The TUI adapter subscribes to a broadcast channel and renders a running-task pane. Direct routing is a Brain-level parser change: `@name` tokens route to `Spawner.SpawnAsync(name, ...)` directly, with tool-filter enforcement reused from existing `Filtered()` logic.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/spawner.go` | Modified | Add `SpawnAsync`, keep `Spawn` as sync wrapper |
| `internal/agent/task_manager.go` | New | TaskManager, TaskID, TaskStatus, channels |
| `internal/core/ports/ports.go` | Modified | Add `AsyncSpawner` port |
| `internal/adapters/tui/` | Modified | Running-task pane, completion toasts |
| `internal/agent/brain.go` (or equivalent) | Modified | `@name` routing parser |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Goroutine leaks on cancellation | Med | Context propagation + `defer` cleanup; goleak in tests |
| TUI race on concurrent updates | Med | Single tea.Program message queue; all updates via `tea.Cmd` |
| `@name` collisions with content | Low | Require `@name` at line start or after whitespace; escape with `\@` |

## Rollback Plan

`SpawnAsync` is additive; existing `Spawn` callers remain valid. Feature-gate direct routing behind a config flag (`brain.direct_routing: false` default). Revert = remove new files + flip flag.

## Dependencies

- None external. Builds on existing `Spawner`, `Registry`, `ToolRegistry.Filtered`.

## Success Criteria

- [ ] TUI remains responsive while a subagent runs for >30s
- [ ] `@propose write a proposal for X` routes directly to the propose subagent
- [ ] Task cancellation via `Ctrl+X` stops the goroutine within 2s
- [ ] All existing sync `Spawn` tests still pass
