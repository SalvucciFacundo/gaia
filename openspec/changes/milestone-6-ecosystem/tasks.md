# Tasks: Milestone 6 — Ecosystem

## Review Workload Forecast
- **400-line budget risk**: High (3 PRs of ~400 lines each)
- **Chained PRs recommended**: Yes
- **Decision needed before apply**: No (stacked-to-main resolved)
- **Chain strategy**: stacked-to-main

## PR 1: Gateway + Browser

- [x] 1.1 Wire Telegram adapter into gateway (implement GatewayAdapter port, wrap existing telebot adapter)
- [x] 1.2 Create `internal/gateway/` — GatewayAdapter interface, multi-platform dispatch, IncomingMessage normalization
- [x] 1.3 Discord MCP bridge: MCP client configured for Discord, register as gateway adapter
- [x] 1.4 Browser tools MCP plugin: optional install via config, MCPModule wrapper
- [x] 1.5 `gaia gateway` CLI: start, stop, status subcommands
- [x] 1.6 Gateway and browser tools tests

## PR 2: Plugins + Community Skills

- [x] 2.1 Plugin API: Plugin interface (Manifest, Start, Stop, Tools, Execute), plugin.json manifest, loaded from ~/.gaia/plugins/
- [x] 2.2 Plugin manager: scan, load, list, enable, disable plugins
- [x] 2.3 Community skills taps: Hub method for adding/removing GitHub tap sources
- [x] 2.4 Tap installer: git clone tap, scan for SKILL.md directories, register in hub index
- [x] 2.5 `gaia plugin` CLI: list, enable, disable, install + `gaia skills add-tap/remove-tap/list-taps`
- [x] 2.6 Plugin and tap tests

## PR 3: Webhooks + Scripts + LSP

- [x] 3.1 Webhook listener: HTTP server on configurable port, GitHub event parsing, HMAC-SHA256 verification
- [x] 3.2 Webhook triggers automations: dispatch verified events as agent tasks (reuse cron executeJob model)
- [x] 3.3 Script injection: pre-processing scripts via config, stdout→context injection, [SILENT] pattern
- [x] 3.4 LSP client: connect to gopls/pylsp via stdio transport, get diagnostics as tool output
- [x] 3.5 `gaia webhook` CLI (start, list-subs) + `gaia lsp` CLI (list, diagnostics)
- [x] 3.6 Webhook, script, and LSP tests
