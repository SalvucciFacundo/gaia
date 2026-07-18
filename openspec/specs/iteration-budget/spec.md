# Iteration Budget Specification

## Purpose

Safety caps on LLM ↔ tool iterations to prevent runaway loops, with per-call and per-subagent limits.

## Requirements

### Requirement: Per-Call Budget

The system MUST limit each `Brain.ProcessMessage()` call to a configurable max iterations (default: 25). Each LLM call + tool execution cycle counts as one iteration.

#### Scenario: Budget not exhausted

- GIVEN max iterations is 25 and 10 iterations have occurred
- WHEN the LLM returns a final text response (no tool calls)
- THEN the response is returned to the user normally

#### Scenario: Budget exhausted

- GIVEN max iterations is 25 and 25 iterations have occurred
- WHEN the LLM issues another tool call
- THEN the system halts execution and returns a budget-exceeded message to the user

### Requirement: Per-Subagent Budget

The system SHALL enforce a separate iteration budget per subagent when subagents are active. Each subagent MUST have its own independent counter.

#### Scenario: Subagent isolation

- GIVEN parent budget is 25 and subagent budget is 15
- WHEN the subagent reaches 15 iterations
- THEN the subagent stops but the parent may continue

### Requirement: Refund for execute_code

The system MAY refund one iteration when a tool call is `execute_code` (code execution is expensive and counts double). The refund MUST NOT exceed the iterations consumed.

#### Scenario: Refund applied

- GIVEN 10 iterations consumed and an execute_code call completes
- WHEN the refund is applied
- THEN the counter decreases by 1 (to 9)

### Requirement: Configurable Limits

The system SHALL read budget limits from config (`budget.max_iterations`). The system MUST allow override per invocation via options.

#### Scenario: Config override

- GIVEN config sets `budget.max_iterations: 50`
- WHEN ProcessMessage is called without override
- THEN the budget is 50 iterations
