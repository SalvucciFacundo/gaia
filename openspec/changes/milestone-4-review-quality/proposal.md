# Proposal: Milestone 4 — Review & Quality

## Intent

Milestones 1-3 delivered the core agent loop, 12 subagents (8 SDD + 4 ops), skills hub, and headless mode. Milestone 4 implements the formal review system that GAIA's quality guarantees depend on: GGA 4-lens review with content-bound receipts, pre-commit/pre-push/pre-PR gate validation, Judgment Day adversarial review for high-risk changes, AGENTS.md team standards parsing, and a `gaia review` CLI for orchestrating the review lifecycle.

## Scope

### In Scope
- **gga-review**: Full GGA review engine — 4 lenses (risk, resilience, readability, reliability), risk taxonomy classification, lens selection by risk level, bounded receipt generation with SHA256 snapshot
- **review-gates**: Pre-commit, pre-push, pre-PR gate hooks that validate review receipts against current content; git hook installation
- **judgment-day**: Blind dual review protocol — two independent judges (judge-a, judge-b), comparison, fix-agent with surgical corrections, max 2 fix rounds
- **agents-md**: AGENTS.md standards parser — YAML frontmatter + markdown rules for team coding standards, injected into review prompts
- **gaia-review-cli**: `gaia review start/status/validate/list` commands for review lifecycle management

### Out of Scope
- Review UI in TUI (display only — no interactive review editing)
- Cross-repository review (single project scope)
- Custom lens creation (fixed 4 lenses)
- Review scheduling / cron-based reviews
- Integration with external review tools (GitHub PR comments, etc.)

## Capabilities

### New Capabilities
- `gga-review`: Review engine in `internal/review/` — `Engine` struct with `Start()`, `RunLenses()`, `GenerateReceipt()`. Risk taxonomy: 8 risk codes (`configuration_change`, `executable_change`, `executable_mode`, `hot_path`, `large_change`, `non_executable_only`, `service_token`, `shell_source`). Risk level determination: only `non_executable_only` → Low (no lens); any other → Medium (one dominant lens); `hot_path`/`large_change`/`service_token`/`shell_source` → High (all 4 lenses). Lens implementations: `LensRisk`, `LensResilience`, `LensReadability`, `LensReliability`. Each returns findings classified as BLOCKER / WARNING / SUGGESTION.
- `review-receipt`: Content-bound receipt with SHA256 — `Receipt` struct with `lineage_id`, `snapshot_hash`, `selected_lenses`, `risk_level`, `correction_budget`, `correction_used`, `state`, `final_verification_hash`. Receipt stored in Engram under `gaia/review/{change-name}/{transaction-id}`.
- `review-state-machine`: Formal state machine — `unreviewed → reviewing → findings_frozen → evidence_classified → ready_final_verification → final_verifying → approved`. Branches: `fix_required → fixing → fix_validating`, `escalated`, `invalidated`. Each transition recorded in mutation journal.
- `review-gates`: Gate validators in `internal/review/gates/` — `PreCommitGate`, `PrePushGate`, `PrePRGate`. Each re-validates receipt against current content hash. Git hook installer writes `.git/hooks/pre-commit` and `.git/hooks/pre-push` scripts that call `gaia review validate`.
- `judgment-day`: Adversarial review in `internal/review/judgment/` — `JudgmentDay` orchestrator spawns judge-a and judge-b as independent reviewer instances with isolated contexts. Comparison phase merges findings. Fix-agent applies corrections. Max 2 rounds.
- `agents-md`: Parser in `internal/review/agentsmd/` — parses AGENTS.md files with YAML frontmatter (rules, conventions, forbidden patterns) and markdown body. Returns `Standards` struct injected into review prompts.
- `gaia-review-cli`: `gaia review` subcommand family — `start` (begin review of staged/committed changes), `status` (show current review state), `validate` (check receipt against current content), `list` (list all reviews with state).

### Modified Capabilities
- `reviewer-subagent`: Existing `internal/agent/ops/reviewer.go` upgraded from stub to full GGA engine integration. Uses `internal/review/Engine` instead of prompt-only review. Receipt generation is now programmatic (not LLM-generated text).
- `sdd-archive`: Archive phase gains receipt validation gate — blocks archive if receipt is missing, pending, invalidated, or scope-changed.

## Approach

Three chained PRs (stacked-to-main), each under 400 lines:

1. **review-engine (~380 LOC)**: Core review engine in `internal/review/`. `Engine` struct with risk classification, lens selection, 4 lens implementations, receipt generation, state machine. Upgrade `internal/agent/ops/reviewer.go` to use the engine. `internal/review/agentsmd/` parser for AGENTS.md standards. Unit tests for risk taxonomy, lens selection, receipt SHA256, state transitions, AGENTS.md parsing.

