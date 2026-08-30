# Architecture

GAIA follows a **hexagonal (ports & adapters) architecture** written in Go. The core is framework-agnostic, with all integrations behind interface boundaries.

---

## Package Layout

```text
gaia/
├── cmd/gaia/                 # CLI entry points (main, exec, review, cron, gateway, etc.)
├── internal/
│   ├── agent/                # Subagent system (spawner, registry, sdd, ops, learn)
│   ├── core/                 # Core domain (domain, ports, kernel, policyguard, registry)
│   ├── modules/              # Tool modules (shell, fileops, gitops, security)
│   ├── review/               # BR review engine (engine, states, lenses, CAS store)
│   ├── skills/               # Skills Hub (registry, downloader, tap, AST audit)
│   ├── cron/                 # Cron scheduler & delivery targets
│   ├── mcp/                  # MCP client (JSON-RPC stdio & SSE)
│   ├── gateway/              # Multi-platform messaging gateway (Telegram, Discord, Slack)
│   ├── webhook/              # Webhook listener (HTTP + HMAC-SHA256)
│   ├── lsp/                  # LSP client & diagnostics parser
│   ├── plugins/              # Plugin loader and manager
│   ├── browser/              # Headless browser MCP plugin
│   ├── doctor/               # System diagnostics and self-healing
│   └── config/               # YAML configuration loader & saver
├── internal/adapters/        # Infrastructure Adapters
│   ├── llm/                  # 19 LLM providers router & credential pool
│   ├── tui/                  # Cyberpunk Bubbletea TUI
│   ├── desktop/              # Wails Desktop UI
│   ├── db/                   # SQLite persistence & migrations
│   └── output/               # JSON/text output formatters
├── docs/                     # 18 official documentation guides
└── Makefile
```

---

## The Agent Loop (Brain)

`internal/core/kernel.go` — The Brain is the heart of GAIA:

```mermaid
flowchart TD
    IN["User Message / Prompt"] --> PROC["ProcessMessage(ctx, content)"]
    
    PROC --> SDD{"Is Substantial Change?"}
    SDD -- "Yes" --> SDD_RUN["Delegate to SDD Pipeline (@subagents)"]
    SDD -- "No" --> DIRECT["Direct Brain Execution"]
    
    DIRECT --> KG["Knowledge Graph Recall (Query Facts)"]
    KG --> SKILL["Skill Registry (Select Active Skills)"]
    SKILL --> ROUTE["Provider Router (Select LLM & Pool)"]
    ROUTE --> LLM["LLM Call (Stream or Chat)"]
    
    LLM --> TOOLS{"Tool Calls Requested?"}
    TOOLS -- "Yes" --> GUARD["PolicyGuard & ConfirmGuard (Permission Check)"]
    GUARD --> EXEC["ToolRegistry.Execute(module)"]
    EXEC --> REDACT["RedactSecrets (Mask Output)"]
    REDACT --> LLM
    
    TOOLS -- "No" --> BUD["Consume Iteration Budget"]
    BUD --> SAVE["Save Turn to SQLite Session Store"]
    SAVE --> OUT["Stream Output to TUI / Web / Gateway"]
```

---

## Subagent System

`internal/agent/` — Each subagent is an autonomous LLM-powered worker with its own memory namespace:

```mermaid
flowchart TD
    ORCH["🧠 GAIA Orchestrator"] --> SPAWN["Spawner.RunLoop(ctx, systemPrompt, tools)"]
    
    SPAWN --> BRAIN["1. Spawns Isolated Brain instance"]
    SPAWN --> TOOLS["2. Filters tool allowlist (Scope Confinement)"]
    SPAWN --> MEM["3. Injects dedicated SQLite Memory Namespace"]
    SPAWN --> BUDGET["4. Executes bounded iteration & token budget"]
    
    BUDGET --> RES["5. Returns Terminal SubagentResult"]
    RES --> ORCH
```

---

## Review State Machine

`internal/review/state.go` — 13 states with content-bound SHA256 receipts:

```mermaid
flowchart TD
    UNREV["unreviewed"] --> REV["reviewing\n(Start review)"]
    REV --> JD{"Judgment Day?"}
    JD -- "Yes" --> CONF["judges_confirmed\n(Dual Judges)"]
    JD -- "No" --> FROZEN["findings_frozen"]
    CONF --> FROZEN
    
    FROZEN --> CLASS["evidence_classified\n(Classify Severity)"]
    CLASS --> CHK{"Blockers Found?"}
    CHK -- "Yes" --> FIX["fix_required ➔ fixing ➔ fix_validating"]
    FIX --> READY["ready_final_verification"]
    CHK -- "No" --> READY
    
    READY --> FINAL["final_verifying\n(Tests + Build check)"]
    FINAL --> APP["approved\n(SHA256 CAS Receipt)"]
    FINAL --> ESC["escalated\n(Human decision)"]
    FINAL --> INV["invalidated\n(Code changed)"]
```

---

## LLM Provider Router & Failover

`internal/adapters/llm/router.go` — Manages 19 providers with automatic failover and rate-limit cooldown:

```mermaid
flowchart TD
    REQ["Router.Chat / Stream(ctx, messages)"] --> P1["Try Primary Provider (e.g. Anthropic)"]
    P1 -- "Success" --> OK["Return Stream / Response"]
    P1 -- "Rate Limit / 429" --> COOL["Set Cooldown & Try Fallback (e.g. DeepSeek / OpenAI)"]
    COOL -- "Success" --> OK
    COOL -- "Fail" --> P3["Try Local Provider (e.g. Ollama)"]
    P3 -- "Success" --> OK
    P3 -- "All Failed" --> ERR["Return Structured Provider Error"]
```

Each provider adapter implements:
- `Chat(ctx, messages) → (*Message, error)`
- `Stream(ctx, messages) → (<-chan TokenChunk, error)`
- `Tools() → []ToolDef`

---

## Persistence & Storage

```mermaid
graph TD
    subgraph Storage ["SQLite Storage Engine (~/.gaia/gaia.db)"]
        S1["Sessions & Conversation History"]
        S2["Knowledge Graph Facts & FTS5 Index"]
        S3["Cron Scheduled Jobs & History"]
        S4["Skills Metadata & Trust Registry"]
        S5["CAS Review Receipts & Verification Evidence"]
    end
```
