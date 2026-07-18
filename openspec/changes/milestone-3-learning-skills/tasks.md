# Tasks: Milestone 3 — Learning & Skills

## Review Workload Forecast
- **Decision needed before apply**: Yes
- **Chained PRs recommended**: Yes (4 stacked PRs, ~400 LOC each)
- **400-line budget risk**: High
- **Chain strategy**: stacked-to-main — PR1 targets main, PR2 targets PR1, PR3 targets PR2, PR4 targets PR3

---

## PR 1: SDD Extension (Designer, Planner, Archiver) — CORE

### 1.1 Designer Subagent
- [x] Create `internal/agent/sdd/designer.go`
  - [x] Designer struct with spawner
  - [x] NewDesigner constructor
  - [x] Name/Description methods
  - [x] Execute method: read-only tools (file_read, file_list, git_status, git_log, git_diff), RunLoop, parseSDDResult default next: sdd-tasks
  - [x] designerPrompt: architecture decisions, file changes, data flow, design.md output
  - [x] Compile-time interface check

### 1.2 Planner Subagent
- [x] Create `internal/agent/sdd/planner.go`
  - [x] Planner struct with spawner
  - [x] NewPlanner constructor
  - [x] Name/Description methods
  - [x] Execute method: read + shell_exec tools, RunLoop, parseSDDResult default next: sdd-implement
  - [x] plannerPrompt: task breakdown, workload forecast, phase grouping, hierarchical numbering
  - [x] Compile-time interface check

### 1.3 Archiver Subagent
- [x] Create `internal/agent/sdd/archiver.go`
  - [x] Archiver struct with spawner
  - [x] NewArchiver constructor
  - [x] Name/Description methods
  - [x] Execute method: read + write tools (file_read, file_write, file_list, git_status, git_log, git_diff), RunLoop, parseSDDResult default next: none
  - [x] archiverPrompt: delta spec merge, archive directory move, audit trail, destructive delta warning
  - [x] Compile-time interface check

### 1.4 Main Registration
- [x] Update `cmd/gaia/main.go`: register designer, planner, archiver alongside existing 5

### 1.5 Tests
- [x] Extend `internal/agent/sdd/sdd_test.go` with new subagent tests
  - [x] Interface contract tests (Designer, Planner, Archiver — Name, Description, compile-time check)
  - [x] Execute tests with stub provider (all 3)
  - [x] AllowedTools tests (Designer: read-only, Planner: read+shell, Archiver: read+write)
  - [x] Prompt content tests (Designer: architecture/DATA FLOW, Planner: WORKLOAD FORECAST, Archiver: ARCHIVE WORKFLOW)
  - [x] Provider error handling tests (all 3)

---

## PR 2: Non-SDD Subagents (Reviewer, Debugger, Researcher, Learner)

### 2.1 Ops Package Foundation
- [x] Create `internal/agent/ops/` package
  - [x] Package-level doc.go
  - [x] Common ops helpers (parseOpsResult or reuse parseSDDResult)

### 2.2 Reviewer Subagent
- [x] Create `internal/agent/ops/reviewer.go`
  - [x] Reviewer struct with spawner
  - [x] NewReviewer constructor
  - [x] Execute method: read-only tools, GGA 4 lenses (risk, resilience, readability, reliability), bounded receipt output
  - [x] reviewerPrompt with 4-lens rubric
  - [x] Compile-time interface check

### 2.3 Debugger Subagent
- [x] Create `internal/agent/ops/debugger.go`
  - [x] Debugger struct with spawner
  - [x] NewDebugger constructor
  - [x] Execute method: read + shell_exec tools, bug analysis → root cause → fix → verify flow
  - [x] debuggerPrompt with structured debugging workflow
  - [x] Compile-time interface check

### 2.4 Researcher Subagent
- [x] Create `internal/agent/ops/researcher.go`
  - [x] Researcher struct with spawner
  - [x] NewResearcher constructor
  - [x] Execute method: read + file_list tools, web search + extraction, cite sources
  - [x] researcherPrompt with source citation requirements
  - [x] Compile-time interface check

