# Hermes Commands — GAIA Gap Analysis

Commands that Hermes has and GAIA doesn't. Organized by category for review.

---

## Category: Session Management

| Command | Description | Effort |
|---|---|---|
| `/new` / `/reset` | Start a fresh conversation (new session ID, clear history) | 🟢 Trivial |
| `/clear` | Clear screen + new session | 🟢 Trivial |
| `/history` | Show full conversation history | 🟢 Trivial |
| `/save` | Save the current conversation explicitly | 🟢 Trivial |
| `/resume <id>` | Resume a previous session by ID or title | 🟡 GAIA has `gaia session restore`, no in-TUI command |
| `/sessions` | Interactive session picker (browse + resume) | 🟡 UI work |
| `/title <name>` | Name the current session | 🟢 Trivial |
| `/branch` / `/fork` | Fork the conversation at a point (explore alternative paths) | 🟡 Requires conversation branching logic |
| `/compress` | Manually compress context (summarize old turns) | 🟢 GAIA has auto-compaction (A3), no manual trigger |
| `/snapshot` / `/snap` | Create/restore state snapshots (config + state) | 🟡 Medium |
| `/rollback` | List or restore filesystem checkpoints | 🟡 GAIA has checkpoint (B6), no rollback UI |
| `/background <p>` | Run a prompt in a separate background session | 🟡 GAIA has SpawnAsync, no /background command |
| `/queue <p>` / `/q` | Queue a message without interrupting the agent | 🟡 Medium |
| `/steer <p>` | Inject mid-turn guidance (arrives after next tool call) | 🔴 Complex |
| `/goal <text>` | Persistent goal across turns (auto-continues until done) | 🔴 Complex, judge model needed |
| `/subgoal <text>` | Append criterion to active goal | 🔴 Complex (depends on /goal) |
| `/handoff <platform>` | Move session from CLI to Telegram/Discord/etc | 🟡 Medium |
| `/moa <prompt>` | One-shot Mixture of Agents (run prompt through MoA, restore model) | 🟢 B1 exists, no one-shot command |

## Category: Configuration

| Command | Description | Effort |
|---|---|---|
| `/model <name>` | Switch LLM model/provider mid-session | 🟡 Requires hot provider switching |
| `/personality <name>` | Change agent personality | 🟢 GAIA has persona system, no TUI command |
| `/reasoning <level>` | Change reasoning effort (low/medium/high) | 🟢 Trivial |
| `/verbose` | Cycle tool progress display (off/new/all/verbose) | 🟢 Trivial |
| `/fast <mode>` | Toggle fast mode (OpenAI Priority / Anthropic Fast) | 🟢 Trivial |
| `/skin <name>` | Change TUI theme/colors | 🟢 Trivial |
| `/voice <on|off>` | Toggle voice mode | 🟡 Requires TTS integration |
| `/yolo` | Toggle auto-approve (skip dangerous confirmations) | 🟢 GAIA has trust modes, no /yolo |
| `/statusbar` / `/sb` | Toggle context status bar | 🟢 Trivial |
| `/footer` | Toggle metadata footer on responses | 🟢 Trivial |
| `/busy <mode>` | Control what Enter does while agent is working | 🟡 Medium |
| `/indicator <style>` | Change busy animation style | 🟢 Trivial |
| `/timestamps` | Toggle timestamps on messages | 🟢 Trivial |
| `/codex-runtime` | Toggle Codex app-server runtime | 🔴 Very specific to Codex |

## Category: Tools & Skills

| Command | Description | Effort |
|---|---|---|
| `/skills` browse/search/install | Skill management from within chat | 🟢 GAIA has CLI, not in-TUI |
| `/memory approve|reject` | Review pending memory writes | 🟡 Medium (write approval gate) |
| `/cron` | Manage scheduled tasks from chat | 🟢 GAIA has cron, no TUI command |
| `/learn <what>` | Create a skill from anything (dir, URL, workflow) | 🟡 Medium |
| `/suggestions` | Review agent-suggested automations | 🟡 Medium |
| `/blueprint <name>` | Create automation from template | 🟡 Medium |
| `/browser connect` | Manage browser connection for web tools | 🟢 GAIA has browser module, no TUI command |
| `/curator` | Background skill maintenance | 🟡 Medium |
| `/kanban` | Project board from chat | 🔴 Complex |
| `/bundles` | List/load skill bundles | 🟢 GAIA has skills, no bundles concept |
| `/reload-mcp` | Reload MCP servers from config | 🟢 Trivial |
| `/reload-skills` | Re-scan skills directory | 🟢 Trivial |
| `/plugins` | List plugins | 🟢 GAIA has plugins, no TUI command |

## Category: Info & System

| Command | Description | Effort |
|---|---|---|
| `/help` | Show help/command list | 🟢 Trivial |
| `/version` | Show GAIA version and build info | 🟢 Trivial |
| `/insights` | Usage analytics (last 30 days) | 🟡 Depends on cost tracking |
| `/credits` | Show credit balance | 🟡 Requires billing integration |
| `/billing` | Manage billing | 🟡 Requires billing integration |
| `/debug` | Upload debug report, get shareable link | 🟡 Medium |
| `/platforms` / `/gateway` | Show gateway adapter status | 🟢 Trivial (already implemented in gateway) |
| `/copy <n>` | Copy last response to clipboard | 🟢 Trivial |
| `/image <path>` | Attach image for next prompt | 🟡 Medium (vision support) |
| `/paste` | Attach clipboard image | 🟡 Medium |

## Category: Messaging-Only

| Command | Description | Effort |
|---|---|---|
| `/sethome` | Set current chat as delivery home | 🟢 Trivial |
| `/approve` | Approve pending dangerous command | 🟢 GAIA has ConfirmGuard |
| `/deny` | Deny pending dangerous command | 🟢 Trivial |
| `/commands` | Browse all commands (paginated) | 🟢 Trivial |
| `/restart` | Gracefully restart gateway | 🟢 Trivial |
| `/update` | Update GAIA to latest version | 🟡 Medium |
| `/topic` | Multi-session DM mode (Telegram) | 🟡 Medium |

## Summary

| Category | Missing |
|---|---|
| **Session** | 18 commands |
| **Configuration** | 14 commands |
| **Tools & Skills** | 13 commands |
| **Info** | 10 commands |
| **Messaging** | 7 commands |
| **Total** | **~62 commands** |

Many are trivial (< 30 min each). Some are major features (/goal, /kanban, /handoff).

---

*Generated 2026-07-21. Based on Hermes Agent v0.19.0.*
