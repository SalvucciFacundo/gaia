# Subagent Base Specification

## Purpose

Core subagent infrastructure: interface contract, spawner with context isolation, registry for lookup, and orchestrator delegation mechanism.

## Requirements

### Requirement: Subagent Interface

The system MUST define a `Subagent` interface with `Name() string`, `Description() string`, and `Execute(ctx context.Context, task Task) (Summary, error)`. Every subagent MUST implement this interface.

#### Scenario: Interface implementation

- GIVEN a struct implementing `Subagent`
- WHEN `Name()` is called
- THEN it returns a non-empty unique identifier

#### Scenario: Execute returns summary

- GIVEN a subagent with a valid task
- WHEN `Execute` completes successfully
- THEN it returns a `Summary` with `Status`, `Artifacts`, and `Observations` fields

### Requirement: Spawner Isolation

The Spawner MUST create an isolated execution context for each subagent invocation. The context MUST include a fresh message history, filtered tool set, and scoped system prompt. Subagents MUST NOT see other subagents' intermediate state.

#### Scenario: Fresh context per invocation

- GIVEN two sequential subagent spawns
- WHEN the second subagent starts
- THEN its message history does not contain the first subagent's tool calls or responses

#### Scenario: Tool filter enforcement

- GIVEN an Explorer subagent with read-only tool filter
- WHEN the subagent attempts a write tool call
- THEN the Spawner rejects the call before execution

### Requirement: Summary Envelope

The `Summary` struct MUST contain `Status` (success/failure/partial), `Artifacts` (list of produced artifact paths), `Observations` (key findings for orchestrator synthesis), and `TokenUsage` (tokens consumed).

#### Scenario: Successful summary

- GIVEN a subagent completes its task
- WHEN it returns a Summary
- THEN Status is "success" and Artifacts lists at least one path

#### Scenario: Failed summary

- GIVEN a subagent encounters an unrecoverable error
- WHEN it returns a Summary
- THEN Status is "failure" and Observations contains the error context

### Requirement: Orchestrator Delegation

The orchestrator (Brain) MUST support a delegation step where it selects a subagent by name, constructs a Task with context, invokes Execute, and receives the Summary for synthesis.

#### Scenario: Delegation flow

- GIVEN the orchestrator determines a task requires subagent work
- WHEN it delegates to the "explorer" subagent
- THEN the explorer executes with the task context and returns a Summary

#### Scenario: Summary synthesis

- GIVEN the orchestrator receives a Summary from a subagent
- WHEN it processes the result
- THEN it incorporates Observations into its response to the user
