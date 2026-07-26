# Proposal: Policy Guard

## Intent

The current `ConfirmGuard` only protects the Brain's main loop. Sub-agents via `Spawner.RunLoop`, async tasks, and SDD pipeline phases execute tool calls without any permission check. This creates a security gap where destructive operations can run silently. We need a unified policy system that enforces permissions across ALL execution paths with tier-based autonomy and smart escalation.

## Scope

### In Scope
- `PolicyGuard` struct with 3 tiers (`read`, `sandbox`, `full`) + per-tool overrides + hardline blocklist
- Integration into `Spawner.RunLoop` — every tool call passes through policy evaluation
- TUI permission panel (`/permisos`) showing tier, overrides, path rules
- YAML persistence at project (`.gaia/permissions.yaml`) and global (`~/.config/gaia/permissions.yaml`) levels
- Install wizard question: "Modo Seguridad" vs basic mode
- `/seguridad on|off` runtime toggle
- Smart escalation: skip → alternative → ask-user fallback chain
- User-defined deny rules via YAML glob patterns

### Out of Scope
- Audit log UI (deferred — data captured, no viewer)
- Container/sandbox isolation (handled by existing docker executor)
- Supply-chain or dependency scanning

## Capabilities

### New Capabilities
- `policy-guard`: Policy-based permission system with tier evaluation, overrides, hardline blocklist, persistence, and TUI panel for autonomous agent execution

### Modified Capabilities
- `confirmation-system`: Replaced by policy-guard tier model. Existing `TrustMode` constants and `ConfirmGuard` become a thin adapter over `PolicyGuard`. `/trust` commands map to tier equivalents.

## Approach

1. **New types**: `PolicyTier` (read|sandbox|full), `OverridePolicy` (allow|deny|skip|ask-once|ask-session|ask-always|audit), `PolicyConfig`
2. **PolicyGuard** implements a `Policy` port (interface) — evaluated before every tool execution in both `kernel.go:handleToolCalls` and `spawner.go:RunLoop`
3. **PolicyStore** interface for YAML load/save (project + global merge, global wins on deny)
4. **Hardline blocklist**: immutable rules compiled into the binary, never overridable
5. **Smart escalation**: tier miss → skip silently → try registered alternatives → block and notify user with context
6. **TUI panel**: Bubbletea component bound to `PolicyGuard` state, editable overrides
7. **Install wizard**: `survey` prompt during first-run setup
8. **Migration**: existing `ConfirmGuard` preserved as adapter; `--yes`/`--dry-run` flags map to `full`/`read` tiers

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/core/guard.go` | Modified | Refactor to PolicyGuard + Policy port |
| `internal/core/domain/models.go` | Modified | Add PolicyTier, OverridePolicy, PolicyConfig types |
| `internal/agent/spawner.go` | Modified | Inject PolicyGuard into RunLoop |
| `internal/core/kernel.go` | Modified | Use Policy port instead of ConfirmGuard directly |
| `internal/adapters/tui/` | New + Modified | Permission panel component |
| `internal/core/policy_store.go` | New | PolicyStore port for YAML persistence |
| `internal/adapters/persistence/` | New | YAML adapter for PolicyStore |
| `cmd/gaia/exec.go` | Modified | Map flags to tiers, wizard integration |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Performance: tier check on every tool call | Low | Map lookup O(1); blocklist compiled as trie |
| Breaking change on upgrade | Medium | Default to basic mode (current behavior + blocklist); opt-in to full policy |
| TUI panel complexity | Medium | Isolated in `adapters/tui/`; no core coupling |

## Rollback Plan

Revert the `PolicyGuard` injection in `spawner.go` and `kernel.go` to use the original `ConfirmGuard`. The old types remain as adapter during migration — removal is a single commit.

## Dependencies

- None external. Uses existing YAML libraries if present; otherwise `gopkg.in/yaml.v3`.

## Success Criteria

- [ ] Every tool call in Brain loop AND Spawner RunLoop passes through PolicyGuard
- [ ] Hardline blocklist prevents `rm -rf /`, fork bombs, and similar — verified by integration test
- [ ] `/permisos` panel displays and edits overrides in real-time
- [ ] YAML persistence survives restart (project + global merge)
- [ ] Install wizard offers security mode selection
- [ ] `/seguridad on|off` toggles policy enforcement at runtime
- [ ] Existing `--yes`/`--dry-run` flags continue working via tier mapping
