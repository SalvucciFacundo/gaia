# Capability Specification: Review Transitions v2 & Opt-In Mode

## Capability: `review-transitions-v2`

The Review Engine provides typed state machine transitions (`execute`, `collect`, `stop`) and an opt-in review switch (`enable|disable|status`), enforcing a strict mathematical correction budget.

---

### Requirement: REV-001 — Opt-In Review Mode Switch

Receipt-Driven Development (RDD) review MUST be opt-in and off by default.

#### Scenario: Review disabled
- **Given** review mode is set to `disabled`
- **When** delivery gates (`pre-commit`, `pre-push`) are evaluated
- **Then** gates SHALL pass under ordinary repository policy without launching review subagents

#### Scenario: Review enabled
- **Given** review mode is set to `enabled`
- **When** a review is initiated
- **Then** the engine SHALL run the 4 lenses and generate a content-bound receipt

---

### Requirement: REV-002 — Correction Line Budget

The Review Engine MUST enforce a correction line budget of `min(200, ceil(original_changed_lines / 2))` with at most 1 correction round.

#### Scenario: Correction within budget
- **Given** an original change of 100 lines and an admitted review finding
- **When** the Implementer submits a correction of 30 lines (budget ceiling = 50 lines)
- **Then** the Review Engine SHALL accept the correction and finalize the receipt
