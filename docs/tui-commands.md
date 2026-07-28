# TUI Commands Reference

This document provides a comprehensive reference for all in-session interactive commands (slash commands) available in the GAIA terminal user interface (TUI) and gateways.

---

## 💬 Conversation Flow

| Command | Description |
|---------|-------------|
| `/undo` | Reverses the last turn — removes the last user message and everything the AI generated in response. Useful when the agent misunderstood or went in the wrong direction. Can be used multiple times to go back several turns. |
| `/retry` | Removes the last AI response and re-runs the last user message through the full agent loop. The agent will re-process your request from scratch with a clean context. |
| `/new` or `/reset` | Completely clears the conversation and starts a fresh session. All message history is deleted from the current context. Configuration and model settings are preserved. |
| `/clear` | Clears the TUI display without affecting the conversation state. Messages remain in context and will reappear as the conversation progresses. |

---

## 💾 Persistence & History

| Command | Description |
|---------|-------------|
| `/history` | Displays the full conversation history of the current session, showing all user messages and AI responses with role prefixes. Messages are truncated for readability. |
| `/save` or `/save <name>` | Saves the current conversation as a named session in the SQLite database. If no name is provided, one is auto-generated (e.g., `Session 2026-07-22 14:30`). |
| `/title <name>` | Renames the current session. If the session was saved, the name is updated in the database. |
| `/sessions` | Lists all saved sessions with their ID, name, and creation date. |
| `/resume <id>` | Loads and resumes a previously saved session by its ID prefix (partial match supported). |

---

## 🧠 Context & Memory

| Command | Description |
|---------|-------------|
| `/compress` | Forces manual context compaction. GAIA normally auto-compacts when the conversation exceeds the configured threshold (default: 50 messages). Summarizes older messages while keeping recent messages verbatim. |
| `/kg` | Show Knowledge Graph status and fact count. |
| `/kg on` | Enable Knowledge Graph recall for this session (automatically injects relevant facts as context before each message). |
| `/kg off` | Disable Knowledge Graph recall. |
| `/kg stats` | Show facts stored in the Knowledge Graph grouped by topic. |
| `/kg clear` | Clear all stored facts in the Knowledge Graph for this session. |

---

## 🔀 Parallel & Background Work

| Command | Description |
|---------|-------------|
| `/moa <prompt>` | One-shot Mixture of Agents. Fans out the prompt to all configured LLM providers in parallel, collects all responses, and synthesizes them into a single answer using the primary model. |
| `/background <prompt>` | Spawns an `explorer` subagent in a background goroutine and returns a task ID immediately. Track progress with `/tasks` and cancel with `/cancel <taskid>`. |
| `/queue <prompt>` or `/q <prompt>` | Adds a message to the processing queue. The queued message is automatically processed after the current task completes. Use `/queue` or `/q` alone to view queued items, or `/queue clear` to empty the queue. |

---

## 🔄 Session Handoff

| Command | Description |
|---------|-------------|
| `/handoff <platform>` | Saves the current session and shows step-by-step instructions for resuming the conversation on another messaging platform. Supports: `telegram`, `discord`, `slack`, `whatsapp`, `signal`, and `cli`. |

---

## 🌲 State & Branching

| Command | Description |
|---------|-------------|
| `/branch` or `/branch <name>` | Creates a named branch point by saving the full conversation state to a JSON snapshot file. Branches are stored in `%TEMP%/gaia-snapshots/` with a `branch-` prefix. |
| `/branches` | Lists all saved branch points. |
| `/snapshot save <name>` | Saves the raw conversation state to a JSON file in `%TEMP%/gaia-snapshots/` (useful for backups/transfer). |
| `/snapshot load <name>` | Restores a previously saved snapshot, inserting all messages into the current conversation. |

---

## 📱 Session Mode

| Command | Description |
|---------|-------------|
| `/session` | Displays the current session mode and active sessions. |
| `/session unify` | Sets session mode to **unify** (default). All messages from all platforms go to the same session with the same conversation history, prefixed with the platform name. |
| `/session isolate` | Sets session mode to **isolate**. Each platform gets its own independent session with its own history (e.g., `telegram-default`). |
| `/session ask` | Sets session mode to **ask** (smart prompt). When a message arrives from a platform other than the active one, GAIA asks whether to unify or isolate. |

---

## 🎯 Goal System

| Command | Description |
|---------|-------------|
| `/goal <text>` | Sets a persistent goal that the agent works toward across multiple turns. The agent automatically continues after each turn until the goal is evaluated as complete. |
| `/subgoal <text>` | Adds a specific criterion to the active goal (up to 10 subgoals). |
| `/goals` | Displays the current active goal and all subgoals. |
| `/goal clear` | Clears the active goal and all subgoals, returning the agent to normal single-turn operation. |

---

## 🎮 Mid-Execution Control

