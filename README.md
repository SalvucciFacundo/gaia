# GAIA — Go Autonomous Intelligence Agent

<p align="center">
  <img src="assets/hero_banner.png" alt="GAIA — Go Autonomous Intelligence Agent" width="100%">
</p>

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![Website](https://img.shields.io/badge/Website-gaia.facundosalvucci.dev-00f0ff?logo=googlechrome&logoColor=white)](https://gaia.facundosalvucci.dev)
[![Docs](https://img.shields.io/badge/Docs-Live%20Portal-00ff9d?logo=googledocs&logoColor=white)](https://gaia.facundosalvucci.dev/docs)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/SalvucciFacundo/gaia?logo=github)](https://github.com/SalvucciFacundo/gaia/releases)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](https://github.com/SalvucciFacundo/gaia)
[![CI](https://github.com/SalvucciFacundo/gaia/actions/workflows/release.yml/badge.svg)](https://github.com/SalvucciFacundo/gaia/actions)

<p align="center">
  ·
</p>
**GAIA (Go Autonomous Intelligence Agent)** is an enterprise-grade, single-binary autonomous AI assistant and software engineering platform built from the ground up in Go.

GAIA couples autonomous agency with strict operational boundaries:
- **Zero External Runtimes**: Compiles into a single, self-contained native binary with embedded SQLite, FTS5, web assets, templates, and migrations.
- **Organic Driven Development (ODD)**: Dynamically scales from quick inline direct execution to multi-subagent teams without context bloat.
- **Decoupled Architecture**: Profiles define cognitive bias and identity (`SOUL.md`), while tools, multi-channel gateways, and audio engines live in a shared, globally configurable catalog.
- **Omnichannel Access**: Interact via interactive Cyberpunk TUI, built-in Web UI Dashboard (`templ` + `htmx`), OpenAI-compatible REST/SSE API, or native messaging gateways (Telegram, WhatsApp Cloud API, Discord, Slack) with bi-directional voice notes.

---

## ⚡ Quick Start

### 1. One-Line Installation

**Linux & macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/SalvucciFacundo/gaia/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/SalvucciFacundo/gaia/main/install.ps1 | iex
```

### 2. Build from Source

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go build -o gaia ./cmd/gaia/
./gaia
```

### 3. First Run Wizard

On first launch, GAIA automatically starts an interactive terminal setup wizard:
- Select your primary LLM provider (OpenAI, Anthropic, DeepSeek, Ollama, Groq, OpenRouter, etc.).
- Choose and activate your starting profile (`dev`, `assistant`, `art`).
- Optionally initialize messaging gateways and audio synthesizers.

---

## 🧩 Architectural Foundations

GAIA is built across **60 modular Go packages** adhering to Hexagonal Architecture (Ports and Adapters):

```mermaid
graph TD
    subgraph Interfaces ["User & Transport Interfaces"]
        TUI["Cyberpunk TUI (BubbleTea)"]
        WEB["Web UI Dashboard (templ + htmx)"]
        API["OpenAI API Server (/v1/chat/completions)"]
        GW["Messaging Gateways (TG, WA, DC, Slack)"]
    end

    subgraph Core ["Cognitive Core"]
        Brain["GAIA Brain (Orchestrator)"]
        Profiles["Profiles Engine (dev, assistant, art)"]
        Soul["Bayesian Soul & Persona Evolution"]
        Catalog["Global Tool Catalog"]
        Subagents["12+ Autonomous Subagents"]
    end

    subgraph Infra ["Engine & Storage"]
        Mneme["Mnemosyne Memory (FTS5 + Vector + RRF)"]
        Audio["Audio Engine (Whisper STT + ElevenLabs/OpenAI TTS)"]
        Browser["Headless Browser Driver (Chromedp)"]
        Ops["Backup & Self-Updater"]
    end

    Interfaces --> Brain
    Brain --> Profiles
    Brain --> Catalog
    Brain --> Subagents
    Profiles --> Soul
    Catalog --> Mneme
    Catalog --> Audio
    Catalog --> Browser
    Brain --> Ops
```

---

## 🌟 Core Architectural Pillars

Click any pillar to explore its dedicated in-depth documentation guide:

### 🎭 [Modular Profiles & Extensible Personas](docs/persona.md)
Profiles define identity and reasoning demeanor (`SOUL.md`), completely decoupled from tool access. Ships with `dev` (software architect), `assistant` (daily assistant), and `art` (visual reasoning), plus support for unlimited user-defined custom profiles with dynamic Bayesian Soul Evolution and real-time Prompt Injection Defense.  
👉 **[Read the Profiles & Persona Guide →](docs/persona.md)**

### 🧰 [Global Tool Catalog & Decoupling](docs/skills.md)
Every tool lives in a central catalog. No tool is hardcoded to a profile: configure any tool for any profile at runtime (`gaia profile tools enable <profile> <tool>`) or declaratively in `config.yaml`. Supports dynamic parameter inspection (`tool_search`, `load_tool`) and autonomous skill learning (`create_skill`, `read_skill`).  
👉 **[Explore Tool Catalog & Skills →](docs/skills.md)**

### 📡 [Multi-Channel Messaging & Voice Engine](docs/gateway.md)
Run GAIA as an autonomous background daemon across Telegram, WhatsApp (Meta Cloud API), Discord, and Slack. Features bi-directional audio with inbound speech-to-text (OpenAI Whisper STT) and outbound speech synthesis (ElevenLabs / OpenAI TTS), plus interactive inline approval buttons for sensitive operations.  
👉 **[Configure Gateways & Voice →](docs/gateway.md)**

### 🌐 [OpenAI-Compatible REST & Streaming API Server](docs/cli.md#server-mode-openai-restsse-api--web-ui)
Drop-in replacement server for OpenAI endpoints (`gaia serve` / `gaia api`) exposing `POST /v1/chat/completions` with JSON and real-time streaming SSE (`text/event-stream`). Connect any existing AI client (Cursor, LibreChat, Open WebUI, Continue) with constant-time Bearer token authentication and configurable CORS.  
👉 **[Inspect Server API & CLI Commands →](docs/cli.md)**

### 🌐 [Headless Browser Automation](docs/architecture.md#package-layout-60-modular-packages)
Embedded headless browser driver powered by `chromedp` with zero-config auto-detection for Chrome, Chromium, Brave, Edge, and Flatpak. Supports complex client-side JavaScript execution, DOM navigation, full-page screenshots, and structured DOM-to-markdown extraction.  
👉 **[Read Browser & Architecture Overview →](docs/architecture.md)**

### 🧠 [Mnemosyne Persistent Memory Engine](docs/architecture.md#mnemosyne-memory-architecture)
Hybrid memory engine combining SQLite FTS5 Full-Text Search with Vector Cosine Similarity via Reciprocal Rank Fusion (RRF). Observations and durable facts are partitioned by project workspace and lifecycle states (`active` vs `needs_review`) with automatic background compaction.  
👉 **[Learn about Memory & Storage →](docs/architecture.md)**

### 💾 [Reminis Blackboard & Ephemeral Workers](docs/reminis-blackboard.md)
Port of the OS-LLM architecture from [**Reminis**](https://github.com/SalvucciFacundo/Reminis): replaces quadratic context explosion ($O(N^2)$) with an in-memory L1 key-value ledger (`sync.RWMutex`), automatic 4KB disk spillover pass-by-reference, and bounded ephemeral subagents whose conversational message history is strictly garbage-collected upon completion. Includes SHA-256 verified SQLite snapshots.  
👉 **[Explore Reminis Blackboard Architecture →](docs/reminis-blackboard.md)**

### 🌙 [Overnight Autonomous Execution](docs/overnight.md)
Run multi-stage engineering and content creation tasks unattended across all profiles without interactive blocking. Features an interactive pre-flight alignment interview, Git Worktree / staging sandboxes, a non-blocking deferred review queue, and consolidated morning review packs (`gaia overnight review/merge`).  
👉 **[Read Overnight Execution Guide →](docs/overnight.md)**

### 🛡️ [Simplified 4x3 Permissions & Blast Radius Control](docs/security.md#layer-4-policyguard-4x3-permissions--blast-radius-matrix-internalcore)
Intuitive security matrix mapping tools into 4 functional categories (`Filesystem`, `Terminal`, `Network`, `Memory`) with a 3-state traffic light (`🟢 Allow`, `🟡 Ask`, `🔴 Block`) and clear Blast Radius indicators (`Project Workspace`, `Local Machine`, `External Internet`, `GAIA Storage`). Controlled via Cyberpunk TUI (`gaia policy panel`, `/permisos`), Web UI modal (`🛡️ Permissions`), and CLI (`gaia policy status -v`).  
👉 **[Review Security & Permissions Matrix →](docs/security.md)**

### ⚡ [Go-Native Code Mode (CodeAct) with Yaegi](docs/token-efficiency.md#6-go-native-code-mode-codeact-with-yaegi)
Pure Go dynamic script interpretation via embedded Yaegi interpreter (`github.com/traefik/yaegi`) with ergonomic `GaiaSDK`. Allows models to bundle complex multi-step tool workflows into a single Go script, cutting latency and multi-turn token costs by 80%.  
👉 **[Read Token Efficiency & Code Mode →](docs/token-efficiency.md)**

### 🎯 [Surgical Search & Replace + Compiler Auto-Repair Loop](docs/token-efficiency.md#8-surgical-search--replace-file_patch--apply_patch)
Precision file patching with exact & fuzzy matching (`file_patch` / `apply_patch`), saving 70%–80% of editing tokens. Includes a closed-loop compiler auto-repair hook: whenever a file is modified, GAIA runs instant syntax/compile checks (Go, Rust, TypeScript, Python) and injects compiler diagnostics directly into the turn so the LLM self-heals before concluding.  
👉 **[Learn about Surgical Patching & Auto-Repair →](docs/token-efficiency.md)**

### ⌨️ [Modern Terminal DX & Multimodal Input](docs/tui-commands.md#dx-de-terminal-autocompletado-menciones-archivo-e-im%C3%A1genes)
High-productivity TUI with Tab-based autocompletion for slash commands, subagents, and filesystem paths; instant `@path` multi-file context injection (skipping initial `file_read` roundtrips); image attachment via `@screenshot.png` or `/paste` from clipboard; and an expanded 10,000-character input buffer.  
👉 **[Read TUI & Terminal DX Reference →](docs/tui-commands.md)**

---

## 📋 Complete CLI Command Summary

| Command | Category | Description |
|---|---|---|
| `gaia` | Core | Launches the interactive Cyberpunk TUI |
| `gaia exec "<prompt>"` | Core | Executes prompt in headless batch mode |
| `gaia serve [--port N]` | Server | Starts the OpenAI-compatible REST/SSE API & Web Dashboard |
| `gaia gateway [start\|stop\|status]` | Gateway | Starts/manages the Telegram, WhatsApp, Discord, Slack daemon |
| `gaia review [start\|status\|list\|validate]` | Review & Gates | 4R review and Judgment Day adversarial dual-review protocol |
| `gaia delivery [list\|status\|release\|diff]` | Delivery | Stacked branch release queue with CAS receipt validation |
| `gaia policy [status\|panel\|init]` | Security | 4x3 permissions control, blast radius audit, Cyberpunk TUI |
| `gaia blackboard snapshot [list\|inspect\|restore]` | Memory | Reminis L1 Blackboard checkpoints with SHA-256 tamper verification |
| `gaia overnight [start\|list\|review\|merge\|discard]` | Autonomous | Unattended execution with pre-flight interview and morning review pack |
| `gaia profile [subcommand]` | Profiles | Manage profiles (`list`, `switch`, `enable`, `disable`, `tools`) |
| `gaia profile tools [enable\|disable\|list\|reset]` | Tools | Configure tools enabled for any profile |
| `gaia skill [list\|create\|show\|delete]` | Skills | Manage agent skills |
| `gaia cron [create\|list\|pause\|resume\|remove\|start]` | Scheduled | Manage and execute automated scheduled cron jobs |
| `gaia plugin [list\|enable\|disable\|install\|remove]` | Ecosystem | Manage native GAIA plugins |
| `gaia webhook [start\|list-subs]` | Automation | GitHub webhook listener daemon and event triggers |
| `gaia lsp [list\|diagnostics]` | Code Intel | Language Server Protocol (LSP) diagnostics and client |
| `gaia tracker [check\|report\|port]` | Sync | Upstream release monitoring and parity tracking |
| `gaia onboard` | Onboarding | Interactive walkthrough of Spec-Driven Development (SDD) |
| `gaia backup [create\|restore\|list]` | Ops | Create or inspect profile backup archives |
| `gaia restore <archive>` | Ops | Restore configuration and databases from archive |
| `gaia update [--check\|--backup]` | Ops | Check for updates and upgrade GAIA binary |
| `gaia session [list\|restore]` | Session | Manage and inspect past multi-turn session contexts |
| `gaia desktop` | Desktop | Launch native Wails desktop application |
| `gaia doctor` | Diagnostics | Run environment health checks and diagnostics |

---

## 📖 Documentation

> 🌐 **Interactive Documentation Portal:** Browse the full live documentation on **[gaia.facundosalvucci.dev/docs](https://gaia.facundosalvucci.dev/docs)** with real-time search, Mermaid architecture diagrams, and responsive navigation.

Explore the complete guides in the [docs/](docs) directory:

| Topic | Guide |
|---|---|
| **CLI Commands** | [docs/cli.md](docs/cli.md) |
| **Reminis Blackboard & Memory Port** | [docs/reminis-blackboard.md](docs/reminis-blackboard.md) |
| **Overnight Autonomous Execution** | [docs/overnight.md](docs/overnight.md) |
| **Security & 4x3 Permissions Matrix** | [docs/security.md](docs/security.md) |
| **Token Efficiency & Code Mode** | [docs/token-efficiency.md](docs/token-efficiency.md) |
| **TUI & In-Session Commands** | [docs/tui-commands.md](docs/tui-commands.md) |
| **Configuration Reference** | [docs/configuration.md](docs/configuration.md) |
| **Architecture Overview** | [docs/architecture.md](docs/architecture.md) |
| **Unified Ecosystem Master Plan** | [docs/unified-gaia-ecosystem.md](docs/unified-gaia-ecosystem.md) |
| **AGIS to GAIA Porting Guide** | [docs/agis-to-gaia-porting-guide.md](docs/agis-to-gaia-porting-guide.md) |
| **Profiles & Persona System** | [docs/persona.md](docs/persona.md) |
| **Gateways & Voice Engine** | [docs/gateway.md](docs/gateway.md) |
| **Skills Hub** | [docs/skills.md](docs/skills.md) |
| **Subagents System** | [docs/subagents.md](docs/subagents.md) |
| **Context Management Layers** | [docs/context-layers.md](docs/context-layers.md) |
| **Design System** | [docs/design-system.md](docs/design-system.md) |
| **Full Specification** | [SPEC.md](SPEC.md) |

---

## 📊 Technical Specifications

| Characteristic | Specification |
|---|---|
| **Language** | Go 1.23+ |
| **Packages** | 60 internal and cmd packages |
| **Architectural Style** | Hexagonal (Ports & Adapters) |
| **Supported LLMs** | 19 providers (OpenAI, Anthropic, DeepSeek, Ollama, Groq, OpenRouter, etc.) |
| **Database** | Embedded SQLite 3 with FTS5 and Vector Search |
| **Web Server** | Native Go `net/http` with SSE streaming and JSON API |
| **User Interfaces** | BubbleTea TUI, `templ` + `htmx` Web Dashboard, OpenAI API |
| **Gateways** | Telegram, WhatsApp Meta Cloud API, Discord, Slack |
| **Audio Processing** | Whisper STT, ElevenLabs TTS, OpenAI TTS |
| **License** | MIT License |

---

## 🤝 Contributing

Contributions are welcome! Please run test suites before submitting PRs:

```bash
git clone https://github.com/SalvucciFacundo/gaia.git
cd gaia
go test -count=1 ./...
go build ./cmd/gaia
```

---

## 🙏 Acknowledgments

GAIA stands on the shoulders of several innovative open-source projects:
- **[Reminis](https://github.com/SalvucciFacundo/Reminis)** by Facundo Salvucci — Core inspiration for the OS-LLM architecture, working memory Blackboard (L1 RAM ledger with automatic 4KB disk spillover), and ephemeral subagent workers with complete conversational context destruction.
- **[Gentle AI / Gentle Shell](https://github.com/Gentleman-Programming/gentle-ai)** — Core workflow inspiration, Organic Driven Development (ODD), BR review, and Engram memory model.
- **[Hermes Agent](https://github.com/NousResearch/hermes-agent)** — Learning loop, skill creation/improvement, and tool approval system.
- **[ogcode](https://github.com/...)** — Token efficiency and Knowledge Graph recall.
- **[pi-go](https://github.com/...)** — Go-native agent structure.

---

## 📄 License

MIT — see [LICENSE](LICENSE).
