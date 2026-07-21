# Plugin System Architecture & Guide

GAIA features a flexible third-party plugin system. Plugins allow extending GAIA with custom tools, external integrations, or specialized subagents without modifying the core binary.

---

## Key Concepts

- **Language Agnostic**: Plugins are standalone binaries or scripts written in any language (Go, Python, Node.js, Rust, Bash, etc.).
- **JSON-RPC Transport**: Plugins communicate with GAIA over `stdio` using JSON-RPC (compatible with Model Context Protocol - MCP standards).
- **Isolated Execution**: Each plugin runs as a child process with explicit tool and subagent declarations.

---

## Plugin Directory Structure

Plugins reside in `~/.gaia/plugins/<plugin-name>/`:

```
~/.gaia/plugins/my-plugin/
├── plugin.json       # Required manifest
└── bin/
    └── my-plugin     # Executable binary or entry script
```

---

## Manifest Format (`plugin.json`)

Every plugin directory must contain a valid `plugin.json` manifest file:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "Custom integration plugin for external telemetry",
  "command": "./bin/my-plugin",
  "args": ["--mode", "server"],
  "tools": [
    "telemetry_fetch",
    "telemetry_post"
  ],
  "subagents": [
    "telemetry_analyzer"
  ]
}
```

### Fields

| Field | Type | Description | Required |
|---|---|---|---|
| `name` | string | Unique plugin name | Yes |
| `version` | string | SemVer version string | Yes |
| `description` | string | Short summary of plugin capabilities | Yes |
| `command` | string | Path to executable command or binary | Yes |
| `args` | array | Arguments passed to command on startup | No |
| `tools` | array | List of tool names exposed by this plugin | No |
| `subagents` | array | List of subagent names provided by this plugin | No |

---

## Plugin Management CLI

GAIA provides subcommands to manage local plugins:

| Command | Description |
|---|---|
| `gaia plugin list` | List installed plugins, status (enabled/disabled), version, and tools |
| `gaia plugin install <path>` | Install a plugin from a local source directory |
| `gaia plugin enable <name>` | Enable a plugin |
| `gaia plugin disable <name>` | Disable a plugin |
| `gaia plugin remove <name>` | Uninstall and remove a plugin |

### Example Workflow

```bash
# Install plugin from local directory
gaia plugin install ./my-custom-plugin

# Check installed plugins
gaia plugin list

# Enable or disable plugin
gaia plugin enable my-custom-plugin
gaia plugin disable my-custom-plugin
```
