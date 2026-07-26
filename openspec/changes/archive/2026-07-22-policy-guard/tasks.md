# Tasks: Policy Guard

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~780 (4 new files + 5 modified) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Foundation (types + store + guard) → PR 2: Core logic + integration + TUI panel → PR 3: Tests + cleanup |
| Delivery strategy | auto-forecast (auto) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Types + PolicyStore + blocklist | PR 1 | `go test ./internal/core/ -run TestPolicyStore` | N/A — pure types, no wiring | Revert `policy.go`, `policy_store.go`, `models.go` changes |
| 2 | PolicyGuard.Evaluate + dual injection + TUI | PR 2 | `go test ./internal/core/ -run TestPolicyGuard` | `gaia exec "hi" --policy-tier=read` | Revert kernel.go, spawner.go, exec.go, policy_panel.go |
| 3 | Tests + RED tests + deprecation markers | PR 3 | `go test ./internal/core/ -run TestPolicy` | Integration: shell blocklist test | Revert policy_test.go, guard.go deprecation |

## Phase 1: Foundation — Types, Store, Blocklist

- [x] 1.1 Create `internal/core/policy.go` — `PolicyTier`, `OverridePolicy`, `PolicyConfig`, `PolicyGuard` struct with `SessionApprovals`, `toolTiers` classification map, `hardlinePatterns` blocklist
- [x] 1.2 Create `internal/core/policy_store.go` — `PolicyStore` interface (`LoadGlobal`, `LoadProject`, `Save`, `Merge`); YAML file adapter with `~/.config/gaia/policy.yaml` + `.gaia/policy.yaml`
- [x] 1.3 Add `PolicyConfig` struct + `Policy PolicyConfig` field to `domain.Config` in `internal/core/domain/models.go` — deferred to PR 2 (outside scope of PR 1 per precision rules)
- [x] 1.4 Code hardline blocklist in `policy.go`: `rm -rf /`, fork bomb patterns, `dd` to block devices, `mkfs`, `curl|sh` / `wget|sh` pipes — implemented as compiled-in regex + literal patterns

## Phase 2: Core Logic — Evaluation, Merge, Escalation

- [x] 2.1 Implement `PolicyGuard.Evaluate(toolName, args)` in `policy.go` — hardline → denies, user deny rules check, tier check, per-tool override lookup
- [x] 2.2 Implement `PolicyStore.Merge(global, project)` with precedence: hardline > global deny > project deny > tier defaults > tool overrides + PathRules
- [x] 2.3 Implement smart escalation chain in `policy.go`: skip (skippable tools) → alternative (registered fallback) → block + notify with context
- [x] 2.4 Write `Merge` + `LoadGlobal`/`LoadProject` YAML round-trip logic in `policy_store.go`
- [x] 2.5 Implement path-based rules (`PathRule`) — glob match on tool args that reference filesystem paths

## Phase 3: Integration — Dual Injection + Flags + TUI

- [x] 3.1 Add `Policy *core.PolicyGuard` field to `SpawnerConfig` in `internal/agent/spawner.go`; evaluate before `filtered.Execute` at line 198
- [x] 3.2 Replace `b.guard.ShouldConfirm` with `b.policy.Evaluate` in `kernel.go:handleToolCalls` (line 579); wire `PolicyGuard` into `NewBrain` or via setter
- [x] 3.3 Add `--policy-tier` flag to `cmd/gaia/exec.go`; map `--yes` → `TierFull`, `--dry-run` → `TierRead`
- [x] 3.4 Wire `PolicyGuard` through `exec.go` → `NewBrain` construction, passing `PolicyConfig` from config
- [x] 3.5 Create `internal/adapters/tui/policy_panel.go` — Bubbletea component for `/permisos` command showing tier, overrides, path rules; support toggle editing

## Phase 4: Testing — Unit + Integration + RED

- [x] 4.1 Write table-driven test: tier evaluation (`read` blocks mutation, `full` permits, `sandbox` scopes to project)
- [x] 4.2 Write table-driven test: hardline blocklist overrides `full` tier; blocklist cannot be overridden by user rules
- [x] 4.3 Write table-driven test: user deny rules (glob match, project-level deny, global deny overrides project allow)
- [x] 4.4 Write table-driven test: `PolicyStore.Merge` precedence (hardline > global > project > tier > overrides)
- [x] 4.5 Write RED test: hardline blocks `rm -rf /`, fork bombs, `curl | sh` even if shell allowlist permits `rm`/`curl` (integration with shell module)
- [x] 4.6 Write unit test: smart escalation chain (skip → alternative → escalate)
- [x] 4.7 Verify `Spawner.RunLoop` gating: wire `PolicyGuard` with mock, verify sub-agent tool calls are blocked/allowed per tier

## Phase 5: Cleanup — Deprecation + Docs

- [x] 5.1 Add `// Deprecated: replaced by PolicyGuard` comment to `ConfirmGuard` struct in `guard.go`
- [x] 5.2 Run `go vet ./...` and `golangci-lint run ./...` — fix any lint issues from new code
- [x] 5.3 Verify `--yes` / `--dry-run` continue working via tier mapping with existing test suite
