# Design: Async Delegation + Dynamic Generator + Upstream Tracker

## Technical Approach

Three independent features sharing SQLite and the existing Registry/Spawner infrastructure. Async Delegation is the foundation — it adds a TaskManager and `SpawnAsync` that Dynamic and Tracker both reuse. Dynamic Generator extends the Registry with runtime-constructed subagents satisfying the existing `Subagent` interface. Upstream Tracker is a net-new subsystem under `internal/tracker/` with no coupling to the agent loop.

## Architecture Decisions

### Async — TaskManager integration with Spawner

| Option | Tradeoff | Decision |
|--------|----------|----------|
| TaskManager as SpawnerConfig field | Spawner owns lifecycle; clean injection | **Chosen** — `SpawnerConfig.TaskManager *TaskManager` |
| TaskManager as global singleton | Faster to implement, harder to test | Rejected — violates hexagonal principle |
| TaskManager as separate port | More flexible but adds indirection | Rejected — v1 single-process, no need |

**Rationale**: Spawner already holds config dependencies (Provider, Tools, Namespace). Adding TaskManager follows the same pattern. Existing `Spawn()` remains untouched; `SpawnAsync()` is a new method that wraps it.

### Async — TUI notification bridge

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `tea.Cmd` wrapping `<-chan TaskState` | Idiomatic Bubbletea; blocks until event | **Chosen** |
| Polling ticker | Simpler but wastes CPU | Rejected |
| Direct model mutation from goroutine | Race conditions | Rejected — Bubbletea requires all state changes via `tea.Msg` |

**Flow**: `TaskManager.SubscribeAll() <-chan TaskState` → `waitForTaskUpdate()` returns `tea.Cmd` → on receive, dispatches `taskUpdateMsg{state}` → `Update()` handles it, re-renders task pane.

### Async — @name parsing location

**Decision**: Parse in `Brain.ProcessMessage()`, not TUI. Brain already owns routing logic. TUI only handles display and input collection.

**Parser**: `strings.HasPrefix(input, "@")` → split on first space → `name = tokens[0][1:]`, `message = rest`. If name not in `Spawner.Available()`, return error with available list.

### Async — SDD pipeline as async tasks

**Decision**: A `PipelineRunner` goroutine that chains phases sequentially. Each phase calls `SpawnAsync()`, waits on `WaitTask(ctx, taskID)`, then starts the next phase. Cancellation propagates via shared context.

```
PipelineRunner goroutine:
  for i, phase := range phases:
    taskID, _ := spawner.SpawnAsync(ctx, phase, task)
    result, err := taskMgr.WaitTask(ctx, taskID)
    if cancelled → break (remaining phases marked Cancelled)
    if failed → break
    // feed result into next phase's task.KGContext
```

### Dynamic — DynamicSubagent vs hardcoded

**Decision**: `DynamicSubagent` implements `Subagent` identically to hardcoded subagents — it calls `Spawner.RunLoop()` with its def's system prompt and tool filter. The only difference: the system prompt and tools come from a `SubagentDef` (SQLite row) instead of compiled code.

**No special-casing in Spawner**: `Spawner.Spawn()` looks up the factory from Registry. Dynamic factories are registered the same way as compiled ones via `Registry.Register(name, factory)`.

### Dynamic — SQLite schema

```sql
CREATE TABLE IF NOT EXISTS subagent_defs (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    allowed_tools TEXT NOT NULL DEFAULT '[]',  -- JSON array
    skills TEXT NOT NULL DEFAULT '[]',         -- JSON array
    system_prompt TEXT NOT NULL DEFAULT '',
    personality TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Dynamic — Interview flow state machine

```
[Start] → name → description → tools → skills → personality → confirm → [Done]
              ↑         ↑          ↑        ↑          ↑
              └─────────┴──────────┴────────┴──────────┘
                         (back navigation)
