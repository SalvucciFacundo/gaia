# Tasks: Async Delegation + Dynamic Generator + Upstream Tracker

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1700–2000 (15 new, 8 modified) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Async Delegation → PR 2: Dynamic Generator → PR 3: Upstream Tracker |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Async Delegation (TaskManager + SpawnAsync + TUI + @routing) | PR 1 | `go test ./internal/agent/... -run TestTaskManager` | `gaia` TUI with `@explorer` input | revert `task_manager.go`, changes to spawner.go, tui.go, ports.go, models.go, kernel.go |
| 2 | Dynamic Generator (SubagentDef + DynamicSubagent + Interview + /create-agent) | PR 2 | `go test ./internal/agent/... -run TestDynamic` | `gaia` → `/create-agent` → `@<newagent>` | revert `dynamic.go`, `interview.go`, `subagent_defs.go` |
| 3 | Upstream Tracker (monitor + analyzer + manifest + report + CLI) | PR 3 | `go test ./internal/tracker/...` | `gaia tracker check` (network) | revert `internal/tracker/`, `db/tracker.go`, tracker dispatch in main.go |

## Phase 1: Foundation (Shared Infrastructure)

- [x] 1.1 Add `IsDirectChat bool` to `domain.SubagentTask` in `internal/core/domain/models.go`
- [x] 1.2 Add `AsyncSpawner` port interface (SpawnAsync + TaskManager accessor) to `internal/core/ports/ports.go`
- [x] 1.3 Add `DynamicPrefix(name)` to `internal/agent/memory/namespace.go` for gaia/subagent/{name}/ isolation
- [x] 1.4 Add SQLite migrations for 3 tables (subagent_defs, tracker_state, tracker_releases) in `internal/adapters/db/sqlite.go`

## Phase 2: Async Delegation (~600-700 lines)

- [x] 2.1 Create `internal/agent/task_manager.go` with TaskManager (map, RWMutex, cancel funcs, SubscribeAll/SubscribeTask channels, UUID gen)
- [x] 2.2 Add `SpawnerConfig.TaskManager *TaskManager` field and `SpawnAsync(ctx, name, task) (string, error)` method to spawner.go — goroutine with defer panic recovery, wraps existing Spawn
- [x] 2.3 Add Registry.Unregister(name) to `internal/agent/registry.go` for future dynamic subagent removal
- [x] 2.4 Add `taskUpdateMsg` type, `waitForTaskUpdate()` tea.Cmd, and task-pane rendering to `internal/adapters/tui/tui.go` — SubscribeAll fan-in, status line + elapsed time
- [x] 2.5 Add `@name` prefix detection in `Brain.ProcessMessage()` — parse, validate against Spawner.Available(), route via SpawnAsync with IsDirectChat:true
- [x] 2.6 Add `/tasks` (list all) and `/cancel <id>` TUI commands — wired in tui.go Update loop
- [x] 2.7 Wire PipelineRunner goroutine for async SDD — chain phases via WaitTask, cancel cascades on shared context
- [x] 2.8 Write/tests: task_manager_test.go — concurrent safety, lifecycle (Pending→Running→Completed|Failed|Cancelled), panic recovery, SubscribeAll fan-out, CancelTask propagation, goroutine leak detection (goleak)

## Phase 3: Dynamic Generator (~500-600 lines)

- [ ] 3.1 Create `internal/adapters/db/subagent_defs.go` — DefRepository CRUD against SQLite subagent_defs table, JSON serialization for AllowedTools/Skills, validate tool names against ToolRegistry
- [ ] 3.2 Create `internal/agent/dynamic.go` — DynamicSubagent (implements Subagent interface), DynamicSubagentFactory, system prompt assembly from def.SystemPrompt + def.Personality, AllowedTools enforcement on Execute
- [ ] 3.3 Create `internal/adapters/tui/interview.go` — interviewModel with step enum (name→desc→tools→skills→personality→confirm), back-navigation, multi-select for tools, input validation
- [ ] 3.4 Wire Brain `/create-agent` command — routes to interviewModel, on confirm calls DefRepository.CreateDef + Registry.RegisterDynamic(name, factory) + namespace creation
- [ ] 3.5 Add startup loader in main.go — after SQLite init, ListDefs() → RegisterDynamic() for each persisted def
- [ ] 3.6 Write/tests: dynamic_test.go — DynamicSubagent Execute with mock Spawner, tool validation at CreateDef time, startup reload

## Phase 4: Upstream Tracker (~500-600 lines)

- [ ] 4.1 Create `internal/adapters/db/tracker.go` — tracker_releases + tracker_state CRUD (etag, last_checked tag, release cache)
- [ ] 4.2 Create `internal/tracker/monitor.go` — GitHubReleaseMonitor with http.Client, ETag caching (If-None-Match), GITHUB_TOKEN auth, CheckLatest/ListReleases methods
- [ ] 4.3 Create `internal/tracker/analyzer.go` — ChangelogAnalyzer with regex-based conventional-commit classification (feat/fix/breaking_change/refactor/docs/other), AnalyzeRelease/DiffReleases methods
- [ ] 4.4 Create `internal/tracker/manifest.go` — PortManifest (Load/Save with atomic write via temp file + os.Rename)
- [ ] 4.5 Create `internal/tracker/report.go` — markdown report generator (grouped by ManifestStatus table)
- [ ] 4.6 Add `gaia tracker {check,report,port}` CLI dispatch to `cmd/gaia/main.go` — check calls CheckLatest + DiffWithManifest, report prints markdown, port updates manifest entry status
- [ ] 4.7 Seed `tracker/manifest.yaml` with features from SPEC.md Section 9.1
- [ ] 4.8 Write/tests: monitor_test.go (httptest.Server — 200/304/403 scenarios), analyzer_test.go (table-driven classification), manifest_test.go (atomic write concurrency)

## Total: 4 phases, 21 tasks

### Tasks by feature
- Foundation: 4 tasks (parallelizable)
- Async: 7 tasks (sequential — 2.1→2.2→2.4→2.5→2.3 can parallel some)
- Dynamic: 6 tasks (sequential — 3.1→3.2→3.3→3.4→3.5→3.6)
- Tracker: 8 tasks (sequential — 4.1→4.2→4.3→4.4→4.5→4.6→4.7→4.8)

### Parallelism note
Tasks 1.1–1.4 are independent and parallelizable. Within each feature, some DB/test tasks can parallel with implementation tasks.
