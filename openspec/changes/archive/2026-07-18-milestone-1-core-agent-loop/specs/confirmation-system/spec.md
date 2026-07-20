# Confirmation System Specification

## Purpose

Per-session trust levels controlling when tool execution requires user confirmation, with `/trust` slash commands.

## Requirements

### Requirement: Trust Modes

The system MUST support 4 modes: `always` (confirm every tool call), `per-session` (confirm once per tool per session), `per-action` (confirm each invocation), `never` (no confirmation). The default SHALL be `always`.

#### Scenario: Always mode

- GIVEN trust mode is `always`
- WHEN any tool call is issued
- THEN the system prompts the user for confirmation before execution

#### Scenario: Per-session mode

- GIVEN trust mode is `per-session` and the user approved "shell_exec"
- WHEN another "shell_exec" call arrives in the same session
- THEN the system executes without prompting

### Requirement: /trust Commands

The system MUST support `/trust <mode>` to change the mode at runtime. The system MUST support `/trust list` to show current mode and per-tool approvals.

#### Scenario: Change mode via slash command

- GIVEN current mode is `always`
- WHEN the user types `/trust per-session`
- THEN the mode switches to `per-session` for the current session

### Requirement: Headless Mode

When running without a TUI (headless/CI), the system MUST default to `never` and MUST NOT prompt for confirmation.

#### Scenario: Headless execution

- GIVEN the system runs in headless mode
- WHEN a tool call is issued
- THEN the tool executes without confirmation

### Requirement: Session State

Trust approvals MUST persist for the session lifetime. On session end, all approvals MUST be cleared.

#### Scenario: Session cleanup

- GIVEN a session with 3 approved tools
- WHEN the session ends
- THEN all approvals are discarded
