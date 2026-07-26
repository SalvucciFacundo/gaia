# Design: Policy Guard

## Technical Approach

Replace the incomplete `ConfirmGuard` (kernel-only) with a unified `PolicyGuard` that enforces tier-based permissions across ALL execution paths: `Brain.handleToolCalls`, `Spawner.RunLoop`, and async SDD tasks. The guard wraps the existing `ConfirmGuard` as an adapter for backward compatibility while adding hardline blocklist evaluation, per-tool overrides, YAML persistence, and smart escalation.

## Architecture Decisions

### Decision: New PolicyGuard wrapping ConfirmGuard

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Extend ConfirmGuard in-place | Breaks existing `ShouldConfirm`/`Approve` contract, ripples to all callers | **Rejected** |
| New PolicyGuard + ConfirmGuard adapter | Clean separation, old code keeps working during migration | **Chosen** |
| Replace ConfirmGuard entirely | No migration path; all callers break at once | **Rejected** |

**Rationale**: `ConfirmGuard` is wired into `Brain` (kernel.go:579) and `exec.go:105`. Wrapping preserves the `ShouldConfirm`/`Approve` interface while `PolicyGuard` adds tier evaluation on top.

### Decision: Dual injection points (kernel + spawner)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Inject only in kernel.go | Sub-agents remain unguarded — the exact gap this change fixes | **Rejected** |
| Inject in kernel.go + spawner.go | Covers both paths; spawner gets policy via `SpawnerConfig` | **Chosen** |
| Inject in ToolRegistry.Execute | Single chokepoint, but registry is a dispatch layer — mixing policy into routing violates SRP | **Rejected** |

**Rationale**: `Filtered()` controls WHICH tools are visible; `PolicyGuard` controls WHAT visible tools can do. These are orthogonal concerns.

### Decision: YAML persistence with merge precedence

| Layer | Path | Override power |
|-------|------|----------------|
| Hardline | compiled-in `[]string` | Absolute — never overridable |
| Global | `~/.config/gaia/policy.yaml` | User deny wins over project |
| Project | `.gaia/policy.yaml` | Per-project customization |
| Session | in-memory map | Ephemeral, cleared on restart |

**Merge order**: hardline > global deny > project deny > tier defaults > tool overrides.

### Decision: Smart escalation chain

```
tool call → hardline block? → ERROR (no fallback)
           → user deny?     → ERROR (no fallback)
           → tier allows?   → EXECUTE
           → override?      → allow/deny/skip/ask per policy
           → skip           → agent gets error, tries alternative
           → all exhausted  → escalation prompt to user via TUI
```

## Data Flow

```
Agent/Spawner ──→ PolicyGuard.Evaluate(toolName, args)
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
     Hardline      UserDeny     TierCheck
     (compiled)    (YAML)      (config)
          │            │            │
          └────────────┼────────────┘
                       ▼
                 Override? ──→ allow/deny/skip/ask
                       │
                       ▼
              Execute or Escalate
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/core/policy.go` | Create | `PolicyGuard` struct, `PolicyTier`, `OverridePolicy`, `Evaluate()` method, smart escalation |
| `internal/core/policy_store.go` | Create | `PolicyStore` interface + YAML adapter (global + project load/save/merge) |
| `internal/core/policy_test.go` | Create | Table-driven tests for tier evaluation, blocklist, overrides, merge precedence |
| `internal/core/guard.go` | Modify | Add `// Deprecated` comment; `ConfirmGuard` kept as adapter |
| `internal/agent/spawner.go` | Modify | Add `Policy *PolicyGuard` to `SpawnerConfig`; evaluate before `filtered.Execute` at line 198 |
| `internal/core/kernel.go` | Modify | Replace `b.guard.ShouldConfirm` with `b.policy.Evaluate` in `handleToolCalls` (line 579) |
| `internal/adapters/tui/policy_panel.go` | Create | `/permisos` Bubbletea panel — shows tier, overrides, path rules; editable |
| `cmd/gaia/exec.go` | Modify | Add `--policy-tier` flag; map `--yes`→`full`, `--dry-run`→`read` |
| `internal/core/domain/models.go` | Modify | Add `PolicyConfig` struct; add `Policy PolicyConfig` field to `Config` |

