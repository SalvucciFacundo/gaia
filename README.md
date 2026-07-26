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
| **Reviewer** | On-demand | BR code review (4 lenses) |
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

### 📝 BR Code Review
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

### 📋 Session Management — 18 commands

GAIA has a full session management system. Commands are available both as in-chat slash commands and CLI subcommands.

#### Conversation Flow

| Command | Description |
|---------|-------------|
| `/undo` | Reverses the last turn — removes the last user message and everything the AI generated in response. Useful when the agent misunderstood or went in the wrong direction. Can be used multiple times to go back several turns. |
| `/retry` | Removes the last AI response and re-runs the last user message through the full agent loop. The agent will re-process your request from scratch with a clean context. Useful when the first response wasn't satisfactory. |
| `/new` or `/reset` | Completely clears the conversation and starts a fresh session. All message history is deleted from the current context. The configuration and model settings are preserved — only the conversation resets. Previous sessions can still be accessed via `/sessions` if they were saved. |
| `/clear` | Clears the TUI display without affecting the conversation state. All messages remain in context and will reappear as the conversation progresses. Useful for decluttering the terminal without losing progress. |

#### Persistence & History

| Command | Description |
|---------|-------------|
| `/history` | Displays the full conversation history of the current session, showing all user messages and AI responses with role prefixes. Messages are truncated to 120 characters for readability. Includes the total message count at the top. |
| `/save` or `/save <name>` | Explicitly saves the current conversation as a named session in the SQLite database. If no name is provided, it auto-generates one with the current date and time (`Session 2026-07-22 14:30`). Saved sessions persist across GAIA restarts and can be resumed later. |
| `/title <name>` | Renames the current session. If the session was previously saved via `/save`, the name is persisted in the database. If not yet saved, the name will be used when you eventually save it. Useful for organizing multiple work streams. |
| `/sessions` | Lists all saved sessions with their ID, name, and creation date. Each entry shows the first 12 characters of the session ID for use with `/resume`. Sessions are ordered by creation date (newest first). |
| `/resume <id>` | Loads and displays a previously saved session by its ID prefix (partial match supported). Restores the session name as the current session and shows all stored messages. Use `/sessions` to find available session IDs. |

#### Context & Memory

| Command | Description |
|---------|-------------|
| `/compress` | Forces manual context compaction. GAIA normally auto-compacts when the conversation exceeds the configured threshold (default: 50 messages). This command triggers compaction immediately, summarizing older messages into a condensed form while keeping recent messages verbatim. Useful before sending a complex prompt to free up context window. |
| `/history` | (see above — also serves as context inspection) |

#### Parallel & Background Work

| Command | Description |
|---------|-------------|
| `/moa <prompt>` | One-shot Mixture of Agents. Fans out the prompt to **all configured LLM providers** in parallel (30s timeout per model), collects all responses, and synthesizes them into a single answer using the primary model. If only one extra provider is available, returns its response directly without synthesis. Requires at least one additional provider configured in `config.yaml` under the `subagents` section. The synthesis step reconciles contradictions and removes repetition across models. |
| `/background <prompt>` | Spawns an `explorer` subagent in a background goroutine and returns immediately with a task ID. The subagent works independently while you continue the main conversation. Track progress with `/tasks` and cancel with `/cancel <taskid>`. Useful for long-running investigations that don't need your immediate attention. |
| `/queue <prompt>` or `/q <prompt>` | Adds a message to the processing queue. The queued message is automatically processed after the current task completes, one at a time. Use `/queue` or `/q` alone to see queued items, `/queue clear` to empty the queue. This lets you give the agent follow-up instructions without interrupting its current work. |

#### Session Handoff

| Command | Description |
|---------|-------------|
| `/handoff <platform>` | Saves the current session and shows step-by-step instructions for resuming the conversation on another messaging platform. Supports: `telegram`, `discord`, `slack`, `whatsapp`, `signal`, and `cli`. The session is saved to the shared SQLite database with a name like `handoff-telegram-2026-07-22-14-30`. To resume, start the gateway (`gaia gateway start`) and send `/resume <session-id>` on the target platform. Since both the CLI and gateway share the same database, the conversation continues exactly where you left off — including all context, history, and tool results. |

#### State & Branching