2. **gates-cli (~370 LOC)**: Gate validators in `internal/review/gates/` — content hash validation, git hook installer. `gaia review` CLI in `cmd/gaia/review.go` — start, status, validate, list subcommands. Wire receipt storage to Engram namespace. Integration tests for gate validation (receipt matches/stale content).

3. **judgment-day (~350 LOC)**: `internal/review/judgment/` — JudgmentDay orchestrator, blind judge spawning (two independent reviewer instances with different seeds), comparison/merge logic, fix-agent with surgical correction application, 2-round limit enforcement. Integration test: mock two judges with conflicting findings → fix → re-judgment.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/` | New | Core review engine: risk taxonomy, lens selection, 4 lenses, receipt, state machine |
| `internal/review/gates/` | New | Gate validators: pre-commit, pre-push, pre-PR; git hook installer |
| `internal/review/judgment/` | New | Judgment Day: blind dual review, comparison, fix-agent, round limiting |
| `internal/review/agentsmd/` | New | AGENTS.md parser: YAML frontmatter + markdown rules → Standards struct |
| `internal/agent/ops/reviewer.go` | Modified | Upgrade from prompt-only stub to engine-backed review with programmatic receipt |
| `cmd/gaia/review.go` | New | `gaia review` CLI: start, status, validate, list |
| `cmd/gaia/main.go` | Modified | Register `review` subcommand in CLI dispatch |
| `internal/agent/sdd/archiver.go` | Modified | Add receipt validation gate before archive |
| `internal/core/domain/models.go` | Modified | Add `ReviewReceipt`, `ReviewState`, `ReviewFinding` types |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| SHA256 snapshot mismatch on Windows (line endings) | High | Normalize to LF before hashing; document in receipt metadata |
| Git hook installation fails on non-git repos | Medium | `gaia review start` detects git repo; hooks are optional, gate validation works without hooks |
| Judgment Day LLM cost (2 judges + fix agent = 3 LLM calls) | Medium | Only triggered for high-risk changes; correction_budget limits fix scope |
| AGENTS.md format fragmentation (no standard) | Low | Support YAML frontmatter + markdown body (same as SKILL.md pattern); document format in openspec |
| State machine complexity (8 states, multiple transitions) | Medium | Formal enum + transition table; unit test every valid/invalid transition |
| Receipt storage in Engram exceeds observation size limits | Low | Receipts are small JSON (~500 bytes); well within Engram limits |

## Rollback Plan

Each PR is independently revertable:
- PR1 (review-engine): New package `internal/review/` with no consumers except upgraded reviewer.go. Revert reviewer.go to stub, delete `internal/review/`.
- PR2 (gates-cli): New CLI subcommand + gate package. Revert main.go CLI dispatch, delete `cmd/gaia/review.go` and `internal/review/gates/`.
- PR3 (judgment-day): New package `internal/review/judgment/` with no callers until explicitly invoked. Delete package.

## Dependencies

- Milestone 3 complete (subagent infrastructure, skills hub, headless mode)
- Existing `internal/agent/ops/reviewer.go` stub (upgraded, not replaced)
- Existing `internal/modules/security/` for path validation in snapshot hashing
- Engram memory for receipt persistence (via `internal/agent/memory/` namespace manager)
- Git repository for gate hooks (optional — gates work without hooks via CLI)

## Success Criteria

- [ ] Risk taxonomy classifies all 8 risk codes correctly (unit tested)
- [ ] Lens selection matches risk level: Low → none, Medium → 1 dominant, High → all 4
- [ ] Receipt SHA256 matches reviewed content; mismatch invalidates receipt
- [ ] State machine enforces valid transitions only (unit tested)
- [ ] Pre-commit gate blocks commit when receipt is stale or missing
- [ ] `gaia review start/status/validate/list` all functional
- [ ] Judgment Day: two judges produce independent findings; comparison merges them; fix-agent corrects; max 2 rounds enforced
- [ ] AGENTS.md parser extracts rules from YAML frontmatter + markdown body
- [ ] Upgraded reviewer.go produces programmatic receipts (not LLM text)
- [ ] `go test ./...` passes, `go build ./cmd/gaia` succeeds

## Review Workload Forecast
- **Decision needed before apply**: No
- **Chained PRs recommended**: Yes (3 stacked PRs)
- **400-line budget risk**: Medium
- **Chain strategy**: stacked-to-main — PR1 targets main, PR2 targets PR1 branch, PR3 targets PR2 branch
