# Design: Milestone 5 — Production

## Technical Approach

Milestone 5 extends GAIA's hexagonal architecture with 7 production capabilities across 4 stacked PRs. The core strategy is **executor abstraction** for backends (Docker/SSH share a `CommandExecutor` port), **adapter isolation** for desktop (Wails wraps existing Brain), and **port-based integration** for MCP (tools discovered at runtime, registered into existing ToolRegistry). Cron and Doctor are standalone packages with minimal coupling. Onboard is a CLI flow using existing subagent infrastructure.

## Architecture Decisions

### Decision: Backend Executor Abstraction

**Choice**: Introduce `CommandExecutor` interface in `internal/modules/shell/` with three implementations: `LocalExecutor`, `DockerExecutor`, `SSHExecutor`. Module reads `terminal.backend` from config and selects executor at construction time.

**Alternatives considered**: 
- Separate modules per backend (rejected: duplicates allowlist/validation logic)
- Middleware/wrapper pattern (rejected: over-engineering for 3 implementations)

**Rationale**: The shell module's value is in allowlist enforcement, path validation, and secret redaction — these are backend-agnostic. Only the execution step changes. A strategy pattern keeps one module, one tool definition, one set of security checks.

### Decision: SSH Library

**Choice**: `golang.org/x/crypto/ssh` — the de facto standard Go SSH library.

**Alternatives considered**: 
- `melbahja/goph` (higher-level, but wraps x/crypto/ssh anyway)
- Shelling out to `ssh` binary (rejected: no key management, platform-dependent)

**Rationale**: Already in the Go ecosystem, well-maintained, supports key auth, agent forwarding, and host key verification. No new dependency tree.

### Decision: Docker Execution Strategy

**Choice**: `docker exec` into a running container (user must start container). GAIA does NOT manage container lifecycle.

**Alternatives considered**: 
- Docker SDK for Go (rejected: heavy dependency, overkill for exec)
- `docker run --rm` per command (rejected: too slow, 1-2s overhead per command)

**Rationale**: Shelling out to `docker exec` is simple, debuggable, and lets users manage their own containers. Config specifies `terminal.docker.container` (name/ID). If container isn't running, shell_exec returns a clear error.

### Decision: Desktop Architecture

**Choice**: Wails v3 with Brain-as-library. `internal/adapters/desktop/` creates a Brain instance identical to TUI, but with a `DesktopUI` adapter implementing `ports.UIService` via Wails event bridge. Frontend is a webview (HTML/JS/CSS).

**Alternatives considered**: 
- Electron (rejected: Node.js dependency, large binary)
- Fyne (rejected: custom widget toolkit, less flexible than webview)
- Tauri (rejected: Rust dependency, Wails is pure Go)

**Rationale**: Wails produces a single binary (~15MB), uses the OS webview (no bundling), and the Go backend shares 100% of Brain logic. The `ports.UIService` interface was designed for this — DesktopUI is another adapter alongside TUI and NullUI.

### Decision: Cron Storage

**Choice**: SQLite table `cron_jobs` in the existing database. Schema: `id, name, schedule, task, deliver_to, deliver_target, enabled, last_run, next_run, created_at`.

**Alternatives considered**: 
- JSON file (rejected: no atomic updates, no concurrent access)
- Separate BoltDB (rejected: unnecessary second database)

**Rationale**: GAIA already uses SQLite via `internal/adapters/db/`. Adding a table is trivial. The `Repository` port gains cron methods. WAL mode (already default) prevents read/write contention.

### Decision: MCP Tool Registration

**Choice**: MCP client discovers tools via `tools/list` and wraps each as a `domain.ToolCall` registered into the existing `ToolRegistry`. MCP tool execution goes through `tools/call`. Tools are prefixed `mcp_{server}_{name}` to avoid collisions.

**Alternatives considered**: 
- Separate MCP tool registry (rejected: Brain only knows one registry)
- Dynamic module loading (rejected: MCP tools ARE dynamic by nature)

**Rationale**: The ToolRegistry already supports dynamic registration via `Register(mod ports.Module)`. An `MCPModule` implements `ports.Module` — its `GetTools()` calls MCP `tools/list`, and `Execute()` calls MCP `tools/call`. Zero changes to Brain.

### Decision: Doctor Check Architecture