| Command | Description |
|---------|-------------|
| `/branch` or `/branch <name>` | Creates a named branch point by saving the full conversation state to a JSON snapshot file. The name is auto-generated with a timestamp if not provided. After branching, you continue in the current conversation on a new exploration path. Branches are stored in `%TEMP%/gaia-snapshots/` with a `branch-` prefix. Use `/branches` to list all saved branches. |
| `/branches` | Lists all saved branch points with their names. Each branch shows as `branch-<name>`. To switch to a branch at any time, use `/snapshot load branch-<name>`. This restores the conversation to exactly the state it was in when the branch was created. |
| `/snapshot save <name>` | Saves the current conversation state to a JSON file in `%TEMP%/gaia-snapshots/`. Unlike `/save` which persists to SQLite as a named session, snapshots are raw JSON dumps of all messages. Useful for manual backup, inspection, or transfer between machines. |
| `/snapshot load <name>` | Restores a previously saved snapshot, re-inserting all messages into the current conversation. Messages are re-saved to the SQLite database in order. Use `/history` to view the restored conversation. Supports loading branch snapshots with `branch-<name>`. |

#### Session Mode

| Command | Description |
|---------|-------------|
| /session | Displays the current session mode and active sessions. Shows whether all platforms share one session (unify) or each has its own (isolate), along with session IDs and creation times. |
| /session unify | Sets session mode to **unify** (default). All messages from all platforms (TUI, Telegram, Discord, etc.) go to the **same session** with the same conversation history. Messages are prefixed with the platform name so the agent knows where each message came from. Ideal when you're working on the same project from multiple devices. |
| /session isolate | Sets session mode to **isolate**. Each platform gets its **own independent session** with its own history. The first message from a new platform creates a new session automatically (e.g., platform-telegram-xxx). Messages from different platforms don't interfere with each other. Ideal when you use each platform for completely different purposes. |
| /session ask | Sets session mode to **ask** (smart prompt). When you send a message from a different platform than the last one, GAIA asks what to do: continue the unified session or start fresh. Your choices are remembered as preferences. Once you choose, the mode can auto-switch to unify or isolate based on your answer. |

#### Goal System

| Command | Description |
|---------|-------------|
| `/goal <text>` | Sets a persistent goal that the agent works toward across multiple turns. Once set, the agent automatically continues after each turn until the goal is complete. At the end of each response, GAIA evaluates progress by asking the LLM "Is the goal complete? (YES/NO)" — if NO, it sends a continuation prompt automatically. Use `/goals` to check the current goal, `/goal clear` to cancel it. |
| `/subgoal <text>` | Adds a specific criterion to the active goal. Subgoals are included in the evaluation prompt and in the continuation prompt. Example: after `/goal refactor auth to JWT`, add `/subgoal all tests must pass`. The agent will see this requirement in every evaluation and continuation. Up to 10 subgoals can be active simultaneously. |
| `/goals` | Displays the current active goal and all subgoals. Shows the full text of each item. If no goal is active, shows usage instructions. |
| `/goal clear` | Clears the active goal and all subgoals. The agent stops auto-continuing and returns to normal single-turn operation. |

#### Mid-Execution Control

| Command | Description |
|---------|-------------|
| `/steer <message>` | Injects a guidance message that the agent sees **before its next tool call**, without waiting for the current turn to finish. Unlike a normal message (which is processed after the current turn completes), `/steer` is sent directly to the agent's active loop via a buffered channel. The agent receives it as "MID-EXECUTION GUIDANCE" at the start of the next iteration and adjusts its approach immediately. Useful for real-time corrections: if you see the agent heading in the wrong direction, `/steer "use JWT instead of sessions"` redirects it before it writes more code. |

#### CLI Equivalents

```bash
gaia session list              # Same as /sessions
gaia session restore <id>      # Same as /resume
gaia policy init --tier=...    # Configure security policy
```

### ⚙️ Configuration Commands

