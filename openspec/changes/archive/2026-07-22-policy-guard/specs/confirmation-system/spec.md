# Delta for Confirmation System

## MODIFIED Requirements

### Requirement: Trust Modes

The system MUST support 4 modes: `always`, `per-session`, `per-action`, `never`. Only `never` (TrustNever) SHALL remain active as the legacy execution path. The modes `always`, `per-session`, and `per-action` are REPLACED by PolicyGuard tiers (`read`, `sandbox`, `full`). The default SHALL be `never` for the legacy path; all new permission decisions are delegated to PolicyGuard.
(Previously: All 4 modes were active with `always` as default; each mode independently controlled confirmation behavior)

#### Scenario: Never mode (legacy active)

- GIVEN trust mode is `never` (TrustNever)
- WHEN any tool call is issued through the legacy path
- THEN the system delegates to PolicyGuard for policy evaluation

#### Scenario: Deprecated modes redirect to PolicyGuard

- GIVEN a caller requests `always`, `per-session`, or `per-action` mode
- WHEN the mode is resolved
- THEN the system maps the request to the equivalent PolicyGuard tier (`read`, `sandbox`, or `full`) and evaluates via PolicyGuard

### Requirement: /trust Commands

The system MUST support `/trust <mode>` to change the mode at runtime. The system MUST support `/trust list` to show current mode and per-tool approvals. Mode changes via `/trust` SHALL be translated to PolicyGuard tier changes. Legacy per-tool approvals are preserved only for TrustNever backward compatibility.
(Previously: `/trust` directly changed the 4-mode trust state; now it maps to PolicyGuard tiers)

#### Scenario: Change mode via slash command

- GIVEN current effective tier is `read`
- WHEN the user types `/trust full`
- THEN the PolicyGuard tier switches to `full` for the current session

### Requirement: Headless Mode

When running without a TUI (headless/CI), the system MUST default to `never` and MUST NOT prompt for confirmation. PolicyGuard SHALL operate in `full` tier in headless mode unless overridden by flags.
(Previously: no change to behavior — unchanged)

#### Scenario: Headless execution

- GIVEN the system runs in headless mode
- WHEN a tool call is issued
- THEN the tool executes without confirmation via PolicyGuard full tier

## REMOVED Requirements

### Requirement: Session State

Trust approvals MUST persist for the session lifetime. On session end, all approvals MUST be cleared.
(Reason: Session-based per-tool approvals are replaced by PolicyGuard tier evaluation. Policy state is managed by PolicyGuard, not the confirmation system.)
(Migration: PolicyGuard handles session-scoped tier state. Existing session cleanup logic is absorbed into PolicyGuard lifecycle.)
