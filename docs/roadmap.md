# GAIA Strategic Architecture Roadmap

This document outlines high-impact architectural enhancements for GAIA to elevate its capabilities as an autonomous programming agent.

---

## Pillar 1: Concurrency & Process Isolation

### 1. Git Worktree Isolation for Background Subagents (`in_progress`)
- **Problem**: When subagents run asynchronously in the background (`/background` or async delegation), writing files directly to the current working tree can conflict with the user's active edits.
- **Solution**: The `TaskManager` automatically provisions an ephemeral `git worktree` and isolated branch per background subagent execution. The subagent implements, compiles, and tests in isolation; upon completion, it produces a clean diff/patch or ready-to-merge branch.
- **Benefits**: Zero race conditions between human and background agents; true parallel execution.

---

## Pillar 2: Deep Code Understanding

### 2. CodeGraph & Local AST Semantic Index in SQLite (`completed`)
- **Problem**: Large repositories (>100 files) overwhelm grep/find and waste tokens during exploratory reads.
- **Solution**: A Go-native AST indexer (`internal/codegraph/`) that builds a relational symbol graph in SQLite:
  - Interface implementations (who implements `ports.Repository`?).
  - Function call hierarchies (callers and callees).
  - Dependency paths from HTTP handlers to database models.
- **Benefits**: Sub-millisecond architecture queries (<0.52ms); 80%+ token reduction during subagent exploration.

---

## Pillar 3: Developer Experience & Control

### 3. Interactive TUI Diff Viewer (Human-in-the-Loop) (`completed`)
- **Problem**: Terminal output only shows text summaries after code modifications, without visual hunk-level inspection.
- **Solution**: An interactive Bubbletea component (`internal/adapters/tui/diff_viewer.go` & `internal/diff/`) providing a unified colored diff viewer with:
  - Hunk-by-hunk review and staging (`s`/`u`/`d`).
  - Next/previous hunk navigation (`n`/`p`).
  - Line-level feedback steering (`e`/`r`) before commit.
  - `/diff` in-session TUI command.
- **Benefits**: Enforces "AI is a tool, human always directs" with zero friction.

---

## Pillar 4: Semantic Precision

### 4. LSP-Powered Active Refactoring Tools
- **Problem**: Text/regex based replacement can cause broken imports, syntax mismatches, or missed references across packages.
- **Solution**: Expose active Language Server Protocol (LSP) tools to `Implementer` and `Debugger`:
  - `lsp_rename_symbol`: Project-wide safe renaming.
  - `lsp_find_references`: Impact radius calculation before signature changes.
  - `lsp_code_actions`: Direct compiler/linter quick-fixes.
- **Benefits**: 100% compilation safety during multi-file refactoring.