| Command | Description |
|---------|-------------|
| `/model` | Lists all available LLM providers. The current one is marked with ➤. Providers come from your `config.yaml` configuration. |
| `/model <name>` | Switches the active LLM provider mid-session (e.g., `/model anthropic`, `/model openai`). The change takes effect immediately for the next LLM call. Note: the switch is runtime-only — edit `config.yaml` to make it permanent. |
| `/reasoning <level>` | Changes the reasoning effort of the LLM. Accepts `low`, `medium`, or `high`. Higher effort produces more thorough analysis but takes longer and costs more. Lower effort is faster and cheaper for simple tasks. |
| `/personality <name>` | Switches the agent's personality/behavior style. Options: `teacher` (explica el por qué), `professional` (directo y eficiente), `strict` (exigente, prioriza tests), `friendly` (relajado y alentador). |
| `/yolo` | Toggles YOLO mode. When ON, sets the PolicyGuard to `full` tier — all commands are auto-approved (the hardline blocklist for catastrophic commands like `rm -rf /` remains active). When OFF, returns to `sandbox` tier. Shows a prominent warning when active. |
| `/verbose` | Cycles through 4 levels of tool output display: `off` → `results` → `tool calls` → `all`. Controls how much detail you see when the agent executes tools. |
| `/timestamps` | Toggles timestamps on/off for each message in the conversation. Useful for tracking when things happened in long sessions. |
| `/statusbar` or `/sb` | Toggles the status bar at the bottom of the TUI. Shows connection status, active model, and other metadata. |
| `/footer` | Toggles metadata footers on AI responses. Shows token usage, model name, and response time for each message. |
| `/indicator` | Cycles through 4 spinner styles: `dots`, `line`, `pipe`, `circle`. Changes the animation shown while the agent is processing. |
| `/skin <name>` | Changes the TUI color theme. Options: `default`, `rose-pine`, `dark`, `light`. The change applies immediately for most elements; some可能需要 restart to take full effect. |

### 🛠️ Tools & Skills Commands

| Command | Description |
|---------|-------------|
| `/skills` | Shows the skill management menu with available subcommands. |
| `/skills list` | Lists all installed skills. Redirects to `gaia skills list` for full output. |
| `/skills search <query>` | Searches the Skills Hub for available skills matching your query. |
| `/skills install <name>` | Installs a skill by name from the Skills Hub. |
| `/skills remove <name>` | Uninstalls a skill. |
| `/skills stats` | Shows usage statistics for installed skills — how often each skill was loaded and by which subagents. |
| `/skills audit` | Runs a security audit on all installed skills. Scans for 10+ dangerous patterns including credential leaks, destructive commands, obfuscated scripts, and pipe-to-shell patterns. |
| `/cron` | Shows the cron job management menu. GAIA's built-in scheduler lets you run tasks on a cron schedule and deliver results to terminal or messaging platforms. |
| `/cron list` | Lists all scheduled cron jobs with their schedule, status, and next run time. |
| `/cron add <schedule> <task>` | Creates a new cron job. Schedule uses standard cron syntax (`"0 2 * * *"` for daily at 2 AM). Task is the prompt to execute. |
| `/cron remove <id>` | Removes a scheduled job by ID. |
| `/cron pause <id>` | Pauses a job without removing it. |
| `/cron resume <id>` | Resumes a paused job. |
| `/cron run <id>` | Runs a job immediately, regardless of its schedule. |
| `/reload-mcp` | Shows instructions for reloading MCP servers. MCP servers are configured in `config.yaml` and require a gateway restart to pick up changes. |
| `/reload-skills` | Shows instructions for reloading skills. Skills are loaded from disk on each subagent spawn — no restart needed, but an explicit reload ensures the index is fresh. |
| `/plugins` | Shows plugin management instructions. Plugins extend GAIA's capabilities and are managed via the CLI: `gaia plugin list`, `install`, `remove`. |
| `/browser` or `/browser connect` | Shows browser automation configuration. Browser tools are available when a browser MCP server is configured in `config.yaml` under `browser_tools`. |
| `/memory pending` | Lists recent memory operations from the current session that may need review. Shows a preview of each saved memory. |
| `/memory approve <id>` | Approves a pending memory write, confirming it should be persisted. |
| `/memory reject <id>` | Rejects a memory write, preventing it from being persisted. |
| `/learn <source>` | Creates a new skill automatically. If `<source>` is a directory path, it scans the directory and generates a SKILL.md with patterns detected from the code. If `<source>` is a text description, it creates a skill skeleton with that description. The skill is saved to `~/.gaia/skills/` and is immediately available. |
| `/suggestions` | Analyzes your project and recommends skills to install. Detects your tech stack (Go, Angular, etc.) and suggests relevant skills like `go-testing`, `angular-signals`, or `go-concurrency`. Each suggestion shows the install command. |
| `/blueprint <name>` | Creates a new skill from a predefined template. Available blueprints: `daily-report` (end-of-day summary), `nightly-backup` (automated git backup), `code-review` (standardized review checklist), `api-test` (API endpoint testing). The blueprint is saved as a SKILL.md that you can customize. |
| `/curator` | Scans all installed skills and reports issues. Checks for missing SKILL.md files, empty or minimal skill directories, and structural problems. Complements `/skills audit` which focuses on security rather than structure. |

