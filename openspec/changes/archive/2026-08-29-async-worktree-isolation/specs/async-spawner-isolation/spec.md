# Capability Specification: Async Spawner Worktree Isolation

## Capability: `async-spawner-isolation`

The Spawner and TaskManager automatically provision and attach ephemeral worktrees to asynchronous subagent executions.

---

### Requirement: ASYNC-001 — Isolated Execution for Background Tasks

When `SpawnAsync` is called for a subagent that modifies files (e.g., `Implementer`, `Debugger`), the Spawner MUST execute the subagent inside an ephemeral worktree.

#### Scenario: Background Implementer runs in isolated worktree
- **Given** an async task for `implementer`
- **When** `SpawnAsync` executes
- **Then** all tool calls (`file_read`, `file_write`, `shell_exec`, `git_*`) SHALL execute relative to the isolated worktree directory
- **And** the user's primary working directory SHALL remain unmodified until approved

---

### Requirement: ASYNC-002 — Task Completion with Patch Attachment

Upon successful completion of an isolated async task, the TaskManager MUST attach the unified patch to the task result.

#### Scenario: Attaching patch artifact to task result
- **Given** an isolated background task that created `auth.go`
- **When** the task completes successfully
- **Then** `SubagentResult.Artifacts` SHALL include the path to the patch or file changes
- **And** the worktree directory SHALL be cleaned up cleanly
