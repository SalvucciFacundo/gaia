# Proposal: SDD v2 & Upstream Alignment for Permanent Auto-Learning Subagents

## Intent

GAIA was initially built around Gentle AI v1.x concepts (lineal SDD phases, basic 4-lens review, and local memory). Since then, upstream Gentle AI has evolved into v2.x with hardened execution guards (Attempt Ledgers, Strict TDD state machines, Review v2 transition contracts, Store policies, and Path-based skill injection).

However, **Gentle AI uses ephemeral subagents (temporary scripts/tasks created and destroyed per run)**, whereas **GAIA uses permanent, specialized subagents with persistent domain memory and self-learning loops**.

This change adapts the best architectural innovations of Gentle AI v2 to GAIA's permanent subagent architecture, ensuring subagents do not just follow hard constraints, but actively record failure patterns, learn from budget overruns, and reinforce test-driven discipline across sessions.

## Scope

### In Scope
- **attempt-ledger**: Transactional attempt and line-budget manager (`Acquire`/`Settle`) that feeds failure and success reflections into the permanent subagent's learning loop (`internal/agent/learn` and `internal/agent/memory`).
- **strict-tdd**: Enforcement engine for the RED -> GREEN -> TRIANGULATE -> REFACTOR cycle between `Implementer` and `Verifier`, recording recurring test anti-patterns in domain memory.
- **store-policy**: Multi-backend artifact store resolution (`engram`, `openspec`, `hybrid`, `none`) with memory namespace routing.
- **chained-prs**: Review Workload Guard (400-line budget limit) and task slicing (`stacked-to-main`, `feature-branch-chain`).
- **review-transitions-v2**: Review state machine with typed transitions (`execute`, `collect`, `stop`), opt-in review switch (`enable|disable|status`), and mathematical correction budget `min(200, ceil(lines/2))`.
- **skill-path-injection**: Progressive skill loader that injects file paths (`## Skills to load before work`) rather than full summaries into subagent prompts.

### Out of Scope
- Direct copy-pasting of Gentle AI's ephemeral CLI binaries without permanent memory hooks.
- Cloud-hosted external review oracles (all reviews remain local to GAIA).
- Non-Go test runner integrations for Strict TDD (stdlib `go test` and generic CLI runners first).

## Capabilities

### New Capabilities
- `attempt-ledger`: Bounded retry and line-budget management with learning feedback.
- `strict-tdd`: Formal TDD state machine enforced across Implementer and Verifier subagents.
- `store-policy`: Configurable artifact store resolution preventing blind file or memory queries.
- `chained-prs`: Task planning and slicing guard to prevent human reviewer fatigue.
- `review-transitions-v2`: Typed transition-based review execution with opt-in switch.
- `skill-path-injection`: Lightweight path-based skill reference protocol for subagent prompts.

### Modified Capabilities
- `implementer`: Updated to participate in Strict TDD (RED/GREEN/REFACTOR) and Attempt Ledger budget checks.
- `verifier`: Updated to validate test failure in RED phase before confirming passing tests in GREEN phase.
- `planner`: Updated to slice tasks exceeding the 400-line review budget.
- `reviewer`: Updated to support v2 typed transition contracts and correction budget constraints.

## Approach

### Delivery Strategy: 3 Slices (Stacked to Main)

| Slice | Capabilities | Rationale |
|-------|-------------|-----------|
| Slice 1 | `attempt-ledger`, `strict-tdd` | Core execution guards and TDD learning cycle for Implementer/Verifier |
| Slice 2 | `store-policy`, `skill-path-injection`, `chained-prs` | Planning, memory routing, and context optimization |
| Slice 3 | `review-transitions-v2` | Review engine alignment with v2 transition contracts and opt-in switch |

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/sdd/` | Modified & New | Attempt ledger, TDD engine, store policy, prompt updates |
| `internal/agent/learn/` | Modified | Integration of attempt failure/success insights into learning loop |
| `internal/agent/memory/` | Modified | Store routing between SQLite/Engram and filesystem OpenSpec |
| `internal/skills/` | Modified | Path-based skill resolution and injection |
| `internal/review/` | Modified | v2 transition state machine and review mode switch |
| `tracker/` | Modified | Upstream manifest tracking port status |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Strict TDD halts build if tests cannot be written first | Medium | Provide explicit override when modifying non-testable configuration/docs |
| Attempt Ledger blocks legitimate complex tasks | Low | Configurable `MaxAttempts` and `MaxChangedLines` per task with explicit reset |
| Memory bloat from recording failure insights | Low | Store concise one-line insights; auto-compact stale reflections |

## Rollback Plan

- Attempt Ledger and Strict TDD are gated via `openspec/config.yaml` (`rules.apply.tdd: true|false` and `rules.apply.budget_guard: true|false`).
- If disabled, subagents fall back to standard direct execution without breaking existing flows.

## Success Criteria

1. Attempt Ledger prevents infinite subagent retry loops and writes failure observations to subagent domain memory.
2. Strict TDD rejects code implementation if a failing unit test was not written and verified first.
3. Subagents receive skill file paths instead of bulky prompt summaries, reducing context usage.
4. Review engine supports opt-in switch and enforces correction line limits.
5. All tests in `go test ./...` pass with 100% clean build.