### ℹ️ Info & System Commands

| Command | Description |
|---------|-------------|
| `/help` | Displays a categorized list of all available commands organized by group: Session, State, Goals, Queue/Steer, Background, Handoff, Config, Permissions, Skills, Cron, Memory, Info, and Tools. |
| `/version` | Shows the GAIA version, Go runtime version, and license information. Useful for bug reports and verifying installations. |
| `/platforms` or `/gateway` | Shows gateway platform configuration and status. Displays which messaging platforms (Telegram, Discord, Slack, WhatsApp, Signal) are configured and how to start the gateway with `gaia gateway start`. |
| `/copy` | Copies the last AI response to the system clipboard. Supports Windows (`clip`), macOS (`pbcopy`), and Linux (`wl-copy`). Use `/copy <n>` to copy the n-th previous response instead of the most recent one. |
| `/insights` | Shows session analytics including message counts by role (user, AI, tool, system), total tool calls, model name, session ID, iteration budget usage, and LLM cost if cost tracking is enabled. |
| `/debug` | Collects and displays system diagnostic information: Go version, provider/model names, config path, session message count, budget settings, compaction state, policy tier, and active goal if set. |
| `/credits` | Shows credit/usage balance information based on your LLM provider. Redirects to provider-specific dashboards: OpenRouter, OpenAI, Anthropic, or GitHub Copilot. Use `/insights` for session-level cost tracking if cost_tracker is enabled. |
| `/billing` | Shows billing management information and links to your LLM provider's billing dashboard. Billing is handled externally by each provider — GAIA doesn't process payments. |
| `/image <path>` | Loads an image file from the specified path for vision processing. Currently a stub — vision support will be available in a future release. Validates that the file exists and reports the path. |
| `/paste` | Attaches an image from the system clipboard for vision processing. Currently a stub — requires vision support which is not yet implemented. |

### 💬 Messaging Commands (Gateway)

These commands are designed for gateway mode (Telegram, Discord, Slack) but also work in the TUI.

| Command | Description |
|---------|-------------|
| `/sethome` | Marks the current chat as the delivery home for notifications. Cron job results, background task completions, and alerts will be delivered to this chat. In TUI mode, delivery is always the terminal. In gateway mode, binds to the current Telegram/Discord/Slack chat. |
| `/approve` | Approves a pending dangerous command confirmation. When the agent detects a potentially destructive operation (e.g., recursive delete, system config change), it pauses and waits for confirmation. `/approve` authorizes the operation. Use `/trust full` to auto-approve all commands in the session. |
| `/deny` | Denies a pending dangerous command confirmation. Rejects the operation and tells the agent to find an alternative approach. Use `/trust read` for strictest restrictions. |
| `/commands` | Lists all available slash commands organized by category. Aliased to `/help` and `/h`. Useful in gateway mode where there's no tab-completion or autocomplete. |
| `/restart` | Shows restart instructions for the current mode. In gateway/daemon mode, use with a process manager (systemd, supervisord) for auto-restart on exit. Automatic restart is not built-in — the process must be managed externally. |
| `/update` | Shows update instructions. GAIA can be updated by rebuilding from source (`git pull && go build`), downloading a new release, or re-running the installer. Automatic in-place update is planned for a future release. |
| `/topic` | Shows help for multi-session DM mode. `/topic new <name>` starts a new conversation topic, `/topic switch <name>` switches between topics, `/topic list` shows active topics. Each topic has its own independent conversation history — useful for parallel conversations in one chat. |

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

### 🛡️ PolicyGuard — Permission System

Tier-based security for autonomous agent execution:

