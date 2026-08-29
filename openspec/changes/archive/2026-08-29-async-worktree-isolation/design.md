# Technical Design: Git Worktree Isolation for Asynchronous Subagents

## Architecture Overview

```
+-------------------------------------------------------------------------------+
¦                             GAIA Async Spawner                                ¦
¦                                                                               ¦
¦   User Workspace: /home/user/myproject  (Active working tree - Untouched)    ¦
¦                                                                               ¦
¦   +-----------------------------------------------------------------------+   ¦
¦   ¦                      Worktree Lifecycle Manager                       ¦   ¦
¦   ¦   - Provisions: .gaia/worktrees/<task-id>                             ¦   ¦
¦   ¦   - Isolated branch: gaia-wt/<task-id>                                ¦   ¦
¦   +-----------------------------------T-----------------------------------+   ¦
¦                                       ¦                                       ¦
¦        +------------------------------+------------------------------+        ¦
¦        ¦                                                             ¦        ¦
¦   +----v-----------------------------+   +---------------------------v----+   ¦
¦   ¦   Background Subagent Runner     ¦   ¦     Patch & Diff Collector     ¦   ¦
¦   ¦   - Working Dir: .gaia/worktrees/¦   ¦   - Generates unified diff     ¦   ¦
¦   ¦   - Compiles & Runs Tests in WT  ¦   ¦   - Emits patch artifact       ¦   ¦
¦   +----T-----------------------------+   +---------------------------T----+   ¦
¦        ¦                                                             ¦        ¦
¦        +------------------------------+------------------------------+        ¦
¦                                       ¦                                       ¦
¦   +-----------------------------------v-----------------------------------+   ¦
¦   ¦                   TaskManager Result & Notification                   ¦   ¦
¦   ¦   - Result: Completed + Patch Artifact                                ¦   ¦
¦   ¦   - Automatic ephemeral worktree cleanup in defer                     ¦   ¦
¦   +-----------------------------------------------------------------------+   ¦
+-------------------------------------------------------------------------------+
```

---

## Architecture Decisions

### AD-1: Ephemeral Worktrees under `.gaia/worktrees/<task-id>`
- **Context:** Multiple background tasks could run concurrently and need isolated filesystem trees.
- **Decision:** Create worktrees under `.gaia/worktrees/<task-id>` using standard `git worktree add -b gaia-wt/<task-id> <path> HEAD`.
- **Consequence:** Each task gets a 100% independent filesystem and git index. The main workspace is never dirtied during background runs.

### AD-2: Automatic Diff Generation & Cleanup on Task Completion
- **Context:** Once the background task finishes, the worktree is no longer needed, but the user needs the changes.
- **Decision:** Generate the unified git diff (`git diff HEAD` inside the worktree) and attach it to `SubagentResult.Artifacts`. Then run `git worktree remove --force` and delete the temporary branch in a `defer` block.
- **Consequence:** Zero leftover disk waste, zero leaked git locks, and the user receives a clean, reviewable patch.

---

## Data Models & Interfaces

```go
type WorktreeContext struct {
    TaskID      string    `json:"task_id"`
    BasePath    string    `json:"base_path"`
    WorktreeDir string    `json:"worktree_dir"`
    BranchName  string    `json:"branch_name"`
    CreatedAt   time.Time `json:"created_at"`
}

type WorktreeManager interface {
    Create(ctx context.Context, baseRepoPath, taskID string) (*WorktreeContext, error)
    GetDiff(ctx context.Context, wt *WorktreeContext) (string, error)
    Remove(ctx context.Context, wt *WorktreeContext) error
    Prune(ctx context.Context, baseRepoPath string) error
}
```