**Choice**: List of `HealthCheck` functions, each returning `CheckResult{Name, Status, Message, Duration}`. Run sequentially (not parallel — diagnostics should be readable in order). Output as table (terminal) or JSON (`--json`).

**Alternatives considered**: 
- Plugin-based checks (rejected: over-engineering for a fixed set)
- Parallel execution (rejected: order matters for readability, checks are fast)

**Rationale**: Simple, extensible (add a function to the list), and testable (each check is a pure function).

## Data Flow

### Backend Execution Flow
```
User Input → Brain → shell_exec tool call
    → ShellModule.Execute()
        → config.Backend?
            local  → LocalExecutor.Exec(cmd)
            docker → DockerExecutor.Exec(docker exec -it <container> cmd)
            ssh    → SSHExecutor.Exec(ssh user@host cmd)
        → security.RedactSecrets(output)
    → ToolResult → Brain → LLM
```

### Cron Flow
```
gaia cron start
    → Load jobs from SQLite
    → Scheduler.Loop():
        for each job where now >= next_run:
            → Brain.ProcessMessage(job.Task)  [via NullUI or delivery adapter]
            → Update last_run, compute next_run
            → Deliver result (terminal/telegram)
```

### MCP Flow
```
Brain init → MCPClient.Connect(serverURL)
    → tools/list → []MCPTool
    → Wrap as MCPModule (implements ports.Module)
    → Brain.RegisterModule(mcpModule)
    → LLM can now call mcp_{server}_{tool}
    → Execute → MCPClient.CallTool(name, args) → result
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/modules/shell/executor.go` | Create | `CommandExecutor` interface + `LocalExecutor` |
| `internal/modules/shell/docker.go` | Create | `DockerExecutor` — runs via `docker exec` |
| `internal/modules/shell/ssh.go` | Create | `SSHExecutor` — runs via `x/crypto/ssh` |
| `internal/modules/shell/shell.go` | Modify | Accept executor in constructor; delegate execution |
| `internal/core/domain/models.go` | Modify | Add `Terminal` config struct (Backend, Docker, SSH fields) |
| `internal/config/config.go` | Modify | Apply defaults for Terminal config |
| `internal/adapters/desktop/desktop.go` | Create | Wails v3 app, DesktopUI adapter |
| `internal/adapters/desktop/bindings.go` | Create | Go↔JS bridge functions (SendMessage, GetHistory) |
| `cmd/gaia/desktop.go` | Create | `gaia desktop` subcommand handler |
| `internal/cron/scheduler.go` | Create | Cron scheduler loop with job evaluation |
| `internal/cron/store.go` | Create | SQLite-backed job persistence |
| `internal/cron/delivery.go` | Create | Delivery adapters (terminal, telegram) |
| `internal/cron/cron.go` | Create | Package entry: NewScheduler, job types |
| `internal/adapters/db/cron.go` | Create | Cron table migration + CRUD queries |
| `internal/core/ports/ports.go` | Modify | Add `CronRepository` interface |
| `cmd/gaia/cron.go` | Create | `gaia cron` subcommand family |
| `internal/mcp/client.go` | Create | MCP JSON-RPC client (stdio/HTTP transport) |
| `internal/mcp/module.go` | Create | `MCPModule` implementing `ports.Module` |
| `internal/mcp/types.go` | Create | MCP protocol types (Tool, CallResult) |
| `cmd/gaia/doctor.go` | Create | `gaia doctor` — health check runner |
| `internal/doctor/checks.go` | Create | Individual health check implementations |
| `cmd/gaia/onboard.go` | Create | `gaia onboard` — guided SDD walkthrough |
| `cmd/gaia/main.go` | Modify | Register `desktop`, `cron`, `doctor`, `onboard` subcommands |

## Interfaces / Contracts

