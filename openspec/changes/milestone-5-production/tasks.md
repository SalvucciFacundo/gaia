# Tasks: Milestone 5 — PRs 2-4: Desktop + Cron + MCP/Doctor/Onboard

## PR 2: Wails Desktop

- [x] 2.1 Create `internal/adapters/desktop/desktop.go` with DesktopUI adapter implementing ports.UIService
- [x] 2.2 Create `internal/adapters/desktop/bindings.go` — Go↔JS bridge functions (SendMessage, GetHistory)
- [x] 2.3 Create `cmd/gaia/desktop.go` — `gaia desktop` subcommand that wires Brain with DesktopUI
- [x] 2.4 Tests for desktop service layer (DesktopUI unit tests) — 11 tests pass

## PR 3: Cron Scheduler

- [x] 3.1 Create `internal/cron/cron.go` — package entry, NewScheduler, CronJob types, Scheduler struct
- [x] 3.2 Create `internal/cron/store.go` — SQLite-backed job persistence via cronRepository (in adapters/db/cron.go)
- [x] 3.3 Create `internal/cron/scheduler.go` — Cron scheduler loop with job evaluation
- [x] 3.4 Create `internal/cron/delivery.go` — Delivery adapters (terminal stdout)
- [x] 3.5 Create `internal/adapters/db/cron.go` — Cron table migration + CRUD queries
- [x] 3.6 Add `CronRepository` interface to `internal/core/ports/ports.go`
- [x] 3.7 Create `cmd/gaia/cron.go` — `gaia cron` CLI: create, list, pause, resume, remove
- [x] 3.8 Tests for cron package (scheduler, store, delivery) — 10 tests pass

## PR 4: MCP + Doctor + Onboard

- [x] 4.1 Create `internal/mcp/types.go` — MCP protocol types (MCPTool, MCPCallResult, MCPServerConfig)
- [x] 4.2 Create `internal/mcp/client.go` — MCP JSON-RPC client connecting to MCP servers via stdio
- [x] 4.3 Create `internal/mcp/module.go` — MCPModule implementing ports.Module for tool registration
- [x] 4.4 Add MCP config struct to `internal/core/domain/models.go` (MCPServer, MCPConfig)
- [x] 4.5 Apply MCP config defaults in `internal/config/config.go`
- [x] 4.6 Create `internal/doctor/checks.go` — individual health check implementations (6 checks)
- [x] 4.7 Create `cmd/gaia/doctor.go` — `gaia doctor` CLI (checks LLM, modules, git, memory, config)
- [x] 4.8 Create `cmd/gaia/onboard.go` — `gaia onboard` guided SDD walkthrough with demo mode
- [x] 4.9 Tests for MCP (client, module — 6 tests) and doctor (checks — 8 tests)

## PR 2-4 Wiring

- [x] W.1 Update `cmd/gaia/main.go` — register desktop, cron, doctor, onboard subcommands
- [x] W.2 Final build + test verification — `go build ./...` PASS, `go vet ./...` clean, 24/24 packages pass (0 failures)
