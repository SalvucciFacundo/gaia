# GAIA Engineering Rules & Architecture Contract

This document defines the strict engineering standards, package taxonomy, and Go skill mapping for GAIA.
Every AI agent or developer contributing to this repository **MUST** follow these rules.

---

## 🏛️ 1. Hexagonal Architecture & Package Boundaries

GAIA strictly enforces Hexagonal Architecture (Ports & Adapters) with inward dependency flow:

```
     cmd/gaia (Composition Root: wires dependencies, dispatches CLI)
        │
        ├──> internal/adapters (Concrete infra: DB SQLite, LLM, Web, TUI, Audio, Desktop)
        ├──> internal/gateway  (External messaging: Telegram, Discord, Slack, WhatsApp)
        ├──> internal/modules  (OS Sandboxes: shell, fileops, gitops, codemode)
        ├──> internal/tools    (LLM-callable tools & tool catalog)
        ├──> internal/agent    (Autonomous strategies: ODD, SDD, autofix, MoA, spawner)
        ├──> internal/commands (Slash commands: /session, /config, etc.)
        ├──> internal/review   (Adversarial review & CAS gates)
        ├──> internal/mneme    (Memory engine: native SQLite FTS5 + Vector hybrid)
        │
        └──> internal/core     (Neutral core: domain entities & ports)
                 ├── domain/   (Pure entities, models, value objects)
                 └── ports/    (Abstract consumer-driven interfaces)
```

### Inviolable Dependency Rules:
1. **`internal/core/` is isolated**: It MUST NEVER import `adapters`, `agent`, `commands`, `modules`, `tools`, `gateway`, `review`, or external network/SQL drivers.
2. **Ports define contracts**: Infrastructure layers in `internal/adapters/` MUST implement interfaces defined in `internal/core/ports/`.
3. **No circular dependencies**: Use dependency inversion via `ports/` whenever bidirectional communication is needed.
4. **Composition Root**: `cmd/gaia/` is the ONLY place allowed to wire concrete adapters into core/agent structures.

---

## 🗂️ 2. Package Taxonomy (Where Code Belongs)

- **`cmd/gaia/`**: Entry point and CLI flag parsing. Do not place business logic here.
- **`internal/core/`**: Core iteration engine (`kernel.go`), session routing (`session.go`), token budget (`cost.go`, `usage.go`), permission matrix (`guard.go`, `policy.go`), and abstract `ports/` & `domain/`.
- **`internal/agent/`**: Subagent lifecycle, orchestration (`workflow_router.go`), SDD pipeline (`sdd/`), ops subagents (`ops/`), MoA fan-out (`moa.go`), autofix verification (`autofix.go`).
- **`internal/commands/`**: User-facing slash commands (`/config`, `/session`, `/workflow`, `/skill`, etc.) implementing `ports.CommandDispatcher`.
- **`internal/tools/`**: LLM tool definitions, catalog registry, search/fetch runners.
- **`internal/modules/`**: Execution sandboxes for OS interactions (shell, file operations, git operations, Yaegi code mode).
- **`internal/adapters/`**: Database (SQLite), LLM API clients, TUI (Bubbletea), Web dashboard (SSE/CSRF), Desktop (Wails bindings).
- **`internal/gateway/`**: Multiplexed messaging adapters (Telegram, Discord, Slack, WhatsApp).
- **`internal/mneme/`**: Zero-dependency embedded memory engine (SQLite FTS5 + vectors).

---

## 🛠️ 3. Go Skills Trigger Matrix (Consult Under Demand)

Do not guess Go idioms or patterns. When working on any of the following technical domains, **consult the matching skill** in the environment before writing code:

| Technical Area | Skill to Consult | Enforced Standards |
| :--- | :--- | :--- |
| **Concurrency & Goroutines** | `golang-concurrency`<br>`golang-safety` | No goroutine leaks; sender closes channels; RWMutex without `defer` inside hot loops; zero data races. |
| **Contexts & Cancellation** | `golang-context`<br>`golang-troubleshooting` | `ctx context.Context` is always 1st param; early checks for `ctx.Err()` before and inside loops; propagate cancel. |
| **Error Handling** | `golang-error-handling`<br>`golang-samber-oops` | Always wrap errors with `%w`; use `errors.Is`/`errors.As`; never ignore errors with `_ = err`. |
| **Data Structures & Allocations** | `golang-data-structures`<br>`golang-performance`<br>`golang-benchmark` | Preallocate slices `make([]T, 0, n)`; reuse buffers (`sync.Pool`); stream with `bufio.Scanner` rather than loading full files into memory. |
| **Security & Input Validation** | `golang-security`<br>`golang-safety` | Prevent path traversal (`filepath.Clean`); protect against SSRF; enforce `io.LimitReader` on all network bodies/JSON unmarshaling. |
| **Database & Persistence** | `golang-database` | SQLite transactions; parameterized queries against SQL injection; connection pool tuning. |
| **Project Layout & Interfaces** | `golang-project-layout`<br>`golang-structs-interfaces`<br>`golang-design-patterns` | Keep interfaces small and consumer-driven; adhere to Hexagonal boundaries; avoid stuttering in package names. |
| **CLI & Process Lifecycle** | `golang-cli` | Clean signal handling (`signal.NotifyContext`); graceful shutdown; structured CLI flags. |
| **Testing & Verification** | `golang-testing`<br>`golang-linter` | Table-driven tests; race detection (`-race`); hermetic mocks with `httptest`; zero reliance on live internet endpoints. |
| **Modern Go Features & Style** | `golang-code-style`<br>`golang-naming`<br>`golang-modernize` | Idiomatic Go naming; `log/slog` structured logging; Go 1.22+ idioms. |

---

## 🚦 4. Verification Gate (Pre-flight Requirement)

Every proposed change or task completion **MUST** pass:
1. `go vet ./...` (zero lint warnings/errors)
2. `go test -race -count=1 ./...` (100% green tests under active race detector)
3. Conventional Commits format (`refactor(pkg): ...`, `fix(pkg): ...`, `test(pkg): ...`) with no AI attribution or Co-Authored-By trailers.
