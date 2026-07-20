# SDD Subagents Specification

## Purpose

Five specialized subagents implementing the SDD protocol: Explorer, Proposer, Specifier, Implementer, and Verifier. Each has role-specific tools, system prompts, and execution rules.

## Requirements

### Requirement: Explorer Subagent

The Explorer MUST investigate the codebase using read-only tools (file read, grep, glob, codegraph). It MUST NOT modify files or execute shell commands. It SHALL return findings as structured Observations.

#### Scenario: Codebase investigation

- GIVEN the orchestrator sends an exploration task
- WHEN the Explorer executes
- THEN it returns Observations describing relevant files, patterns, and architecture

#### Scenario: Write attempt blocked

- GIVEN the Explorer receives a task
- WHEN it attempts to call a write tool
- THEN the Spawner rejects the call with a tool-filter error

### Requirement: Proposer Subagent

The Proposer MUST generate change proposals with intent, scope, approach, and rollback plan. It SHALL have read-only tools plus Engram memory access. It MUST NOT write code or execute commands.

#### Scenario: Proposal generation

- GIVEN exploration findings as context
- WHEN the Proposer executes
- THEN it returns a Summary containing a proposal artifact path

#### Scenario: Scope definition

- GIVEN a change intent
- WHEN the Proposer defines scope
- THEN the proposal includes explicit in-scope and out-of-scope items

### Requirement: Specifier Subagent

The Specifier MUST produce delta specs with RFC 2119 requirements and Given/When/Then scenarios. It SHALL have read-only tools plus Engram access. It MUST NOT write code or execute shell commands.

#### Scenario: Spec generation

- GIVEN a proposal as input context
- WHEN the Specifier executes
- THEN it returns Summary with spec artifact paths and requirement counts

#### Scenario: Scenario coverage

- GIVEN a new requirement
- WHEN the Specifier writes it
- THEN the requirement includes at least one happy-path and one edge-case scenario

### Requirement: Implementer Subagent

The Implementer MUST write code from tasks and specs. It SHALL have write tools (file write, edit) and shell access. It MUST follow existing code patterns and respect the tool filter.

#### Scenario: Code generation

- GIVEN tasks and specs as context
- WHEN the Implementer executes
- THEN it produces code files and returns artifact paths in the Summary

#### Scenario: Pattern compliance

- GIVEN the codebase uses hexagonal architecture
- WHEN the Implementer writes new code
- THEN new code follows ports-and-adapters patterns

### Requirement: Verifier Subagent

The Verifier MUST execute tests and validate implementation against specs. It SHALL have shell access (test commands) and read tools. It MUST NOT write or modify code.

#### Scenario: Test execution

- GIVEN an implementation to verify
- WHEN the Verifier executes
- THEN it runs the test suite and returns pass/fail status in the Summary

#### Scenario: Spec compliance check

- GIVEN specs and implementation artifacts
- WHEN the Verifier checks compliance
- THEN Observations list any spec requirements not satisfied by the implementation