```
gaia policy init                    # Create project policy (.gaia/policy.yaml)
gaia policy init --global           # Create global policy (~/.config/gaia/policy.yaml)
gaia policy init --tier=read       # Lock to read-only tools
```

**Three tiers** (configured at install or runtime):

| Tier | Tools Allowed | Use Case |
|------|--------------|----------|
| **read** | glob, grep, read, file_info, mem_search | Exploration only |
| **sandbox** | read + write + safe shell commands | Development |
| **full** | Everything (hardline blocklist still active) | Full trust |

**Smart escalation**: when a tool is denied, PolicyGuard skips → tries alternatives → asks user only when blocked.

**Hardline blocklist** (immutable): `rm -rf /`, fork bombs, `dd` to block devices, `curl\|sh` — never executed.

**Per-tool overrides** via `/permisos` panel:

```
/permisos
  → Select any tool with ↑ ↓
  → Press Enter to set: allow | deny | skip | ask-once | ask-session | ask-always | audit
  → Or remove override to use tier default
```

**At install**: the wizard asks "Enable Security Mode?" — answers `Yes` sets sandbox tier, `No` uses basic mode (only hardline blocklist).

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
| **Session Management** | README above (18 commands) |
| **SDD Workflow** | [docs/sdd.md](docs/sdd.md) |
| **Architecture** | [docs/architecture.md](docs/architecture.md) |
| **Subagent System** | [docs/subagents.md](docs/subagents.md) |
| **Review System** | [docs/review.md](docs/review.md) |
| **Token Efficiency** | [docs/token-efficiency.md](docs/token-efficiency.md) |
| **Skills Hub** | [docs/skills.md](docs/skills.md) |
| **Plugin System** | [docs/plugins.md](docs/plugins.md) |
| **Design System** | [docs/design-system.md](docs/design-system.md) |
| **Security** | [docs/security.md](docs/security.md) |
| **Configuration** | [docs/configuration.md](docs/configuration.md) |
| **Persona System** | [docs/persona.md](docs/persona.md) |
| **Hermes Gap Analysis** | [docs/hermes-commands-review.md](docs/hermes-commands-review.md) |
| **Pending Features** | [docs/pending-implementations.md](docs/pending-implementations.md) |
| **Unified Architecture** | [docs/unified-architecture.md](docs/unified-architecture.md) |

---

## 📊 Project Stats

| Metric | Value |
|---|---|---|
| **Language** | Go 1.22+ |
| **Architecture** | Hexagonal (ports & adapters) |
| **Packages** | 31 |
| **Tests** | All passing |
| **Subagents** | 12+ (static + dynamic at runtime) |
| **Review Lenses** | 4 (risk, resilience, readability, reliability) |
| **LLM Providers** | OpenAI, Anthropic, Ollama, Copilot |
| **Gateway Platforms** | Telegram (direct), Discord (direct), Slack (direct), WhatsApp MCP, Signal MCP |
| **CLI Commands** | 30+ (exec, skills, review, cron, doctor, gateway, plugin, webhook, lsp, serve, session, tracker, policy, onboard, desktop) |
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

## 🙏 Acknowledgments

GAIA stands on the shoulders of several open-source projects:

| Project | Role | Inspiration |
|---|---|---|
| **[Gentle AI](https://github.com/Gentleman-Programming/gentle-ai)** | ⭐ Core workflow | SDD phases, BR review (4 lenses + receipts), Engram memory model, Judgment Day protocol, delegation rules, skill registry |
| **[Hermes Agent](https://github.com/NousResearch/hermes-agent)** | Agent design | Learning loop, skill creation/improvement, memory nudge, subagent spawning, tool approval system |
| **[ogcode](https://github.com/...)** | Token efficiency | Knowledge graph recall for 70%+ token savings on long sessions |
| **[pi-go](https://github.com/...)** | Architecture | Go-native agent structure, subagent spawning patterns |

**Gentle AI** in particular deserves special recognition — its SDD workflow, bounded review system, and orchestrator-driven delegation model form the backbone of GAIA's architecture. The SDD phase skills (`sdd-propose`, `sdd-spec`, `sdd-design`, etc.) and review protocol are direct adaptations of Gentle AI's patterns to a standalone Go agent.

---

## 📄 License

MIT — see [LICENSE](LICENSE).

---




