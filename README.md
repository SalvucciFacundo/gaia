# GAIA — Go Autonomous Intelligence Agent

<p align="center">
  <img src="assets/hero_banner.png" alt="GAIA — Go Autonomous Intelligence Agent" width="100%">
</p>

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/SalvucciFacundo/gaia?logo=github)](https://github.com/SalvucciFacundo/gaia/releases)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](https://github.com/SalvucciFacundo/gaia)
[![CI](https://github.com/SalvucciFacundo/gaia/actions/workflows/release.yml/badge.svg)](https://github.com/SalvucciFacundo/gaia/actions)

<p align="center">
  <a href="https://github.com/sponsors/SalvucciFacundo"><b>💖 Sponsor GAIA</b></a>
  ·
  <a href="https://ko-fi.com/"><b>☕ Buy a coffee</b></a>
</p>

**GAIA is a programming-first autonomous agent** written in Go.  
Single binary, zero external dependencies, Windows/macOS/Linux.

---

## 🚀 Quick Start

Get GAIA running in your local environment or remote containers instantly.

### Windows

```powershell
# Clone or download, then run the installer:
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

On the first run, GAIA starts an interactive setup wizard in the terminal to configure your LLM provider and install recommended skills.

### Docker / SSH Backends

Execute commands directly inside containers or remote hosts:
```bash
gaia exec "explain this project" --backend docker
gaia exec "list files" --backend ssh://user@server
```

---

## ✨ Features

### 🧠 12+ Specialized Autonomous Subagents
Each subagent learns independently in its domain and improves with use. Talk to any subagent with `@name`:
`@explorer investigate this codebase` or `@implementer refactor the auth module`.

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
| **Reviewer** | On-demand | BR code review (4 lenses) |
| **Debugger** | On-demand | Root cause analysis + fix |
| **Researcher** | On-demand | Web search + documentation |
| **Learner** | Background | Creates and improves skills |

### 🔀 Mixture of Agents (MoA)
Run multiple LLM models in parallel on the same task. Collect all responses and synthesize them into one coherent result.
- **Parallel execution**: Goroutine fan-out with a 30-second timeout per model.
- **Synthesis**: Primary model automatically merges responses and reconciles contradictions.

### 📋 Spec-Driven Development (SDD)
Built-in planning and implementation pipeline:
`explore → propose → spec → design → tasks → apply → verify → archive`.
Each phase is handled by a specialized subagent with its own memory namespace, learning loop, and model configuration.

### 📝 BR Code Review
Bounded code review with 4 lenses (Risk, Resilience, Readability, Reliability) + content-bound receipts (SHA256). Pre-commit/pre-push gates validate against the same receipt — no silent re-reviews.

### 🧠 Knowledge Graph — Three-Scope Learning
GAIA structures memory across three scopes to balance context size and fidelity:
- **User scope**: Your coding habits (crosses all projects)
- **Language scope**: Language-specific patterns (crosses same-language projects)
- **Project scope**: Specific project context and details (single project only)

### 🛠️ Progressive Skills + Security Audit
Manage and install skills for your project stack from the official registry or community taps.
- **AST Security Audit**: Scans installed skills for dangerous command execution, obfuscated scripts, or credential leaks.
- **Auto-Learning**: Subagents automatically save and refine patterns into skills.

### 🔐 Credential Pool
Configure multiple API keys per provider with automatic failover and cooldown when encountering rate limits or credential errors.

### ⏩ Async Background Tasks & Queueing
Run tasks in the background while continuing the main conversation, or queue follow-up prompts to execute sequentially.

### 🛡️ PolicyGuard — Permission System
Tier-based security (`read`, `sandbox`, `full`) for autonomous command execution. Denies dangerous operations automatically (e.g. `rm -rf /`) and escalates overrides to the user interactively.

---

## 📖 Documentation

Explore the full documentation in the [docs/](docs) directory:

| Topic | Guide |
|---|---|
| **CLI Commands** | [docs/cli.md](docs/cli.md) |
| **TUI & In-Session Commands** | [docs/tui-commands.md](docs/tui-commands.md) |
| **Configuration Reference** | [docs/configuration.md](docs/configuration.md) |
| **SDD Workflow** | [docs/sdd.md](docs/sdd.md) |
| **Architecture Overview** | [docs/architecture.md](docs/architecture.md) |
| **Subagent System** | [docs/subagents.md](docs/subagents.md) |
| **BR Review System** | [docs/review.md](docs/review.md) |
| **Context Management Layers** | [docs/context-layers.md](docs/context-layers.md) |
| **Token Efficiency & KG** | [docs/token-efficiency.md](docs/token-efficiency.md) |
| **Skills Hub** | [docs/skills.md](docs/skills.md) |
| **Plugin System** | [docs/plugins.md](docs/plugins.md) |
| **Design System** | [docs/design-system.md](docs/design-system.md) |
| **Security Architecture** | [docs/security.md](docs/security.md) |
| **Persona System** | [docs/persona.md](docs/persona.md) |
| **Unified Gateway Proposal** | [docs/unified-architecture.md](docs/unified-architecture.md) |
| **Hermes Gap Analysis** | [docs/hermes-commands-review.md](docs/hermes-commands-review.md) |
| **Pending Features** | [docs/pending-implementations.md](docs/pending-implementations.md) |
| **Full Specification** | [SPEC.md](SPEC.md) |

---

## 📊 Project Stats

| Metric | Value |
|---|---|
| **Language** | Go 1.22+ |
| **Architecture** | Hexagonal (ports & adapters) |
| **Packages** | 31 |
| **Subagents** | 12+ (static + dynamic at runtime) |
| **LLM Providers** | OpenAI, Anthropic, Ollama, Copilot |
| **Gateway Platforms** | Telegram, Discord, Slack, WhatsApp MCP, Signal MCP |
| **License** | MIT |

---

## 🤝 Contributing

Substantial changes follow the built-in SDD workflow. PRs are welcome!

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go build ./cmd/gaia
go test ./...
```

---

## 🙏 Acknowledgments

GAIA stands on the shoulders of several open-source projects:
- **[Gentle AI](https://github.com/Gentleman-Programming/gentle-ai)** — Core workflow inspiration, SDD phases, BR review, and Engram memory model.
- **[Hermes Agent](https://github.com/NousResearch/hermes-agent)** — Learning loop, skill creation/improvement, and tool approval system.
- **[ogcode](https://github.com/...)** — Token efficiency and Knowledge Graph recall.
- **[pi-go](https://github.com/...)** — Go-native agent structure.

---

## 📄 License

MIT — see [LICENSE](LICENSE).
