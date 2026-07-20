# Automatic SDD Trigger Specification

## Purpose

Orchestrator monitors conversation for substantial change signals and automatically routes through the SDD subagent pipeline (Explorer → Proposer → Specifier → Implementer → Verifier).

## Requirements

### Requirement: Change Signal Detection

The orchestrator MUST analyze user messages for substantial change indicators: multi-file modifications, new feature requests, architecture changes, or explicit SDD commands. The detection SHALL use keyword heuristics combined with scope estimation.

#### Scenario: Feature request detected

- GIVEN the user says "Add authentication with JWT tokens"
- WHEN the orchestrator analyzes the message
- THEN it classifies this as a substantial change and triggers SDD flow

#### Scenario: Trivial change bypass

- GIVEN the user says "Fix the typo in the README"
- WHEN the orchestrator analyzes the message
- THEN it does NOT trigger SDD flow and handles directly

### Requirement: SDD Pipeline Execution

When triggered, the orchestrator MUST execute the pipeline: Explorer → Proposer → Specifier → Implementer → Verifier. Each stage MUST receive the previous stage's Summary as input context. The pipeline SHALL be sequential — no parallel stages.

#### Scenario: Full pipeline execution

- GIVEN SDD trigger fires for a feature request
- WHEN the pipeline runs
- THEN Explorer investigates, Proposer creates proposal, Specifier writes specs, Implementer codes, Verifier validates

#### Scenario: Stage failure handling

- GIVEN the Specifier fails during pipeline execution
- WHEN the failure is detected
- THEN the pipeline stops and the orchestrator reports the failure to the user with the Specifier's Observations

### Requirement: User Override

The user MUST be able to bypass automatic SDD triggering with `/direct` command. The user MUST be able to force SDD with `/sdd` command even for trivial changes.

#### Scenario: Direct bypass

- GIVEN the user types "/direct fix this bug in auth.go"
- WHEN the orchestrator processes the message
- THEN it handles the request directly without SDD pipeline

#### Scenario: Force SDD

- GIVEN the user types "/sdd update the config format"
- WHEN the orchestrator processes the message
- THEN it triggers the full SDD pipeline regardless of change scope
