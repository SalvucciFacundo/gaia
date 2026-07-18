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

---

# Apply Progress: Dynamic Generator (PR 2 of 3)

## Status: COMPLETE

**Branch**: `feat/async-subagent-generator-tracker-02-dynamic`
**Base**: `feat/async-subagent-generator-tracker-01-async` (merged)
**Date**: 2026-07-18

## Completed Tasks

### Phase 3: Dynamic Generator (6/6)
- [x] 3.1 `internal/adapters/db/subagent_defs.go` — DefRepo CRUD against SQLite, JSON serialization for AllowedTools/Skills
- [x] 3.2 `internal/agent/dynamic.go` — DynamicSubagent, SubagentDef, DefRepository interface, DynamicLoader, tool validation
- [x] 3.3 `internal/adapters/tui/interview.go` — InterviewModel with 6-step state machine (name→desc→tools→skills→personality→confirm), back-navigation, tool multi-select
- [x] 3.4 `/create-agent` wiring — TUI detects command, starts interview, calls DynamicLoader.CreateFromDef on confirm
- [x] 3.5 Startup loader in `cmd/gaia/main.go` — DefRepo creation, DynamicLoader init, LoadAll() call, TUI dynamic creator + tool names wiring
- [x] 3.6 `internal/agent/dynamic_test.go` — 7 tests (interface, LoadAll, CreateFromDef with validation, invalid tools, duplicates, RemoveDynamic, factory closure, prompt building)

## Files Changed
| File | Action | Lines |
|------|--------|-------|
| `internal/agent/dynamic.go` | Created | ~260 lines |
| `internal/agent/dynamic_test.go` | Created | ~270 lines (7 tests) |
| `internal/adapters/db/subagent_defs.go` | Created | ~160 lines |
| `internal/adapters/db/sqlite.go` | Modified | +5 lines (DB() getter) |
| `internal/adapters/tui/interview.go` | Created | ~280 lines |
| `internal/adapters/tui/tui.go` | Modified | +55 lines (interview integration, SetDynamicCreator, SetToolNames) |
| `cmd/gaia/main.go` | Modified | +30 lines (DynamicLoader wiring, LoadAll, TUI creator/tools) |

## Test Results
```
=== TestDynamic (7/7 PASS) ===
TestDynamicSubagent_Interface              PASS
TestDynamicLoader_CreateFromDef_ToolValidation  PASS
TestDynamicLoader_CreateFromDef_InvalidTool     PASS
TestDynamicLoader_LoadAll                  PASS
TestDynamicLoader_CreateFromDef_DuplicateName   PASS
TestDynamicLoader_RemoveDynamic            PASS
TestDynamicSubagent_FactoryClosure         PASS
TestBuildDynamicPrompt                     PASS

Full suite: ok  gaia/internal/agent  0.663s
All 21 TUI tests: PASS (1.374s)
All existing tests: no regressions
```

## Deviations from Design
- Task 3.4 wire point changed from "Brain /create-agent" to "TUI /create-agent" — TUI detects `/create-agent` directly (not via Brain.ProcessMessage), which is the correct location per the design's data flow diagram showing TUI → interviewModel → Brain flow.
- SubagentDef and DefRepository are defined in `internal/agent/dynamic.go` instead of separate files — keeps the dynamic contract together.
- DB() getter added to SQLiteRepo to share the `*sql.DB` connection with DefRepo.
- The interview uses a sub-model pattern within the main TUI Model rather than a separate tea.Program, integrating naturally with the existing Update/View routing.
- No `agent` package import in `internal/core/kernel.go` — the creation flow is wired via the TUI callback directly to DynamicLoader, avoiding cross-package coupling.

## Rollback Boundary
- Revert: `dynamic.go`, `dynamic_test.go`, `subagent_defs.go`, `interview.go`
- Remove from: `tui.go` (interview integration + creator fields), `main.go` (DynamicLoader wiring), `sqlite.go` (DB() getter)
