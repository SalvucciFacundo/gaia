# GAIA — Go Autonomous Intelligence Agent

<p align="center">
  <img src="assets/hero_banner.png" alt="GAIA — Go Autonomous Intelligence Agent" width="100%">
</p>

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/SalvucciFacundo/gaia?logo=github)](https://github.com/SalvucciFacundo/gaia/releases)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](https://github.com/SalvucciFacundo/gaia)
[![CI](https://github.com/SalvucciFacundo/gaia/actions/workflows/release.yml/badge.svg)](https://github.com/SalvucciFacundo/gaia/actions)

**GAIA is a programming-first autonomous agent** written in Go.  
Single binary, zero external dependencies, Windows/macOS/Linux.

---

## ✨ Features

### 🧠 12+ Specialized Autonomous Subagents
Each subagent learns independently in its domain — and improves with use.

| Subagent | Type | Role |
|---|---|---|
| **Explorer** | SDD | Investigates codebase patterns |
| **Proposer** | SDD | Creates change proposals |
| **Specifier** | SDD | Writes detailed specifications |
| **Designer** | SDD | Technical architecture |
| **Planner** | SDD | Task breakdown |
| **Implementer** | SDD | Writes code following specs |
| **Verifier** | SDD | Runs tests against specs |
| **Archiver** | SDD | Closes and archives changes |
| **Reviewer** | On-demand | GGA code review (4 lenses) |
| **Debugger** | On-demand | Root cause analysis + fix |
| **Researcher** | On-demand | Web search + documentation |
| **Learner** | Background | Creates and improves skills |

Talk directly to any subagent with `@name`:

```
@explorer investigate this codebase
@implementer refactor the auth module
```

### 🔀 Mixture of Agents (MoA)
Run multiple LLM models in parallel on the same task. Collect all responses and synthesize them into one coherent result.

```yaml
subagents:
  implementer:
    moa:
      enabled: true
      models:
        - provider: anthropic
          model: claude-sonnet-4-20250514
        - provider: openai
          model: gpt-4o
```

- **Per-subagent**: enable/disable independently (orchestrator never uses MoA)
- **Parallel**: goroutine fan-out with 30s timeout per model
- **Synthesis**: primary model merges all responses
- **Transparent**: subagents don't know they're in MoA

### 📋 Spec-Driven Development (SDD)
Built-in planning pipeline — not an external tool:

```
explore → propose → spec → design → tasks → apply → verify → archive
```

Each phase is a specialized subagent with its own memory, learning loop, and model config.

### 📝 GGA Code Review
Bounded code review with 4 lenses + content-bound receipts (SHA256):

| Lens | Focus |
|---|---|
| **Risk** | Security, permissions, data exposure |
| **Resilience** | Fallbacks, retry, graceful degradation |
| **Readability** | Naming, structure, maintainability |
| **Reliability** | Tests, determinism, regressions |

Pre-commit/pre-push gates validate against the same receipt — no silent re-reviews.

### 🧠 Knowledge Graph — Three-Scope Learning
GAIA learns at three levels, keeping knowledge organized:

```
👤 User scope      → Your coding habits (crosses all projects)
📚 Language scope  → Framework patterns (crosses same-language projects)
📁 Project scope   → Specific details (single project only)
```

Language auto-detection from build files (`go.mod`, `pom.xml`, `package.json`, etc.).

### 🛠️ Progressive Skills + Audit

```bash
gaia skills search go           # Find Go skills
gaia skills install go-testing  # Install
gaia skills audit               # Security scan (10 patterns)
gaia skills stats               # Usage statistics
gaia skills audit               # Security scan (10 patterns)
gaia skills add-tap owner/repo  # Add community skill source
```

Skills from GitHub taps:
```bash
gaia skills add-tap owner/repo    # Community skill source
gaia skills add-tap vercel-labs/agent-skills
```

### 🔐 Credential Pool
Multiple API keys per provider with automatic failover and cooldown:

```yaml
credential_pools:
  openai:
    - key: "sk-1..."
    - key: "sk-2..."    # auto fallback on 429/401/402
    - key: "sk-3..."
```

### ⏩ Async Background Tasks

```
> @explorer investigate this repo
✓ Task abc-123 started
> tasks
  abc-123  explorer  running  00:32
> cancel abc-123
```

### 🧬 Dynamic Subagents

```bash
> /create-agent
  → Name: "documentarian"
  → Description: "Writes project documentation"
  → Tools: read, glob, grep
✓ Subagent 'documentarian' created. Type @documentarian to chat.
```

### 💰 Cost Tracking

```bash
/cost
── LLM Cost ───────────────────────────
  Session: 12m34s
  Calls:   47
  Total:   $2.35
```

### 📦 Tool Output Cache
Read-only tools (read, glob, grep) cache results for 5s. If a subagent reads the same file twice in the same loop, the second call returns instantly — zero tokens, zero execution time.

### 📋 Session Management

```bash
gaia session list              # Recent sessions
gaia session restore <id>      # Load previous conversation
/undo                          # Undo last turn
/retry                         # Retry last user message
```

### 🚀 Remote Server Mode

```bash
# On your VPS
gaia serve 8080

# From your machine
curl -X POST http://vps:8080/message \
  -d '{"content": "explain this project"}'
```

### 🌐 Multi-Platform Gateway

```yaml
# ~/.gaia/config.yaml
telegram:
  token: "123:ABC"
discord:
  token: "Bot ..."
slack:
  token: "xoxb-..."
```

Start all adapters:

```bash
gaia gateway start
```

### 📊 Context Usage

```
/usage
── Context Usage ──────────────────────
  Model:    openai / gpt-4o
  Window:   128000 tokens
  Usage:    ████████████░░░░░░  45%
  Conversation:  45000 tok  35%
  Tools:          8000 tok   6%
  Skills:         6000 tok   5%
```

### 🔄 Undo / Retry / Checkpoint

```bash
/undo     # Remove last turn
/retry    # Re-run last user message
```

Failed subagent tasks are automatically rolled back — no partial state.

---

## 🚀 Quick Start

### Windows

```powershell
# Clone or download, then:
.\install.ps1
```

Choose **Full Install** (agent runs locally) or **Remote Client** (connects to VPS).

### macOS / Linux

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go build -o gaia ./cmd/gaia/
./gaia
```

### First Run

```bash
gaia
```

On first run, GAIA opens the setup wizard to configure your LLM provider and install recommended skills.

### Docker / SSH Backends

```bash
gaia exec "explain this project" --backend docker
gaia exec "list files" --backend ssh://user@server
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
| **Plugin System** | [docs/plugins.md](docs/plugins.md) |
| **Security** | [docs/security.md](docs/security.md) |
| **Configuration** | [docs/configuration.md](docs/configuration.md) |
| **Persona System** | [docs/persona.md](docs/persona.md) |

---

## 📊 Project Stats

| Metric | Value |
|---|---|
| **Language** | Go 1.22+ |
| **Architecture** | Hexagonal (ports & adapters) |
| **Packages** | 31 |
| **Tests** | All passing |
| **Subagents** | 12+ (static + dynamic at runtime) |
| **Review Lenses** | 4 (risk, resilience, readability, reliability) |
| **LLM Providers** | OpenAI, Anthropic, Ollama, Copilot |
| **Gateway Platforms** | Telegram (direct), Discord (direct), Slack (direct), WhatsApp MCP, Signal MCP |
| **CLI Commands** | 30+ |
| **License** | MIT |

---

## 🤝 Contributing

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go build ./cmd/gaia
go test ./...
```

Substantial changes follow the built-in SDD workflow.  
PRs welcome!

---

## 📄 License

MIT — see [LICENSE](LICENSE).

---

*Built with Go. Inspired by Hermes Agent, Gentle-AI, ogcode, and pi-go.*


