# Tasks: SDD v2 & Upstream Alignment for Permanent Auto-Learning Subagents

## Review Workload Forecast

- **Estimated changed lines**: ~320 lines (across 3 core implementation slices)
- **400-line budget risk**: Low
- **Chained PRs recommended**: No (Under 400 lines; can be implemented in coordinated slices)
- **Decision needed before apply**: No

---

## Slice 1: Attempt Ledger & Strict TDD Engine

- [x] 1.1 Complete `AttemptLedger` unit tests in `internal/agent/sdd/attempt_ledger_test.go`
  - [x] Test `Acquire` happy path and token generation
  - [x] Test retry limit block when `MaxAttempts` exceeded
  - [x] Test line budget violation on `Settle`
  - [x] Test failure insight propagation to permanent subagent memory
- [x] 1.2 Implement `TDDEngine` in `internal/agent/sdd/tdd.go` and `internal/agent/sdd/tdd_test.go`
  - [x] Define `TDDPhase` enum (Red, Green, Refactor, Complete)
  - [x] Enforce failing test requirement in `TDDPhaseRed`
  - [x] Enforce passing implementation in `TDDPhaseGreen`
  - [x] Add subagent learning hooks for TDD discipline
- [x] 1.3 Wire Attempt Ledger and TDD into `Implementer` and `Verifier` subagents
  - [x] Update `implementer.go` prompt with TDD phase awareness
  - [x] Update `verifier.go` to distinguish between RED assertion failures and unexpected errors

---

## Slice 2: Store Policy, Chained PRs & Path-Based Skills

- [x] 2.1 Implement `StorePolicy` in `internal/agent/sdd/store_policy.go`
  - [x] Support `engram`, `openspec`, `hybrid`, and `none` modes
  - [x] Add dispatcher guards to prevent missing-file errors when in engram-only mode
- [x] 2.2 Update `Planner` in `internal/agent/sdd/planner.go` with Review Workload Guard
  - [x] Calculate estimated lines and emit `Review Workload Forecast`
  - [x] Recommend chained PR boundaries for tasks exceeding 400 lines
- [x] 2.3 Implement Path-Based Skill Ingestion in `internal/skills/` & `internal/agent/spawner.go`
  - [x] Add `## Skills to load before work` resolver injecting relative paths
  - [x] Ensure subagents load full skills on demand

---

## Slice 3: Review Transitions v2 & Opt-In Mode

- [ ] 3.1 Implement Review Mode Switch in `internal/review/mode.go`
  - [ ] Add `enable`, `disable`, and `status` controls with clone/global scopes
  - [ ] Bypass review gates when disabled under ordinary repo policy
- [ ] 3.2 Update Review Engine with v2 typed transition contracts
  - [ ] Implement `min(200, ceil(lines/2))` mathematical correction budget
  - [ ] Restrict to at most 1 ordinary correction round
- [ ] 3.3 Update `tracker/manifest.yaml` and verify all tests pass
  - [ ] Update ported features in tracker manifest
  - [ ] Run full test suite: `go test ./...`
