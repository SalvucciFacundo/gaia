# GAIA Architecture Proposals & Roadmap

This document captures the approved proposals for GAIA's next evolution phases, organized for sequential review and implementation.

---

## 1. Profile Memory Isolation with Cross-Reading & RBAC
- **Status**: Completed (`completed`)
- **Objective**: Each profile maintains its own dedicated memory namespace, can query context from other profiles, but requires explicit human authorization to mutate another profile's data.
- **Architecture**:
  - **Namespacing**: Topic keys formatted as `gaia/profile/{profile_id}/{project}/{topic}`.
  - **Shared Reading**: All profiles have read access to other profiles and to `gaia/shared/{project}`.
  - **Write Guard**: Any attempt by profile $A$ to update or delete records under namespace $B$ triggers `PolicyGuard`, halting execution until the human grants permission.
- **Implemented Files**:
  - `internal/agent/memory/namespace.go`
  - `internal/core/policy.go`
  - `cmd/gaia/main.go`
  - `cmd/gaia/exec.go`

---

## 2. Tool Calling Optimization: Parallel Batches & Code Mode (CodeAct)
- **Status**: Completed (`completed`)
- **Objective**: Drastically reduce token consumption and latency by batching tool calls and introducing programmatic tool orchestration.
- **Architecture**:
  - **Parallel Tool Calling**: Dispatch independent read-only tool calls in a single turn via concurrent Go routines, preserving strict output order while isolating mutations.
  - **Code Mode (CodeAct)**: Programmatic tool execution via embedded pure Go interpreter (`github.com/traefik/yaegi`) with `GaiaSDK`, prelude wrapper, and sandboxing.
- **Implemented Files**:
  - `internal/core/kernel.go`
  - `internal/agent/spawner.go`
  - `internal/modules/codemode/engine.go`
  - `internal/modules/codemode/sdk.go`
  - `internal/modules/codemode/module.go`
  - `cmd/gaia/main.go`
  - `cmd/gaia/exec.go`

---

## 3. Native Reminis Engine Port (Token & Context Optimization)
- **Status**: Completed (`completed`)
- **Objective**: Extract the OS-LLM architecture and memory patterns from Reminis and implement them natively in pure Go, eliminating quadratic context bloat ($O(N^2)$).
- **Architecture**:
  - **Blackboard (L1 Working Memory)**: Concurrent, thread-safe memory ledger (`sync.RWMutex`) with automatic 4KB disk spillover pass-by-reference.
  - **Blackboard Tools**: Exposes `blackboard_get`, `blackboard_set`, and `blackboard_list` across all profiles and subagents.
  - **Ephemeral Workers & Context Destruction**: Bounded tool execution turns (max 3 turns), auto-resolution of Blackboard `InputKeys`, and guaranteed garbage collection of conversation history upon return.
- **Implemented Files**:
  - `internal/blackboard/entry.go`
  - `internal/blackboard/blackboard.go`
  - `internal/blackboard/blackboard_test.go`
  - `internal/modules/blackboard/module.go`
  - `internal/modules/blackboard/module_test.go`
  - `internal/core/domain/models.go`
  - `internal/agent/spawner.go`
  - `internal/agent/ephemeral_test.go`
  - `cmd/gaia/main.go`
  - `cmd/gaia/exec.go`

---

## 4. Overnight Autonomous Execution (Worktree Sandbox & Deferred Review)
- **Status**: Completed (`completed`)
- **Objective**: Allow the agent to execute multi-stage, long-running projects continuously (e.g., overnight) across all profiles without getting blocked waiting for human confirmation between steps, presenting a single consolidated review before merging or publishing.
- **Architecture**:
  - **Pre-flight Alignment Questionnaire**: Discovers architectural forks and sensitive boundaries, prompting the user before sleep to seal directives into an immutable `ActionPlan`.
  - **Multi-Profile Sandboxing**: Git Worktree isolation for git projects (`gaia-wt/<plan_id>`) or filesystem staging (`.gaia/staging/<plan_id>`) for non-git tasks.
  - **Deferred Review Queue**: Intercepts `PolicyGuard` `ask_user` actions into an append-only log without halting unattended execution.
  - **Consolidated Review Pack**: Structured summary presenting code diffs, generated assets, test execution results, and pending deferred actions for one-click morning approval (`gaia overnight merge/apply`).
- **Implemented Files**:
  - `internal/overnight/types.go`
  - `internal/overnight/preflight.go`
  - `internal/overnight/deferred_queue.go`
  - `internal/overnight/runner.go`
  - `internal/overnight/overnight_test.go`
  - `internal/adapters/db/sqlite.go`
  - `cmd/gaia/overnight.go`
  - `cmd/gaia/main.go`

---

## 5. Simplified Permissions & Policy Control Panel
- **Status**: Completed (`completed`)
- **Objective**: Replace obscure security configurations with an intuitive 4x3 permissions matrix and blast radius indicators in both the Web UI and Cyberpunk TUI.
- **Architecture**:
  - **4 Functional Categories**: `Filesystem & Disk`, `Terminal & Commands`, `Network & Internet`, `Memory & State`.
  - **3-State Traffic Light**: `🟢 Always Allow`, `🟡 Ask User`, `🔴 Block`.
  - **Blast Radius Indicators**: Project Workspace, Local Machine (OS), External Internet, GAIA Storage.
  - **Cyberpunk TUI & Web UI**: Interactive panel with accordion progressive disclosure for tool-by-tool breakdowns and spacebar/button state toggling.
  - **CLI Command Suite**: `gaia policy status [-v]`, `gaia policy panel`, `gaia policy init`.
- **Implemented Files**:
  - `internal/core/policy_categories.go`
  - `internal/core/policy_categories_test.go`
  - `internal/core/kernel.go`
  - `internal/adapters/tui/policy_panel.go`
  - `internal/adapters/web/server.go`
  - `internal/adapters/web/server_test.go`
  - `internal/adapters/web/views/layout.go`
  - `internal/adapters/web/views/policy_modal.go`
  - `cmd/gaia/policy.go`
  - `cmd/gaia/policy_test.go`