```

**Decision**: Implement as a TUI sub-model (`interviewModel`) with a `step` enum and `answers` struct. Each step validates before advancing. Back = decrement step. Confirm = create SubagentDef + register.

### Dynamic — Tool validation at def creation time

**Decision**: Validate `AllowedTools` against `ToolRegistry.Names()` at CreateDef time. Store the validated list. At Execute time, trust the stored list — if a tool is removed from the registry later, `ToolRegistry.Filtered()` will simply not find it and return an error to the LLM. This avoids stale-def invalidation complexity.

### Tracker — GitHub API integration

**Decision**: Use `net/http` directly (not `gh` CLI) for portability. Endpoint: `GET /repos/Gentleman-Programming/gentle-ai/releases`. Auth: `GITHUB_TOKEN` env var → `Authorization: Bearer <token>` header. Caching: store ETag in `tracker_state` table; send `If-None-Match` on subsequent calls.

### Tracker — PortManifest YAML schema

```yaml
version: 1
last_checked: "2026-07-18T10:00:00Z"
features:
  - upstream_feature: "streaming-tools"
    status: "ported"           # ported | partial | not-ported | not-applicable
    gaia_location: "internal/core/toolengine/streaming.go"
    upstream_version: "v1.2.0"
    notes: "Full parity with TS implementation"
  - upstream_feature: "review-lens"
    status: "partial"
    gaia_location: "internal/agent/ops/reviewer.go"
    upstream_version: "v1.3.0"
    notes: "Missing risk-classification lens"
```

### Tracker — Issue creation for port command

**Decision**: Use `gh issue create` CLI (not GitHub API directly). Rationale: `gh` handles auth, respects user's GitHub config, and is already a dependency for PR workflows. Shell out via `exec.CommandContext("gh", ...)`.

## Data Flow

### Async Spawn Flow

```
TUI (user types "@explorer find bugs")
  │
  ▼
Brain.ProcessMessage()
  │ parses @name prefix
  ▼
Spawner.SpawnAsync(ctx, "explorer", task)
  │ creates TaskID (UUID)
  │ TaskManager.CreateTask() → Pending
  │ launches goroutine
  ▼                                    ┌─────────────────────┐
returns TaskID immediately             │ goroutine:           │
  │                                    │  TaskManager → Running│
  ▼                                    │  Spawner.Spawn()     │
TUI shows "Task abc-123 started"       │  TaskManager → Done   │
  │                                    └─────────────────────┘
  ▼ (tea.Cmd waiting on channel)
taskUpdateMsg{state} received
  │
  ▼
TUI renders completion notification
```

### Dynamic Subagent Creation Flow

```
User: /create-agent
  │
  ▼
TUI enters interviewModel (step: name)
  │ user answers each step
  ▼
Brain.CreateDynamicSubagent(def)
  │ validates tools against ToolRegistry
  │ SQLite INSERT into subagent_defs
  │ Registry.Register(name, factory)
  ▼
"Subagent 'X' created. Type @X to chat."
```

### Tracker Check Flow

```
User: gaia tracker check
  │
  ▼
GitHubReleaseMonitor.CheckLatest(ctx)
  │ HTTP GET /repos/.../releases (with ETag)
  │ 304 → return cached, hasNew=false
  │ 200 → parse, store in tracker_releases
  ▼
ChangelogAnalyzer.AnalyzeRelease(release)
  │ regex classify each entry
  ▼
Compare with PortManifest
  │ diff upstream features vs manifest
  ▼
Print delta: "2 new features since last check"
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/agent/task_manager.go` | Create | TaskManager, TaskState, TaskStatus, UUID gen, channels |
| `internal/agent/task_manager_test.go` | Create | Concurrent safety, lifecycle, cancellation tests |
| `internal/agent/spawner.go` | Modify | Add TaskManager to SpawnerConfig; add SpawnAsync method |
| `internal/agent/registry.go` | Modify | Add Unregister(name) method for dynamic subagent removal |
| `internal/core/domain/models.go` | Modify | Add IsDirectChat field to SubagentTask |
| `internal/core/ports/ports.go` | Modify | Add AsyncSpawner port interface |
| `internal/agent/dynamic.go` | Create | DynamicSubagent, SubagentDef, DynamicLoader |
| `internal/agent/dynamic_test.go` | Create | DynamicSubagent execution, loader, validation tests |
| `internal/adapters/db/subagent_defs.go` | Create | CRUD for subagent_defs table |
| `internal/adapters/db/sqlite.go` | Modify | Add subagent_defs + tracker_state + tracker_releases migrations |
| `internal/agent/memory/namespace.go` | Modify | Add DynamicPrefix(name) for gaia/subagent/{name}/ format |
| `internal/tracker/monitor.go` | Create | GitHubReleaseMonitor with ETag caching |
| `internal/tracker/analyzer.go` | Create | ChangelogAnalyzer with conventional-commit regex |
| `internal/tracker/manifest.go` | Create | PortManifest Load/Save with atomic writes |
| `internal/tracker/report.go` | Create | Markdown report generator |
| `internal/tracker/monitor_test.go` | Create | HTTP mock tests for monitor |
| `internal/tracker/analyzer_test.go` | Create | Classification tests |
| `internal/tracker/manifest_test.go` | Create | Load/save/atomic write tests |
| `internal/adapters/tui/tui.go` | Modify | Add task pane, taskUpdateMsg, waitForTaskUpdate cmd |
| `internal/adapters/tui/interview.go` | Create | Interview state machine for /create-agent |
| `internal/adapters/db/tracker.go` | Create | tracker_releases + tracker_state CRUD |
| `tracker/manifest.yaml` | Create | Default port manifest seeded from SPEC.md |

## Interfaces / Contracts

```go
// --- Async ---

