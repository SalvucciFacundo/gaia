# Capability Specification: Strict TDD Engine

## Capability: `strict-tdd`

Strict TDD enforces the test-first cycle (RED -> GREEN -> TRIANGULATE -> REFACTOR) between `Implementer` and `Verifier`, teaching subagents test-driven discipline.

---

### Requirement: TDD-001 — Test-First RED Phase Enforcement

The Implementer MUST author a failing test file before writing production code. The Verifier MUST confirm that the test suite fails due to expected assertion failures before implementation begins.

#### Scenario: Advancing through valid RED phase
- **Given** the TDD state machine is in phase `TDDPhaseRed`
- **When** the Implementer writes `foo_test.go` and the test runner reports a failing test exit code with assertion failure
- **Then** the TDD engine SHALL transition to `TDDPhaseGreen`

#### Scenario: Rejecting code change without failing test
- **Given** the TDD state machine is in phase `TDDPhaseRed`
- **When** the Implementer writes production code `foo.go` without any new/modified test files
- **Then** the TDD engine SHALL reject the transition with `status: blocked` and report "Strict TDD violation: failing test required before implementation"

---

### Requirement: TDD-002 — GREEN Phase Verification

The Implementer MUST write minimal code to make the failing test pass. The Verifier MUST execute tests to prove green status.

#### Scenario: Advancing through valid GREEN phase
- **Given** the TDD state machine is in phase `TDDPhaseGreen`
- **When** the Implementer writes production code and the test runner reports 100% passing tests
- **Then** the TDD engine SHALL transition to `TDDPhaseRefactor` or `TDDPhaseComplete`

#### Scenario: Remaining blocked if tests still fail in GREEN phase
- **Given** the TDD state machine is in phase `TDDPhaseGreen`
- **When** the test runner reports continuing test failures
- **Then** the TDD engine SHALL keep state in `TDDPhaseGreen` and increment the Attempt Ledger retry count

---

### Requirement: TDD-003 — TDD Learning & Anti-Pattern Memory

The TDD engine MUST record recurring testing violations and syntax errors in the `Implementer`'s domain memory.

#### Scenario: Reinforcing test-first habit
- **Given** an Implementer that successfully completes a RED -> GREEN -> REFACTOR cycle
- **When** the cycle is marked complete
- **Then** the TDD engine SHALL record a successful TDD observation in `gaia/implementer/tdd-patterns`