| Command | Description |
|---------|-------------|
| `/steer <message>` | Injects a guidance message that the agent sees **before its next tool call** without waiting for the current turn to finish. |

---

## ⚙️ Configuration Commands

| Command | Description |
|---------|-------------|
| `/model` | Lists all available LLM providers. The active provider is marked with `➤`. |
| `/model <name>` | Switches the active LLM provider mid-session (e.g., `/model anthropic`). |
| `/models` | Lists all available models from the current provider. Switch to a model with `/model <name>`. |
| `/reasoning <level>` | Changes the reasoning effort of the LLM. Accepts `low`, `medium`, or `high`. |
| `/personality <name>` | Switches the agent's personality. Options: `teacher`, `professional`, `strict`, `friendly`. |
| `/yolo` | Toggles YOLO mode. When ON, all commands are auto-approved (except catastrophic commands). |
| `/verbose` | Cycles through 4 levels of tool output display: `off` → `results` → `tool calls` → `all`. |
| `/timestamps` | Toggles message timestamps in the conversation. |
| `/statusbar` or `/sb` | Toggles the status bar at the bottom of the TUI. |
| `/footer` | Toggles metadata footers on AI responses (shows tokens, model, response time). |
| `/indicator` | Cycles through spinner styles: `dots`, `line`, `pipe`, `circle`. |
| `/skin <name>` | Changes the TUI color theme (e.g., `rose-pine`, `dark`, `light`). |

---

## 🛠️ Tools & Skills Commands

| Command | Description |
|---------|-------------|
| `/skills` | Shows the skill management menu. |
| `/skills list` | Lists all installed skills. |
| `/skills search <query>` | Searches the Skills Hub for skills matching the query. |
| `/skills install <name>` | Installs a skill by name from the Skills Hub. |
| `/skills remove <name>` | Uninstalls a skill. |
| `/skills stats` | Shows usage statistics for installed skills. |
| `/skills audit` | Runs a security audit on all installed skills, scanning for dangerous patterns. |
| `/cron` | Shows the cron job management menu. |
| `/cron list` | Lists all scheduled cron jobs. |
| `/cron add <schedule> <task>` | Creates a new cron job (standard cron syntax). |
| `/cron remove <id>` | Removes a scheduled job by ID. |
| `/cron pause <id>` | Pauses a scheduled job. |
| `/cron resume <id>` | Resumes a paused scheduled job. |
| `/cron run <id>` | Runs a cron job immediately. |
| `/reload-mcp` | Displays instructions for reloading MCP servers. |
| `/reload-skills` | Reloads the skills index from disk. |
| `/plugins` | Shows plugin management instructions. |
| `/browser` or `/browser connect` | Shows browser automation configuration. |
| `/memory pending` | Lists recent memory operations from the current session that need review. |
| `/memory approve <id>` | Approves a pending memory write. |
| `/memory reject <id>` | Rejects a pending memory write. |
| `/learn <source>` | Scans a directory or uses a text description to generate a new skill saved in `~/.gaia/skills/`. |
| `/suggestions` | Analyzes your project and recommends skills to install. |
| `/blueprint <name>` | Creates a new skill from a predefined template (e.g., `daily-report`, `code-review`). |
| `/curator` | Scans all installed skills and reports structural or formatting issues. |

---

## ℹ️ Info & System Commands

| Command | Description |
|---------|-------------|
| `/help` | Displays a categorized list of all available commands. |
| `/version` | Shows the GAIA version, Go runtime version, and license information. |
| `/platforms` or `/gateway` | Shows gateway platform configuration and status. |
| `/copy` | Copies the last AI response to the system clipboard (use `/copy <n>` for the n-th previous response). |
| `/insights` | Shows session analytics (message counts, total tool calls, LLM cost). |
| `/debug` | Collects and displays system diagnostic information. |
| `/credits` | Shows credit/usage balance information based on your LLM provider. |
| `/billing` | Shows billing management information and links to your provider's dashboard. |
| `/image <path>` | Loads an image file from the specified path for vision processing (stub/pending). |
| `/paste` | Attaches an image from the system clipboard for vision processing (stub/pending). |

---

## 💬 Messaging Commands (Gateway)

These commands are designed for gateway mode (Telegram, Discord, Slack) but also work in the TUI:

| Command | Description |
|---------|-------------|
| `/sethome` | Marks the current chat as the delivery home for notifications, cron results, and alerts. |
| `/approve` | Approves a pending dangerous command confirmation. |
| `/deny` | Denies a pending dangerous command confirmation, prompting the agent to find an alternative. |
| `/commands` | Lists all available slash commands organized by category. |
| `/restart` | Shows restart instructions for the current gateway mode. |
| `/update` | Shows update instructions for GAIA. |
| `/topic` | Manages multi-session DM topics. `/topic new <name>`, `/topic switch <name>`, `/topic list`. |