type TaskStatus string
const (
    TaskPending   TaskStatus = "pending"
    TaskRunning   TaskStatus = "running"
    TaskCompleted TaskStatus = "completed"
    TaskFailed    TaskStatus = "failed"
    TaskCancelled TaskStatus = "cancelled"
)

type TaskState struct {
    TaskID       string
    SubagentName string
    Status       TaskStatus
    Result       *domain.SubagentResult
    Error        string
    CreatedAt    time.Time
    CompletedAt  time.Time
}

type TaskManager struct { /* unexported: map, mutex, cancel funcs, subscriber chans */ }
func NewTaskManager() *TaskManager
func (tm *TaskManager) CreateTask(name string, task domain.SubagentTask) string
func (tm *TaskManager) UpdateStatus(taskID string, status TaskStatus, result *domain.SubagentResult, err error)
func (tm *TaskManager) CancelTask(taskID string) error
func (tm *TaskManager) WaitTask(ctx context.Context, taskID string) (*TaskState, error)
func (tm *TaskManager) SubscribeTask(taskID string) <-chan TaskState
func (tm *TaskManager) SubscribeAll() <-chan TaskState
func (tm *TaskManager) ListTasks() []TaskState

// Spawner additions
func (s *Spawner) SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error)

// ports addition
type AsyncSpawner interface {
    SubagentPort
    SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error)
    TaskManager() *TaskManager
}

// --- Dynamic ---

type SubagentDef struct {
    Name         string    `json:"name"`
    Description  string    `json:"description"`
    AllowedTools []string  `json:"allowed_tools"`
    Skills       []string  `json:"skills"`
    SystemPrompt string    `json:"system_prompt"`
    Personality  string    `json:"personality"`
    CreatedAt    time.Time `json:"created_at"`
}

type DynamicSubagent struct { /* def SubagentDef, spawner *Spawner */ }
func NewDynamicSubagent(def SubagentDef, spawner *Spawner) *DynamicSubagent
func (d *DynamicSubagent) Name() string
func (d *DynamicSubagent) Description() string
func (d *DynamicSubagent) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult

type DefRepository interface {
    CreateDef(ctx context.Context, def SubagentDef) error
    GetDef(ctx context.Context, name string) (*SubagentDef, error)
    ListDefs(ctx context.Context) ([]SubagentDef, error)
    UpdateDef(ctx context.Context, def SubagentDef) error
    DeleteDef(ctx context.Context, name string) error
}

// Registry addition
func (r *Registry) Unregister(name string) error

// --- Tracker ---

type Release struct {
    Tag         string    `json:"tag_name"`
    Body        string    `json:"body"`
    PublishedAt time.Time `json:"published_at"`
    HTMLURL     string    `json:"html_url"`
}

type ChangeType string
const (
    ChangeFeature        ChangeType = "feature"
    ChangeFix            ChangeType = "fix"
    ChangeProtocolChange ChangeType = "protocol-change"
    ChangeRefactor       ChangeType = "refactor"
    ChangeDocs           ChangeType = "docs"
    ChangeOther          ChangeType = "other"
)

type Change struct {
    Raw         string
    Type        ChangeType
    Description string
    Release     string
}

