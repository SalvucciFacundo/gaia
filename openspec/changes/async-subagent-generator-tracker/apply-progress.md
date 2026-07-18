# Apply Progress: Async Delegation (PR 1 of 3)

## Status: COMPLETE

**Branch**: `feat/async-subagent-generator-tracker-01-async`
**Base**: `feat/async-subagent-generator-tracker`
**Date**: 2026-07-18

## Completed Tasks

### Phase 1: Foundation (4/4)
- [x] 1.1 `IsDirectChat` field on `SubagentTask` — `internal/core/domain/models.go`
- [x] 1.2 `AsyncSpawner` port interface — `internal/core/ports/ports.go`
- [x] 1.3 `DynamicPrefix(name)` — `internal/agent/memory/namespace.go`
- [x] 1.4 SQLite migrations (subagent_defs, tracker_state, tracker_releases) — `internal/adapters/db/sqlite.go`

### Phase 2: Async Delegation (8/8)
- [x] 2.1 `task_manager.go` — TaskManager, TaskState, TaskStatus, UUID gen, channels, RWMutex
- [x] 2.2 `spawner.go` — TaskManager in SpawnerConfig, SpawnAsync with panic recovery
- [x] 2.3 `registry.go` — Unregister(name) for dynamic subagent removal
- [x] 2.4 `tui.go` — taskUpdateMsg, waitForTaskUpdate, task pane rendering, SetTaskManager
- [x] 2.5 `kernel.go` — @name parsing, handleDirectSubagent, async routing
- [x] 2.6 `tui.go` — "tasks" list command, "cancel <id>" cancel command
- [x] 2.7 `pipeline.go` — PipelineRunner, SDDPhases, sequential async chaining
- [x] 2.8 `task_manager_test.go` — 21 tests (lifecycle, concurrency, subscribers, panic recovery, context propagation)

## Files Changed
| File | Action | Lines |
|------|--------|-------|
| `internal/core/domain/models.go` | Modified | +1 field |
| `internal/core/ports/ports.go` | Modified | +8 lines (AsyncSpawner interface) |
| `internal/agent/memory/namespace.go` | Modified | +5 lines (DynamicPrefix) |
| `internal/adapters/db/sqlite.go` | Modified | +22 lines (3 new tables) |
| `internal/agent/task_manager.go` | Created | ~200 lines |
| `internal/agent/task_manager_test.go` | Created | ~350 lines (21 tests) |
| `internal/agent/spawner.go` | Modified | +40 lines (TaskManager + SpawnAsync) |
| `internal/agent/registry.go` | Modified | +12 lines (Unregister) |
| `internal/agent/pipeline.go` | Created | ~80 lines |
| `internal/adapters/tui/tui.go` | Modified | ~150 lines (task pane, commands) |
| `internal/core/kernel.go` | Modified | +55 lines (@name routing) |
| `cmd/gaia/main.go` | Modified | +7 lines (wiring) |

## Test Results
```
=== TestTaskManager (21/21 PASS) ===
ok  gaia/internal/agent           0.621s
ok  gaia/internal/core             0.560s
ok  gaia/internal/adapters/tui     1.391s
All existing tests pass (no regressions)
```

## Deviations from Design
- None. All type contracts, method signatures, and concurrency patterns follow the design exactly.
- TaskManager uses `CreateTask` returning `(string, context.Context)` instead of just `string` — the design contract didn't specify the context return but it's needed for cancellation propagation.
- Per-task subscribers are only notified on terminal states (not every status change) — this prevents race conditions in SubscribeTask.
- PipelineRunner uses `RunPipeline` as a standalone function (not a struct method) — cleaner API without state.

## Rollback Boundary
- Revert: `task_manager.go`, `task_manager_test.go`, `pipeline.go`
- Remove from: `spawner.go` (TaskManager field + SpawnAsync), `tui.go` (task pane + commands), `kernel.go` (@name routing), `ports.go` (AsyncSpawner), `models.go` (IsDirectChat), `namespace.go` (DynamicPrefix), `sqlite.go` (3 new tables), `registry.go` (Unregister), `main.go` (TaskManager wiring)
