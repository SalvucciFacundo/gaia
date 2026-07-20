# Proposal: Dynamic Subagent Generator

## Intent

GAIA's 12 subagents are hardcoded Go types registered at startup. Adding a new subagent requires code changes, recompilation, and a release. Users cannot create domain-specific subagents (e.g., "kubernetes-debugger", "sql-optimizer") tailored to their workflow. We need runtime subagent creation without Go plugins (which are Linux-only and fragile).

## Scope

### In Scope
- Orchestrator interview flow: name, description, tools (from existing tool registry), skills, personality/system-prompt hints
- `DynamicSubagent` struct implementing the `Subagent` interface — built from a config blob, not compiled code
- `Registry.RegisterDynamic(cfg)` — registers at runtime, persists to SQLite (`subagents` table)
- Startup loader: reads persisted dynamic subagents and re-registers them on boot
- Per-subagent Engram namespace: `gaia/subagent/<name>/` — isolates learning/memory
- CLI: `gaia subagent {create,list,delete,inspect}`
- Validation: name uniqueness, tool existence, description non-empty

### Out of Scope
- Custom tool implementation at runtime (dynamic subagents reuse existing tools only)
- Subagent marketplace / sharing between users
- Hot-reload without restart (persistence requires restart to re-register)
- Subagents that spawn other dynamic subagents (v1: static subagents only as parents)

## Capabilities

### New Capabilities
- `dynamic-subagent-runtime`: Config-driven subagent construction, interview flow, validation
- `subagent-persistence`: SQLite schema for dynamic subagents, startup loader
- `subagent-namespaced-memory`: Engram namespace per dynamic subagent

### Modified Capabilities
- None at the spec level — `Subagent` interface is unchanged; dynamic subagents satisfy it.

## Approach

Define `DynamicSubagentConfig` (Name, Description, AllowedTools []string, Skills []string, SystemPrompt string). `DynamicSubagent` holds this config and implements `Subagent.Execute` by delegating to `Spawner.RunLoop` with the config's system prompt and tool filter — the same path compiled subagents use. Persistence: a `subagents` table (name PK, config JSON, created_at). A `DynamicLoader` runs at startup, reads rows, calls `Registry.RegisterDynamic`. The interview is a new Brain tool `create_subagent` that collects fields via structured prompts and calls the loader.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/dynamic.go` | New | `DynamicSubagent`, `DynamicSubagentConfig`, `DynamicLoader` |
| `internal/agent/registry.go` | Modified | Add `RegisterDynamic`, `Unregister` |
| `internal/adapters/db/` | Modified | `subagents` table, migrations |
| `internal/agent/memory/` | Modified | Namespace per dynamic subagent |
| `cmd/gaia/subagent.go` | New | CLI subcommands |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Prompt injection via dynamic system prompt | Med | Sanitize; cap length; audit log on create |
| Tool misconfiguration (subagent granted dangerous tool) | Med | Restrict dynamic subagents to a safe-tool allowlist; require confirmation |
| SQLite schema drift | Low | Migration versioning; startup check |

## Rollback Plan

Feature-gate behind `subagents.dynamic_enabled: false` (default). Dynamic table is additive; compiled subagents unaffected. Revert = flip flag; existing dynamic rows become inert.

## Dependencies

- Existing `Spawner.RunLoop`, `Registry`, `ToolRegistry`, SQLite adapter, Engram namespace manager.

## Success Criteria

- [ ] `gaia subagent create` walks through interview, produces working subagent
- [ ] Dynamic subagent survives restart (re-registered from SQLite)
- [ ] Dynamic subagent memory isolated from other subagents in Engram
- [ ] Compiled subagents' behavior unchanged (regression tests pass)
