# Design: Milestone 6 — Ecosystem

## Technical Approach

Milestone 6 extends GAIA's hexagonal architecture with 8 ecosystem capabilities across 3 stacked PRs. The core strategy is **port-based isolation**: each capability defines a port interface in `internal/core/ports/`, with adapters in dedicated packages. The Brain remains unchanged; new capabilities register as modules (tools), delivery targets, or independent listeners.

Key architectural patterns carried forward:
- **Module pattern** (`ports.Module`): browser-tools, lsp-integration, and execute-code-rpc register as modules
- **Adapter pattern** (like `adapters/telegram/`): messaging-gateway wraps platform adapters behind a unified port
- **Delivery pattern** (like `cron/delivery.go`): gateway becomes a new delivery target
- **Hub pattern** (like `skills/hub.go`): taps extend the existing hub with remote sources

## Architecture Decisions

### Decision: Gateway Adapter Interface

**Choice**: Define `GatewayAdapter` port with `Start()`, `Stop()`, `Send()`, `OnMessage()`. Each platform (Telegram, Discord-via-MCP) implements this interface. A `Gateway` multiplexer owns all adapters and routes messages to Brain.

**Alternatives considered**: (1) Separate processes per platform, (2) Channel-based fan-in.
**Rationale**: Single-process multiplexer matches the existing Telegram adapter pattern. Channel fan-in adds complexity without benefit since message volume is low.

### Decision: Plugin Loading via Go Plugin Package

**Choice**: Reject Go's `plugin` package. Use **subprocess + JSON-RPC** for plugins. Plugins are standalone binaries that speak JSON-RPC over stdio (same as MCP). Plugin manifest (`plugin.json`) declares tools, subagents, and capabilities.

**Alternatives considered**: (1) Go `plugin` package, (2) WASM plugins, (3) Lua scripting.
**Rationale**: Go `plugin` has ABI stability issues and no unload. WASM adds runtime dependency. Subprocess JSON-RPC matches the existing MCP pattern, is language-agnostic, and provides process isolation.

### Decision: Execute Code RPC Server

**Choice**: JSON-RPC 2.0 server over TCP (localhost only). Exposes `tools/list` and `tools/call` methods wrapping the existing `ToolRegistry`. Scripts connect, discover tools, and invoke them. Server binds to `127.0.0.1:0` (random port) and writes the port to a temp file for script discovery.

**Alternatives considered**: (1) Unix socket, (2) HTTP REST, (3) gRPC.
**Rationale**: TCP JSON-RPC matches MCP's protocol style, is cross-platform (Windows compatible), and localhost-only avoids network exposure. Random port prevents conflicts.

### Decision: LSP Client Architecture

**Choice**: LSP client in `internal/lsp/` using stdio transport (like MCP client). Connects to configured LSP servers, exposes diagnostics/completions/hover as tools via `ports.Module`. One module per LSP server.

**Alternatives considered**: (1) Single LSP multiplexer, (2) HTTP-based LSP.
**Rationale**: Stdio matches MCP client pattern. Per-server modules allow independent lifecycle.

### Decision: Community Skills Taps

**Choice**: Taps are GitHub repos with a `skills/` directory structure matching the bundled format. `Hub.AddTap(url)` clones the repo to `~/.gaia/taps/<hash>/` and indexes skills with `source: "tap"`. Tap skills are read-only and have lower precedence than user-installed.

**Alternatives considered**: (1) Registry API, (2) Git submodule.
**Rationale**: GitHub taps match Homebrew's model. Clone-based discovery is simple, versioned, and works offline after initial fetch.

### Decision: Webhook Listener

**Choice**: HTTP listener in `internal/webhook/` using `net/http`. Routes GitHub webhook events to registered handlers. HMAC-SHA256 verification using configured secret. Handlers registered via config: `webhook.subscriptions[].events`.

**Alternatives considered**: (1) Framework (chi/gin), (2) gRPC streaming.
**Rationale**: `net/http` is stdlib, sufficient for webhook parsing. No need for full framework overhead.

### Decision: Script Injection

**Choice**: Pre-agent scripts defined in config as `scripts.pre_run: ["path/to/script.sh"]`. Script stdout is captured and injected as a system message before the user message. `[SILENT]` prefix in stdout suppresses notification. Scripts run with the project's working directory. Timeout-bounded (configurable, default 30s).

