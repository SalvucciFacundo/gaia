# Capability Specification: Attempt Ledger for Permanent Subagents

## Capability: `attempt-ledger`

The Attempt Ledger governs execution attempts and line-change budgets per work unit, while feeding outcome insights into permanent subagents' learning loops.

---

### Requirement: ATTEMPT-001 — Attempt Slot Acquisition

The Attempt Ledger MUST authorize execution attempts with a unique token, enforce a maximum retry count, and provide historical failure insights to the calling subagent.

#### Scenario: Successful slot acquisition on first attempt
- **Given** a work unit `unit-1` with `MaxAttempts: 3` and no previous execution history
- **When** `Acquire` is called
- **Then** the ledger SHALL return `State: proceed`, a non-empty `Token`, and `AttemptNumber: 1`

#### Scenario: Slot acquisition blocked when maximum attempts exceeded
- **Given** a work unit `unit-1` with `MaxAttempts: 3` that has already attempted 3 times without completion
- **When** `Acquire` is called
- **Then** the ledger SHALL return `State: blocked` with `BlockedReason` naming the exceeded limit
- **And** the ledger SHALL trigger a learning reflection in the subagent's learning loop

---

### Requirement: ATTEMPT-002 — Line Budget Guard & Settlement

The Attempt Ledger MUST settle attempts by recording diff lines changed and validating against the line budget ceiling.

#### Scenario: Settlement within budget
- **Given** an active attempt with `MaxChangedLines: 400`
- **When** `Settle` is called with `ChangedLines: 150` and `Outcome: complete`
- **Then** the ledger SHALL mark the work unit `State: complete` and record a positive learning insight

#### Scenario: Settlement blocked on line budget violation
- **Given** an active attempt with `MaxChangedLines: 400`
- **When** `Settle` is called with `ChangedLines: 520`
- **Then** the ledger SHALL mark the work unit `State: blocked`
- **And** the ledger SHALL record a budget overrun insight in the subagent's memory namespace

---

### Requirement: ATTEMPT-003 — Auto-Learning Feedback Integration

When an attempt fails or is blocked, the Attempt Ledger MUST persist the error context to the subagent's permanent domain memory.

#### Scenario: Failure pattern recorded for future attempts
- **Given** an active attempt for subagent `implementer` that encounters a compilation failure
- **When** `Settle` is called with `Outcome: blocked` and `Error: "undefined symbol NewFoo"`
- **Then** the ledger SHALL store the error pattern in the work unit's `LearnedInsights`
- **And** subsequent `Acquire` calls for this change SHALL include this insight in `LearnedContext`