## Interfaces / Contracts

```go
// internal/core/policy.go

type PolicyTier string
const (
    TierRead    PolicyTier = "read"    // read-only tools only
    TierSandbox PolicyTier = "sandbox" // write within project scope
    TierFull    PolicyTier = "full"    // all tools allowed
)

type OverridePolicy string
const (
    OverrideAllow      OverridePolicy = "allow"
    OverrideDeny       OverridePolicy = "deny"
    OverrideSkip       OverridePolicy = "skip"
    OverrideAskOnce    OverridePolicy = "ask-once"
    OverrideAskSession OverridePolicy = "ask-session"
    OverrideAskAlways  OverridePolicy = "ask-always"
    OverrideAudit      OverridePolicy = "audit"
)

type PathRule struct {
    Pattern string       `yaml:"pattern"` // glob, e.g. "/etc/**"
    Policy  OverridePolicy `yaml:"policy"`
}

type PolicyConfig struct {
    Tier      PolicyTier                `yaml:"tier"`
    Overrides map[string]OverridePolicy `yaml:"overrides"`
    DenyRules []string                  `yaml:"deny_rules"`
    PathRules []PathRule                `yaml:"path_rules"`
}

type PolicyGuard struct {
    config   PolicyConfig
    session  map[string]bool  // ask-session approvals
    store    PolicyStore
    hardline []string         // compiled-in blocklist
}

// Evaluate returns (allowed, policy, error). If not allowed, the caller
// decides whether to skip, escalate, or return error to the agent.
func (pg *PolicyGuard) Evaluate(toolName string, args map[string]interface{}) (bool, OverridePolicy, error)

// internal/core/policy_store.go

type PolicyStore interface {
    LoadGlobal() (*PolicyConfig, error)
    LoadProject(root string) (*PolicyConfig, error)
    Save(cfg *PolicyConfig, scope string) error // "global" | "project"
    Merge(global, project *PolicyConfig) *PolicyConfig
}
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `PolicyGuard.Evaluate` — tier, blocklist, overrides, path rules | Table-driven tests with mock `PolicyStore` |
| Unit | `PolicyStore` merge precedence | Temp dir + YAML round-trip |
| Unit | Smart escalation chain | Mock agent with blocked tool, verify skip→alternative→escalate |
| Integration | `Spawner.RunLoop` with `PolicyGuard` | Wire real registry, verify sub-agent tool calls are gated |
| Integration | `kernel.go` with `PolicyGuard` | Existing test suite + policy layer |
| RED | Hardline blocklist: `rm -rf /`, fork bombs, `curl | sh` | Integration test with shell module |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|----------|---------------|-----------------|-------------------|
| Shell command allowlist | Applicable | PolicyGuard evaluates BEFORE shell module; hardline blocklist catches `rm -rf /`, `:(){ :\|:& };:` regardless of shell allowlist | Test: hardline blocks destructive patterns even if shell allowlist permits `rm` |
| Git repository selection | N/A | PolicyGuard operates at tool level, not git-cwd level | None |
| Commit state | N/A | No git index interaction | None |
| Push state | N/A | No push logic in policy layer | None |
| PR commands | N/A | No PR automation in policy layer | None |

## Migration / Rollout

1. `ConfirmGuard` stays functional — `PolicyGuard` wraps it, not replaces it.
2. Default tier on upgrade: `full` (matches current `TrustNever` behavior + hardline blocklist).
3. `--yes` maps to `TierFull`, `--dry-run` maps to `TierRead`.
4. Opt-in to stricter tiers via `--policy-tier` flag or `/permisos` TUI panel.
5. Removal of `ConfirmGuard` adapter is a single future commit after adoption.

## Open Questions

- [ ] Should `ask-once` approvals persist across Brain compaction cycles?
- [ ] Should the TUI `/permisos` panel support inline YAML editing or only toggle presets?
