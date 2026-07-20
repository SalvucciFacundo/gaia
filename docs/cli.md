# CLI Reference

GAIA provides a comprehensive command-line interface. All commands follow the pattern:

```bash
gaia <command> [subcommand] [flags]
```

---

## Core Commands

### `gaia`

Start the interactive TUI (default). Opens a Bubbletea terminal interface with chat, streaming, and tool execution.

```bash
gaia
```

**Keybindings:**
- `Enter` — Send message
- `Ctrl+C` / `Esc` — Quit
- `/help` — Show available slash commands
- `/model <name>` — Switch model
- `/persona <name>` — Switch persona
- `/trust <mode>` — Change confirmation mode
- `/plan` — Switch to plan mode (read-only, no code changes)
- `/build` — Switch to build mode (full agent)
- `/sdd` — Force SDD trigger
- `/direct` — Bypass SDD trigger
- `/undo` — Undo last turn
- `/retry` — Retry last turn

### `gaia exec`

Execute a single task without the TUI. Returns result immediately and exits.

```bash
gaia exec "explain this codebase"
gaia exec "refactor main.go to use early returns" --json
gaia exec "list all TODO comments" --quiet
gaia exec "what would change if I migrated to React 19?" --dry-run
```

**Flags:**
| Flag | Description |
|---|---|
| `--json` | Output structured JSON envelope |
| `--quiet` | Suppress non-essential output |
| `--dry-run` | Show plan without executing changes |
| `--yes` | Auto-approve confirmations (overrides confirmation mode) |

### `gaia review`

Code review with GGA (Gentleman Guardian Angel).

```bash
gaia review start                    # Start review of current changes
gaia review start --judgment-day     # Start adversarial review (high-risk)
gaia review status                   # Show current review status
gaia review validate                 # Validate receipt against current content
gaia review list                     # List recent reviews
gaia review install-hooks            # Install pre-commit/pre-push git hooks
```

### `gaia skills`

Manage installed skills.

```bash
gaia skills list                     # List installed skills
gaia skills search "testing"         # Search available skills
gaia skills install go               # Install Go-specific skills
gaia skills install typescript       # Install TypeScript/Node.js skills
gaia skills activate go-testing      # Activate a skill
gaia skills deactivate go-linting    # Deactivate a skill
gaia skills remove go-testing        # Delete a skill
gaia skills add-tap github.com/user/skills  # Add community skill tap
gaia skills list-taps                # List installed taps
gaia skills remove-tap my-tap        # Remove a skill tap
```

---

## SDD Workflow Commands

```bash
gaia sdd-init                        # Bootstrap SDD in current project
gaia sdd-explore                     # Investigate before proposing
gaia sdd-new "feature-name"          # Start a new SDD change
gaia sdd-status                      # Show current SDD state
gaia sdd-continue                    # Continue next SDD phase
gaia sdd-ff "feature-name"           # Fast-forward through planning
gaia sdd-archive                     # Archive completed change
gaia sdd-onboard                     # Guided SDD walkthrough
```

---

## Cron & Automation

```bash
gaia cron create "0 2 * * *" "task" --name "nightly"         # Create cron job
gaia cron create "every 1h" "check status" --name "monitor"   # Human-readable interval
gaia cron list                                                  # List cron jobs
gaia cron pause "nightly"                                       # Pause a job
gaia cron resume "nightly"                                      # Resume a job
gaia cron remove "nightly"                                      # Delete a job
gaia cron start                                                  # Start cron daemon

gaia webhook subscribe pr-review \                               # GitHub webhook
  --events "pull_request" \
  --prompt "Review this PR" \
  --deliver terminal
gaia webhook start                                               # Start webhook listener
gaia webhook list-subs                                           # List subscriptions
```

---

## Gateway (Messaging)

```bash
gaia gateway start                   # Start messaging gateway
gaia gateway stop                    # Stop messaging gateway
gaia gateway status                  # Show gateway status
```

---

## Plugin System

```bash
gaia plugin list                     # List installed plugins
gaia plugin enable my-plugin         # Enable a plugin
gaia plugin disable my-plugin        # Disable a plugin
gaia plugin install ./plugin.so      # Install a plugin
gaia plugin remove my-plugin         # Remove a plugin
```

---

## LSP Integration

```bash
gaia lsp list                        # List connected LSP servers
gaia lsp diagnostics                 # Get diagnostics from LSP
```

---

## System

```bash
gaia doctor                          # System health check
gaia doctor --json                   # Health check as JSON
gaia config get llm.provider         # Read config value
gaia config set llm.provider anthropic  # Set config value
gaia memory export --format obsidian --out ./gaia-memory  # Export memory to Obsidian
gaia memory import --from ./gaia-memory                    # Import edited memory

gaia audit secrets                   # Scan for committed secrets
gaia audit skills                    # Scan skills for dangerous patterns
gaia security log                    # Show security events
```

---

## Session Management

```bash
gaia session restore <id>            # Resume previous session
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error |
| `2` | Invalid arguments |
| `3` | Tool execution error |
| `4` | Review blocked |
| `5` | Budget exhausted |
| `125` | Command not executed (dry-run or blocked) |
