# Tasks: Milestone 4 — Review & Quality

## Review Workload Forecast
- **Decision needed before apply**: No
- **Chained PRs recommended**: Yes (3 stacked PRs, ~380 LOC each)
- **400-line budget risk**: Medium
- **Chain strategy**: stacked-to-main — PR1 targets main, PR2 targets PR1 branch, PR3 targets PR2 branch

---

## PR 1: Review Engine — CORE

### 1.1 Domain Types
- [x] Update `internal/core/domain/models.go`
  - [x] Add `ReviewState` type (string enum with 13 states)
  - [x] Add `ReviewReceipt` struct (schema, lineage_id, snapshot_hash, selected_lenses, risk_level, risk_reasons, correction_budget, correction_used, state, final_verification_hash, findings, timestamps)
  - [x] Add `ReviewFinding` struct (lens, severity, file, line, message, suggestion)

### 1.2 Review State Machine
- [x] Create `internal/review/state.go`
  - [x] State constants: unreviewed, reviewing, judges_confirmed, findings_frozen, evidence_classified, fix_required, fixing, fix_validating, ready_final_verification, final_verifying, approved, escalated, invalidated
  - [x] `validTransitions` map: from → set of valid to states
  - [x] `Transition(from, to State) error` — returns error for invalid transitions
  - [x] `IsTerminal(s State) bool` — approved, escalated, invalidated are terminal

### 1.3 Risk Taxonomy
- [x] Create `internal/review/risk.go`
  - [x] Risk code constants: configuration_change, executable_change, executable_mode, hot_path, large_change, non_executable_only, service_token, shell_source
  - [x] `ClassifyRisk(diff string, files []string) []RiskCode` — returns list of risk codes detected
  - [x] `DetermineRiskLevel(codes []RiskCode) string` — returns "low", "medium", or "high"
  - [x] `SelectLenses(riskLevel string, files []string) []string` — returns lens names based on risk level and file types
  - [x] High-risk codes: hot_path, large_change, service_token, shell_source → all 4 lenses
  - [x] Medium: select dominant lens by file type (config→risk, test→reliability, doc→readability, service→resilience)
  - [x] Low (only non_executable_only): no lenses, auto-approve

### 1.4 Snapshot & Hashing
- [x] Create `internal/review/snapshot.go`
  - [x] `SnapshotFiles(projectRoot string, files []string) ([]FileSnapshot, error)` — reads files, normalizes CRLF→LF, computes per-file SHA256
  - [x] `ComputeSnapshotHash(snapshots []FileSnapshot) string` — sorts by path, concatenates `path\ncontent\n`, SHA256 the stream
  - [x] `FileSnapshot` struct: Path, Content, Hash
  - [x] Uses `internal/modules/security.ValidatePath()` for path safety

### 1.5 Lens Interface & Implementations
- [x] Create `internal/review/lens.go`
  - [x] `Lens` interface: `Name() string`, `Analyze(ctx, []FileSnapshot) ([]Finding, error)`
  - [x] `LensRisk` — security, permissions, data exposure, architecture analysis
  - [x] `LensResilience` — error handling, fallbacks, resource cleanup, observability
  - [x] `LensReadability` — naming, structure, documentation, maintainability
  - [x] `LensReliability` — test coverage, determinism, edge cases, contract adherence
  - [x] Each lens: prompt-based analysis via LLM (uses Spawner.RunLoop pattern)
  - [x] Findings classified as BLOCKER / WARNING / SUGGESTION

### 1.6 Review Engine
- [x] Create `internal/review/engine.go`
  - [x] `Engine` struct with projectRoot, standards (from AGENTS.md)
  - [x] `NewEngine(projectRoot string) *Engine`
  - [x] `Start(files []string) (*Transaction, error)` — creates transaction, snapshots files, sets state to "reviewing"
  - [x] `RunLenses(ctx, tx, lenses) ([]Finding, error)` — runs selected lenses, collects findings
  - [x] `GenerateReceipt(tx, findings) (*Receipt, error)` — computes lineage_id, sets state to "approved", returns receipt
  - [x] `Transaction` struct: ID, ChangeName, State, SnapshotHash, Files, Receipt
  - [x] `ClassifyRisk(diff string) ([]RiskCode, string)` and `SelectLenses(riskLevel string) []string` methods

