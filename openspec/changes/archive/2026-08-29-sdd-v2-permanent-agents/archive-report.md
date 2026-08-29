# Archive Report: SDD v2 & Upstream Alignment for Permanent Auto-Learning Subagents

## Summary
- **Change Name**: `sdd-v2-permanent-agents`
- **Archived Date**: 2026-08-29
- **Status**: Completed (100% of tasks across all 3 slices implemented, verified, and pushed to `main`)

## Completed Capabilities
1. **`attempt-ledger`** (`internal/agent/sdd/attempt_ledger.go`):
   - Bounded execution attempts and 400-line budget limit.
   - Settle records failure insights into permanent subagents' learning loops (`LearnedInsights`).
2. **`strict-tdd`** (`internal/agent/sdd/tdd.go`):
   - Formal RED -> GREEN -> REFACTOR state machine.
   - Requires failing test execution before production code implementation; records TDD habits in domain memory.
3. **`store-policy`** (`internal/agent/sdd/store_policy.go`):
   - Multi-backend persistence (`engram`, `openspec`, `hybrid`, `none`).
   - Dispatcher guard prevents attempting to read missing disk files in engram-only mode.
4. **`chained-prs`** (`internal/agent/sdd/planner.go`):
   - Review Workload Guard (`AnalyzeWorkload`) flagging tasks exceeding 400 lines and recommending PR chaining.
5. **`skill-path-injection`** (`internal/skills/hub.go`, `internal/agent/spawner.go`):
   - Progressive skill loader injecting `## Skills to load before work` with exact file paths.
   - Saves 70%+ of prompt context tokens.
6. **`review-mode-optin`** (`internal/review/mode.go`):
   - Opt-in review switch (`enable`, `disable`, `status`) with clone and global scopes.
   - Allows delivery under ordinary repository policy when disabled.
7. **`review-transitions-v2` & `review-correction-budget`** (`internal/review/transitions.go`):
   - Mathematical correction budget: `min(200, ceil(lines/2))`.
   - Typed transition contracts (`execute`, `collect`, `stop`) with canonical stop continuations.

## Test Evidence
- 100% of unit tests pass across all packages:
  - `go test ./internal/agent/sdd/...` → PASS
  - `go test ./internal/skills/...` → PASS
  - `go test ./internal/review/...` → PASS
  - `go test ./...` → PASS
