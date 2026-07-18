# Tasks: Milestone 1 — Core Agent Loop

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~930 (380 + 350 + 200) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Phase 1) → PR 2 (Phase 2) → PR 3 (Tests + CI) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Provider Router + Agent Loop | PR 1 | `go test ./internal/adapters/llm/... ./internal/core/...` | `go run ./cmd/gaia` with `copilot` provider config | Revert `internal/adapters/llm/*.go`, `internal/core/*.go`, modified ports/models |
| 2 | Tool Engine | PR 2 | `go test ./internal/modules/... ./internal/core/...` | `go run ./cmd/gaia` — LLM issues `shell_exec` call | Revert `internal/modules/`, `internal/core/registry.go` |
| 3 | Tests + CI | PR 3 | `go test ./... && golangci-lint run` | `go test ./...` | Revert `.golangci.yml`, `Makefile`, integration test files |

## Phase 1: Provider Router + Agent Loop

- [x] 1.1 Extend `internal/core/ports/ports.go` — add `Stream()`, `TokenStream` alias, `ChatOpt`, `ChatOptions` to interfaces
- [x] 1.2 Extend `internal/core/domain/models.go` — add `TokenChunk`, `ToolDef`, `ToolResult.Success`, `TrustMode`, `BudgetConfig` domain types
- [x] 1.3 Create `internal/adapters/llm/openai.go` — OpenAI adapter with `Chat()` + `Stream()` via `go-openai`
- [x] 1.4 Create `internal/adapters/llm/anthropic.go` — Anthropic adapter with `Chat()` + `Stream()` via `anthropic-sdk-go`
- [x] 1.5 Create `internal/adapters/llm/ollama.go` — Ollama REST adapter with `Chat()` + `Stream()` (configurable endpoint)
- [x] 1.6 Create `internal/adapters/llm/llm.go` — package doc, helper types, constructor registry
- [x] 1.7 Create `internal/adapters/llm/router.go` — `Router` implementing `LLMProvider`, iterates `FallbackChain` on error
- [x] 1.8 Refactor `internal/adapters/llm/copilot_client.go` — implement new `LLMProvider` port; keep existing functionality
- [x] 1.9 Create `internal/core/guard.go` — `ConfirmGuard` with 4 modes (`always`/`per-session`/`per-action`/`never`) + per-session cache
- [x] 1.10 Modify `internal/core/kernel.go` — add iteration budget loop, tool registry dispatch, streaming fallback
- [x] 1.11 Modify `internal/adapters/tui/tui.go` — wire streaming token display via `AppendToken`, `/trust` slash commands
- [x] 1.12 Modify `internal/adapters/db/sqlite.go` — add `sessions` table + session-scoped message queries
- [x] 1.13 Modify `internal/config/config.go` — load `fallback_chain`, `budget.max_iterations`, `trust_mode` from YAML
- [x] 1.14 Modify `cmd/gaia/main.go` — wire Router → Brain → TUI, instantiate provider from config, register modules
- [x] 1.15 **Tests**: table-driven unit tests for Router fallback logic, ConfirmGuard modes, budget counter

## Phase 2: Tool Engine

- [ ] 2.1 Create `internal/modules/shell/shell.go` — Shell module: command allowlist, path validation, `ConfirmGuard` gate, secret redaction scan on output
- [ ] 2.2 Create `internal/modules/file/file.go` — File module: read/write/list, `filepath.Abs` + prefix check against project root allowlist, reject traversal
- [ ] 2.3 Create `internal/modules/git/git.go` — Git module: `status`/`log`/`diff` (read-only), cwd locked to project root, reject `..` escapes
- [ ] 2.4 Create `internal/core/registry.go` — `ToolRegistry` with flat `map[string]Tool`, `Register(mod)` + `Execute(name, args)`
- [ ] 2.5 **RED tests**: table-driven tests for shell command rejection (`rm -rf /`, `$(curl)`), file path traversal (`../../etc/passwd`), git `-C /etc` blocked
- [ ] 2.6 Wire module registration in `main.go` + write unit tests for each module's `Execute` path

## Phase 3: Tests + CI

- [ ] 3.1 Integration test: mock LLM + real ToolRegistry + iteration budget — verify tool_call loop halts at max iterations
- [ ] 3.2 Integration test: TUI → Brain → mock LLM — scripted input/output with `teatest`
- [ ] 3.3 Create `.golangci.yml` — baseline linters (govet, staticcheck, errcheck, ineffassign)
- [ ] 3.4 Create `Makefile` — targets: `test`, `test-race`, `lint`, `build`, `clean`
- [ ] 3.5 Integration test: secret redaction — tool output scanned for configured patterns, redacted before LLM feedback
