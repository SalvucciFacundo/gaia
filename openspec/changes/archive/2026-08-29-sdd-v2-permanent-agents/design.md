# Technical Design: SDD v2 & Upstream Alignment for Permanent Auto-Learning Subagents

## Architecture Overview

This design aligns GAIA with the core execution safety mechanisms of Gentle AI v2 (Attempt Ledgers, Strict TDD, Review v2 transitions, Store Policies, and Path-based Skill Injection) while adapting each concept to GAIA's **permanent subagents that maintain domain memory and auto-learn**.

```
+-------------------------------------------------------------------------------+
¦                             GAIA SDD v2 Engine                                ¦
¦                                                                               ¦
¦   +-----------------------------------------------------------------------+   ¦
¦   ¦                     Attempt Ledger (Budget Guard)                     ¦   ¦
¦   ¦   - Tracks attempts & line ceiling per work unit                      ¦   ¦
¦   ¦   - Settle() feeds failure/success into Subagent Domain Memory        ¦   ¦
¦   +-----------------------------------T-----------------------------------+   ¦
¦                                       ¦                                       ¦
¦        +------------------------------+------------------------------+        ¦
¦        ¦                                                             ¦        ¦
¦   +----v-----------------------------+   +---------------------------v----+   ¦
¦   ¦     Strict TDD State Machine     ¦   ¦     Store & Memory Router      ¦   ¦
¦   ¦   - RED: Failing test required   ¦   ¦   - Multi-backend: engram,     ¦   ¦
¦   ¦   - GREEN: Passing impl required ¦   ¦     openspec, hybrid, none     ¦   ¦
¦   ¦   - REFACTOR: Code cleanup       ¦   ¦   - Domain Topic Keys          ¦   ¦
¦   +----T------------------------T----+   +--------------------------------+   ¦
¦        ¦                        ¦                                             ¦
¦   +----v---------------+   +----v---------------+                             ¦
¦   ¦ Implementer (Perm) ¦   ¦   Verifier (Perm)  ¦                             ¦
¦   ¦ - Writes test/code ¦   ¦ - Confirms Red/Grn ¦                             ¦
¦   ¦ - Learns TDD rules ¦   ¦ - Learns failures  ¦                             ¦
¦   +--------------------+   +--------------------+                             ¦
+-------------------------------------------------------------------------------+
```

---

## Architecture Decisions

### AD-1: Attempt Ledger as Feedback Source for Permanent Subagents
- **Context:** Gentle AI runs an external ephemeral CLI ledger. In GAIA, subagents are long-lived and learn continuously.
- **Decision:** The `AttemptLedger` provides transactional `Acquire` and `Settle` methods in Go. When `Settle` detects failures, compilation errors, or budget overruns, it writes learning insights directly to `gaia/{subagent}/{project}/attempt-failures` and triggers `learnLoop.RecordExecution(name)`.
- **Consequence:** Permanent subagents become aware of their own past mistakes on retry and future tasks.

### AD-2: State Machine for Strict TDD Collaboration
- **Context:** Verifier and Implementer must enforce the RED -> GREEN -> TRIANGULATE -> REFACTOR cycle without skipping phases.
- **Decision:** Implement a `TDDEngine` state machine. In `TDDPhaseRed`, transitions are rejected unless a test file is created/modified AND the test runner execution fails with assertion failure. In `TDDPhaseGreen`, implementation is required and test runner must pass.
- **Consequence:** Eliminates hallucinations where subagents claim code works without having written or executed unit tests.

### AD-3: Path-Based Skill Injection
- **Context:** Injecting full skill text into subagent prompts inflates context windows (~10k+ tokens).
- **Decision:** The Spawner resolves skill paths from the registry and passes `## Skills to load before work` containing relative file paths (`skills/golang-.../SKILL.md`). Subagents read full skill contents on demand via `file_read`.
- **Consequence:** Saves 70%+ of prompt tokens while maintaining full fidelity of skill guidelines.

### AD-4: Review Workload Guard & Opt-In Switch
- **Context:** Large diffs cause human reviewer fatigue; unmanaged RDD reviews can block normal development when not requested.
- **Decision:** Review mode is opt-in (`enable|disable|status`). Tasks over 400 lines trigger automatic PR chaining recommendation. The Review Engine caps corrections to `min(200, ceil(lines/2))` and allows at most 1 correction round.

---

## Component Interfaces & Data Models

### 1. Attempt Ledger
```go
type AttemptLedger interface {
    Acquire(ctx context.Context, req AcquireRequest) (*AcquireResponse, error)
    Settle(ctx context.Context, req SettleRequest) (*WorkUnitEntry, error)
    GetStatus(changeName, workUnit string) (*WorkUnitEntry, bool)
}
```

### 2. Strict TDD Engine
```go
type TDDPhase string

const (
    TDDPhaseRed      TDDPhase = "red"
    TDDPhaseGreen    TDDPhase = "green"
    TDDPhaseRefactor TDDPhase = "refactor"
    TDDPhaseComplete TDDPhase = "complete"
)

type TDDEngine interface {
    CurrentPhase(changeName string) TDDPhase
    Advance(changeName string, testPassed bool, testFilesChanged, codeFilesChanged []string) (TDDPhase, error)
    Reset(changeName string)
}
```

---

## Sequence: Strict TDD with Attempt Ledger & Subagent Learning

```
Implementer               TDDEngine              Verifier              AttemptLedger
    |                         |                      |                       |
    |---- Acquire(slot) ---------------------------------------------------->|
    |<--- Token & Insights --------------------------------------------------|
    |                         |                      |                       |
    |-- Write foo_test.go --->|                      |                       |
    |                         |-- Verify Red? ------>|                       |
    |                         |<-- Tests Failed (OK)-|                       |
    |<-- Phase: GREEN --------|                      |                       |
    |                         |                      |                       |
    |-- Write foo.go -------->|                      |                       |
    |                         |-- Verify Green? ---->|                       |
    |                         |<-- Tests Passed -----|                       |
    |<-- Phase: COMPLETE -----|                      |                       |
    |                         |                      |                       |
    |---- Settle(complete, lines) ------------------------------------------>|
    |                         |                      |   [Persist Success    |
    |                         |                      |    in Domain Memory]  |
```
