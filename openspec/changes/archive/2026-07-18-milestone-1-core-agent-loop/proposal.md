# Proposal: Milestone 1 — Core Agent Loop

## Intent

GAIA compiles but cannot act: Brain receives no messages, LLM is nil, modules are empty, and 20 skill files have no loader. Milestone 1 closes these gaps so a user can send a prompt through the TUI, have it routed to a real LLM, and execute tool calls end-to-end.

## Scope

### In Scope

- **Phase 1a — Provider Router**: multi-provider LLM abstraction, OpenAI/Anthropic/Ollama adapters, Copilot preserved as one provider, config-driven selection with fallback chain.
- **Phase 1b — Wire the Loop**: TUI ↔ Brain ↔ LLM connected, streaming display, confirmation modes (`always`/`per-session`/`per-action`/`never` + `/trust` commands), per-subagent iteration budget.
- **Phase 1c — Tool Engine**: `Module` interface implemented, shell/file/git tools, tool registry in Brain, path validation + command approval + secret redaction.

### Out of Scope

- Engram integration (Milestone 2)
- Skill runtime loader (Milestone 2)
- Telegram adapter wiring (Milestone 2)
- Subagent orchestration / multi-agent (Milestone 3)
- MCP client support (deferred)

## Capabilities

### New Capabilities

- `multi-provider-llm` — Provider router with streaming interface and per-provider adapters (OpenAI, Anthropic, Ollama, Copilot).
- `agent-loop` — Connected Brain ↔ TUI ↔ LLM message loop with streaming display.
- `tool-engine` — Module registry, tool dispatch, and built-in tools (shell, file, git).
- `confirmation-system` — Per-session trust levels with `/trust` slash commands.
- `iteration-budget` — Safety cap on LLM ↔ tool iterations per subagent turn.

### Modified Capabilities

None (no existing specs to modify).

## Approach

Split into 3 phases, each under 400 lines for review budget (`auto-chain` delivery):

| Phase | Focus | Key Files |
|-------|-------|-----------|
| 1a | Provider Router | `internal/core/ports/ports.go`, `internal/adapters/llm/{openai,anthropic,ollama,router}.go` |
| 1b | Wire the Loop | `internal/core/kernel.go`, `internal/adapters/tui/tui.go`, `cmd/gaia/main.go` |
| 1c | Tool Engine | `internal/core/domain/models.go`, `internal/modules/{shell,file,git}/`, `internal/core/registry.go` |

Each phase ships with `_test.go` files and a `.golangci.yml` baseline.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/core/ports/ports.go` | Modified | Extend `LLMProvider` with streaming + multi-model support |
| `internal/adapters/llm/` | New | 4 adapter files + router |
| `internal/core/kernel.go` | Modified | Accept real LLM, dispatch tool calls, iteration budget |
| `internal/adapters/tui/tui.go` | Modified | Wire `ProcessMessage`, render streaming tokens |
| `internal/modules/` | New | Shell, file, git tool implementations |
| `cmd/gaia/main.go` | Modified | Wire adapters → Brain → TUI |
| `.golangci.yml` | New | Linter baseline |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Streaming API differences across providers | Med | Normalize via `io.Reader`-based token stream in port |
| Shell tool security (command injection) | High | Path validation, allowlist, confirmation gates |
| Iteration budget too low for complex tasks | Low | Configurable cap, default 25, log when hit |
| Copilot adapter regression during refactor | Med | Keep existing `CopilotClient` tests passing, adapter wraps it |

## Rollback Plan

Each phase is an independent PR. Revert the PR to roll back. No schema migrations or persistent state changes in Milestone 1.

## Dependencies

- `github.com/sashabaranov/go-openai` — OpenAI adapter
- `github.com/anthropics/anthropic-sdk-go` — Anthropic adapter
- Ollama — REST API, no SDK needed
- `.golangci.yml` — golangci-lint v1.60+

## Success Criteria

- [ ] User sends prompt in TUI → LLM responds with streamed tokens
- [ ] LLM tool calls execute (shell, file read/write, git status) and results feed back
- [ ] Provider switchable via `config.yaml` without code changes
- [ ] Iteration budget halts runaway loops with clear user message
- [ ] Confirmation modes gate destructive tools correctly
- [ ] `go test ./...` passes, `golangci-lint run` clean, `go build ./cmd/gaia` succeeds
