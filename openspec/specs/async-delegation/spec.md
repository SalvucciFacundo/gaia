# Async Delegation Specification

## Purpose

Asynchronous subagent execution with task lifecycle management, direct subagent chat via `@name` syntax, TUI task display and control, and async SDD pipeline execution.

## Requirements

### Requirement: Task Lifecycle Management

The system MUST provide a `TaskManager` that tracks asynchronous tasks through the lifecycle: Pending → Running → Completed | Failed | Cancelled. Each task MUST have a unique `TaskID` (UUID string) and a `TaskState` containing SubagentName, SubagentTask, Status, Result, Error, CreatedAt, and CompletedAt. The TaskManager MUST be concurrent-safe and MAY persist tasks to SQLite.

#### Scenario: Create and complete a task

- GIVEN the TaskManager is initialized
- WHEN CreateTask is called with a subagent name and task
- THEN a TaskID is returned with status Pending
- AND the task transitions to Running when execution begins
- AND the task transitions to Completed with a SubagentResult on success

#### Scenario: Goroutine panic recovery

- GIVEN a task is in Running status
- WHEN the executing goroutine panics
- THEN the TaskManager MUST recover the panic
- AND the task status MUST transition to Failed with the panic message as Error

#### Scenario: Cancel a running task

- GIVEN a task is in Running status with an active context
- WHEN CancelTask is called with the TaskID
- THEN the task's context is cancelled
- AND the task status transitions to Cancelled
- AND the goroutine exits within a bounded duration

#### Scenario: Subscribe to task completion

- GIVEN a task is in Pending or Running status
- WHEN SubscribeTask is called with the TaskID
- THEN a receive-only channel is returned
- AND the channel receives the TaskState when the task reaches a terminal state

### Requirement: Async Spawn

The Spawner MUST expose `SpawnAsync(ctx, name, task) (TaskID, error)` that launches subagent execution in a goroutine and returns immediately. The goroutine MUST call the existing synchronous Spawn logic internally. The TaskManager MUST be injected into Spawner via SpawnerConfig.

#### Scenario: SpawnAsync returns immediately

- GIVEN a registered subagent "explorer"
- WHEN SpawnAsync is called with a task for "explorer"
- THEN a TaskID is returned before the subagent completes
- AND the TUI remains responsive during execution

#### Scenario: Context cancellation propagation

- GIVEN a task is running via SpawnAsync
- WHEN the parent context is cancelled
- THEN the subagent's execution context is cancelled
- AND the task transitions to Cancelled or Failed

### Requirement: Direct Subagent Routing

The Brain MUST parse `@<name> <message>` syntax and route the message directly to the named subagent via SpawnAsync. The subagent MUST receive a SubagentTask with `IsDirectChat: true`. The Spawner MUST enforce that the subagent only uses tools within its AllowedTools set. If the named subagent does not exist, the system MUST respond with an error listing available subagents. Direct chat sessions MUST be fire-and-forget with no conversation history preserved between `@messages`.

#### Scenario: Route to existing subagent

- GIVEN a registered subagent "proposer"
- WHEN the user types "@proposer write a proposal for auth"
- THEN the message is routed directly to the proposer subagent
- AND the subagent receives IsDirectChat: true

#### Scenario: Unknown subagent name

- GIVEN no subagent named "foobar" is registered
- WHEN the user types "@foobar do something"
- THEN the system responds with "Unknown subagent. Available: explorer, proposer, ..."

#### Scenario: Tool scope enforcement

- GIVEN a direct chat with subagent "explorer" whose AllowedTools are [file_read, shell_exec]
- WHEN the subagent attempts to call "git_commit"
- THEN the tool call is rejected with a scope violation error

#### Scenario: Routing works for dynamic subagents

- GIVEN a dynamically registered subagent "k8s-debugger"
- WHEN the user types "@k8s-debugger check pod status"
- THEN the message is routed to the dynamic subagent

### Requirement: Async SDD Pipeline

SDD phases (explore → propose → spec → design → tasks → apply → verify) MUST be executable as async tasks. Each phase invocation MUST return a TaskID immediately. Phases MUST remain sequential — phase N+1 starts only after phase N completes. Cancelling a phase MUST cancel all subsequent phases in the pipeline.

#### Scenario: Sequential phase execution

- GIVEN an SDD pipeline with phases [propose, spec, design]
- WHEN the pipeline is started
- THEN each phase returns a TaskID immediately
- AND the spec phase starts only after propose reaches Completed
- AND the design phase starts only after spec reaches Completed

#### Scenario: Cancel cascades to subsequent phases

- GIVEN phases [propose, spec, design] are queued
- WHEN the propose phase is cancelled
- THEN spec and design are also cancelled
- AND their statuses transition to Cancelled

### Requirement: TUI Task Display

The TUI MUST display active tasks with their name, status, and elapsed time. When a task reaches a terminal state, the TUI MUST display a notification. The user MUST be able to type "tasks" to list all tasks and "cancel <taskid>" to cancel a running task.

#### Scenario: Active task display

- GIVEN two tasks are running
- WHEN the TUI renders the status area
- THEN both tasks are shown with name, status, and elapsed time

#### Scenario: Completion notification

- GIVEN a task transitions from Running to Completed
- WHEN the TUI processes the state change
- THEN a completion notification is displayed to the user

#### Scenario: List tasks command

- GIVEN three tasks exist (one Running, one Completed, one Failed)
- WHEN the user types "tasks"
- THEN all three tasks are listed with their current status
