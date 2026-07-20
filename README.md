# GAIA — Go Autonomous Intelligence Agent

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-376%20passing-brightgreen)]()
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)]()

**GAIA is a programming-first autonomous agent** written in Go. It combines the best of Hermes Agent's learning loop, Gentle-AI's SDD workflow and bounded review, ogcode's knowledge graph token efficiency, and pi-go's Go-native architecture — into a **single binary** that runs on Windows, macOS, and Linux.

> **Language-agnostic**: GAIA is a programming agent, not a Go agent. It works with any language, any project, any stack.

---

## ✨ Features

### 🧠 12+ Specialized Autonomous Subagents
Each subagent learns independently in its domain. They don't just follow instructions — they improve with use. Talk directly to any subagent with `@name` — no orchestrator needed.

| Subagent | Type | Role | Learns From |
|---|---|---|---|
| **Explorer** | SDD | Investigates codebase patterns | Which search strategies find relevant code faster |
| **Proposer** | SDD | Creates change proposals | Which proposal formats get approved more |
| **Specifier** | SDD | Writes detailed specifications | Which detail level catches requirement gaps |
| **Designer** | SDD | Designs technical architecture | Which design patterns cause less rework |
| **Planner** | SDD | Breaks work into tasks | Which task sizes are most accurate |
| **Implementer** | SDD | Writes code following specs | Which coding patterns cause fewer bugs |
| **Verifier** | SDD | Runs tests against specs | Which test types catch regressions |
| **Archiver** | SDD | Closes and archives changes | Which archive format helps retrieval later |
| **Reviewer** | On-demand | GGA code review (4 lenses) | Which review comments prevent bugs |
| **Debugger** | On-demand | Root cause analysis + fix | Which bug patterns repeat |
| **Researcher** | On-demand | Web search + documentation | Which documentation sources are reliable |
| **Learner** | Background | Creates and improves skills | Which skills are worth creating |

### 📋 Spec-Driven Development (SDD)
SDD is the planning layer for substantial changes — built into GAIA's core, not an external tool.

```
explore → propose → spec → design → tasks → apply → verify → archive
```

Each phase is a specialized subagent with its own memory, its own learning loop, and its own configurable LLM model. The orchestrator delegates, the subagents work, and only summaries return to context.

### 📝 GGA Code Review
Built-in bounded code review with 4 lenses:

| Lens | Focus |
|---|---|
| **Risk** | Security, permissions, data exposure, architecture |
| **Resilience** | Fallbacks, retry, graceful degradation, observability |
| **Readability** | Naming, structure, maintainability, comments |
| **Reliability** | Tests, determinism, regressions, edge cases |

Reviews produce content-bound receipts (SHA256). Pre-commit, pre-push, and pre-PR gates validate against the same receipt. No silent re-reviews.

### ⚡ Token Efficiency (Knowledge Graph)
Traditional agents replay the entire conversation every turn. GAIA uses a knowledge graph (Topic → Concept → Fact) to recall only relevant context per turn — saving **70%+ tokens** on long sessions.

```
  Traditional:  25k tokens at 50 messages → 200k+ at 500 (context overflow)
  GAIA:         ~8.5k tokens per turn, always within context
```

### 🛠️ Progressive Skills
Skills are **not bundled** with GAIA. You install what you need for your stack:

```bash
gaia skills install go          # Install Go-specific skills
gaia skills install typescript  # Install TypeScript/Node.js skills
gaia skills list                # See installed skills
gaia skills activate go-testing # Activate a skill
```

The skill index stays in context (~3k tokens). Full skill content loads on demand. No bloat.

### ⏩ Async Background Tasks
All subagent execution runs in the background. Send a task, get a TaskID, and keep chatting while it works:

```
> @explorer investigate this repo
✓ Task abc-123 started (explorer)
> @debugger analyze this crash log
✓ Task def-456 started (debugger)
> tasks
  abc-123  explorer  running  00:32
  def-456  debugger  running  00:12
> cancel abc-123
✓ Task abc-123 cancelled
```

The orchestrator never blocks. PipelineRunner chains SDD phases asynchronously. TaskManager tracks lifecycle (pending → running → completed/failed/cancelled) with real-time TUI updates.

### 🧬 Dynamic Subagents (`/create-agent`)
Create new subagents at runtime — no recompilation needed:

```
> /create-agent
  → Name: "documentarian"
  → Description: "Writes project documentation"
  → Tools: file_read, file_list, git_log
  → Personality: "Clear, concise, technical English"
✓ Subagent 'documentarian' created. Type @documentarian to chat.
```

Each dynamic subagent is compiled Go code with its own system prompt, tool filter, and Engram namespace for independent learning. Persisted to SQLite — survives restarts.

