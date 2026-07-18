# Proposal: Milestone 5 — Production

## Intent

GAIA has Milestones 1-4: core loop, 12 subagents, skills hub, headless mode, review system with GGA and Judgment Day. Milestone 5 adds production-grade capabilities: remote execution backends (Docker, SSH), a desktop application, scheduled task execution, MCP protocol integration, system diagnostics, and a guided onboarding flow.

These features transform GAIA from a local TUI agent into a production-ready tool that can run in isolated environments, connect to remote servers, operate as a desktop app, execute scheduled tasks, integrate with the broader MCP ecosystem, self-diagnose issues, and onboard new users.

## Scope

### 1. docker-backend
Docker container execution for the shell module. User configures `terminal.backend: docker` with an image name. Shell module detects config and runs commands via `docker exec` instead of local `exec.Command`. Provides isolated, reproducible execution environments.

### 2. ssh-backend
SSH remote execution for the shell module. User configures `terminal.backend: ssh` with host, port, user, and key path. Shell module runs commands via an SSH client library. Enables GAIA to operate on remote servers.

### 3. wails-desktop
Desktop application using Wails v3 (Go backend + webview frontend). New `internal/adapters/desktop/` package. Wraps the same Brain as TUI but with a web-based UI. Single binary distribution. Shares all core logic.

### 4. cron-scheduler
Built-in cron scheduler with platform delivery. New `internal/cron/` package. Jobs defined via CLI: `gaia cron create "0 2 * * *" "task" --deliver telegram`. Delivery targets: terminal (stdout), telegram (via existing adapter). Persistent job store in SQLite.

### 5. mcp-client
MCP (Model Context Protocol) client for extended capabilities. New `internal/mcp/` package. Connects to MCP servers, discovers tools, and exposes them as additional Brain tools via the existing ToolRegistry. Enables GAIA to use any MCP-compatible tool server.

### 6. gaia-doctor
System diagnostics command. `cmd/gaia/doctor.go` — checks LLM provider connectivity, memory/database health, module availability, git status, build info, and config validity. Returns a structured health report.

### 7. sdd-onboard
Guided walkthrough of the SDD workflow. New subagent or CLI command that walks users through a real example: explore → propose → spec → design → tasks → apply → verify. Educational and practical.

## Approach

### Delivery Strategy: 4 Stacked PRs (to main)

| PR | Capabilities | Rationale |
|----|-------------|-----------|
| PR 1 | docker-backend, ssh-backend | Shared executor abstraction in shell module |
| PR 2 | wails-desktop | Independent adapter, no dependency on backends |
| PR 3 | cron-scheduler | Independent package, uses existing Brain + delivery adapters |
| PR 4 | gaia-doctor, mcp-client, sdd-onboard | Diagnostics + extensibility + onboarding (lighter features) |

### Execution Order
PRs are stacked-to-main: each PR targets `main` and includes all previous PRs' changes in its diff. Implementation order follows the stack: PR1 → PR2 → PR3 → PR4.

## Rollback Plan

Each capability is independently feature-gated via config:
- **Backends**: `terminal.backend` defaults to `local`. Setting `docker` or `ssh` activates the new path. Removing the config reverts to local execution.
- **Desktop**: Separate binary (`gaia-desktop`). Does not affect TUI or headless mode.
- **Cron**: Opt-in via `gaia cron` subcommand. No cron jobs = no scheduler activity.
- **MCP**: Opt-in via `mcp.servers` config section. Empty = no MCP tools loaded.
- **Doctor**: Pure diagnostic, read-only. No side effects.
- **Onboard**: CLI subcommand, no side effects on existing flows.

## Non-Goals

- Wails frontend implementation (only the Go adapter skeleton + Brain wiring; frontend is a separate concern)
- Production Docker image for GAIA itself (this is about executing commands IN Docker, not running GAIA in Docker)
- Full MCP server implementation (client only)
- Cron daemon mode (scheduler runs within `gaia cron start`, not as a system service)

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Wails v3 is relatively new | API instability | Pin to specific version; isolate behind adapter |
| SSH library choice | Dependency bloat | Use `golang.org/x/crypto/ssh` (stdlib-adjacent) |
| Cron + SQLite contention | Write conflicts | Use WAL mode (already enabled); serialize cron writes |
| MCP protocol evolution | Breaking changes | Abstract behind port interface; version the client |
