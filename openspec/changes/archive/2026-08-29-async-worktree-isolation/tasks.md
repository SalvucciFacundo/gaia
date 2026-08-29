# Tasks: Git Worktree Isolation for Asynchronous Subagents

## Review Workload Forecast

- **Estimated changed lines**: ~180 lines
- **400-line budget risk**: Low
- **Chained PRs recommended**: No (Under 400 lines; 2 small slices)
- **Decision needed before apply**: No

---

## Slice 1: Worktree Lifecycle Manager

- [x] 1.1 Implement `WorktreeManager` in `internal/agent/worktree.go`
  - [x] Implement `Create` (`git worktree add -b gaia-wt/<id>`)
  - [x] Implement `GetDiff` (`git diff` in worktree)
  - [x] Implement `Remove` (`git worktree remove --force` + `git branch -D`)
  - [x] Implement `Prune` to clean stale worktree records
- [x] 1.2 Write unit tests in `internal/agent/worktree_test.go`
  - [x] Test worktree creation and branch checkout in a real temp git repo
  - [x] Test diff generation from isolated modifications
  - [x] Test clean removal and branch deletion
  - [x] Test graceful fallback on non-git directory

---

## Slice 2: Spawner & TaskManager Integration

- [x] 2.1 Wire WorktreeManager into `Spawner.SpawnAsync` in `internal/agent/spawner.go`
  - [x] Auto-provision worktree when `task.RequireIsolation == true` or for write subagents (`implementer`, `debugger`)
  - [x] Execute subagent inside the worktree directory
  - [x] Extract diff on completion, attach to `Artifacts`, and cleanup in `defer`
- [x] 2.2 Add unit and integration tests in `internal/agent/worktree_test.go` and `spawner.go`
- [x] 2.3 Verify full test suite `go test ./...` and commit/push to `main`