### 2.5 Learner Subagent
- [x] Create `internal/agent/ops/learner.go`
  - [x] Learner struct with spawner
  - [x] NewLearner constructor
  - [x] Execute method: read + Engram tools, cross-subagent pattern extraction, skill proposals
  - [x] learnerPrompt with pattern analysis and skill proposal format
  - [x] Compile-time interface check

### 2.6 Main Registration
- [x] Update `cmd/gaia/main.go`: import ops package, register reviewer, debugger, researcher, learner

### 2.7 Tests
- [x] Create `internal/agent/ops/ops_test.go`
  - [x] Interface contract tests for all 4 subagents
  - [x] Execute tests with stub provider for all 4
  - [x] Tool filter tests for each
  - [x] Prompt content tests for each

---

## PR 3: Skills Hub + Wizard

### 3.1 Skills Package Foundation
- [x] Create `internal/skills/` package
  - [x] `hub.go`: Hub struct with Search, Install, List, Activate, Deactivate, Remove
  - [x] `registry.go`: Active skill registry loading SKILL.md content (embedded in hub.go via LoadSkills)
  - [x] `hub_test.go`: Unit tests for Hub operations

### 3.2 Skills Downloader
- [x] Create `internal/skills/downloader.go`
  - [x] Fetch from registry URL (DownloadFromURL)
  - [x] Validate SKILL.md format
  - [x] Extract to `~/.gaia/skills/{name}/` (DownloadFromDir for local import)

### 3.3 First-Run Wizard
- [x] Update `internal/adapters/tui/wizard.go`
  - [x] New step: detect project type (go, ts, py, etc.)
  - [x] New step: ask user language preference (EN/ES/PT)
  - [x] New step: recommend skills based on language
  - [x] New step: confirm and install selected skills

### 3.4 CLI Commands
- [x] Add `gaia skills` CLI (search, install, list, activate, deactivate, remove)

### 3.5 Tests
- [x] Hub unit tests (search, install, activate, deactivate, remove)
- [x] Registry tests (load, inject, reload) — covered via hub_test.go: TestHubLoadSkills, TestHubIndexInvalidation
- [x] Downloader tests (mocked HTTP via local dir) — TestDownloader
- [x] Wizard flow tests (integration) — wizard flow is tested via TUI test suite (tui_test.go)

---

## PR 4: Headless Mode + Output Modes

### 4.1 Exec Command
- [x] Create `cmd/gaia/exec.go`
  - [x] `gaia exec "task"` — execute once, no TUI
  - [x] Parse flags: --json, --quiet, --verbose, --dry-run, --yes

### 4.2 NullUI
- [x] Create `internal/adapters/tui/null.go` (per design: tui directory)
  - [x] Implement ports.UIService for headless (no display)
  - [x] Capture output for structured return

### 4.3 JSON Output
- [x] Create `internal/adapters/output/formatter.go`
  - [x] `--json` output envelope: {status, result, artifacts, risks}
  - [x] Format options: json, text (quiet/verbose/normal)

### 4.4 Quiet Mode
- [x] `--quiet` suppress non-essential output

### 4.5 Dry-Run Mode
- [x] `--dry-run` show plan, don't execute (NullUI denies tool confirmations)

### 4.6 Main Integration
- [x] Update `cmd/gaia/main.go`: detect exec subcommand alongside skills

### 4.7 Tests
- [x] Exec command tests (flag parsing) — `cmd/gaia/exec_test.go`
- [x] NullUI tests (output capture, confirmation behavior, interface satisfaction) — `internal/adapters/tui/null_test.go`
- [x] JSON formatter tests (envelope structure, constructors) — `internal/adapters/output/formatter_test.go`
- [x] Quiet/verbose/normal filter tests — covered in formatter_test.go
