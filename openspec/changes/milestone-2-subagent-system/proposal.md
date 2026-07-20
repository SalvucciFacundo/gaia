# Proposal: Milestone 2 — Subagent System

## Intent

GAIA's core differentiator is specialized subagents that learn independently per domain. Milestone 1 delivered the single-agent loop; Milestone 2 introduces the subagent infrastructure, five SDD subagents (Explorer, Proposer, Specifier, Implementer, Verifier), per-subagent memory namespaces, independent learning loops, and automatic SDD flow triggering.

## Scope

### In Scope
- Subagent interface, spawner, and orchestrator delegation logic
- Five SDD subagents with domain-specific tool restrictions
- Per-subagent Engram memory namespaces (`gaia/{subagent}/{project}`)
- Independent learning loop per subagent (nudge, session summary, skill creation)
- Shared knowledge graph cross-pollination between subagents
- Orchestrator hook: detect substantial change → trigger SDD flow
- Complete message redaction in tool output (finish Phase 2 work)

### Out of Scope
- Non-SDD subagents (Reviewer, Learner, Researcher, Debugger, Archiver) — Milestone 3
- Persona system and evolution — Milestone 4
- Desktop UI (Wails) — Milestone 5
- Skills Hub (decentralized install) — Milestone 5

## Capabilities

### New Capabilities
- `subagent-base`: Subagent interface (`Name`, `Description`, `Execute`), spawner with isolated context, return envelope, orchestrator delegation (select by task, send context, receive summary)
- `sdd-subagents`: Explorer (read-only codebase investigation), Proposer (change proposals), Specifier (specs with scenarios), Implementer (code from tasks+specs), Verifier (test execution and spec compliance)
- `per-subagent-memory`: Engram topic-key namespaces per subagent (`gaia/{subagent}/{project}`), isolated save/search per domain
- `per-subagent-learning`: Independent learning loop — post-task nudge, session summary generation, domain skill creation/improvement
- `automatic-sdd-trigger`: Orchestrator detects substantial change intent and routes through Explorer→Proposer→Specifier→Implementer→Verifier pipeline

### Modified Capabilities
- `agent-loop`: Orchestrator gains delegation step — when SDD trigger fires, delegate to subagents instead of direct LLM call
- `tool-engine`: Subagents receive filtered tool sets (Explorer: read-only; Implementer: write+shell; Verifier: shell+read)

## Approach

Three chained PRs (stacked-to-main), each under 400 lines:

1. **subagent-base (~380 LOC)**: `internal/agent/` package — `Subagent` interface, `Spawner`, `Registry`, `Summary` envelope. Orchestrator delegation in `internal/core/kernel.go`. Tests for spawn/delegate lifecycle.
2. **sdd-subagents (~380 LOC)**: Five subagent implementations in `internal/agent/sdd/`. Each wires domain-specific tool filters and system prompts. Integration tests with mock LLM.
3. **learning-memory (~380 LOC)**: Engram namespace wrapper in `internal/agent/memory/`. Learning loop hook in `internal/agent/learn/`. SDD trigger heuristic in orchestrator. Message redaction completion in tool output pipeline.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/` | New | Subagent interface, spawner, registry, summary types |
| `internal/agent/sdd/` | New | Five SDD subagent implementations |
| `internal/agent/memory/` | New | Per-subagent Engram namespace wrapper |
| `internal/agent/learn/` | New | Learning loop per subagent |
| `internal/core/kernel.go` | Modified | Add delegation step, SDD trigger detection |
| `internal/core/ports/` | Modified | Add `SubagentPort` interface |
| `internal/adapters/tools/` | Modified | Tool filtering per subagent, message redaction |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Subagent context leak (seeing other subagent work) | Medium | Enforced isolation in Spawner — each gets fresh context |
| SDD trigger false positives (trivial changes entering full pipeline) | Medium | Heuristic thresholds with user override (`/direct` to bypass) |
| Learning loop noise (low-value skill creation) | Low | Nudge threshold — only trigger after N similar observations |
| Tool filter bypass | Low | Filter enforced at Spawner level, not self-reported |

## Rollback Plan

Each PR is independently revertable. The subagent system is additive — no existing behavior changes until the orchestrator delegation hook lands (PR 1). Reverting any PR returns to Milestone 1 single-agent behavior with no data loss.

## Dependencies

- Milestone 1 complete (agent loop, tool engine, multi-provider LLM, security, confirm guard)
- Engram MCP available for memory namespaces
- No external service dependencies

## Success Criteria

- [ ] Subagent interface implemented with `Name()`, `Description()`, `Execute(ctx, task) → Summary`
- [ ] All five SDD subagents spawn, execute, and return structured summaries
- [ ] Per-subagent Engram namespaces isolate memory correctly
- [ ] Learning loop fires post-task and produces session summaries
- [ ] Orchestrator detects substantial change and routes to SDD pipeline
- [ ] Tool filtering prevents Explorer from writing files
- [ ] All new code covered by tests (unit + integration with mock LLM)
- [ ] `go test ./...` passes, `go build ./cmd/gaia` succeeds
