# Design: Milestone 2 — Subagent System

## Technical Approach

Introduce a `Subagent` interface in a new `internal/agent/` package, with a Spawner that creates isolated execution contexts (fresh message history + filtered tool registry + scoped system prompt). The Brain (orchestrator) gains a delegation step before the LLM call: when SDD trigger fires, it selects and runs subagents sequentially, synthesizing their Summaries. Per-subagent memory uses Engram topic-key namespaces via a thin wrapper. The learning loop hooks post-execute via a counter-based nudge mechanism.

## Architecture Decisions

| Decision | Options | Choice | Rationale |
|----------|---------|--------|-----------|
| Subagent location | `internal/core/` vs `internal/agent/` | `internal/agent/` | Keeps core lean; subagents are a higher-level abstraction that depends on core ports |
| Context isolation | Shared history vs fresh context | Fresh context per spawn | Prevents cross-contamination; each subagent sees only its task + relevant artifacts |
| Tool filtering | Self-reported vs Spawner-enforced | Spawner-enforced filter | Security — subagent cannot bypass its own restrictions |
| Memory namespace | Flat Engram keys vs hierarchical topic keys | `gaia/{subagent}/{project}` topic keys | Leverages Engram's existing topic_key upsert; no new infrastructure |
| SDD trigger | LLM-classified vs heuristic | Keyword heuristic + `/direct` override | Avoids extra LLM call for classification; user can always bypass |
| Learning nudge | Every task vs counter-based (N=5) | Counter-based with configurable N | Reduces noise; only nudges after pattern repetition |

## Data Flow

```
User Message
     │
     ▼
┌─────────────┐    trigger?    ┌──────────────┐
│  Brain       │──────────────▶│ SDD Pipeline  │
│ (orchestrator)│              │               │
│              │◀──────────────│  Explorer     │
│              │   Summary     │  Proposer     │
│              │◀──────────────│  Specifier    │
│              │   Summary     │  Implementer  │
│              │◀──────────────│  Verifier     │
└─────────────┘              └──────────────┘
     │                              │
     ▼                              ▼
  UI Display              Engram (per-subagent
                          namespace + shared)
```

Each subagent: `Spawner.Spawn(name, task)` → fresh `Brain` instance with filtered `ToolRegistry` → `Execute()` → `Summary` returned to orchestrator.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/agent/subagent.go` | Create | `Subagent` interface, `Task`, `Summary` types |
| `internal/agent/spawner.go` | Create | Spawner: creates isolated Brain instances with filtered tools |
| `internal/agent/registry.go` | Create | Subagent registry: name → Subagent lookup |
| `internal/agent/sdd/explorer.go` | Create | Explorer subagent (read-only tools) |
| `internal/agent/sdd/proposer.go` | Create | Proposer subagent (read + Engram) |
| `internal/agent/sdd/specifier.go` | Create | Specifier subagent (read + Engram) |
| `internal/agent/sdd/implementer.go` | Create | Implementer subagent (write + shell) |
| `internal/agent/sdd/verifier.go` | Create | Verifier subagent (shell + read) |
| `internal/agent/memory/namespace.go` | Create | Engram namespace wrapper: scoped search/save |
| `internal/agent/learn/loop.go` | Create | Learning loop: counter, nudge, session summary |
| `internal/core/kernel.go` | Modify | Add delegation step + SDD trigger detection |
| `internal/core/ports/ports.go` | Modify | Add `SubagentPort` interface |
| `internal/core/registry.go` | Modify | Add `FilteredRegistry()` for tool subset creation |

## Interfaces / Contracts

```go
// internal/agent/subagent.go
type Subagent interface {
    Name() string
    Description() string
    Execute(ctx context.Context, task Task) (Summary, error)
}

type Task struct {
    ID          string
    Description string
    Context     []domain.Message  // relevant artifacts/history
    Constraints []string          // tool restrictions, rules
}

type Summary struct {
    Status      string            // "success" | "failure" | "partial"
    Artifacts   []string          // produced file paths
    Observations []string         // key findings for synthesis
    TokenUsage  TokenUsage
}

// internal/core/ports/ports.go (addition)
type SubagentPort interface {
    Spawn(ctx context.Context, name string, task Task) (Summary, error)
    Available() []string
}
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Subagent interface contract | Table-driven tests: each subagent returns valid Summary |
| Unit | Spawner isolation | Verify fresh context, tool filter enforcement |
| Unit | Memory namespace | Verify topic key format, search scoping |
| Unit | Learning counter | Verify nudge fires at N=5, resets correctly |
| Integration | Pipeline flow | Mock LLM → Explorer→Proposer→Specifier chain with artifact passing |
| Integration | SDD trigger | Keyword heuristic test cases: substantial vs trivial |
| E2E | Full pipeline | Real LLM (or mock) through all 5 stages with file output verification |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary at this layer. Subagents use existing tool modules (shell, file) which already have their own security boundaries.

## Migration / Rollout

No migration required. The subagent system is purely additive. The Brain's delegation step is opt-in via SDD trigger — existing direct LLM flow remains unchanged when trigger does not fire. Feature flag not needed; the `/direct` command provides runtime override.

## Open Questions

- [ ] Should the learning nudge threshold (N=5) be configurable per-subagent or global?
- [ ] Should shared knowledge graph (`gaia/shared/`) be writable by the orchestrator only, or completely read-only for all?