```go
// internal/modules/shell/executor.go
type CommandExecutor interface {
    Exec(ctx context.Context, cmd string, args []string, dir string) (output string, err error)
    Name() string // "local", "docker", "ssh"
}

// internal/core/domain/models.go — new fields on Config
type TerminalConfig struct {
    Backend string       `yaml:"backend"` // "local", "docker", "ssh"
    Docker  DockerConfig `yaml:"docker"`
    SSH     SSHConfig    `yaml:"ssh"`
}
type DockerConfig struct {
    Container string `yaml:"container"` // container name or ID
    WorkDir   string `yaml:"workdir"`   // working dir inside container
}
type SSHConfig struct {
    Host       string `yaml:"host"`
    Port       int    `yaml:"port"`
    User       string `yaml:"user"`
    KeyPath    string `yaml:"key_path"`
    KnownHosts string `yaml:"known_hosts"`
}

// internal/core/ports/ports.go — new interface
type CronRepository interface {
    ListJobs(ctx context.Context) ([]domain.CronJob, error)
    CreateJob(ctx context.Context, job domain.CronJob) (string, error)
    UpdateJob(ctx context.Context, job domain.CronJob) error
    DeleteJob(ctx context.Context, id string) error
    GetDueJobs(ctx context.Context) ([]domain.CronJob, error)
    MarkRun(ctx context.Context, id string, lastRun time.Time, nextRun time.Time) error
}

// internal/mcp/module.go
type MCPModule struct { /* server connection + cached tool list */ }
func (m *MCPModule) Name() string        { return "mcp_" + m.serverName }
func (m *MCPModule) Description() string { return "MCP tools from " + m.serverName }
func (m *MCPModule) GetTools() []domain.ToolCall { /* from MCP tools/list */ }
func (m *MCPModule) Execute(ctx context.Context, tool string, args map[string]interface{}) (*domain.ToolResult, error) { /* MCP tools/call */ }

// internal/doctor/checks.go
type CheckResult struct {
    Name     string
    Status   string // "ok", "warn", "fail"
    Message  string
    Duration time.Duration
}
type HealthCheck func(ctx context.Context) CheckResult
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Executor selection, Docker command building, SSH config parsing | Table-driven tests with mock executors |
| Unit | Cron schedule parsing (cron expr → next run), job CRUD | Pure function tests |
| Unit | MCP tool wrapping, name prefixing, argument marshaling | Mock MCP server responses |
| Unit | Doctor checks (each check in isolation) | Mock dependencies (DB, config, network) |
| Integration | Shell module with Docker executor against real container | `shell_integration_test.go` pattern |
| Integration | Cron scheduler with SQLite (job persistence across restarts) | In-memory SQLite |
| Integration | MCP client against a test MCP server | net.Pipe or test server |
| E2E | `gaia doctor` full run | Golden file comparison |
| E2E | `gaia cron create` → `gaia cron list` → `gaia cron start` → verify execution | Temp dir + config |

## Threat Matrix

| Boundary | Applicability | Design Response | Planned RED Tests |
|----------|--------------|-----------------|-------------------|
| Documentation-like paths | **Applicable** — Docker executor receives command strings that may contain path-like arguments | Allowlist enforcement happens BEFORE executor dispatch; Docker/SSH executors receive already-validated commands | Test: `shell_exec "cat /etc/passwd"` passes allowlist but Docker executor runs inside container (isolated) |
| Git repository selection | **Applicable** — SSH backend may target a different host with different repo layout | Shell module's `projectRoot` is the local working directory; SSH/Docker executors use their own configured workdir, NOT the local path | Test: SSH executor does NOT leak local absolute paths to remote host |
| Commit state | N/A — no git operations modified in M5 | — | — |
| Push state | N/A — no push operations in M5 | — | — |
| PR commands | N/A — no PR automation in M5 | — | — |
| Subprocess injection | **Applicable** — Docker and SSH executors construct subprocess commands from user input | Commands are already split by `strings.Fields` (no shell interpolation); Docker uses `docker exec <container> <cmd> <args...>` as argv, not shell string; SSH uses channel exec with argv | Test: command `"; rm -rf /"` is allowlist-rejected before reaching executor; Test: Docker args passed as argv slice, not interpolated |

## Migration / Rollout

No data migration required. All features are additive:
- **Config**: New `terminal` section defaults to `backend: local` (backward-compatible)
- **Database**: Cron table created on first `gaia cron` invocation (lazy migration)
- **CLI**: New subcommands don't affect existing ones
- **Dependencies**: `golang.org/x/crypto` (SSH) and `github.com/wailsapp/wails/v3` (desktop) are additive

Feature flags: each backend is activated by config change only. No runtime feature flags needed.

## Open Questions

- [ ] Wails v3 API stability — should we pin to a specific RC version or wait for stable?
- [ ] MCP transport: stdio-only for v1, or also HTTP/SSE from the start?
- [ ] Cron delivery to Telegram: should it use the existing `TelegramAdapter` or a separate lightweight sender?
- [ ] Should `gaia doctor` check for MCP server connectivity if MCP is configured?
