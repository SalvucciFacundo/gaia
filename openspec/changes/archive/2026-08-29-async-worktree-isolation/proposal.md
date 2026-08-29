# Proposal: Git Worktree Isolation for Asynchronous Subagents

## Intent

GAIA supports concurrent asynchronous subagents (`/background` and `SpawnAsync`). However, write-capable subagents (such as `Implementer` or `Debugger`) running concurrently in the main workspace directory can cause file race conditions, dirty git index states, and overwrite changes while the human developer is working.

This change introduces **Git Worktree Isolation**: dynamically provisioning an ephemeral `git worktree` and isolated branch per background subagent, allowing true parallel execution without contaminating the active working tree.

## Scope

### In Scope
- **`worktree-manager`**: A robust Git worktree manager that provisions, inspects, and cleans up ephemeral worktrees and branches.
- **`async-spawner-isolation`**: Integration with `SpawnAsync` and `TaskManager` to automatically route background subagents to their dedicated worktree directory.
- **`patch-merge-reconciliation`**: Generating unified diffs/patches upon background task completion for clean review and merging.

### Out of Scope
- Non-git workspaces (falls back gracefully to direct execution with a warning).
- Long-lived worktrees across application restarts (worktrees are strictly ephemeral per async task).

## Capabilities

### New Capabilities
- `worktree-manager`: Lifecycle management (`Create`, `Remove`, `GetDiff`, `ApplyPatch`) for isolated git worktrees.
- `async-spawner-isolation`: Task-level isolation flag routing tool execution to the isolated worktree directory.

### Modified Capabilities
- `spawner.SpawnAsync`: Provisions an isolated worktree before background subagent execution and cleans up on completion.
- `task_manager`: Tracks worktree directory and branch metadata in `TaskState`.

## Approach

### Execution Strategy: 2 Slices (Stacked to Main)

| Slice | Capabilities | Rationale |
|-------|-------------|-----------|
| Slice 1 | `worktree-manager` | Core git worktree creation, diff generation, and cleanup mechanics |
| Slice 2 | `async-spawner-isolation` | Spawner & TaskManager integration with automated lifecycle management |

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/` | New & Modified | Worktree manager, Spawner async routing, TaskManager metadata |
| `internal/modules/gitops/` | Modified | Support for worktree-aware git commands |
| `docs/` | Modified | Documentation on background worktree isolation |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Git lock contention when creating multiple worktrees concurrently | Low | Mutex-guarded worktree creation and unique branch naming with UUID |
| Disk space accumulation if tasks crash | Low | Automatic cleanup in `defer` and startup pruning of stale `.gaia/worktrees/` |
| Non-git directory invocation | Low | Preflight check; fall back to in-place execution with warning |

## Success Criteria

1. Background subagents modify files and execute tests in a separate worktree without changing the user's active files.
2. Unified diff/patch is generated upon task completion and attached to `SubagentResult`.
3. Ephemeral worktree and branch are cleaned up reliably on success, failure, or cancellation.
4. 100% test coverage with clean `go test ./...` execution.
