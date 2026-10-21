# Architectural Boundaries: `internal/core`

## 1. Core Purpose and Philosophy
`internal/core` is the **neutral, profile-agnostic engine** of GAIA. It orchestrates the fundamental execution loop:
1. Receiving structured turns.
2. Interacting with LLM providers via abstractions.
3. Executing tools through bounded and policy-guarded dispatchers.
4. Enforcing budget, context limits, and security policies.

The core **MUST NOT** possess intrinsic knowledge of specific user tasks, specialized domain methodologies (such as software engineering pipelines, artistic workflows, or general assistant behaviors), or UI presentation concerns.

---

## 2. Allowed Contents (What Belongs Here)

| Component | Responsibility | Current Files |
| :--- | :--- | :--- |
| **Domain Models** | Pure business entities and value objects | `domain/*` |
| **Ports** | Consumer-driven interfaces for external adapters | `ports/*` |
| **Engine Orchestrator** | Agent lifecycle, turn streaming, iteration budget | `kernel.go`, `tool_dispatcher.go`, `registry.go` |
| **Session Engine** | Session synchronization, isolation, and routing | `session.go` |
| **Security & Policies** | Tool safety classification, permission enforcement | `guard.go`, `policy.go`, `policy_categories.go`, `policy_store.go` |
| **Budget & Limits** | Context compaction, token accounting, cost metrics | `cost.go`, `usage.go`, `brain_compaction.go` |
| **Security Sanitization** | Redacting secrets from tool outputs before model ingestion | `redact.go` |

---

## 3. Boundary Violations (What Does NOT Belong Here)

The following components violate the core's single responsibility and should be migrated out:

### A. Slash Command Handlers (UI / Presentation Layer) — [COMPLETED]
- **Files**: `command_dispatcher.go`, `session_commands.go`, `config_commands.go`, `workflow_commands.go`, `integration_commands.go`, `info_commands.go`, `media_commands.go`, `skill_commands.go`.
- **Resolution**: Migrated to `internal/commands/` using `ports.CommandDispatcher`. Core has zero command presentation dependencies.

### B. ODD and SDD Engineering Pipelines (Strategy / Agent Layer) — [COMPLETED]
- **Files**: `odd_router.go`, `sdd_trigger.go`, `sdd_pipeline.go`.
- **Resolution**: Migrated flat into `internal/agent/` (`odd_router.go`, `sdd_trigger.go`, `sdd_pipeline.go`, `workflow_router.go`) via `ports.WorkflowRouter`. Decoupled from neutral kernel.

### C. Compiler Auto-Repair Checks (Tool / Sandbox Layer) — [COMPLETED]
- **Files**: `autofix/*`
- **Resolution**: Migrated flat to `internal/agent/autofix.go` and `internal/agent/autofix_test.go`. Decoupled from `tool_dispatcher.go` via `ports.MutationVerifier` inbound port.

### D. Knowledge Graph Sync & MoA Fanout — [COMPLETED]
- **Files**: `kg_sync.go`, `moa.go`.
- **Resolution**: `moa.go` unified in `internal/agent/moa.go` via `ports.MoARunner`. `kg_sync.go` migrated to `internal/agent/memory/kg_sync.go` via `ports.KnowledgeRecorder`. Direct persistence decoupled from `internal/core`.

---

## 4. Architectural Rules for `internal/core`
1. **Zero External Transport/UI Coupling**: Core never references HTTP, WebSockets, TUI, or Telegram/Discord models.
2. **Strict Inward Dependencies**: Core imports only `gaia/internal/core/*` and standard libraries. It MUST NEVER import `gaia/internal/agent`, `gaia/internal/adapters`, or `gaia/internal/profiles`.
3. **Thread Safety**: All mutable state exposed to concurrent callers must be protected by explicit synchronization primitives.
4. **Context Respect**: Every blocking call must accept and propagate `context.Context` and abort cleanly on `ctx.Done()`.