### 1.7 AGENTS.md Parser
- [x] Create `internal/review/agentsmd/parser.go`
  - [x] `Standards` struct: Rules []Rule, Forbidden []string, Conventions []string, Prose string
  - [x] `Rule` struct: ID, Pattern (regex), Severity, Message
  - [x] `Parse(path string) (*Standards, error)` — reads AGENTS.md, splits YAML frontmatter (---) from markdown body
  - [x] `InjectIntoPrompt(prompt string) string` — appends standards as review context
  - [x] `FindAndParse(projectRoot string) (*Standards, error)` — walks up parent directories

### 1.8 Upgrade Reviewer Subagent
- [x] Modify `internal/agent/ops/reviewer.go`
  - [x] Import `internal/review` package
  - [x] Execute(): create Engine, call Start(), ClassifyRisk(), SelectLenses(), RunLenses(), GenerateReceipt()
  - [x] Keep read-only tool filter (file_read, file_list, git_status, git_log, git_diff)
  - [x] Receipt is now programmatic (not LLM-generated text)
  - [x] LLM used only inside lens Analyze() calls via spawnerLLM adapter
  - [x] Result includes receipt as artifact

### 1.9 Tests
- [x] Create `internal/review/engine_test.go`
  - [x] TestRiskClassification: table-driven for all 8 risk codes
  - [x] TestRiskLevelDetermination: low/medium/high from reason combinations
  - [x] TestLensSelection: Low→0, Medium→1, High→4
  - [x] TestSnapshotHash: known content → known hash; CRLF normalization
  - [x] TestStateTransitions: all valid transitions succeed; invalid return error
  - [x] TestIsTerminal: approved/escalated/invalidated are terminal
- [x] Create `internal/review/agentsmd/parser_test.go`
  - [x] TestParseValid: YAML frontmatter + markdown body
  - [x] TestParseMissingFrontmatter: all prose, no rules
  - [x] TestParseEmpty: empty file
  - [x] TestInjectIntoPrompt: standards appended to prompt string

---

## PR 2: Gates + CLI

### 2.1 Gate Validators
- [x] Create `internal/review/gates/gates.go`
  - [x] `Gate` struct: Name string
  - [x] `GateResult` struct: Passed bool, Receipt *Receipt, Reason string
  - [x] `Validate(projectRoot string) (*GateResult, error)` — loads latest receipt from Engram, re-hashes current files, compares hashes
  - [x] Pre-defined gates: PreCommitGate, PrePushGate, PrePRGate
  - [x] Validation logic: receipt exists → state is "approved" → snapshot_hash matches current content → pass

### 2.2 Git Hook Installer
- [x] Create `internal/review/gates/hooks.go`
  - [x] `WriteHooks(projectRoot string) error` — creates .git/hooks/pre-commit and .git/hooks/pre-push
  - [x] Pre-commit hook: `#!/bin/sh\ngaia review validate --gate pre-commit`
  - [x] Pre-push hook: `#!/bin/sh\ngaia review validate --gate pre-push`
  - [x] Detects if .git/hooks/ exists; returns error if not a git repo
  - [x] Appends to existing hooks (does not overwrite)

### 2.3 Review CLI
- [x] Create `cmd/gaia/review.go`
  - [x] `handleReviewCLI(args []string)` — dispatches to subcommands
  - [x] `review start`: creates Engine, snapshots staged files (git diff --cached), runs review, outputs receipt
  - [x] `review status`: loads latest receipt from Engram, displays state + findings summary
  - [x] `review validate`: runs gate validation, exits 0 (pass) or 1 (fail) — used by git hooks
  - [x] `review list`: lists all reviews with state, date, risk level
  - [x] `review install-hooks`: calls gates.WriteHooks()
  - [x] Flags: `--files <glob>`, `--lens <name>`, `--judgment-day`, `--gate <name>`, `--state <state>`

### 2.4 Main CLI Integration
- [x] Modify `cmd/gaia/main.go`
  - [x] Add `"review"` case to CLI dispatch switch (alongside "skills" and "exec")
  - [x] Call `handleReviewCLI(os.Args[2:])`

