# Architecture

GAIA follows a **hexagonal (ports & adapters) architecture** written in Go. The core is framework-agnostic, with all integrations behind interface boundaries.

---

## Package Layout

```
gaia/
├── cmd/gaia/                 # CLI entry points
│   ├── main.go               # TUI mode (default)
│   ├── exec.go               # Headless mode
│   ├── review.go             # Review CLI
│   ├── cron.go               # Cron CLI
│   ├── skills.go             # Skills CLI
│   ├── gateway.go            # Gateway CLI
│   ├── doctor.go             # Doctor CLI
│   ├── desktop.go            # Desktop mode (Wails)
│   ├── webhook.go            # Webhook CLI
│   ├── lsp.go                # LSP CLI
│   ├── plugins.go            # Plugin CLI
│   └── onboard.go            # Onboard CLI
├── internal/
│   ├── agent/                # Subagent system
│   │   ├── agent.go          # Subagent interface
│   │   ├── spawner.go        # Isolated Brain spawner
│   │   ├── registry.go       # Subagent factory registry
│   │   ├── redact.go         # Secret redaction
│   │   ├── sdd/              # 8 SDD subagents
│   │   ├── ops/              # 4 non-SDD subagents
│   │   ├── memory/           # Engram namespace wrapper
│   │   └── learn/            # Learning loop
│   ├── core/                 # Core domain
│   │   ├── domain/           # Domain models
│   │   ├── ports/            # Interface definitions
│   │   ├── kernel.go         # Brain (agent loop)
│   │   ├── guard.go          # ConfirmGuard
│   │   ├── registry.go       # ToolRegistry
│   │   └── sdd_trigger.go    # SDD trigger detection
│   ├── modules/              # Tool modules
│   │   ├── shell/            # Shell execution (local/Docker/SSH)
│   │   ├── fileops/          # File operations
│   │   ├── gitops/           # Git operations
│   │   └── security/         # Path validation, URL safety, redaction
│   ├── review/               # BR review engine
│   │   ├── engine.go         # Review engine
│   │   ├── state.go          # State machine (13 states)
│   │   ├── risk.go           # Risk taxonomy (8 codes)
│   │   ├── lens.go           # 4 review lenses
│   │   ├── snapshot.go       # SHA256 snapshot hashing
│   │   ├── gates/            # Pre-commit/pre-push/pre-PR gates
│   │   ├── judgment/         # Judgment Day protocol
│   │   └── agentsmd/         # AGENTS.md parser
│   ├── skills/               # Skills Hub
│   │   ├── skills.go         # SkillMeta parser
│   │   ├── hub.go            # Skill registry
│   │   ├── downloader.go     # Skill downloader
│   │   └── tap.go            # Community skill taps
│   ├── cron/                 # Cron scheduler
│   │   ├── cron.go           # Cron parser
│   │   ├── scheduler.go      # Job scheduler
│   │   └── delivery.go       # Delivery targets
│   ├── mcp/                  # MCP client
│   │   ├── client.go         # JSON-RPC stdio client
│   │   ├── types.go          # MCP types
│   │   └── module.go         # MCP tool module
│   ├── gateway/              # Messaging gateway
│   │   ├── gateway.go        # Multi-platform dispatcher
│   │   ├── telegram_adapter.go
│   │   └── discord_mcp_adapter.go
│   ├── webhook/              # Webhook listener
│   │   └── listener.go       # HTTP + HMAC-SHA256
│   ├── scripts/              # Script injection
│   │   └── runner.go         # Pre-processing script runner
│   ├── lsp/                  # LSP client
│   │   ├── client.go         # LSP stdio client
│   │   ├── diagnostics.go    # Diagnostic parser
│   │   └── module.go         # LSP tool module
│   ├── plugins/              # Plugin system
│   │   └── manager.go        # Plugin loader
│   ├── browser/              # Browser MCP plugin
│   │   └── browser.go        # Browser module
│   ├── doctor/               # System diagnostics
│   │   └── checks.go         # Health checks
│   └── config/               # Configuration
│       └── config.go         # Config loader/saver
├── internal/adapters/        # Adapters
│   ├── llm/                  # LLM providers
│   │   ├── llm.go            # Provider registry
│   │   ├── router.go         # Provider router
│   │   ├── openai.go         # OpenAI adapter
│   │   ├── anthropic.go      # Anthropic adapter
│   │   ├── ollama.go         # Ollama adapter
│   │   └── copilot_client.go # GitHub Copilot adapter
│   ├── tui/                  # Bubbletea TUI
│   │   ├── tui.go            # TUI model
│   │   ├── wizard.go         # Setup wizard
│   │   └── null.go           # NullUI (headless)
│   ├── desktop/              # Wails Desktop adapter
│   │   ├── desktop.go        # DesktopUI
│   │   └── bindings.go       # Wails bindings
│   ├── db/                   # SQLite persistence
│   │   ├── sqlite.go         # DB init + migrations
│   │   └── cron.go           # Cron job store
│   ├── output/               # Output formatter
│   │   └── formatter.go      # JSON/text formatter
│   └── telegram/             # Telegram bot (unwired, used via gateway)
├── openspec/                 # SDD artifacts
│   ├── config.yaml
│   ├── specs/
│   └── changes/
│       └── archive/
├── skills/                   # Bundled Go skills
├── docs/                     # Documentation
├── SPEC.md                   # Full specification
└── Makefile
```