**Alternatives considered**: (1) Lua/V8 embedding, (2) stdin piping.
**Rationale**: Shell scripts are the simplest cross-language mechanism. Stdout-as-context is a proven pattern (like shell profile scripts).

## Data Flow

```
                    ┌─────────────────────────────────────────┐
                    │              GAIA Process                │
                    │                                          │
 Telegram ──┐       │  ┌──────────┐    ┌──────────────┐       │
            ├──→ Gateway ──→ Brain ──→ ToolRegistry   │       │
 Discord ───┘   (adapter)   │       │  ├─ shell       │       │
 (via MCP)      │           │       │  ├─ fileops     │       │
                │           │       │  ├─ mcp_browser │       │
                │           │       │  ├─ lsp_gopls   │       │
                │           │       │  └─ plugin_*    │       │
                │           │       └──────────────┘       │
                │           │                               │
                │    ┌──────┴──────┐                        │
                │    │ Delivery    │                        │
                │    │ ├─ terminal │                        │
                │    │ ├─ telegram │                        │
                │    │ └─ discord  │                        │
                │    └─────────────┘                        │
                    │                                        │
                    │  ┌──────────┐    ┌──────────────┐      │
                    │  │ Webhook  │──→ │ Automation   │      │
                    │  │ Listener │    │ Handlers     │      │
                    │  └──────────┘    └──────────────┘      │
                    │                                        │
                    │  ┌──────────┐    ┌──────────────┐      │
                    │  │ RPC      │←── │ Python/Go    │      │
                    │  │ Server   │──→ │ Scripts      │      │
                    │  └──────────┘    └──────────────┘      │
                    │                                        │
                    │  ┌──────────┐                          │
                    │  │ Scripts  │──→ stdout → system msg   │
                    │  │ (pre-run)│                          │
                    │  └──────────┘                          │
                    └─────────────────────────────────────────┘
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/gateway/gateway.go` | Create | Gateway multiplexer + adapter interface |
| `internal/gateway/telegram_adapter.go` | Create | Telegram adapter implementing GatewayAdapter |
| `internal/gateway/discord_mcp_adapter.go` | Create | Discord via MCP bridge adapter |
| `internal/plugins/loader.go` | Create | Plugin discovery, manifest parsing, lifecycle |
| `internal/plugins/manifest.go` | Create | Plugin manifest schema (plugin.json) |
| `internal/plugins/rpc_client.go` | Create | JSON-RPC client for plugin subprocess |
| `internal/rpc/server.go` | Create | JSON-RPC server exposing ToolRegistry |
| `internal/rpc/handler.go` | Create | tools/list and tools/call handlers |
| `internal/lsp/client.go` | Create | LSP client (stdio transport) |
| `internal/lsp/module.go` | Create | LSP module implementing ports.Module |
| `internal/lsp/diagnostics.go` | Create | Diagnostic parsing and formatting |
| `internal/webhook/listener.go` | Create | HTTP webhook listener |
| `internal/webhook/github.go` | Create | GitHub event parsing + HMAC verification |
| `internal/webhook/handler.go` | Create | Webhook subscription routing |
| `internal/scripts/runner.go` | Create | Pre-agent script execution |
| `internal/scripts/context.go` | Create | Stdout capture + system message injection |
| `internal/skills/tap.go` | Create | Tap management (add, remove, refresh, list) |
| `internal/skills/hub.go` | Modify | Add tap scanning to buildIndex() |
| `internal/adapters/telegram/telegram.go` | Modify | Implement GatewayAdapter interface |
| `internal/cron/delivery.go` | Modify | Add gateway delivery targets |
| `internal/core/ports/ports.go` | Modify | Add GatewayAdapter, Plugin, WebhookHandler ports |
| `internal/core/domain/models.go` | Modify | Add PluginManifest, WebhookSubscription, TapConfig, ScriptConfig |
| `cmd/gaia/gateway.go` | Create | `gaia gateway start` command |
| `cmd/gaia/rpc.go` | Create | `gaia rpc start` command |
| `cmd/gaia/webhook.go` | Create | `gaia webhook start` command |
| `cmd/gaia/plugins.go` | Create | `gaia plugins list/install/remove` commands |
| `cmd/gaia/skills.go` | Modify | Add `gaia skills add-tap/remove-tap` commands |

