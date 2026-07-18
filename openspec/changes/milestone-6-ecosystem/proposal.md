# Proposal: Milestone 6 — Ecosystem

## Intent

GAIA has Milestones 1-5 complete (core loop, subagents, skills hub, review/quality, production backends). Milestone 6 transforms GAIA from a standalone agent into an **ecosystem platform**: messaging integrations, extensible plugins, community skill distribution, webhook-driven automation, code analysis via LSP, and scriptable pipelines.

## Scope

### In Scope
- **messaging-gateway**: Telegram + Discord via MCP bridge; unified adapter pattern
- **browser-tools**: Optional MCP plugin for web navigation/screenshots
- **execute-code-rpc**: JSON-RPC server exposing agent tools to Python/Go scripts
- **lsp-integration**: LSP client for diagnostics, completions, hover
- **community-skills-taps**: GitHub-based skill distribution (taps)
- **plugin-api**: Third-party plugin interface (tools, subagents, UI)
- **webhook-subscriptions**: GitHub event listener with HMAC verification
- **script-injection**: Pre-processing scripts before agent runs

### Out of Scope
- Full Discord native adapter (Discord via MCP bridge only)
- Plugin marketplace/UI (filesystem-based plugin loading only)
- LSP server implementation (client only)
- Webhook delivery (receive-only; outbound webhooks deferred)
- Script injection for non-shell languages

## Capabilities

### New Capabilities
- `messaging-gateway`: Multi-platform messaging adapter with Telegram + Discord (via MCP)
- `browser-tools`: Optional MCP plugin for browser automation
- `execute-code-rpc`: JSON-RPC server for external script tool invocation
- `lsp-integration`: LSP client integration for code analysis
- `community-skills-taps`: GitHub tap distribution for skills hub
- `plugin-api`: Third-party plugin loading from `~/.gaia/plugins/`
- `webhook-subscriptions`: HTTP webhook listener for GitHub events
- `script-injection`: Pre-agent script execution with stdout context injection

### Modified Capabilities
None (all new capabilities; no existing spec-level behavior changes).

## Approach

### Delivery Strategy: 3 Stacked PRs (to main)

| PR | Capabilities | Rationale |
|----|-------------|-----------|
| PR 1 | messaging-gateway, browser-tools | External communication + MCP plugin pattern |
| PR 2 | plugin-api, community-skills-taps | Extensibility foundation + community distribution |
| PR 3 | webhook-subscriptions, script-injection, lsp-integration | Automation + code analysis |

### Execution Order
PRs are stacked-to-main: PR1 -> PR2 -> PR3. Each targets `main`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/gateway/` | New | Messaging gateway with platform adapters |
| `internal/plugins/` | New | Plugin API, loader, and lifecycle |
| `internal/rpc/` | New | JSON-RPC server for external scripts |
| `internal/lsp/` | New | LSP client for diagnostics |
| `internal/webhook/` | New | HTTP webhook listener |
| `internal/scripts/` | New | Pre-agent script injection |
| `internal/skills/hub.go` | Modified | Add tap support for GitHub repos |
| `internal/adapters/telegram/telegram.go` | Modified | Wire into gateway adapter interface |
| `internal/mcp/` | Modified | Add browser-tools MCP package |
| `internal/cron/delivery.go` | Modified | Add gateway delivery target |
| `cmd/gaia/` | Modified | New subcommands: `gateway`, `rpc`, `webhook`, `plugins` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Discord MCP bridge instability | Medium | Abstract behind gateway port; fallback to terminal-only |
| Plugin security (arbitrary code) | High | Sandboxed loading; manifest validation; no hot-reload |
| Webhook HMAC bypass | Medium | Strict timing validation; reject unsigned payloads |
| Script injection path traversal | High | Resolve + validate script paths; no symlink escape |
| LSP server lifecycle management | Low | Lazy connect; reconnect on failure; timeout-bounded |

## Rollback Plan

All capabilities are independently feature-gated via config:
- **Gateway**: `gateway.enabled` defaults to false. No gateway = existing TUI/headless unaffected.
- **Browser tools**: Opt-in via `gaia mcp install browser`. Not loaded by default.
- **RPC**: Opt-in via `gaia rpc start`. Separate listener, no impact on main loop.
- **LSP**: Opt-in via `lsp.servers` config. Empty = no LSP tools.
- **Taps**: Additive. Existing skills hub works without taps.
- **Plugins**: `plugins.enabled` defaults to false. No plugins dir = no loading.
- **Webhooks**: `webhook.enabled` defaults to false. No listener started.
- **Scripts**: `scripts.pre_run` config. Empty = no pre-processing.

## Dependencies

- `golang.org/x/crypto` for HMAC webhook verification (already in go.sum)
- Telegram bot token (existing `telebot.v3` dependency)
- Discord MCP server binary (user-installed)
- LSP server binaries (user-installed, e.g., gopls, pylsp)

## Success Criteria

- [ ] Telegram messages trigger Brain.ProcessMessage and receive responses
- [ ] Discord via MCP bridge sends/receives messages
- [ ] External Python script can call agent tools via JSON-RPC
- [ ] LSP diagnostics appear as tool output in agent context
- [ ] `gaia skills add-tap github.com/user/repo` discovers and installs skills
- [ ] Third-party plugin adds a tool visible in `gaia tools list`
- [ ] GitHub push event triggers configured automation
- [ ] Pre-run script stdout injected as system context
