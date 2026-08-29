## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 450 - 650 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Diff Parser & Domain Engine → PR 2: Bubbletea Diff Viewer Model → PR 3: TUI & Kernel Integration |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Implementation Tasks

### Slice 1: Diff Parsing & Domain Engine
- [x] Implement unified diff parser supporting headers, hunk parsing (`@@ ... @@`), and `DiffLine`, `DiffHunk`, `DiffFile` structs in `internal/diff/parser.go`. <!-- sdd-owner: implementation -->
- [x] Write unit tests for parsing standard unified diffs, empty diffs, and binary file notations in `internal/diff/parser_test.go`. <!-- sdd-owner: implementation -->
- [x] Implement Lipgloss syntax highlighting and ANSI styling utilities for additions, deletions, hunk headers, and file paths in `internal/diff/style.go`. <!-- sdd-owner: implementation -->

### Slice 2: Bubbletea Diff Viewer Model
- [x] Implement `DiffViewerModel` conforming to `tea.Model` with viewport integration, keybinding dispatcher, and state tracking in `internal/tui/diff/viewer.go`. <!-- sdd-owner: implementation -->
- [x] Implement hunk navigation (`n`/`p`), line scrolling (`j`/`k`/`Up`/`Down`), and model state rendering in `internal/tui/diff/viewer.go`. <!-- sdd-owner: implementation -->
- [x] Implement selective staging (`s`), unstaging (`u`), and discarding (`d`) via `git apply` plumbing calls in `internal/tui/diff/git.go`. <!-- sdd-owner: implementation -->
- [x] Implement steering input prompt overlay (`e`/`r`) and feedback dispatching command in `internal/tui/diff/steering.go`. <!-- sdd-owner: implementation -->
- [x] Write unit and component tests for keyboard navigation, staging actions, and steering prompt behavior in `internal/tui/diff/viewer_test.go`. <!-- sdd-owner: implementation -->

### Slice 3: TUI & Kernel Integration
- [x] Implement `/diff` slash command router handler and overlay activation logic within the main application model in `internal/tui/app.go`. <!-- sdd-owner: implementation -->
- [x] Implement pre-commit review integration point to trigger the diff viewer and handle steering callbacks in `internal/kernel/review.go`. <!-- sdd-owner: implementation -->
- [x] Write integration and end-to-end command tests verifying command routing, overlay lifecycle, and agent context injection in `internal/tui/integration_test.go`. <!-- sdd-owner: implementation -->

## Parent Review Actions
- [ ] Perform bounded review and verify behavior against design requirements. <!-- sdd-owner: parent -->
