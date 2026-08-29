# Capability Specification: Git Worktree Lifecycle Manager

## Capability: `worktree-lifecycle`

The Worktree Manager provisions and destroys ephemeral Git worktrees and branches for isolated execution.

---

### Requirement: WT-001 — Worktree Creation

The Worktree Manager MUST create an isolated worktree directory and unique branch from the repository's current HEAD.

#### Scenario: Successful worktree provisioning
- **Given** a valid Git repository at `/workspace`
- **When** `CreateWorktree(ctx, "/workspace", "task-123")` is called
- **Then** the manager SHALL create a worktree at `.gaia/worktrees/task-123`
- **And** the manager SHALL check out a new branch `gaia-wt/task-123`
- **And** the manager SHALL return a non-empty worktree path and branch name

#### Scenario: Non-git repository fallback
- **Given** a directory `/tmp/no-git` that is not a git repository
- **When** `CreateWorktree` is called
- **Then** the manager SHALL return an error indicating the repository is not managed by Git

---

### Requirement: WT-002 — Worktree Cleanup

The Worktree Manager MUST remove the ephemeral worktree and delete its isolated branch upon task completion.

#### Scenario: Worktree removal on completion
- **Given** an active worktree at `.gaia/worktrees/task-123` with branch `gaia-wt/task-123`
- **When** `RemoveWorktree(ctx, wt)` is called
- **Then** the worktree directory SHALL be unlinked and deleted
- **And** the ephemeral branch `gaia-wt/task-123` SHALL be deleted

---

### Requirement: WT-003 — Patch & Diff Generation

The Worktree Manager MUST generate a unified diff capturing all changes made within the worktree relative to the base branch.

#### Scenario: Generating diff of worktree changes
- **Given** an active worktree where `file.go` was created
- **When** `GetDiff(ctx, wt)` is called
- **Then** the manager SHALL return the unified diff string containing the changes in `file.go`