### 2.5 Archiver Gate Integration
- [x] Modify `internal/agent/sdd/archiver.go`
  - [x] Before archive: check for valid receipt in Engram
  - [x] Missing/pending/invalidated/escalated receipt → block archive with error message
  - [x] Approved receipt → proceed with archive

### 2.6 Tests
- [x] Create `internal/review/gates/gates_test.go`
  - [x] TestGateValidationPass: receipt hash matches current content
  - [x] TestGateValidationFail: content changed since receipt
  - [x] TestGateNoReceipt: no receipt found → fail
  - [x] TestHookGeneration: generated script contains correct gate name
  - [x] TestHookAppend: existing hook content preserved
- [x] Extend `cmd/gaia/review_test.go` (if applicable) or integration tests
  - [x] TestReviewCLIStart: mock engine → verify receipt output
  - [x] TestReviewCLIValidate: mock gate → verify exit code

---

## PR 3: Judgment Day

### 3.1 Judgment Day Orchestrator
- [x] Create `internal/review/judgment/judgment.go`
  - [x] `JudgmentDay` struct: spawner, maxRounds (default 2)
  - [x] `NewJudgmentDay(spawner *agent.Spawner) *JudgmentDay`
  - [x] `Run(ctx, tx) (*JudgmentResult, error)` — orchestrates: judge-a → judge-b → compare → fix → re-judge (max 2 rounds)
  - [x] `JudgmentResult` struct: JudgeAFindings, JudgeBFindings, MergedFindings, Rounds, Approved

### 3.2 Blind Judge Spawning
- [x] Extend `internal/review/judgment/judgment.go`
  - [x] `runJudge(ctx, tx, judgeName, focus string) ([]Finding, error)` — spawns reviewer subagent with isolated context
  - [x] Judge-a system prompt: focus on security, data flow, permissions
  - [x] Judge-b system prompt: focus on error handling, edge cases, resource cleanup
  - [x] Judges cannot see each other's findings (separate Spawner invocations)

### 3.3 Comparison & Merge
- [x] Create `internal/review/judgment/compare.go`
  - [x] `CompareFindings(a, b []Finding) ([]Finding, error)` — merges findings from both judges
  - [x] Deduplication: same file+line+severity → merge messages
  - [x] Conflict resolution: same file+line, different severity → take higher severity
  - [x] Unique findings: kept as-is
  - [x] LLM-assisted comparison for ambiguous cases (optional, falls back to deterministic)

### 3.4 Fix Agent
- [x] Create `internal/review/judgment/fix.go`
  - [x] `ApplyFixes(ctx, tx, findings []Finding, budget int) (*FixResult, error)`
  - [x] Fix agent: reviewer subagent with write access (file_write) — applies surgical corrections
  - [x] Correction budget: max 85 tokens of changes (configurable)
  - [x] Only fixes BLOCKER and WARNING findings; SUGGESTION skipped
  - [x] Returns list of applied fixes with before/after content

### 3.5 Round Limiting
- [x] Extend `internal/review/judgment/judgment.go`
  - [x] After fix: re-run judges on fixed content
  - [x] If both judges approve → state = "approved", receipt issued
  - [x] If round < maxRounds (2): another fix cycle
  - [x] If round == maxRounds and not approved → state = "escalated" (human intervention)
  - [x] Each round recorded in mutation journal

### 3.6 CLI Integration
- [x] Modify `cmd/gaia/review.go`
  - [x] `review start --judgment-day` flag triggers JudgmentDay instead of normal review
  - [x] `review status` shows Judgment Day round count and judge findings

### 3.7 Tests
- [x] Create `internal/review/judgment/judgment_test.go`
  - [x] TestBlindReview: two judges produce independent findings (mock LLM)
  - [x] TestCompareFindings: dedup, conflict resolution, unique findings
  - [x] TestFixAgent: applies BLOCKER fixes, skips SUGGESTION
  - [x] TestMaxRounds: 2 rounds enforced; 3rd attempt returns escalated
  - [x] TestJudgmentApproval: both judges agree after fix → approved
  - [x] TestJudgmentEscalation: judges disagree after 2 rounds → escalated
