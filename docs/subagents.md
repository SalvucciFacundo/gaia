# Subagent System

GAIA has **12 specialized subagents**, each autonomously learning in its domain. They are divided into two groups: **SDD subagents** (pipeline-coupled) and **on-demand subagents** (dispatched by user request).

---

## How Subagents Work

Every subagent follows the same pattern:

```
Orchestrator delegates → Spawner creates isolated Brain
                         → Filters tools by domain
                         → Injects Engram namespace
                         → Runs subagent with system prompt
                         → Subagent returns structured summary
                         → Orchestrator synthesizes into response
```

Each subagent:
- Has its **own LLM calls** (doesn't share context with other subagents)
- Has its **own tool set** (filtered by domain)
- Has its **own Engram namespace** (`gaia/{subagent}/{project}`)
- Has its **own learning loop** (nudge, session summary, skill creation)
- Can have its **own LLM model config** (provider, model, reasoning_effort)
- Returns only a **summary** to the orchestrator (intermediate work never enters main context)

---

## SDD Subagents

These 8 subagents form the SDD pipeline. They run sequentially for substantial changes.

### Explorer

**Role**: Investigate codebase before proposing changes.
**Tools**: Read-only (file_read, file_list, git_status, git_log, git_diff)
**Trigger**: Before any SDD change
**Learns From**: Which search strategies find relevant code faster

```go
subagent "explorer" {
  tools: [file_read, file_list, git_status, git_log, git_diff, glob, grep]
  system_prompt: "You are an explorer. Investigate the codebase to understand
    current patterns, architecture, and conventions before proposing changes.
    Report findings, not opinions."
}
```

### Proposer

**Role**: Create structured change proposals.
**Tools**: Read-only (same as Explorer)
**Trigger**: After exploration
**Learns From**: Which proposal formats get approved more

### Specifier

**Role**: Write detailed specifications with scenarios.
**Tools**: Read-only + write files
**Trigger**: After proposal approval
**Learns From**: Which detail level catches requirement gaps

### Designer

**Role**: Design technical architecture.
**Tools**: Read-only
**Trigger**: After specs are written
**Learns From**: Which design patterns cause less rework

### Planner

**Role**: Break work into concrete tasks.
**Tools**: Read + shell
**Trigger**: After design
**Learns From**: Which task sizes are most accurate

### Implementer

**Role**: Write code following specs and design.
**Tools**: Full (read, write, edit, shell, git)
**Trigger**: After tasks are defined
**Learns From**: Which coding patterns cause fewer bugs

### Verifier

**Role**: Run tests, validate against specs.
**Tools**: Shell + read (NO write)
**Trigger**: After implementation
**Learns From**: Which test types catch regressions

### Archiver

**Role**: Close completed changes.
**Tools**: Read + write
**Trigger**: After verification passes
**Learns From**: Which archive format helps retrieval

---

## On-Demand Subagents

These 4 subagents are dispatched on demand by user request or keyword detection.

### Reviewer

**Role**: Code review using BR's 4 lenses.
**Tools**: Read-only
**Trigger**: `/review` command or `gaia review start`
**Learns From**: Which review comments prevent bugs

The reviewer runs the BR engine:
1. Classify risk (8 codes → low/medium/high)
2. Select lenses (none/1/all 4)
3. Run lens analyzers (each LLM-based)
4. Freeze findings
5. Generate bounded receipt (SHA256)
6. Validate at delivery gates

### Debugger

**Role**: Root cause analysis, fix, verify.
**Tools**: Full (read, write, shell, git)
**Trigger**: `/debug` command or bug report detection
**Learns From**: Which bug patterns repeat

Follows: `analyze → root_cause → fix → verify`

### Researcher

**Role**: Web search, documentation lookup, API discovery.
**Tools**: Read + shell (web search via curl/wget)
**Trigger**: `/research` command or research intent detection
**Learns From**: Which documentation sources are reliable

### Learner

**Role**: Analyze usage patterns, propose skill creation/improvement.
**Tools**: Read-only
**Trigger**: Background (periodic), not user-facing
**Learns From**: Which skills are worth creating

Produces SKILL PROPOSAL format:
```yaml
name: proposed-skill
description: "Trigger: {triggers}. What it does."
rationale: "Noticed pattern: {evidence}"
```

---

## Model Assignment

Each subagent can use a different LLM provider and model:

```yaml
subagents:
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: high
  implementer:
    provider: openai
    model: gpt-4o
    reasoning_effort: medium
  explorer:
    provider: openrouter
    model: qwen/qwen3-30b-a3b:free
    reasoning_effort: low
```

If all subagents use the same model, just configure different `reasoning_effort`:

```yaml
subagents:
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: high       # Deep thinking
  explorer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    reasoning_effort: low        # Quick scans
```

---

## Learning Loop

Each subagent has an independent learning loop:

```
After every task:
  1. Nudge (after N tool calls) → "What did you learn?"
  2. Session summary → domain-specific learnings
  3. Skill check → propose new skill or improvement
  4. Cross-pollinate → share cross-domain facts to knowledge graph

During tasks:
  1. Memory recall → pull relevant context from own Engram namespace
  2. Skill load → load domain skills on demand
  3. Knowledge graph query → pull cross-domain concepts
```