### 🔭 Gentle AI Upstream Tracker
GAIA was inspired by [Gentle AI](https://github.com/Gentleman-Programming/gentle-ai). The tracker keeps parity visible:

```bash
gaia tracker check     # Check for new features in latest release
gaia tracker report    # Full port status table
gaia tracker port <id> # Generate issue for unported feature
```

The manifest at `tracker/manifest.yaml` maps 22 features with port status (ported/partial/not-ported/not-applicable). Automated release monitoring with ETag caching.

### 🔐 Security Built-In

| Layer | Protection |
|---|---|
| **API Keys** | OS keychain storage, automatic redaction |
| **Tool Execution** | Path traversal protection, URL safety, command allowlist |
| **Skills** | Provenance tracking, AST audit before loading |
| **Confirmation** | 4 modes: always / per-session / per-action / never |
| **Secrets** | 8 redaction patterns (API keys, PEM, JWT, AWS tokens) |

---

## 🚀 Quick Start

### Install

```bash
# Download the latest binary for your platform
# (or build from source with Go 1.26+)
go install github.com/SalvucciFacundo/gaia/cmd/gaia@latest
```

### First Run

```bash
gaia
```

On first run, GAIA will:
1. Detect your project language
2. Open the setup wizard
3. Recommend and install relevant skills
4. Start the interactive TUI

### Configure LLM Provider

```yaml
# ~/.gaia/config.yaml
llm:
  default_provider: anthropic
  default_model: claude-sonnet-4-20250514

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

### Hello World

```bash
gaia exec "explain this codebase" --json
gaia skills install go
gaia
```

---

## 📖 Documentation

| Topic | Guide |
|---|---|
| **CLI Commands** | [docs/cli.md](docs/cli.md) |
| **SDD Workflow** | [docs/sdd.md](docs/sdd.md) |
| **Architecture** | [docs/architecture.md](docs/architecture.md) |
| **Subagent System** | [docs/subagents.md](docs/subagents.md) |
| **Review System** | [docs/review.md](docs/review.md) |
| **Token Efficiency** | [docs/token-efficiency.md](docs/token-efficiency.md) |
| **Skills Hub** | [docs/skills.md](docs/skills.md) |
| **Security** | [docs/security.md](docs/security.md) |
| **Configuration** | [docs/configuration.md](docs/configuration.md) |
| **Persona System** | [docs/persona.md](docs/persona.md) |

---

## 🏗️ Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                         GAIA CLI                             │
│  gaia | gaia exec | gaia review | gaia skills               │
│  gaia cron | gaia doctor | gaia tracker                     │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────────┐
│  ORCHESTRATOR — Main Agent Loop                              │
│  • Think → Act → Learn → Persist                             │
│  • Delegates to specialized subagents (async, non-blocking)  │
│  • @name direct subagent routing                             │
│  • Progressive skill index + knowledge graph recall           │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────────┐
│  12+ SUBAGENTS — Autonomous & Runtime-Extensible            │
│                                                              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│  │Explorer │ │Proposer │ │Specifier│ │Designer │  SDD       │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│  │Planner  │ │Implement│ │Verifier │ │Archiver │  SDD       │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│  │Reviewer │ │Debugger │ │Researche│ │Learner  │  On-demand │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘            │
└──────────────────────────┬───────────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────────┐
│  INFRASTRUCTURE                                              │
│  LLM Router  │  Tool Engine  │  Engram  │  KG  │  MCP       │
│  TUI/Desktop │  Gateway      │  Skills  │ Cron │  Webhooks  │
│  Tracker     │  Dynamic Subagents                            │
└──────────────────────────────────────────────────────────────┘
```

---

## 🆚 Comparison

| Feature | Hermes (Python) | Gentle-AI (Go) | **GAIA (Go)** |
|---|---|---|---|
| **Type** | Autonomous agent | Ecosystem configurator | **Autonomous agent** |
| **Binary** | Python + uv + Node.js | Go binary (config CLI) | **Single Go binary** |
| **Memory** | FSRS + Honcho | Engram (MCP server) | **Engram (native)** |
| **Token Efficiency** | Full transcript replay | N/A | **Knowledge graph (70%+)** |
| **SDD Workflow** | No (external skill) | Configures for others | **Native subagent phases** |
| **Code Review** | No (background) | GGA (bash CLI) | **Built-in GGA + 4 lenses** |
| **Subagents** | Generic delegation | No | **12+ specialized + dynamic, learn per-domain** |
| **Learning Loop** | Single, all domains | No | **Per-subagent independent** |
| **Skills** | 40+ bundled | Registers for others | **Per-language, user-installed** |
| **Knowledge Graph** | No | No | **Yes (Topic→Concept→Fact)** |
| **Desktop** | No | No | **Wails** |
| **Async Tasks** | No | No | **TaskManager + @name routing** |
| **Dynamic Subagents** | No | No | **Runtime /create-agent** |
| **Upstream Tracking** | No | No | **`gaia tracker` CLI** |
| **Multi-Provider** | Yes | No | **Yes + per-subagent config** |
| **Persona** | SOUL.md | Persona system | **Seed + evolution** |

---

## 📊 Project Stats

| Metric | Value |
|---|---|
| **Language** | Go 1.26 |
| **Architecture** | Hexagonal (ports & adapters) |
| **Tests** | 415+ (unit + integration) |
| **Packages** | 38 |
| **Milestones** | 7 (including async + dynamic + tracker) |
| **Subagents** | 12+ (static + dynamic at runtime) |
| **Review Lenses** | 4 (risk, resilience, readability, reliability) |
| **LLM Providers** | OpenAI, Anthropic, Ollama, Copilot |
| **CLI Commands** | 30+ |
| **License** | MIT |

---

## 🤝 Contributing

PRs are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go build ./cmd/gaia
go test ./...
```

### SDD for Contributors

Substantial changes follow the SDD workflow:

```bash
gaia exec "explore the codebase"         # Understand before proposing
gaia exec "propose feature X"            # Formal proposal
gaia exec "implement tasks 2.1-2.3"      # Guided implementation
gaia review start                        # Bounded review
```

---

## 📄 License

MIT — see [LICENSE](LICENSE).

---

*Built with ❤️ using Go. Inspired by Hermes Agent, Gentle-AI, ogcode, and pi-go.*
