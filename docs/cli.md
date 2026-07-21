# GAIA CLI Reference

## Core

| Command | Description |
|---|---|
| `gaia` | Start interactive TUI |
| `gaia exec <task>` | Execute task in headless mode |
| `gaia exec <task> --json` | JSON output |
| `gaia exec <task> --quiet` | Minimal output |

## Server Mode

| Command | Description |
|---|---|
| `gaia serve` | Start HTTP server (default port 8080) |
| `gaia serve 9090` | Custom port |

The server exposes:
```
GET  /health           → {"status": "ok"}
POST /message          → {"content": "..."} → {"status": "ok"}
```

## Skills

| Command | Description |
|---|---|
| `gaia skills list` | List all available skills |
| `gaia skills search <q>` | Search skills by name/tags |
| `gaia skills install <name>` | Install a bundled skill |
| `gaia skills remove <name>` | Remove a user skill |
| `gaia skills activate <name>` | Activate a skill |
| `gaia skills deactivate <name>` | Deactivate a skill |
| `gaia skills add-tap <url>` | Add a community tap (GitHub) |
| `gaia skills remove-tap <url>` | Remove a tap |
| `gaia skills list-taps` | List installed taps |
| `gaia skills search-hub <q>` | Search GitHub for skill repos |
| `gaia skills stats` | Show skill usage statistics |
| `gaia skills audit` | Security audit all skills |

### Tap URL formats accepted
```
gaia skills add-tap owner/repo
gaia skills add-tap github.com/owner/repo
gaia skills add-tap https://github.com/owner/repo
```

## Plugins

| Command | Description |
|---|---|
| `gaia plugin list` | List all installed plugins and their status |
| `gaia plugin enable <name>` | Enable an installed plugin |
| `gaia plugin disable <name>` | Disable a plugin |
| `gaia plugin install <path>` | Install a plugin from a local directory |
| `gaia plugin remove <name>` | Remove an installed plugin |

## Review

| Command | Description |
|---|---|
| `gaia review start` | Start bounded review |
| `gaia review validate --gate <gate>` | Validate receipt at gate |
| `gaia review staged` | Review staged changes |

## Gateway

| Command | Description |
|---|---|
| `gaia gateway start` | Start messaging gateway |
| `gaia gateway stop` | Stop messaging gateway |
| `gaia gateway status` | Show gateway status |

## Session

| Command | Description |
|---|---|
| `gaia session list` | List recent conversations |
| `gaia session restore <id>` | Load a previous session |

## Other

| Command | Description |
|---|---|
| `gaia doctor` | System diagnostics |
| `gaia onboard` | Guided SDD walkthrough |
| `gaia desktop` | Launch Wails desktop app |
| `gaia cron` | Cron scheduler |
| `gaia tracker check` | Check upstream releases |
| `gaia tracker report` | Port status report |
| `gaia tracker port <id>` | Create port issue |
| `gaia webhook` | Webhook subscriptions |
| `gaia lsp` | LSP integration |

## TUI Commands (in-session)

| Command | Description |
|---|---|
| `@name <msg>` | Direct subagent chat |
| `/undo` | Undo last turn |
| `/retry` | Retry last user message |
| `/usage` | Show context usage breakdown |
| `/cost` | Show LLM cost summary |
| `/tools` | List available tools |
| `/tasks` | List async tasks |
| `/cancel <id>` | Cancel an async task |
| `/create-agent` | Create dynamic subagent |
| `/trust <mode>` | Set confirmation trust mode |
| `/plan` | Switch to plan mode |
| `/build` | Switch to build mode |
| `/mode` | Show current mode |

## MCP Server Configuration

Servers with OAuth can pass tokens via config:

\\\yaml
mcp:
  servers:
    - name: discord
      command: ./discord-mcp
      access_token: "your-oauth-token"
      token_url: "https://discord.com/api/oauth2/token"
\\\

Tokens are injected as environment variables \MCP_ACCESS_TOKEN\, \ACCESS_TOKEN\, and \MCP_TOKEN_URL\.
