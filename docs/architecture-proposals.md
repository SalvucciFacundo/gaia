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
- **Status**: Planned
- **Objective**: Allow the agent to execute multi-stage, long-running projects continuously (e.g., overnight) without getting blocked waiting for human confirmation between steps, presenting a single consolidated review before merging or publishing.
- **Architecture**:
  - **Ephemeral Git Worktree**: The agent works entirely inside an isolated worktree branch (`gaia/draft/{task_id}`).
  - **Deferred Approval Queue**: Actions flagged as requiring approval (mutations, destructive actions) are staged in a transaction log rather than blocking execution.
  - **Consolidated Review Pack**: Upon task completion, GAIA generates a structured report (diff summary, verification tests, decisions made) for one-click human approval.
- **Target Files**:
  - `internal/task/worktree.go`
  - `internal/core/kernel.go`
  - `internal/adapters/tui/diff_viewer.go`

---

## 5. Simplified Permissions & Policy Control Panel
- **Status**: Planned
- **Objective**: Replace obscure security configurations with an intuitive permissions matrix in both the Web UI and Cyberpunk TUI.
- **Architecture**:
  - **4 Functional Categories**: File Read/Write, Shell Execution, Network/Gateways, Memory Mutation.
  - **3 Simple States**: `Always Allow`, `Ask Once Per Session`, `Block`.
- **Target Files**:
  - `internal/core/policy.go`
  - `internal/adapters/web/`
  - `internal/adapters/tui/`
