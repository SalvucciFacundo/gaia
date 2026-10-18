# GAIA CLI Reference

## Core

| Command | Description |
|---|---|
| `gaia` | Start interactive TUI |
| `gaia exec <task>` | Execute task in headless mode |
| `gaia exec <task> --json` | JSON output |
| `gaia exec <task> --quiet` | Minimal output |

## Profiles & Per-Profile Tools

| Command | Description |
|---|---|
| `gaia profile list` | List all profiles with active/enabled indicators and tool counts |
| `gaia profile switch <name>` | Switch active profile (e.g. `dev`, `assistant`, `art`) |
| `gaia profile enable <name>` | Enable a profile |
| `gaia profile disable <name>` | Disable a profile (cannot disable active profile) |
| `gaia profile tools list [profile]` | List active tools and available catalog tools for a profile |
| `gaia profile tools enable <profile> <tool>` | Enable a specific catalog tool for a profile |
| `gaia profile tools disable <profile> <tool>` | Disable a specific tool for a profile |
| `gaia profile tools reset <profile>` | Reset profile to its suggested default toolset |

## Server Mode (OpenAI REST/SSE API & Web UI)

| Command | Description |
|---|---|
| `gaia serve` | Start API server on default port `8080` (alias: `gaia api`) |
| `gaia serve --port 9090` | Start server on custom port |
| `gaia serve --host 0.0.0.0` | Bind server to specific network interface |
| `gaia serve --key <token>` | Require Bearer token authentication (constant-time verification) |

The server provides:
```
POST /v1/chat/completions → OpenAI-compatible chat (JSON or SSE stream text/event-stream)
GET  /v1/models           → List available models and active profiles
GET  /healthz             → Health and liveness check
GET  /api/health          → Detailed health status
GET  /                    → Built-in Web UI Dashboard (templ + TailwindCSS + htmx)
```

## Messaging Gateways & Voice Engine

| Command | Description |
|---|---|
| `gaia gateway` or `gaia gateway run` | Start messaging gateway daemon for enabled platforms |
| `gaia gateway run --config <path>` | Run gateway with explicit configuration file |

Gateways run as a background service:
- **Platforms**: Telegram (polling/webhooks), WhatsApp (Meta Cloud API), Discord, Slack.
- **Audio Engine**: Inbound voice note transcription via OpenAI Whisper; outbound spoken notes via ElevenLabs / OpenAI TTS.
- **Interactive Approver**: Inline message buttons for human authorization on dangerous operations.

## Skills

| Command | Description |
|---|---|
| `gaia skill list [--json]` | List all installed and bundled skills |
| `gaia skill create <name> [flags]` | Create or update a skill (`--desc`, `--trigger`, `--body`, `--overwrite`) |
| `gaia skill show <name> [--raw] [--json]` | Inspect skill definition, workflow, and rules |
| `gaia skill delete <name> [-f\|--force]` | Remove a skill with confirmation prompt |
| `gaia skills search <q>` | Search skills by name or tags |
| `gaia skills install <name>` | Install a bundled skill |
| `gaia skills add-tap <url>` | Add a community tap (GitHub repo) |
| `gaia skills audit` | Security audit all skills for dangerous AST patterns |

## Backup & Restore

| Command | Description |
|---|---|
| `gaia backup create <profile> [-o file]` | Create manifest-backed TAR archive for profile |
| `gaia backup <file>` | Inspect archive manifest without unpacking |
| `gaia backup list [-dir <path>]` | List existing backup archives |
| `gaia restore <file> [-profile <name>] [-force]` | Restore databases, skills, and configuration from archive |

All archives feature SHA256 file validation, permission checks, and path-traversal attack protection.

## Self-Updater

| Command | Description |
|---|---|
| `gaia update --check` | Check GitHub releases for newer GAIA versions |
| `gaia update` | Download and apply the latest release binary |
| `gaia update --backup` | Create pre-update backup of data before upgrading binary |
| `gaia update --version <tag>` | Download and pin a specific release version (e.g. `v0.5.0`) |
| `gaia update --force` | Force update even if version is current or downgrade |

## Review & Gates (4R Receipt-Driven Development)

