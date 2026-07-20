# GAIA — Go Autonomous Intelligence Agent

## Specification v2.0

---

## 1. Vision

GAIA is a **programming-exclusive autonomous agent** written in Go. It is not "Hermes in Go" nor "Gentle-AI as an agent." It is a **multi-agent system where specialized subagents learn independently in their domain**, orchestrated by a main agent that delegates and synthesizes.

GAIA combines:
- **Hermes Agent** — Learning loop, skill creation/improvement, memory nudge, subagents
- **Gentle-AI concepts** — SDD phases (10), GGA review (4 lenses + receipts), Judgment Day protocol, Engram memory model
- **ogcode** — Knowledge graph recall for 70%+ token savings on long sessions
- **pi-go architecture** — Go-native agent structure, subagent spawning patterns

**Core philosophy:**
- **Programming-first**: No TTS, image gen, Home Assistant, Spotify, or Discord bloat. Pure coding agent.
- **Language-agnostic**: Works with any language the user chooses. Skills are installed per language.
- **Specialized learning**: Each subagent learns independently in its domain. The Designer doesn't get worse because the Implementer had a bad day with Rust.
- **Progressive skills**: Only the skill index stays in context (~3k tokens). Full skill content loads on demand.
- **Single binary**: `gaia` and done. Windows, macOS, Linux. No Python, no Node.js, no ffmpeg.

---

## 2. Why GAIA? — Core Differentiators

| What exists | What's missing | What GAIA does |
|---|---|---|
| **Hermes** (Python) | One learning loop for everything. Skills come pre-installed. No SDD. No bounded review. No knowledge graph. Full transcript replay. | Per-domain learning. Skills on demand. SDD native. Bounded review with receipts. Knowledge graph recall. |
| **Gentle-AI** (Go configurator) | Not an agent. No conversation loop. No LLM client. No tool execution. No TUI for chat. | Full autonomous agent loop. Multi-provider LLM. Native tool execution. TUI + Desktop. |
| **Hermes + Gentle-AI** configured | Runtime: Python + 5 dependencies. No knowledge graph. Skills bundled. No per-domain learning. No SDD native. | Single Go binary. Knowledge graph recall. Skills per language. Per-domain specialized learning. SDD as native workflow. |
| **pi-agent** | Minimal wrapper. No subagent specialization. No learning loop. No knowledge graph. | Full autonomous agent with specialized subagents that learn. |
| **CodeSwarm** (Python) | Pipeline only, not interactive. Requires 6 cloud services. No per-domain learning. | Interactive agent. Zero external services. Per-domain learning. |

---

## 3. Architecture Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                     GAIA (single binary)                         │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  ORCHESTRATOR — Main Agent Loop                          │    │
│  │  • Think → Act → Learn → Persist                        │    │
│  │  • Delegates to specialized subagents                   │    │
│  │  • Synthesizes results, never does the work itself      │    │
│  │  • Progressive skill index in context (~3k tokens)      │    │
│  │  • Per-turn knowledge graph recall (~500 tokens)        │    │
│  │  • Context compaction (summarize stale history)         │    │
│  └────────────────────────┬────────────────────────────────┘    │
│                           │                                      │
│  ┌────────────────────────▼────────────────────────────────┐    │
│  │  SUBAGENT SYSTEM — Autonomous & Specialized              │    │
│  │                                                          │    │
│  │  Each subagent has:                                      │    │
│  │  • Own memory namespace in Engram (topic key)            │    │
│  │  • Independent learning loop (nudge + skill creation)    │    │
│  │  • Configurable LLM model (different per subagent)       │    │
│  │  • Own skill index (only what it needs)                  │    │
│  │  • Shared knowledge graph for cross-domain concepts      │    │
│  │                                                          │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │ Explorer  │ │Proposer  │ │Specifier │ │Designer  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │Planner   │ │Implement.│ │Verifier  │ │Reviewer  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │Learner   │ │Researcher│ │Archiver  │ │Debugger  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  └────────────────────────┬────────────────────────────────┘    │
│                           │                                      │
│  ┌────────────────────────▼────────────────────────────────┐    │
│  │  INFRASTRUCTURE                                         │    │
│  │                                                          │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │LLM Multi-│ │Tool Exec │ │Memory    │ │ KG       │   │    │
│  │  │Provider  │ │Engine    │ │(Engram)  │ │Recall    │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │TUI       │ │Desktop   │ │MCP       │ │Skills    │   │    │
│  │  │Bubbletea │ │Wails     │ │Client    │ │Loader    │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  └─────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

---

## 4. Subagent System — 12 Autonomous Specialists