type GitHubReleaseMonitor struct { /* httpClient, owner, repo, db */ }
func NewGitHubReleaseMonitor(owner, repo string, db ReleaseStore) *GitHubReleaseMonitor
func (m *GitHubReleaseMonitor) CheckLatest(ctx context.Context) (*Release, bool, error)
func (m *GitHubReleaseMonitor) ListReleases(ctx context.Context, since string) ([]Release, error)

type ChangelogAnalyzer struct{}
func (a *ChangelogAnalyzer) AnalyzeRelease(release Release) []Change
func (a *ChangelogAnalyzer) DiffReleases(from, to Release) []Change

type ManifestStatus string
const (
    StatusPorted      ManifestStatus = "ported"
    StatusPartial     ManifestStatus = "partial"
    StatusNotPorted   ManifestStatus = "not-ported"
    StatusNotApplicable ManifestStatus = "not-applicable"
)

type ManifestEntry struct {
    UpstreamFeature string         `yaml:"upstream_feature"`
    Status          ManifestStatus `yaml:"status"`
    GaiaLocation    string         `yaml:"gaia_location"`
    UpstreamVersion string         `yaml:"upstream_version"`
    Notes           string         `yaml:"notes"`
}

type PortManifest struct { Version int `yaml:"version"`; LastChecked time.Time `yaml:"last_checked"`; Features []ManifestEntry `yaml:"features"` }
func LoadManifest(path string) (*PortManifest, error)
func (m *PortManifest) Save(path string) error
```

## Concurrency Model

| Component | Mechanism | Details |
|-----------|-----------|---------|
| TaskManager task map | `sync.RWMutex` | Read-lock for ListTasks/WaitTask, write-lock for CreateTask/UpdateStatus |
| Task cancellation | `context.WithCancel` stored per taskID | CancelTask calls the stored cancel func |
| Subscriber fan-out | Per-task buffered `chan TaskState` (cap 1) + broadcast `chan TaskState` (cap 64) | Non-blocking send with select/default for broadcast |
| SpawnAsync goroutine | `go func()` with `defer` panic recovery | Recover sets task to Failed with panic message |
| TUI updates | `tea.Cmd` wrapping channel receive | All state mutations go through Bubbletea message queue |
| PortManifest atomic write | Write to temp file → `os.Rename` | Prevents partial reads |

## Threat Matrix

| Boundary | Applicability | Design response |
|----------|---------------|-----------------|
| Documentation-like paths | N/A — no file classification | — |
| Git repository selection | N/A — no git operations in new code | — |
| Commit state | N/A — tracker reads releases, not commits | — |
| Push state | N/A — no push operations | — |
| PR commands | Applicable — `gaia tracker port` may create issues via `gh` | Validate feature name against manifest before shelling out; use `exec.CommandContext` with timeout; reject names with shell metacharacters |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | TaskManager lifecycle, concurrent access, panic recovery | Table-driven tests with goroutine leak detection (goleak) |
| Unit | DynamicSubagent Execute, tool validation | Mock Spawner; verify AllowedTools enforcement |
| Unit | ChangelogAnalyzer classification | Table-driven: conventional commit → ChangeType mapping |
| Unit | PortManifest atomic write | Concurrent read/write test |
| Integration | SpawnAsync end-to-end with real Spawner | Use test LLM provider; verify task completes |
| Integration | GitHubReleaseMonitor with httptest.Server | Mock GitHub API responses (200, 304, 403) |
| Integration | Dynamic loader startup flow | Insert defs into test SQLite; verify Registry population |
| E2E | @name routing through Brain | Send "@explorer test" → verify SpawnAsync called |

## Migration / Rollout

- **SQLite**: Three additive `CREATE TABLE IF NOT EXISTS` in existing `migrate()`. No data migration needed.
- **Feature flags**: `brain.direct_routing` (default: true), `subagents.dynamic_enabled` (default: true). Both in `domain.Config`.
- **Backward compatibility**: `Spawn()` unchanged. Existing sync callers unaffected. New `IsDirectChat` field defaults to false (zero value).
- **Manifest seeding**: Initial `tracker/manifest.yaml` committed to repo with features from SPEC.md Section 9.1.

## Open Questions

- [ ] Should TaskManager persist tasks to SQLite in v1, or defer to v2? (Spec says MAY; proposal says in-memory v1)
- [ ] Should `gaia tracker port <feature>` auto-create a GitHub issue, or just update the manifest?