| Command | Description |
|---|---|
| `gaia review mode enable` | Enable opt-in Receipt-Driven Development (RDD) review (`--scope clone` or `--scope global`) |
| `gaia review mode disable` | Disable review mode (delivery uses ordinary repo policy) |
| `gaia review mode status` | Show active review mode and deciding source (clone vs global) |
| `gaia review start` | Start bounded 4R review (`--files <list>`, `--lens <name>`, `--judgment-day`) |
| `gaia review status` | Show review receipts status (`--change <name>` for specific change details) |
| `gaia review list` | List all review receipts (`--state <approved\|escalated\|invalidated>`) |
| `gaia review validate --gate <gate>` | Validate receipt at gate (`pre-commit`, `pre-push`, `pre-pr`) |
| `gaia review install-hooks` | Install git hook scripts (`.git/hooks/pre-commit`, `.git/hooks/pre-push`) |

## Policy & Permissions

| Command | Description |
|---|---|
| `gaia policy init` | Initialize a policy configuration (`.gaia/policy.yaml`) with default `sandbox` tier |
| `gaia policy init --tier=<tier>` | Initialize policy with specified tier (`read`, `sandbox`, `full`) |
| `gaia policy init --global` | Initialize global policy at `~/.config/gaia/policy.yaml` |

## Delivery

| Command | Description |
|---|---|
| `gaia delivery list` | List all queued delivery items (outbox) |
| `gaia delivery list --status <status>` | Filter queued items by status (`standby`, `released`, `discarded`, `failed`) |
| `gaia delivery status` | Show queue metrics summary |
| `gaia delivery release <id>` | Release a specific delivery item by ID (with CAS receipt verification) |
| `gaia delivery release --all` | Release all standby items in topological stacked branch order |
| `gaia delivery discard <id>` | Mark a queued item as discarded |
| `gaia delivery diff <id>` | Inspect PR title, body, and git diff stat for a queued item |

## Session & Diagnostics

| Command | Description |
|---|---|
| `gaia session list` | List recent conversations and multi-channel sessions |
| `gaia session restore <id>` | Restore and inspect previous session context |
| `gaia doctor` | Comprehensive environment, tool, and database diagnostics |
| `gaia desktop` | Launch Wails desktop UI app |


## Policy Guard & Permissions Control

| Command | Description |
|---|---|
| `gaia policy status` | Display effective permissions across 4 functional categories and blast radiuses |
| `gaia policy status -v` | Granular tool-by-tool policy evaluation breakdown |
| `gaia policy panel` | Launch interactive Cyberpunk TUI permissions control panel in standalone mode |
| `gaia policy init` | Initialize security policy (`.gaia/policy.yaml`) |
| `gaia policy init --tier=full` | Set default execution tier (`read`, `sandbox`, `full`) |
| `gaia policy init --global` | Initialize global user policy (`~/.config/gaia/policy.yaml`) |

## Reminis Blackboard Working Memory

| Command | Description |
|---|---|
| `gaia blackboard snapshot list` | List stored working memory snapshots with entry counts and SHA-256 prefixes |
| `gaia blackboard snapshot inspect <id>` | Verify SHA-256 cryptographic checksum and inspect snapshot entries |
| `gaia blackboard snapshot restore <id>` | Restore verified snapshot state into active Blackboard memory |

## Overnight Autonomous Execution

| Command | Description |
|---|---|
| `gaia overnight start "<prompt>"` | Start unattended task with interactive pre-flight alignment questionnaire |
| `gaia overnight list` | List all overnight plans with execution status |
| `gaia overnight review <plan_id>` | Inspect morning review pack (diffs, test execution logs, deferred actions) |
| `gaia overnight merge <plan_id>` | Merge/apply sandboxed worktree changes into main branch |
| `gaia overnight discard <plan_id>` | Discard sandboxed worktree without applying |

## TUI Commands (in-session)

| Command | Description |
|---|---|
| `@name <msg>` | Direct subagent chat |
| `/undo` | Undo last turn |
| `/retry` | Retry last user message |
| `/usage` | Show context usage breakdown |
| `/insights` | Show session analytics & token usage |
| `/permisos` | Interactive policy & permission guard panel |
| `/diff` | Interactive diff viewer |
| `/tasks` | List async tasks |
| `/cancel <id>` | Cancel an async task |
| `/create-agent` | Create dynamic subagent |
| `/trust <mode>` | Set confirmation trust mode |
| `/model` or `/models` | Show or change active model configuration |

## MCP Server Configuration

Servers with OAuth can pass tokens via config:

```yaml
mcp:
  servers:
    - name: discord
      command: ./discord-mcp
      access_token: "your-oauth-token"
      token_url: "https://discord.com/api/oauth2/token"
```

Tokens are injected as environment variables `MCP_ACCESS_TOKEN`, `ACCESS_TOKEN`, and `MCP_TOKEN_URL`.

---

## TUI Commands Reference

For a complete and detailed reference of all in-session interactive commands, see [TUI Commands Reference](tui-commands.md).