## Interfaces / Contracts

```go
// ports.go additions

// GatewayAdapter is the port for a messaging platform adapter.
type GatewayAdapter interface {
    Name() string
    Start(ctx context.Context, handler MessageHandler) error
    Stop() error
    Send(ctx context.Context, target string, content string) error
}

// MessageHandler processes an incoming gateway message and returns a response.
type MessageHandler func(ctx context.Context, msg IncomingMessage) (string, error)

// IncomingMessage is a normalized message from any gateway adapter.
type IncomingMessage struct {
    Platform  string // "telegram", "discord"
    SenderID  string
    SenderName string
    Content   string
    ChatID    string
    ThreadID  string // optional
}

// Plugin defines the contract for a third-party plugin.
type Plugin interface {
    Manifest() PluginManifest
    Start(ctx context.Context) error
    Stop() error
    Tools() []domain.ToolCall
    Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error)
}

// WebhookHandler processes a verified webhook event.
type WebhookHandler interface {
    Handle(ctx context.Context, event WebhookEvent) error
    Events() []string // subscribed event types
}
```

```go
// domain/models.go additions

type PluginManifest struct {
    Name        string   `json:"name"`
    Version     string   `json:"version"`
    Description string   `json:"description"`
    Command     string   `json:"command"`     // binary path
    Args        []string `json:"args"`
    Tools       []string `json:"tools"`       // tool names provided
}

type WebhookSubscription struct {
    ID       string   `json:"id"`
    Name     string   `json:"name"`
    Secret   string   `json:"secret"`    // HMAC secret
    Events   []string `json:"events"`    // ["push", "pull_request"]
    Action   string   `json:"action"`    // task description for Brain
    Enabled  bool     `json:"enabled"`
}

type TapConfig struct {
    URL    string `yaml:"url"`
    Name   string `yaml:"name"`
    Branch string `yaml:"branch"` // default: "main"
}

type ScriptConfig struct {
    PreRun  []string `yaml:"pre_run"`  // script paths
    Timeout int      `yaml:"timeout"`  // seconds, default 30
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Gateway adapter message normalization | Table-driven tests with mock handlers |
| Unit | Plugin manifest parsing + validation | Table-driven with valid/invalid manifests |
| Unit | RPC server tool dispatch | Mock ToolRegistry; verify JSON-RPC framing |
| Unit | LSP diagnostic parsing | Parse sample LSP JSON responses |
| Unit | Webhook HMAC verification | Known test vectors from GitHub docs |
| Unit | Script stdout capture + [SILENT] parsing | Table-driven with various stdout patterns |
| Unit | Tap URL validation + path sanitization | Table-driven with valid/invalid URLs |
| Integration | Gateway end-to-end (Telegram adapter + Brain) | Use telebot test mode or mock HTTP |
| Integration | RPC server + client round-trip | Start server, connect client, call tool |
| Integration | Webhook listener + GitHub event parsing | httptest.Server with signed payloads |
| Integration | Script injection → system message | Run script, verify message in repo |
| E2E | `gaia gateway start` receives Telegram message | Manual or CI with test bot token |

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | Script paths that look like docs (e.g., `README.md`) | Applicable | Script runner validates executable bit + extension whitelist (.sh, .py, .go, .bat) | Test: `.md` file rejected; non-executable rejected |
| Git repository selection | Tap URLs with path traversal (`../`) or non-GitHub hosts | Applicable | Tap URL validation: must be `github.com`, no `..` in resolved path | Test: `../etc/passwd` rejected; non-GitHub URL rejected |
| Commit state | N/A | N/A — no git commit operations in this milestone | — | — |
| Push state | N/A | N/A — no git push operations | — | — |
| PR commands | N/A | N/A — no PR automation in this milestone | — | — |

## Migration / Rollout

No migration required. All capabilities are opt-in via config. Existing installations are unaffected.

## Open Questions

- [ ] Discord MCP bridge: which existing MCP server implementation to recommend? (evaluate `@modelcontextprotocol/server-discord` vs community alternatives)
- [ ] Plugin binary distribution: should `gaia plugins install` support downloading from GitHub releases, or filesystem-only for M6?
- [ ] LSP auto-detect: should GAIA auto-detect `gopls`/`pylsp` in PATH, or require explicit config?