Each subagent is an autonomous LLM-powered agent with:
- Isolated context (never sees other subagents' intermediate work)
- Own memory namespace via Engram (topic key: `gaia/{subagent}/{domain}`)
- Independent learning loop (nudge + skill creation + improvement)
- Configurable LLM model + fallback
- Own skill index (only loads skills relevant to its domain)
- Returns only summaries to the orchestrator

### 4.1 Subagent Catalog

| Subagent | SDD Phase | Responsibility | Key Skills | Default Model Tier |
|---|---|---|---|---|
| **Explorer** | sdd-explore | Investigate codebase, find patterns, understand architecture before proposing | code-search, grep, glob, git-log | Cheap (fast) |
| **Proposer** | sdd-propose | Create change proposals with scope, approach, risks | architecture, planning | Premium |
| **Specifier** | sdd-spec | Write detailed specs with requirements, scenarios, acceptance criteria | documentation, api-design | Premium |
| **Designer** | sdd-design | Technical architecture, component design, data flow | architecture, planning | Premium |
| **Planner** | sdd-tasks | Break specs into concrete implementation tasks | planning | Cheap |
| **Implementer** | sdd-apply | Write code following specs and tasks | language-specific skills (per stack) | Standard |
| **Verifier** | sdd-verify | Run tests, validate implementation against spec | testing, debugging | Standard |
| **Reviewer** | GGA (4 lenses) | Code review: risk, resilience, readability, reliability + bounded receipts | code-review, security | Premium |
| **Learner** | n/a | Analyze usage, create/improve skills, consolidate learning | all skills (read-only) | Cheap |
| **Researcher** | n/a | Web research, documentation lookup, API discovery | web-search, web-extract | Cheap |
| **Archiver** | sdd-archive | Close completed changes, sync specs, persist final state | documentation | Cheap |
| **Debugger** | n/a | Bug analysis, root cause, fix, verify | debugging, testing | Standard |

### 4.2 Orchestrator — The Main Agent

The orchestrator is the only agent the user interacts with directly. It:
- Maintains the conversation with the user
- Maintains the progressive skill index (~3k tokens)
- Operates in explicit **mode** controlled by the user (plan / build)
- Delegates work to subagents
- Synthesizes subagent summaries into coherent responses
- Never does the subagent's work itself

**Interaction Modes:**

The user can switch between modes at any time. The orchestrator enforces the mode boundary:

| Mode | Can do | Cannot do | Comandos |
|---|---|---|---|
| **plan** | Explorer, Proposer, Specifier, Designer, Researcher, Learner | Write code, modify files, run terminal commands, execute git write ops | `/plan`, `/plan on` |
| **build** (default) | All subagents including Implementer, Verifier, Debugger, Reviewer | Nothing — full agent | `/build`, `/plan off` |

- `/plan` → Switch to plan mode. The orchestrator rejects any request to write code or run destructive commands with: "I'm in plan mode. Switch to /build to implement."
- `/build` → Switch to build mode. Full agent capabilities including file writes and terminal execution.
- `/mode` → Show current mode.

The orchestrator also auto-detects intent: if the user says "what do you think about..." it stays in whatever mode it's in. If user says "implement this" while in plan mode, it responds with the plan-mode warning and optionally offers to create a proposal.

**When to trigger SDD (in build mode):**
- User asks for a new feature or substantial change → Explorer → Proposer → ... → Archiver
- User asks a quick question → Direct response (no subagents)
- User reports a bug → Debugger → Verifier
- User asks for code review → Reviewer
- User asks for research → Researcher

### 4.3 Per-Subagent Model Configuration

Each subagent can have its own LLM provider and model:

```yaml
# ~/.gaia/config.yaml
subagents:
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    fallback: claude-haiku-3-5-20241022
    reasoning_effort: high

  implementer:
    provider: openai
    model: gpt-4o
    fallback: gpt-4o-mini
    reasoning_effort: medium

  explorer:
    provider: openrouter
    model: qwen/qwen3-30b-a3b:free
    reasoning_effort: low

  # ... etc
```

### 4.4 SDD Protocol — Shared Rules (from Gentle-AI)

> These rules are inherited from Gentle-AI's SDD workflow and are **mandatory** for all SDD-phase subagents. Losing them breaks the pipeline.

#### 4.4.1 Executor Boundary (all phases)

Every SDD phase subagent is an **executor**, not an orchestrator. Do the phase work yourself. Do NOT launch sub-subagents, do NOT delegate, and do NOT bounce work back unless the phase rules explicitly say to stop and report a blocker.

#### 4.4.2 Skill Loading (Section A)

1. The orchestrator injects a `## Skills to load before work` block with exact `SKILL.md` paths
2. The subagent reads those files BEFORE task-specific work
3. Fallback: search skill registry via `mem_search("skill-registry")` or read `.atl/skill-registry.md`
4. Skill loading is NOT delegation — it's reading files

#### 4.4.3 Artifact Retrieval (Section B — Engram Mode)

**CRITICAL**: `mem_search` returns 300-char PREVIEWS. You MUST call `mem_get_observation(id)` for EVERY artifact. Skipping this produces wrong output.

Retrieval flow:
```
STEP A — SEARCH (get IDs, run all in parallel):
  mem_search(query: "sdd/{change-name}/proposal", ...) → save ID
  mem_search(query: "sdd/{change-name}/spec", ...) → save ID

STEP B — RETRIEVE (run all in parallel):
  mem_get_observation(id: {proposal_id})  ← REQUIRED, not optional
  mem_get_observation(id: {spec_id})      ← REQUIRED, not optional
```

#### 4.4.4 Artifact Persistence (Section C — Mandatory)

Every phase that produces an artifact **MUST persist it**. Skipping this BREAKS the pipeline.

**Engram mode:**
```go
mem_save(
  title: "sdd/{change-name}/{artifact-type}",
  topic_key: "sdd/{change-name}/{artifact-type}",
  type: "architecture",
  project: "{project}",
  capture_prompt: false,  // mandatory for pipeline artifacts
  content: "{full artifact markdown}"
)
```

**OpenSpec mode:** Write file to filesystem during the phase's main step.
**Hybrid mode:** Do BOTH (file + mem_save).
**None mode:** Return result inline only. No files, no mem_save.

#### 4.4.5 Return Envelope (Section D — Mandatory)

> **CRITICAL**: The subagent's FINAL output MUST be text (the return envelope), NOT a tool call. If you need to call `mem_save`, do it BEFORE your final text response. Do NOT call `mem_session_summary`.

Every phase returns:
- `status`: `success`, `partial`, or `blocked`
- `executive_summary`: 1-3 sentence summary
- `artifacts`: list of artifact keys/paths written
- `next_recommended`: the next phase to run, or "none"
- `risks`: risks discovered, or "None"
- `skill_resolution`: how skills were loaded (`paths-injected`, `fallback-registry`, `fallback-path`, or `none`)

#### 4.4.6 Review Workload Guard (Section E)

- Default PR review budget: **400 changed lines** (additions + deletions)
- Generated goldens excluded from risk count, included in snapshot identity
- `sdd-tasks` MUST forecast whether planned work exceeds that budget
- Forecast MUST include text guard lines: `Decision needed before apply`, `Chained PRs recommended`, `400-line budget risk`
- `sdd-apply` MUST NOT start oversized work unless resolved to chained PRs or explicit `size:exception`
- Each chained PR slice: clear start, clear finish, autonomous scope, verification, rollback boundary

#### 4.4.7 Engram Naming Convention

All SDD artifacts MUST use deterministic naming:

| Field | Value |
|---|---|
| `title` | `sdd/{change-name}/{artifact-type}` |
| `topic_key` | `sdd/{change-name}/{artifact-type}` |
| `type` | `architecture` |
| `capture_prompt` | `false` (Engram v1.15.3+); omit if schema lacks it |

| Artifact Type | Produced By |
|---|---|
| `explore` | Explorer |
| `proposal` | Proposer |
| `spec` | Specifier |
| `design` | Designer |
| `tasks` | Planner |
| `apply-progress` | Implementer |
| `verify-report` | Verifier |
| `archive-report` | Archiver |

### 4.5 Per-Subagent SDD Rules (from Gentle-AI)

Each SDD subagent has specific rules inherited from the corresponding Gentle-AI SDD phase skill. Non-SDD subagents (Reviewer, Learner, Researcher, Debugger) follow their own protocols.

#### 4.5.1 Explorer (sdd-explore)

**Purpose**: Investigate codebase before committing to a change. Read existing code, understand architecture, identify patterns.

**Rules:**
- Read the actual codebase — never guess or assume patterns
- Identify entry points, module structure, conventions, dependencies
- Check test infrastructure and existing test patterns
- If exploration reveals the change is larger than expected, report estimated scope
- Return structured findings: affected areas, patterns found, risks discovered
- **Size budget**: exploration artifact under 600 words. Prefer bullet points over prose

#### 4.5.2 Proposer (sdd-propose)

**Purpose**: Create change proposals with intent, scope, approach, and risks.

**Rules (inherited from Gentle-AI sdd-propose):**
- **Before writing**: offer the user a proposal question round (3-5 questions) to clarify business understanding, edge cases, and scope boundaries
- **Capabilities section is the CONTRACT** with Specifier. Every new capability becomes a new spec file. Every modified capability becomes a delta spec.
- **Mandatory sections**: Intent, Scope (in/out), Capabilities (new/modified), Approach, Affected Areas, Risks, Rollback Plan, Dependencies, Success Criteria
- **Rollback plan is MANDATORY** — every proposal must say how to revert
- **Success criteria are MANDATORY** — measurable outcomes
- **Size budget**: under 450 words. Bullet points and tables over prose
- Artifact type: `proposal`
- Detection: `sdd/{change-name}/proposal`

#### 4.5.3 Specifier (sdd-spec)

**Purpose**: Write delta specifications with structured requirements and scenarios.

**Rules (inherited from Gentle-AI sdd-spec):**
- **Read the proposal's Capabilities section first** — it tells you exactly which spec files to create
- **RFC 2119 keywords** are mandatory: MUST/SHALL for absolute requirements, SHOULD for recommendations, MAY for options
- **Given/When/Then format** for ALL scenarios
- **Every requirement MUST have at least ONE scenario** (happy path + edge case)
- **MODIFIED requirements**: copy the ENTIRE existing requirement block (all scenarios), THEN edit. Never write partial MODIFIED blocks — they lose content at archive time
- **REMOVED requirements** MUST include Reason and SHOULD include Migration
- **RENAMED requirements** MUST state both old and new names explicitly
- **Specs describe WHAT, not HOW** — no implementation details
- Specs MUST be testable — someone should be able to write an automated test from each scenario
- **Size budget**: under 650 words. Requirement tables over narrative
- Artifact type: `spec`
- Detection: `sdd/{change-name}/spec`

**Delta Spec Structure:**
```
## ADDED Requirements  → Append to main spec at archive time
## MODIFIED Requirements → Replace matching requirement in main spec (FULL block copy-then-edit)
## REMOVED Requirements → Delete from main spec (with Reason + Migration)
## RENAMED Requirements → Rename in main spec (old name → new name)
```

#### 4.5.4 Designer (sdd-design)

**Purpose**: Create technical design with architecture decisions, data flow, file changes.

**Rules (inherited from Gentle-AI sdd-design):**
- **ALWAYS read the actual codebase** before designing — never guess file paths or patterns
- **Every decision MUST have a rationale** (the why, alternatives considered)
- Use the project's **ACTUAL** patterns and conventions, not generic best practices
- If codebase uses a different pattern than recommended, **follow the existing pattern** unless the change explicitly addresses it
- Include **concrete file paths**, not abstract descriptions
- **Threat matrix** (from `references/threat-matrix.md`): required when the design touches routing, shell commands, subprocesses, VCS/PR automation, or process integration. Mark every row `Applicable` or `N/A` with reason
- **Testing strategy** per layer (unit, integration, e2e) must be defined
- **Size budget**: under 800 words. Architecture decisions as tables
- Artifact type: `design`
- Detection: `sdd/{change-name}/design`

#### 4.5.5 Planner (sdd-tasks)

**Purpose**: Break specs and design into concrete, actionable implementation tasks.

**Rules (inherited from Gentle-AI sdd-tasks):**
- **Review Workload Forecast is MANDATORY** — estimate changed lines, 400-line budget risk, chained PR recommendation
- **Forecast MUST include exact guard lines**:
  ```
  Decision needed before apply: Yes|No
  Chained PRs recommended: Yes|No
  Chain strategy: stacked-to-main|feature-branch-chain|size-exception|pending
  400-line budget risk: Low|Medium|High
  ```
- **Tasks MUST be**: Specific (concrete file), Actionable (one logical unit), Verifiable (testable), Small (completable in one session)
- Order by dependency — Phase 1 tasks shouldn't depend on Phase 2
- If project uses TDD: RED (write failing test) → GREEN (pass) → REFACTOR
- **Size budget**: under 530 words. Checklist format, 1-2 lines per task
- Artifact type: `tasks`
- Detection: `sdd/{change-name}/tasks`

#### 4.5.6 Implementer (sdd-apply)

**Purpose**: Write code following specs, design, and tasks.

**Rules (inherited from Gentle-AI sdd-apply):**
- **ALWAYS read specs BEFORE implementing** — specs are your acceptance criteria
- **ALWAYS follow the design decisions** — don't freelance a different approach
- **ALWAYS match existing code patterns** and conventions in the project
- **Merge Protocol (CRITICAL)**: When apply-progress exists, READ it first, then MERGE your progress with existing progress. Never OVERWRITE.
- **Work Unit Evidence is MANDATORY** for every assigned batch:
  - Focused test command and exact result
  - Runtime harness command/scenario (or explicit `N/A` with reason)
  - Rollback boundary (exact files/behavior that can be reverted)
- **Before returning**: re-read the persisted tasks artifact. Confirm every completed task is marked `[x]`. If not, fix it before reporting.
- If design is wrong or incomplete, NOTE IT in your return — don't silently deviate
- If task is blocked, STOP and report back
- If workload forecast requires a decision and none was provided, STOP before writing code
- **Size budget**: no fixed limit, but prefer focused, reversible commits
- Artifact type: `apply-progress`
- Detection: `sdd/{change-name}/apply-progress`

#### 4.5.7 Verifier (sdd-verify)

**Purpose**: Validate implementation against specs, run tests, report compliance.

**Rules (inherited from Gentle-AI sdd-verify):**
- **Run actual tests** — static analysis alone is never verification
- **A spec scenario is compliant ONLY when a covering test passed at runtime**
- Compare specs first, design second, task completion third
- **Do NOT fix issues** — report them for the orchestrator/user
- **Compliance matrix**: map every spec scenario to verdict (COMPLIANT, FAILING, UNTESTED)
- **Graceful handling**:
  - Tasks only → verify task completion only (spec/design: SKIPPED)
  - Tasks + specs → verify completeness + correctness (design: SKIPPED)
  - All artifacts → verify all dimensions
- **Severity**:
  - Test command exits non-zero → CRITICAL
  - Spec scenario has no passing test → CRITICAL
  - Design deviation exists → WARNING (unless it breaks a spec)
- Strict TDD verify: load `strict-tdd-verify.md` when active
- **Final verdict**: `PASS`, `PASS WITH WARNINGS`, or `FAIL`
- Artifact type: `verify-report`
- Detection: `sdd/{change-name}/verify-report`

#### 4.5.8 Archiver (sdd-archive)

**Purpose**: Merge delta specs into main specs, close the change, persist audit trail.

**Rules (inherited from Gentle-AI sdd-archive):**
- **Native Review Receipt Gate**: Before any operation, require a valid review receipt. Missing, pending, scope-changed, invalidated, or escalated blocks archive.
- **Task Completion Gate**: If any implementation task is unchecked (`- [ ]`), STOP and block archive. Only proceed if orchestrator explicitly approves stale-checkbox reconciliation with proof from `apply-progress` and `verify-report`.
- **Delta merge**:
  - ADDED → Append to main spec
  - MODIFIED → Replace matching requirement (full block)
  - REMOVED → Delete (require Reason + Migration notes)
  - RENAMED → Rename (require explicit old/new names)
- **Move to archive**: `openspec/changes/{change-name}/ → openspec/changes/archive/YYYY-MM-DD-{change-name}/`
- **Archive is AUDIT TRAIL** — never delete or modify archived changes
- Artifact type: `archive-report`
- Detection: `sdd/{change-name}/archive-report`

#### 4.5.9 Non-SDD Subagents

| Subagent | Rules |
|---|---|
| **Reviewer** | Follows GGA protocol: 4 lenses (risk, resilience, readability, reliability). Bounded review with content-bound receipt. Never modifies code. Findings: BLOCKER / WARNING / SUGGESTION. |
| **Learner** | Analyzes subagent usage patterns. Proposes skill creation/improvement. Never modifies code or artifacts directly. Reports to orchestrator. |
| **Researcher** | Web search + extraction. Must cite sources. Never modifies code. |
| **Debugger** | Bug analysis → root cause → fix → verify. Follows scientific method: hypothesis, test, confirm. Reports fix + verification evidence. |

---

## 5. Learning Model — Hybrid

Each subagent learns independently, but cross-domain knowledge is shared.

### 5.1 Per-Subagent Memory (Engram Namespaces)

| Subagent | Engram Topic Key | What It Learns |
|---|---|---|
| Explorer | `gaia/explorer/{project}` | Codebase patterns, architecture conventions, file locations |
| Proposer | `gaia/proposer/{project}` | What proposals were accepted/rejected, scope patterns |
| Specifier | `gaia/specifier/{project}` | Requirements patterns, common edge cases, acceptance criteria |
| Designer | `gaia/designer/{project}` | Architecture decisions, trade-offs, design patterns used |
| Planner | `gaia/planner/{project}` | Task breakdown patterns, estimation accuracy |
| Implementer | `gaia/implementer/{project}` | Code patterns, common bugs, library quirks |
| Verifier | `gaia/verifier/{project}` | Test patterns, flaky tests, regression history |
| Reviewer | `gaia/reviewer/{project}` | Common code issues per team, review standards |
| Debugger | `gaia/debugger/{project}` | Bug patterns, root causes, fix strategies |

### 5.2 Shared Knowledge Graph

Cross-domain concepts are stored in a shared knowledge graph (Topic → Concept → Fact):

```
Topic: "Authentication"
├── Concept: "JWT in this project"
│   ├── Fact: "Tokens expire in 24h, refresh in 7d"  (contributed by Designer)
│   ├── Fact: "Common bug: missing token refresh on 401"  (contributed by Debugger)
│   └── Fact: "Test helper: auth.NewTestToken()"  (contributed by Verifier)
├── Concept: "User roles"
│   ├── Fact: "Roles: admin, user, viewer"  (contributed by Specifier)
│   └── Fact: "Middleware: requireRole('admin')"  (contributed by Explorer)
```

Any subagent can query the knowledge graph. The orchestrator decides what to share.

### 5.3 Learning Loop (per subagent)

```
After each subagent task:
  1. Session Summary → auto-generate domain-specific learnings
  2. Memory Nudge → persist important observations to its Engram namespace
  3. Skill Check → create/improve domain skills if needed
  4. Knowledge Graph → share cross-domain learnings

During tasks:
  1. Memory Recall → pull relevant context from its own memory
  2. Skill Load → load domain skills on demand
  3. Knowledge Graph Query → pull cross-domain context
```

---

## 6. Skill System — Progressive Loading

### 6.1 Philosophy

Skills are **NOT bundled** with GAIA. The user installs only what they need.

**On first run** (wizard):
1. GAIA asks what languages/frameworks the user works with
2. GAIA queries the Skills Hub for popular skills matching those languages
3. User selects which to install
4. Installed skills go to `~/.gaia/skills/`

**After install:**
- `gaia skills search <query>` — Search available skills
- `gaia skills install <name>` — Install a specific skill
- `gaia skills list` — List installed skills
- `gaia skills activate/deactivate <name>` — Toggle without uninstalling
- `gaia skills remove <name>` — Delete a skill

### 6.2 Progressive Disclosure

```
Context at all times (ORCHESTRATOR):
  Level 0: skills_list() → [{name, description, tags}, ...]   (~3k tokens)
            Only installed + activated skills shown

Loaded on demand (SUBAGENT LEVEL):
  Level 1: skill_view(name) → Full SKILL.md content           (varies)
  Level 2: skill_view(name, path) → Specific reference file    (varies)
```

The orchestrator only holds Level 0. When a subagent is spawned, the orchestrator passes matching skill names and the subagent loads Level 1 as needed.

### 6.3 Skills Hub

Skills come from a **decentralized hub**:
- Official GAIA skill repository (curated, per-language collections)
- Community taps (GitHub repos following the SKILL.md format)
- User-created skills in `~/.gaia/skinks/custom/`

**Skill metadata:**
```yaml
---
name: go-testing
description: "Write Go tests — table-driven, subtests, parallel, fakes"
version: 1.0.0
languages: [go]
tags: [testing, tdd]
category: development
---
```

### 6.4 Per-Subagent Skill Filtering

Each subagent only loads skills relevant to its domain:
- **Implementer**: language-specific skills (go, typescript, rust, etc.)
- **Reviewer**: code-review, security, readability skills
- **Designer**: architecture, planning skills
- **Verifier**: testing, debugging skills

---

## 7. LLM Integration — Multi-Provider with Per-Subagent Routing

### 7.1 Supported Providers

| Provider | Go Library | Status |
|---|---|---|
| Anthropic (Claude) | `github.com/anthropics/anthropic-sdk-go` | ✅ Planned |
| OpenAI (GPT) | `github.com/sashabaranov/go-openai` | ✅ Planned |
| Gemini (Google) | Custom REST client | ✅ Planned |
| Ollama (Local) | REST API | ✅ Planned |
| GitHub Copilot | Existing copilot_client.go | ✅ Existing |
| OpenRouter | OpenAI-compatible API | ✅ Planned |

### 7.2 Provider Router

```
Request → Router → Provider (based on config)
         → Fallback (on failure)
         → Streaming (SSE for TUI)
         → Tool definition conversion (per-provider format)
```

---

## 8. Token Efficiency — Knowledge Graph Recall (from ogcode)

### 8.1 The Problem

Traditional agents replay the **entire conversation** every turn. At 50 messages, ~25k tokens. At 500 messages, ~200k+ tokens. Context window exhausted.

### 8.2 GAIA's Solution

```
                  PER-TURN CONTEXT
  System Prompt (fixed)               ~2k tokens
  + Active Skills Index (Level 0)     ~3k tokens
  + Knowledge Graph Recall            ~500 tokens
  + Recent Messages (last 5)          ~2k tokens
  + Compacted Summary                 ~1k tokens
  ─────────────────────────────────────────
  TOTAL per turn:                    ~8.5k tokens

  vs. Hermes (50 msgs):              ~25k tokens
  vs. Hermes (500 msgs):             ~200k+ tokens

  SAVINGS: 70%+ on long sessions
```

### 8.3 Mechanisms

| Mechanism | How It Works | Token Impact |
|---|---|---|
| **Knowledge Graph Recall** | Per-turn: query KG for facts relevant to current task. Each fact has embedding + labels. | Largest saving — grows with session length |
| **Context Compaction** | Summarize stale messages instead of replaying verbatim. Triggered at configurable threshold. | Caps prompt size on long sessions |
| **Progressive Skills** | Level 0 (index, ~3k) → Level 1 (full skill, varies) → Level 2 (references). Only load what's needed. | Only loads relevant skills |
| **Memory Recall** | Pull precise facts from Engram instead of re-deriving. | Fewer exploration turns |
| **Session Search** | FTS5 search past sessions, no LLM calls needed. | Fast recall without context cost |

### 8.4 Token Budget Per Subagent

Each subagent has a configured token budget. When the budget is low:
1. Compact oldest messages
2. Fall back to cheaper model (if configured)
3. Return partial results with summary

---

## 9. Gentle-AI Concepts — Native Integration

### 9.1 SDD Phases → GAIA Subagents

Gentle-AI's SDD phases map directly to GAIA subagents:

| Gentle-AI SDD | GAIA Subagent | Implementation |
|---|---|---|
| `sdd-init` | Orchestrator | Bootstrap project context, detect testing capabilities |
| `sdd-explore` | Explorer | Investigate codebase before proposing |
| `sdd-propose` | Proposer | Create change proposals |
| `sdd-spec` | Specifier | Write specifications with scenarios |
| `sdd-design` | Designer | Technical design + architecture |
| `sdd-tasks` | Planner | Break down into tasks |
| `sdd-apply` | Implementer | Write code following specs |
| `sdd-verify` | Verifier | Validate implementation vs specs |
| `sdd-archive` | Archiver | Archive completed changes |
| `sdd-onboard` | Orchestrator | Guided onboarding walkthrough |

### 9.2 GGA Review → Reviewer Subagent

| Gentle-AI GGA Lens | GAIA Reviewer Mode | Focus |
|---|---|---|
| `review-risk` | Risk mode | Security, permissions, data exposure, architecture |
| `review-resilience` | Resilience mode | Fallbacks, retry, degradation, observability |
| `review-readability` | Readability mode | Naming, structure, maintainability, comments |
| `review-reliability` | Reliability mode | Tests, determinism, regressions, edge cases |

**Review Flow (bounded):**
1. Freeze candidate (snapshot of files/changes)
2. Run selected lens(es) once
3. Produce findings with severity (BLOCKER / WARNING / SUGGESTION)
4. Generate content-bound receipt (SHA256)
5. Pre-commit/pre-push validates against same receipt
6. Never re-reviews unchanged content

### 9.3 Judgment Day (Adversarial Review)

For high-risk changes (auth, security, payments, >400 lines):
1. Two independent judges (judge-a, judge-b) review blindly
2. Compare findings, resolve conflicts
3. Fix agent (fix-agent) applies surgical corrections
4. Maximum 2 rounds of fix + re-judgment

### 9.4 Review Protocol — Formal State Machine & Receipts

GAIA's review system follows the formal Gentle-AI review integration contract, adapted for native execution.

#### 9.4.1 Risk Reasons Taxonomy

When the Reviewer determines risk level, it classifies each risk reason with a code:

| Risk Code | Signal | When It Fires |
|---|---|---|
| `configuration_change` | — | Changes to config files, env vars, feature flags |
| `executable_change` | — | Changes to executable binary outputs |
| `executable_mode` | permissions | File permission mode changes (e.g., +x) |
| `hot_path` | auth/security/payments | Changes to authentication, authorization, payments, or security-critical paths |
| `large_change` | — | More than 400 changed lines |
| `non_executable_only` | — | Only documentation, comments, formatting, typo fixes (no lens needed) |
| `service_token` | auth | New or modified service account tokens, API keys embedded in code |
| `shell_source` | shell_process | New or modified shell scripts, Makefile targets, or subprocess invocations |

Risk level is determined by combining reasons:
- Only `non_executable_only` → **Low** (no lens needed)
- Any other reason → **Medium** (select one dominant lens)
- `hot_path`, `large_change`, `service_token`, or `shell_source` → **High** (run all 4 lenses)

#### 9.4.2 Review State Machine

Each review transaction progresses through formal states, tracked in Engram:

```
  unreviewed (initial state)
      │
      ▼
  reviewing (review in progress)
      │
      ├── judges_confirmed (Judgment Day judges have reported)
      │
      ▼
  findings_frozen (no more changes to findings)
      │
      ▼
  evidence_classified (each finding classified as BLOCKER/WARNING/SUGGESTION)
      │
      ├── fix_required → fixing → fix_validating (correction loop, max 1 round normal, 2 rounds Judgment Day)
      │
      ▼
  ready_final_verification
      │
      ▼
  final_verifying (running tests + build to confirm fix)
      │
      ▼
  approved (receipt issued)
      │
      ├── escalated (unresolvable — human intervention needed)
      └── invalidated (content changed since receipt — new review needed)
```

Each state transition is recorded in Engram under `gaia/review/{change-name}/{transaction-id}` for full traceability.

#### 9.4.3 Review Receipt Structure

The receipt is a content-bound artifact with SHA256 of the reviewed snapshot:

```json
{
  "schema": "gentle-ai.review-receipt/v2",
  "lineage_id": "{sha256 of the review transaction chain}",
  "snapshot_hash": "sha256:{hash of all reviewed files}",
  "selected_lenses": ["review-risk", "review-readability"],
  "risk_level": "medium",
  "correction_budget": 85,
  "correction_used": 0,
  "state": "approved",
  "final_verification_hash": "sha256:{hash of verification evidence}"
}
```

The receipt is validated at five delivery gates:
- **post-apply**: Before reporting implementation ready to orchestrator
- **pre-commit**: Before allowing git commit
- **pre-push**: Before allowing git push
- **pre-pr**: Before allowing pull request creation
- **release**: Before allowing release/tag creation

Each gate re-validates the receipt against the current content. If content has changed, the receipt is invalidated and a new review is needed.

#### 9.4.4 Mutation Journal

Config changes and review state transitions are tracked in a journal for auditability:

```
gaia journal --change {change-name}
→ Lists all state transitions, who triggered them, and timestamps
```

Every `mem_save` call for review artifacts includes the mutation journal entry automatically.

### 9.5 Engram → GAIA Memory

Engram's memory model maps directly:

| Engram Feature | GAIA Usage |
|---|---|
| `mem_save` | All subagents save learnings after tasks |
| `mem_search` | Subagents search their namespace for relevant context |
| `mem_context` | Orchestrator checks recent context at session start |
| `mem_session_summary` | Each subagent generates domain-specific summaries |
| Topic keys | Per-subagent namespaces (`gaia/{subagent}/{project}`) |
| Conflict detection | Cross-subagent memory conflicts flagged for review |
| Lifecycle review | Stale context marked for refresh |

### 9.5 Memory Export & Human Visualization (Phase 3+)

While GAIA's memory lives in Engram + Knowledge Graph (machine-native), the user may want to **read, edit, or explore** what the agent has learned. For this, GAIA can optionally export memories to a structured format viewable in any markdown editor — with Obsidian as a first-class target.

**Export structure** (generated after each session or on demand):

```
gaia-memory-export/
├── Project/
│   ├── Auth-System.md              # Facts del KG sobre auth
│   │   - Token expiration: 24h, refresh: 7d
│   │   - Common bug: missing refresh on 401
│   │   - Test helper: auth.NewTestToken()
│   ├── API-Design.md               # Decisiones de arquitectura
│   │   - Hexagonal architecture with ports/adapters
│   │   - All mutations go through service layer
│   └── Common-Bugs.md              # Bugs frecuentes y fixes
│       - N+1 query in UserList → fixed with eager loading
├── User/
│   ├── Preferences.md              # Lenguajes, frameworks, estilo
│   │   - Primary: Go, TypeScript, Rust
│   │   - Style: early returns, table-driven tests
│   │   - Personality: Teacher persona
│   └── Learning-Style.md           # Cómo prefiere que le expliquen
└── Skills/
    ├── go-testing.md               # Skill que GAIA creó o mejoró
    └── react-patterns.md
```

**How it works:**
- Export command: `gaia memory export --format obsidian --out ./gaia-memory`
- Each topic key in Engram becomes a markdown file
- KG facts become bullet points with source attribution (which subagent contributed)
- Session summaries become chronological entries
- User can **edit** any file (correct a fact, add notes)
- On next import (`gaia memory import --from ./gaia-memory`), edited files update Engram
- Obsidian graph view visualizes connections between projects, decisions, and skills

**Use cases:**
- Verify what the agent has learned about your project
- Correct incorrect memories by editing the markdown file directly
- Share memory exports with team members
- Archive project knowledge when a project concludes

**Not the primary memory backend** — this is export/visualization only. The core memory remains in Engram + Knowledge Graph for performance, conflict detection, and lifecycle management.

---

### 9.6 Persona System — Starting Points That Evolve

GAIA's persona system is fundamentally different from a static `SOUL.md` or a fixed instruction list. **The persona is a starting point, not a cage.** Each subagent's behavior evolves with experience.

> **Design principle**: A persona that tells the subagent exactly how to behave prevents learning. The persona sets initial tone and values; the learning loop refines them based on what works.

### 9.6.1 How Personas Evolve

```
Session 1:  Persona base "Strict" → "No acepto código sin tests"
               ↓
Session 10:  El subagente aprendió que en este proyecto
             los tests de integración son más valiosos que
             los unitarios para ciertos casos
               ↓
Session 50:  "Reviso que los tests cubran el happy path
             y 2 edge cases. Si es API, priorizá integración."
```

The learning loop tracks:
- What communication styles get the best results for each user
- Which feedback patterns catch more bugs
- What level of detail the user prefers per context
- When to push back vs when to comply (learned from user reactions)

### 9.6.2 Initial Persona Seeds

The user can choose a starting persona. This is the **seed** — the subagent will evolve from here:

| Persona | Seed Behavior | Can Evolve To |
|---|---|---|
| **Teacher** | Warm but firm. Explica el POR QUÉ. | Maybe discovers user prefers examples over theory → adapts |
| **Professional** | Neutral, directo, eficiente. | Maybe user responds better to encouragement → becomes warmer |
| **Strict** | Exigente. No acepta código sin tests. | Maybe learns which rules matter for THIS project → nuanced strictness |
| **Friendly** | Relajado, alentador. | Maybe user needs more direct feedback → balances friendliness with honesty |
| **Custom** | Definido por el usuario vía archivo. | User's seed gets refined by experience |

**Configuration:**

```yaml
# ~/.gaia/config.yaml
persona:
  orchestrator_seed: teacher       # starting point: teacher, professional, strict, friendly, custom
  evolution_enabled: true          # false = freeze persona, never evolve
  evolution_review: prompt         # prompt = ask before evolving, auto = evolve silently
  language: auto                   # auto-detect from user, or force: es, en, pt, etc.
  custom_file: ~/.gaia/persona.md  # only used when persona_seed: custom
```

### 9.6.3 Per-Subagent Persona Evolution (Phase 3+)

Each subagent starts with a seed persona that matches its role, **but evolves independently** based on its domain experience:

| Subagent | Seed Persona | Evolves Based On |
|---|---|---|
| **Orchestrator** | As selected by user | User reactions, correction patterns |
| **Explorer** | Curious, thorough | Which search patterns find relevant code faster |
| **Proposer** | Structured, clear | Which proposal formats get approved more |
| **Specifier** | Precise, exhaustive | Which detail level catches requirement gaps |
| **Designer** | Architect, pragmatic | Which design patterns worked vs caused rework |
| **Implementer** | Focused, productive | Which coding patterns cause fewer bugs |
| **Verifier** | Skeptical, thorough | Which test types catch regressions in this project |
| **Reviewer** | Strict, constructive | Which review comments actually prevent bugs |
| **Debugger** | Analytical, methodical | Which root cause patterns repeat in this codebase |
| **Learner** | Reflective, curious | Which skills are worth creating vs. not |
| **Researcher** | Thorough, cites sources | Which documentation sources are reliable |
| **Archiver** | Organized, consistent | Which archive format helps retrieval later |

**Example evolution:**
```
Seed: Reviewer persona = "Strict, constructive"

Session 1-5:  "This function lacks error handling (BLOCKER)"
Session 6-10: Reviewer noticed user accepts suggestions better
              when framed as questions → evolves
Session 11+:  "What happens if this function receives nil?
              Consider handling that case — last time we had
              a nil panic in similar code (ref: bug #424)"
```

### 9.6.4 The Persona File (SOUL.md Compatible)

Custom seed personas use a markdown file. Compatible with Hermes `SOUL.md` format so users can migrate existing personas:

```markdown
# GAIA Persona Seed — Senior Rustacean

## Starting Tone
- Direct and precise, like a senior Rust engineer
- Short responses unless asked for details
- This is a seed — expect it to evolve with use

## Core Values (evolve with experience)
- Correctness over speed
- Type safety is non-negotiable

## Communication Preferences
- Start with the conclusion, then justify
- Suggest safer alternatives
```

**Important**: The persona file defines the **starting point only**. After each session, the Learner subagent may propose updates to the evolved persona based on what it learned about the user's preferences.

### 9.6.5 Freezing a Persona

If the user wants the agent to STOP evolving (keep a fixed personality), they can freeze it:

```yaml
persona:
  evolution_enabled: false   # Freeze — never change behavior
  evolution_review: prompt   # or 'auto'
```

When frozen, the persona becomes a static instruction set (like traditional SOUL.md). The subagent stops tracking communication patterns and behavior optimization.

### 9.6.6 Quick Switching

```
/persona strict          → Switch seed to strict
/persona teacher         → Switch seed to teacher
/persona freeze          → Stop evolution, keep current persona
/persona unfreeze        → Re-enable evolution
/persona reset           → Reset to seed persona (clear all learned adaptations)
/persona status          → Show current persona + evolution state
/persona custom my-rust  → Load custom seed from file
```

---

## 10. User Interfaces

### 10.1 TUI (Phase 1)

Built with Bubbletea (existing GAIA codebase):
- Streaming responses with tool call rendering
- Slash commands (/explore, /design, /review, /skills, etc.)
- Conversation history with session management
- Split pane: chat + tool output
- Theme support (Rose Pine, etc.)

### 10.2 Desktop App (Phase 2)

Built with Wails (Go + webview — single binary, double-click to open):
- Same agent backend, different UI layer
- Wails embeds web UI using OS native webview (Edge on Windows, WebKit on Mac/Linux)
- No Chrome/Electron dependency
- Richer UI: syntax highlighting, diff viewer, file tree
- System tray integration
- Notifications for background tasks

### 10.3 Messaging (Phase 3+)

Telegram, Discord, Slack via MCP (after core is stable):
- Gateway pattern (same as Hermes)
- MCP client bridges messaging platforms
- Platform-specific formatting

---

## 11. Tool System

### 11.1 Tool Categories

All tools are **language-agnostic** unless marked. ~50 tools total, programming-exclusive.

| Category | Tools | Status |
|---|---|---|
| **File Operations** | read, write, edit, glob, grep, file_info, list_dir | ✅ Existing |
| **Terminal & Process** | terminal, process, pty | ✅ Existing |
| **Git Operations** | status, diff, commit, branch, log, worktree, blame | ✅ Existing |
| **Memory & KG** | memory_save, memory_search, memory_recall, session_search, knowledge_graph | ⚠️ Refactor needed |
| **Skills** | skills_list, skill_view, skill_manage, skill_search, learn | ⚠️ Redesign needed |
| **Web & Research** | web_search, web_extract, browser_navigate, browser_snapshot, browser_vision | 🔄 Planned |
| **Agent Orchestration** | delegate_task, todo, clarify, execute_code | 🔄 Planned |
| **SDD Workflow** | sdd_init, sdd_explore, sdd_propose, sdd_spec, sdd_design, sdd_tasks, sdd_apply, sdd_verify, sdd_archive, sdd_onboard | 🔄 Planned |
| **Review (GGA)** | review_risk, review_resilience, review_readability, review_reliability, review_pr, review_staged, review_file, install_hook | 🔄 Planned |
| **Judgment Day** | jd_judge_a, jd_judge_b, jd_fix | 🔄 Planned |
| **Scheduling** | cronjob (create, list, update, pause, resume, run, remove) | 🔄 Planned |
| **Config & System** | config_get, config_set, doctor | ✅ Existing |

### 11.2 Tools NOT Included

| Hermes Tool | Why Excluded |
|---|---|
| `text_to_speech` | Not programming-related |
| `image_generate` | Not programming-related |
| `video_generate` | Not programming-related |
| `vision_analyze` | Heavy — optional MCP plugin |
| `ha_*` (Home Assistant) | IoT, not programming |
| `spotify_*` | Music, not programming |
| `discord_*` | Messaging (Phase 3+) |
| `feishu_*` | Not programming |
| `yb_*` (Yuanbao) | Not programming |
| `x_search` | Not programming |
| `computer_use` | Desktop control, not programming |
| `kanban_*` | External MCP (GitHub Issues, Jira) |
| `browser_*` | MCP-only optional browser server |

---

## 12. Configuration

```yaml
# ~/.gaia/config.yaml

llm:
  default_provider: anthropic
  default_model: claude-sonnet-4-20250514

subagents:
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    fallback: claude-haiku-3-5-20241022
    reasoning_effort: high
  implementer:
    provider: openai
    model: gpt-4o
    fallback: gpt-4o-mini
    reasoning_effort: medium
  explorer:
    provider: openrouter
    model: qwen/qwen3-30b-a3b:free
    reasoning_effort: low
  verifier:
    provider: anthropic
    model: claude-haiku-3-5-20241022
    reasoning_effort: low
  reviewer:
    provider: anthropic
    model: claude-opus-4-20250514
    reasoning_effort: high

skills:
  dir: ~/.gaia/skills
  auto_install_wizard: true

memory:
  engram_enabled: true
  knowledge_graph: true
  nudge_interval: 10          # prompt memory save every N tool calls

context:
  max_history: 20
  compaction_threshold: 50
  recall_enabled: true
  kg_recall_enabled: true

terminal:
  backend: local              # local, docker, ssh
  timeout: 180

ui:
  mode: tui                   # tui, desktop
  theme: rose-pine

mcp:
  servers: []

cron: {}

security:
  requires_confirmation: true
```

---

## 13. Directory Structure

```
gaia/
├── cmd/gaia/
│   └── main.go                    # Entry point
├── internal/
│   ├── agent/                     # Agent system
│   │   ├── orchestrator/          # Main agent loop + delegation
│   │   ├── subagents/             # 12 specialized subagents
│   │   │   ├── explorer/
│   │   │   ├── proposer/
│   │   │   ├── specifier/
│   │   │   ├── designer/
│   │   │   ├── planner/
│   │   │   ├── implementer/
│   │   │   ├── verifier/
│   │   │   ├── reviewer/
│   │   │   ├── learner/
│   │   │   ├── researcher/
│   │   │   ├── archiver/
│   │   │   └── debugger/
│   │   └── base.go                # Base subagent behavior
│   ├── core/
│   │   ├── domain/                # Message, ToolCall, Skill, etc.
│   │   ├── ports/                 # Interface definitions
│   │   └── kernel.go              # Initialization
│   ├── modules/
│   │   ├── executor/              # Tool execution engine
│   │   ├── llm/                   # Multi-provider LLM client
│   │   ├── memory/                # Engram integration layer
│   │   ├── knowledge_graph/       # Topic → Concept → Fact (ogcode)
│   │   ├── skills/                # Progressive skill system
│   │   ├── learning/              # Learning loop
│   │   ├── context/               # Context compactor + assembler
│   │   ├── review/                # GGA bounded review
│   │   └── mcp/                   # MCP client
│   ├── adapters/
│   │   ├── llm/                   # Provider-specific clients
│   │   │   ├── anthropic.go
│   │   │   ├── openai.go
│   │   │   ├── gemini.go
│   │   │   ├── ollama.go
│   │   │   └── copilot.go
│   │   ├── db/                    # SQLite (existing, extend)
│   │   ├── tui/                   # Bubbletea TUI
│   │   └── desktop/               # Wails desktop app
│   └── config/
│       └── config.go
├── skills/                        # User-installed skills (runtime)
├── pkg/
├── go.mod
└── go.sum
```

---

## 14. Milestones

### Milestone 0: Foundation (Current State)

- [x] Config loading/saving (YAML)
- [x] SQLite message persistence (basic)
- [x] Bubbletea TUI with chat + Copilot wizard
- [x] GitHub Copilot OAuth + API client
- [x] Brain kernel (message→LLM→tools cycle)
- [x] Domain models + Ports/Interfaces
- [x] 20 Go-specific skills (on disk, no loader)

### Milestone 1: Core Agent Loop (Week 1-3)

- [x] Multi-provider LLM (Anthropic, OpenAI, Gemini, Ollama, Copilot)
- [x] Tool execution engine (registry + dispatch)
- [x] Refactor Copilot into multi-provider architecture
- [x] Progressive skill loader (Level 0 → Level 1)
- [x] Engram integration (via MCP)
- [ ] Knowledge graph store (Topic → Concept → Fact)
- [ ] Per-turn memory recall from KG
- [ ] Context compaction (summarize stale history)
- [x] **Iteration budget** (safety: prevent runaway agents, per-subagent caps) ← from Hermes
- [x] **Confirmation modes** (always / per-session / per-action / never + /trust commands) ← from Hermes
- [x] **Onboarding expansion**: language selection + skill recommendation in wizard ← from Hermes
- [x] **Session persistence** (save/restore conversations by session ID)
- [ ] **Undo/Retry commands** (/undo, /retry)
- [x] Tests for core loop

### Milestone 2: Subagent System (Week 4-6)

- [x] Subagent base (spawn, delegate, return summary)
- [x] Orchestrator: delegation logic + synthesis
- [x] Explorer subagent
- [x] Implementer subagent
- [x] Verifier subagent
- [x] Per-subagent model configuration
- [x] Per-subagent memory namespaces in Engram
- [x] Hook into SDD flow (trigger on substantial changes)
- [x] **Message redaction & sanitization** (redact API keys, tokens, PII) ← from Hermes
- [x] **Tool guardrails** (path security, URL safety, write approval) ← from Hermes

### Milestone 3: Learning & Skills (Week 7-9)

- [x] Learning loop (nudge, session summary, skill creation)
- [x] Per-subagent learning (independent nudge + skill creation)
- [ ] Shared knowledge graph cross-pollination
- [x] Skills Hub (search, install, activate, deactivate, remove)
- [x] Wizard: first-run language selection + skill recommendation
- [x] `gaia skills` commands
- [x] All subagents: Proposer, Specifier, Designer, Planner, Archiver, Reviewer, Debugger, Researcher, Learner
- [x] **Non-interactive / headless mode** (`gaia exec`, `gaia --json`, `gaia --quiet`)
- [x] **Output modes** (`--json`, `--verbose`, `--quiet` flags)

### Milestone 4: Review & Quality (Week 10-12)

- [x] GGA review: 4 lenses (risk, resilience, readability, reliability)
- [x] Bounded review with content-bound receipt (SHA256)
- [x] Pre-commit/pre-push gate validation
- [x] Judgment Day protocol (judge-a, judge-b, fix-agent)
- [x] AGENTS.md standards parser
- [x] `gaia review` commands

### Milestone 5: Production (Week 13-16)

- [x] Docker terminal backend
- [x] SSH terminal backend
- [x] Wails Desktop app
- [x] Cron scheduler
- [x] MCP client
- [x] Desktop notifications
- [x] `gaia doctor` — system diagnostics
- [x] SDD onboard — guided walkthrough
- [x] **Cron delivery to platforms** (scheduled tasks delivering results) ← from Hermes

### Milestone 6: Ecosystem (Week 17+)

- [x] Messaging gateway (Telegram, Discord via MCP)
- [x] Optional browser tools MCP plugin
- [x] `execute_code` RPC tool
- [x] LSP integration
- [x] Community skills taps
- [x] Plugin API
- [x] **Webhook subscriptions** (GitHub events triggering automations) ← from Hermes
- [x] **Script injection** (pre-processing scripts before agent runs) ← from Hermes

---

## 15. Security Model

GAIA is a programming agent with access to file system, terminal, git, and the web. Security is not optional. The following layers protect against accidental and malicious threats, inspired by Hermes' security modules and Gentle-AI's permission system.

### 15.1 API Key & Secret Storage

| Risk | Mitigation |
|---|---|
| Storing keys in plain text config | Keys stored in OS keychain (Windows Credential Manager, macOS Keychain, Linux secret-service) or encrypted config file |
| Keys leaked in conversation | Automatic redaction of `sk-*`, `ghp_*`, `Bearer *`, and custom patterns from messages and tool output |
| Keys leaked in tool output | Redaction engine scans all tool stdout/stderr before returning to LLM |
| Subagent access to secrets | **Secret scoping**: each subagent only sees the secrets it needs. Implementer doesn't see cloud provider keys. |

### 15.2 Tool Execution Security

| Risk | Mitigation |
|---|---|
| Shell injection via tool arguments | All shell commands use parameterized execution (no string interpolation). Path and argument validation before execution. |
| Path traversal (accessing files outside project) | `path_security`: All file operations resolve and validate paths against an allowed root. Symlink resolution. |
| URL safety (SSRF, malicious endpoints) | `url_safety`: Validate URLs before fetch. Block private IP ranges, localhost, internal services. |
| Dangerous commands (rm -rf, dd, etc.) | `threat_patterns`: Known dangerous command patterns are flagged and require explicit override. |
| Fork bombs / resource exhaustion | Iteration budget caps per subagent. Timeout per command. Max output size limit. |

### 15.3 Skill Security

| Risk | Mitigation |
|---|---|
| Malicious skill in SKILL.md | **Skill provenance**: track origin (official hub, community tap, user-created). **AST audit**: parse skill files for dangerous patterns before loading. |
| Skill exfiltration (reading files and sending them) | Skills run in a restricted context. Read/write scope defined in skill metadata. Network access gated by tool permissions. |
| Skill privilege escalation | Skills cannot modify other skills or GAIA's own config. Skills cannot disable security features. |

### 15.4 Confirmation & Approval

| Level | Behavior |
|---|---|
| **always** (default) | Confirm every dangerous operation (write, exec, git push, install) |
| **per-session** | Confirm once per session, then trust for the duration |
| **per-action** | Confirm only the current action |
| **never** | No confirmations (YOLO/CI mode) |

In-session trust commands: `/trust session`, `/trust once`, `/trust always`, `/trust never`.

### 15.5 Communication Security

| Risk | Mitigation |
|---|---|
| Man-in-the-middle on LLM API calls | TLS required for all provider connections. Certificate verification enabled. |
| MCP server security | MCP OAuth support for authenticated servers. Server capability whitelist. |
| Webhook HMAC | Webhook subscriptions verified via HMAC signature. |

### 15.6 Git Security

| Risk | Mitigation |
|---|---|
| Committing secrets | Pre-commit hook checks for credentials, tokens, keys using GGA patterns. |
| Force push | Disabled by default. Requires explicit override. |
| Committing to protected branches | Blocked unless explicitly overridden by user. |

### 15.7 Security Audit Commands

```bash
gaia doctor              # Check security config, key storage, permissions
gaia audit secrets       # Scan project for committed secrets
gaia audit skills        # Scan installed skills for dangerous patterns
gaia security log        # Show security-relevant events (approvals, denials)
```

### 15.8 Configuration

```yaml
security:
  confirmation_mode: always          # always, per-session, per-action, never
  secret_redaction: true             # Auto-redact keys from messages
  path_restriction: true             # Restrict file access to project root
  url_validation: true               # Validate URLs before fetch
  skill_audit: true                  # AST-audit skills before loading
  allowed_hosts:                     # URL whitelist (empty = no restriction)
    - github.com
    - api.github.com
  blocked_paths:                     # Never allow access to these paths
    - ~/.ssh
    - ~/.gnupg
  keychain: auto                     # auto, os-keychain, encrypted-file, plaintext
```

---

## 16. Go Dependencies

```go
// go.mod
module gaia

go 1.26+

require (
    // LLM Clients
    github.com/sashabaranov/go-openai              // OpenAI + OpenRouter
    github.com/anthropics/anthropic-sdk-go          // Anthropic Claude
    
    // TUI
    github.com/charmbracelet/bubbletea               // Terminal UI
    github.com/charmbracelet/lipgloss                // Styling
    github.com/charmbracelet/bubbles                 // Components
    
    // Database
    modernc.org/sqlite                              // Pure Go SQLite
    
    // MCP
    github.com/mark3labs/mcp-go                     // MCP client
    
    // Desktop
    github.com/wailsapp/wails/v3                    // Desktop app (Phase 2)
    
    // CLI
    github.com/spf13/cobra                          // CLI
    github.com/spf13/viper                          // Config
    
    // Utilities
    github.com/google/uuid                          // UUID generation
    github.com/charmbracelet/glamour                // Markdown rendering
)
```

---

## 18. Comparison: GAIA vs Hermes vs Gentle-AI

| Feature | Hermes (Python) | Gentle-AI (Go) | GAIA (Go) |
|---|---|---|---|
| **Type** | Autonomous agent | Ecosystem configurator | Autonomous agent |
| **Binary** | Python + uv + Node.js | Go binary (config CLI) | Single Go binary |
| **Memory** | FSRS + Honcho | Engram (MCP server) | Engram (native) |
| **Token Efficiency** | Full transcript replay | N/A | Knowledge graph recall (70%+) |
| **SDD Workflow** | No (external skill) | Configures for other agents | Native subagent phases |
| **Code Review** | No (background review) | GGA (bash CLI) | Built-in GGA + 4 lenses |
| **Subagents** | Generic delegation | No | 12 specialized, per-domain learning |
| **Learning Loop** | Single, all domains | No | Per-subagent independent learning |
| **Skills** | 40+ bundled | Registers for other agents | Per-language, user-installed, progressive |
| **Skill Creation** | Manual | No | Agent creates + improves skills |
| **Knowledge Graph** | No | No | Yes (Topic → Concept → Fact) |
| **Desktop** | No | No | Wails (Phase 2) |
| **Multi-Provider** | Yes | No | Yes + per-subagent model config |
| **Persona** | SOUL.md | Persona system | Persona system |
| **MCP** | Client + Server | Configures for others | Client |

---

## 19. Hermes Features for Future Consideration

These are features from Hermes that GAIA does NOT include in the initial milestones but should track for future consideration. Some are programming-relevant, others are ecosystem features to add once the core is stable.

### 19.1 Mixture of Agents (MoA)

Hermes supports **Mixture of Agents** — running multiple models cooperatively on the same task:
- A router model decomposes the task
- Specialist models execute subtasks in parallel
- A synthesis model combines results
- Tracing and debugging for MoA flows

**GAIA status**: ✅ **Implemented** — MoA is built into the Spawner via `moaRunner`. When a subagent has `moa.enabled: true` in its config, the first LLM call fans out to multiple models in parallel (goroutines + 30s timeout), collects responses, and synthesizes them via the primary model. Subsequent tool-calling iterations use a single model for coherence. Configurable per subagent (including dynamic subagents created via `/create-agent`). Orchestrator never uses MoA. See `internal/agent/moa.go`.

**Config example:**
```yaml
subagents:
  implementer:
    provider: openai
    model: gpt-4o
    moa:
      enabled: true
      models:
        - provider: anthropic
          model: claude-sonnet-4-20250514
        - provider: google
          model: gemini-2.5-pro
```

### 19.2 Credential Pool & Provider Rotation

Hermes has an extensive credential management system:
- Multi-credential pool per provider (e.g., 3 API keys for OpenAI, round-robin)
- Automatic failover on 429/401/402
- Cooldown timers per credential (5min for 401, 1h for 429)
- OAuth token refresh with single-use refresh token protection
- Cross-process synchronization for shared auth stores
- Custom provider endpoints (OpenAI-compatible)

**GAIA status**: ✅ **Implemented** — `CredentialPool` wraps any `LLMProvider` with multi-key rotation and cooldown. Configure multiple API keys per provider in `credential_pools` config section. The pool tracks per-key rate limits (429), auth errors (401), and quota errors (402) with configurable cooldown timers. Round-robin selection skips cooldowned keys. Falls back through all keys before returning the error. See `internal/adapters/llm/pool.go`.

**Config example:**
```yaml
credential_pools:
  openai:
    - key: "sk-1..."
    - key: "sk-2..."
    - key: "sk-3..."
  anthropic:
    - key: "sk-ant-..."
    - key: "sk-ant-..."
```

### 19.3 Iteration Budget

Hermes enforces a **per-agent iteration budget** (thread-safe):
- Parent agent: default 90 iterations
- Subagents: default 50 iterations
- `execute_code` iterations are refunded (don't count toward budget)
- Prevents runaway agents

**GAIA relevance**: Critical safety feature. Should be in Phase 1 or Phase 2.

### 19.4 Context Breakdown & Visualization

Hermes provides a **live context window breakdown** for the UI:
- Shows exact token usage per category (system prompt, tools, skills, MCP, memory, conversation)
- Estimates token counts using char/4 heuristic
- Color-coded categories for visual debugging
- Percentage of context limit used

**GAIA status**: ✅ **Implemented** — Context usage breakdown via `core.GetUsageStats()` and `core.FormatUsage()`. Shows token estimate per category (system prompt, tools, skills, KG context, conversation), total vs model context window, model/provider name, and iteration budget. Includes known context window sizes for 20+ common models (GPT-4o, Claude, Gemini, Llama, etc.). Unknown models default to 128k. Accessible via `/usage` TUI command. See `internal/core/usage.go`.

### 19.5 Skill Usage Tracking & Analytics

Hermes tracks per-skill usage metrics:
- How many times each skill was loaded
- Success/failure rates per skill
- Which tools each skill uses most
- Skill effectiveness over time

**GAIA relevance**: Important for the skills hub and recommending skills to users. Track for Phase 3.

### 19.6 Skill Provenance & Security

Hermes verifies skill origins:
- Tracks where each skill was installed from (hub URL, GitHub tap, local)
- Cryptographic verification for official skills
- AST-level audit of skill code before execution
- Security guard against dangerous patterns (exfiltration, file access outside scope)

**GAIA relevance**: Critical for a user-installable skills system. Track for Phase 3.

### 19.7 Message Redaction & Sanitization

Hermes automatically redacts sensitive information:
- API keys, tokens, passwords in messages and tool outputs
- PII detection (emails, IPs, phone numbers)
- Configurable redaction patterns
- Secret scoping (which subagents can access which secrets)

**GAIA relevance**: Important safety feature. Track for Phase 2.

### 19.8 Checkpoint & Rollback

Hermes checkpoints agent state for recovery:
- Periodic state snapshots
- Rollback on subagent failure
- Recovery from partial execution

**GAIA relevance**: Valuable for long-running tasks and SDD apply phases. Track for Phase 3.

### 19.9 Tool Search & Discovery

Hermes allows searching available tools by:
- Name, description, category
- Usage frequency
- Recently used
- Fuzzy matching

**GAIA relevance**: Useful for the orchestrator to discover what tools subagents have. Track for Phase 2.

### 19.10 Prompt Caching

Hermes caches LLM responses:
- Identical prompts return cached responses
- Configurable TTL per provider
- Cache invalidation on context changes

**GAIA relevance**: Token and cost savings. Track for Phase 3.

### 19.11 Rate Limiting & Cost Tracking

Hermes tracks:
- Per-provider rate limits (tokens/min, requests/min)
- Usage costs per session/project/month
- Budget alerts and caps
- Billing views for the user

**GAIA relevance**: Important for production users managing API costs. Track for Phase 3.

### 19.12 Onboarding & First-Run Experience

Hermes has a comprehensive onboarding flow:
- First-run setup wizard (`hermes setup`)
- Guided model configuration
- Tool configuration presets
- Migration from OpenClaw

**GAIA relevance**: GAIA already has a basic wizard (Copilot auth). Expand for Phase 1 to include language selection + skill recommendation.

### 19.13 MCP OAuth

Hermes supports OAuth-based authentication for MCP servers:
- Dashboard OAuth flow
- Per-server credential management
- Token refresh for MCP connections

**GAIA relevance**: Track for Phase 4 when MCP client is implemented.

### 19.14 Tool Guardrails & Security Policies

Hermes enforces tool execution policies:
- Threal patterns detection
- Tirith security policy engine
- OSV vulnerability checking for dependencies
- Path security (no traversal outside workspace)
- URL safety validation
- Write approval workflows

**GAIA relevance**: Critical for a programming agent that runs shell commands. Integrate with GGA review. Track for Phase 2.

### 19.15 Non-Interactive / Headless Mode

Multiple agents (Claude Code, Codex CLI, Hermes) support running without a TUI:

```bash
# Execute a task and return result immediately, then exit
gaia exec "refactor esta función para usar early returns"
gaia exec "cuántos TODOs hay?" --json     # Output as JSON for scripting
gaia exec "arregla este bug" --dry-run     # Show what would be done, don't execute
gaia exec "commit los cambios" --quiet     # Minimal output
```

- No TUI, no streaming, no interactive prompts
- Useful for CI/CD pipelines, git hooks, editor integration, shell aliases
- Must respect confirmation mode (see 17.20) — if session says "always ask", headless mode blocks
- Combine with iteration budget to prevent runaway

**GAIA relevance**: Track for Phase 2 (after core loop is stable).

### 19.16 Confirmation Modes (per-session trust level)

Confirming every tool call is tedious. GAIA supports flexible trust levels:

| Mode | Behavior | Best for |
|---|---|---|
| **always** (default) | Ask before every dangerous operation | New users, learning the agent |
| **per-session** | Ask once: "Trust this session?" All subsequent ops auto-confirm | Daily coding sessions |
| **per-action** | Confirm only the current action. Next action asks again | One-off commands |
| **never** (YOLO) | Never ask. All operations execute without confirmation | CI/CD, automation, experienced users |

```yaml
security:
  confirmation_mode: always    # always, per-session, per-action, never
```

In-session commands:
```
/trust session      → Trust all actions this session
/trust once         → Trust only the next action
/trust always       → Revert to always-ask mode
/trust never        → YOLO mode — no confirmations
```

Headless mode (`gaia exec`) respects confirmation mode: `always` blocks headless, `never` allows free execution.

**GAIA relevance**: Track for Phase 1 (critical UX — without this, every tool call asks for confirmation and the agent is unusable for real work).

### 19.17 Webhook Subscriptions (Automation Triggers)

Hermes supports **webhook subscriptions** that trigger agent actions on external events:
- GitHub events (PR opened, issue created, push, etc.)
- Custom webhooks (API-triggered automations)
- Each subscription has: event filter, prompt, skills to load, delivery target
- HMAC auth for webhook security

```
hermes webhook subscribe pr-review \
  --events "pull_request" \
  --prompt "Review PR #{pull_request.number}" \
  --skills "github-code-review" \
  --deliver github_comment
```

**GAIA relevance**: Powerful for automation workflows. Track for Phase 4 (after cron is stable).

### 19.18 Script Injection (Pre-Processing)

Hermes allows running a **pre-processing script** before the agent executes:
- Script output becomes context for the agent
- Handles mechanical work (fetching, diffing, computing)
- Agent handles reasoning on the result
- `[SILENT]` pattern: if script detects no change, agent skips notification

```bash
hermes cron create "every 1h" \
  "If CHANGE DETECTED, summarize. If NO_CHANGE, respond with [SILENT]." \
  --script ~/.hermes/scripts/watch-site.py
```

**GAIA relevance**: Useful for monitors, watchers, and periodic checks. Track for Phase 4.

### 19.19 Session Restore & Undo

Multiple agents support:
- `gaia session restore <id>` — resume a previous conversation by session ID
- `/undo` — undo the last turn (useful when agent goes in wrong direction)
- `/retry` — re-run the last turn with a different approach
- Conversation branching — fork a session at any point

**GAIA relevance**: `/undo` and `/retry` are useful UX features for Phase 1. Session restore for Phase 2.

### 19.20 Model Reuse Across Subagents with Different Reasoning Effort

GAIA allows configuring different models per subagent, but also supports using the **same model** across all subagents with **different reasoning_effort** levels. This covers users who have a single subscription/model:

```yaml
subagents:
  # All use Claude Sonnet, but with different effort levels
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: high       # Deep thinking for architecture
  implementer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: medium      # Balanced for coding
  explorer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: low         # Quick scans
```

This is already supported by the architecture (just repeat the same model). Explicitly documented here so it's not forgotten.

### 19.21 Features Out of Initial Scope (reconsider later)

These Hermes features are **not in GAIA's initial scope**. Some are intentionally excluded (not programming-related), others are deferred to later phases. This list exists so decisions are explicit and can be revisited as GAIA evolves.

> **Note**: "Out of scope" does NOT mean "never". If GAIA gains a plugin/MCP ecosystem, any of these could be added as optional extensions without changing the core agent.

| Feature | Status | Why | Future Path |
|---|---|---|---|
| `text_to_speech` | ❌ Out of scope | Not programming-related | Optional MCP plugin |
| `image_generation` | ❌ Out of scope | Not programming-related | Optional MCP plugin (FAL, etc.) |
| `video_generation` | ❌ Out of scope | Not programming-related | Optional MCP plugin |
| `voice_mode` | ❌ Out of scope | Not programming-related | Optional MCP plugin |
| `transcription` | ❌ Out of scope | Not programming-related | Optional MCP plugin |
| `homeassistant_*` | ❌ Out of scope | IoT, not programming | Community plugin |
| `spotify_*` | ❌ Out of scope | Music, not programming | Community plugin |
| `feishu_*` | ❌ Out of scope | Messaging, defer to platform MCP | Community MCP server |
| `yuanbao_*` | ❌ Out of scope | Tencent-specific, not programming | Community plugin |
| `x_search` | ❌ Out of scope | Twitter/X not core to programming | Optional MCP plugin |
| `computer_use` | ❌ Out of scope | Desktop control, not programming | Optional MCP plugin |
| `kanban_*` | 🔄 Deferred | Use external MCP (GitHub Issues, Jira, Linear) | MCP server integration (Phase 4) |
| `project_*` | 🔄 Deferred | Desktop workspace management | Phase 5+ |
| `read_terminal` | 🔄 Deferred | Desktop GUI specific | Phase 5+ (Desktop app feature) |
| `discord_*` | 🔄 Deferred | Messaging platform | Phase 3+ via MCP gateway |
| `messaging_gateway` | 🔄 Deferred | Telegram, Discord, Slack, WhatsApp, Signal | Phase 3+ via MCP |

---

## 21. Open Questions

| Question | Resolution | Rationale |
|---|---|---|
| **Browser tools** | MCP-only, optional | Heavy dependency (Playwright/Chrome). Plugin if needed. |
| **Messaging** | Phase 3+ | TUI + Desktop first. Telegram/Discord via MCP after core is stable. |
| **Image generation** | NO | Not programming-related. |
| **Skills Hub format** | SKILL.md + metadata | Compatible with agentskills.io standard. |
| **Desktop framework** | Wails | Single binary, no Electron/Chrome, native webview. |
| **Embeddings** | Local (SQLite + vector extension) or lightweight model | No external vector DB needed. |

---

*Spec version: 2.0*
*Last updated: 2026-07-20*