---

## The Agent Loop (Brain)

`internal/core/kernel.go` — The Brain is the heart of GAIA:

```
User Message
    │
    ▼
ProcessMessage(ctx, content)
    │
    ├── Check SDD Trigger (is this a substantial change?)
    │     ├── Yes → Run SDD pipeline (subagent system)
    │     └── No  → Continue with direct response
    │
    ├── Knowledge Graph Recall (query relevant facts)
    ├── Skill Index (select active skills)
    ├── Provider Router (select LLM by config)
    ├── LLM Call (Chat or Stream)
    ├── Handle Tool Calls (if any)
    │     ├── ConfirmGuard (check trust mode)
    │     ├── ToolRegistry.Execute (dispatch to module)
    │     └── RedactSecrets (scan output)
    ├── Iteration Budget (consume, check cap)
    ├── Save to Session (persist message)
    └── Display to UI (TUI streaming or headless output)
```

---

## Subagent System

`internal/agent/` — Each subagent is an autonomous LLM-powered agent:

```
Orchestrator
    │
    ├── Spawner.RunLoop(ctx, systemPrompt, tools)
    │     │
    │     ├── Creates isolated Brain (separate LLM calls)
    │     ├── Filters tools (subagent only sees its allowed tools)
    │     ├── Injects Engram namespace
    │     ├── Runs iteration budget
    │     └── Returns SubagentResult
    │
    └── Registry (maps name → factory function)
          ├── "explorer" → ExplorerFactory
          ├── "designer" → DesignerFactory
          └── ...
```

---

## Review State Machine

`internal/review/state.go` — 13 states with 21 valid transitions:

```
unreviewed
    │
    ▼
reviewing
    │
    ├── judges_confirmed  (Judgment Day)
    │
    ▼
findings_frozen
    │
    ▼
evidence_classified
    │
    ├── fix_required → fixing → fix_validating  (max 1/2 rounds)
    │
    ▼
ready_final_verification
    │
    ▼
final_verifying
    │
    ├── approved
    ├── escalated
    └── invalidated
```

---

## LLM Provider Router

`internal/adapters/llm/router.go` — Selects provider by config, falls back on failure:

```
Router.Chat(ctx, messages)
    Router.Stream(ctx, messages)
    Router.Tools()
    │
    ├── Try primary provider (from config)
    │     ├── Success → return response
    │     └── Fail → log error
    │
    └── Try fallback provider (next in chain)
          ├── Success → return response
          └── Fail → return error
```

Each provider adapter implements:
- `Chat(ctx, messages) → *Message, error`
- `Stream(ctx, messages) → (<-chan TokenChunk, error)`
- `Tools() → []ToolDef`

---

## Security Architecture

Security is applied at multiple layers:

```
Layer 1: Path Security (security module)
  ├── Resolve symlinks
  ├── Block traversal outside project root
  └── Allowlist for dangerous operations

Layer 2: Tool Security (tool guard)
  ├── Shell command allowlist
  ├── URL validation
  └── Write approval

Layer 3: Confirmation (ConfirmGuard)
  ├── always → ask before every dangerous operation
  ├── per-session → ask once, trust for the session
  ├── per-action → ask for each action
  └── never → YOLO mode (CI/automation)

Layer 4: Redaction (redact.go)
  ├── API keys: sk-*, ghp_*, github_pat_*
  ├── PEM: -----BEGIN *-----
  ├── JWT: eyJ.*\.
  ├── Bearer tokens
  └── AWS: AKIA*

Layer 5: Skill Security
  ├── Skill provenance tracking (official hub, community, user)
  ├── AST audit before loading
  └── Restricted execution scope
```

