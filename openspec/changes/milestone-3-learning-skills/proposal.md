# Proposal: Milestone 3 — Learning & Skills

## Intent

Milestone 2 delivered the subagent infrastructure with 5 SDD subagents, per-subagent memory namespaces, and the learning loop. Milestone 3 completes the subagent roster (Designer, Planner, Archiver for SDD; Reviewer, Debugger, Researcher, Learner as non-SDD), introduces the Skills Hub for runtime skill management, a first-run wizard for language + skill onboarding, and headless execution mode for scripting and CI.

## Scope

### In Scope
- 3 new SDD subagents: Designer, Planner, Archiver (extending the SDD pipeline to 8 subagents)
- 4 new non-SDD subagents: Reviewer, Debugger, Researcher, Learner
- Skills Hub: search, install, activate, deactivate, remove skills at runtime
- First-run wizard: detect project type, ask language preference, recommend skills, install
- Headless mode: `gaia exec "task" --json --quiet --dry-run` for CI/scripting
- Output modes: `--json` structured output, `--verbose`, `--quiet`

### Out of Scope
- Persona system and evolution — Milestone 4
- Desktop UI (Wails) — Milestone 5
- Decentralized skill marketplace — Milestone 5+
- Multi-user skill sharing

## Capabilities

### New Capabilities
- `remaining-subagents`: Designer (architecture decisions + design docs), Planner (task breakdown + sequencing), Archiver (delta spec sync + change archival)
- `non-sdd-subagents`: Reviewer (code review + quality gates), Debugger (root cause analysis + fix suggestions), Researcher (web/docs search + knowledge synthesis), Learner (cross-subagent pattern extraction + skill creation)
- `skills-hub`: Runtime skill management — `gaia skills search/install/list/activate/deactivate/remove`. Skills stored in `skills/` with SKILL.md frontmatter. Activation injects skill content into subagent prompts.
- `gaia-wizard`: First-run onboarding — detect project language/framework, ask user language preference (EN/ES/PT), recommend relevant skills from registry, install selected skills.
- `headless-mode`: `gaia exec "task description"` — non-interactive execution with structured output. Flags: `--json` (JSON output), `--quiet` (errors only), `--verbose` (full trace), `--dry-run` (plan only, no side effects).
- `output-modes`: `--json` returns `{status, result, artifacts, risks}` envelope. `--quiet` suppresses all non-error output. `--verbose` includes token counts, timing, and subagent traces.

### Modified Capabilities
- `sdd-subagents`: Extended from 5 to 8 SDD subagents (add Designer, Planner, Archiver). SDD pipeline gains Design and Plan phases before Implementation.
- `subagent-base`: Registry extended to hold both SDD and non-SDD subagents. Spawner gains non-SDD subagent dispatch. Learning loop extended to cover 12 total subagents.

## Approach

Four chained PRs (stacked-to-main), each under 400 lines:

1. **sdd-extension (~380 LOC)**: Designer, Planner, Archiver subagents in `internal/agent/sdd/`. Each follows the existing pattern (struct + constructor + Execute + prompt). Designer gets read-only tools; Planner gets read + Engram; Archiver gets read + write (delta sync). Wire into `main.go` registry. Update SDD pipeline in `kernel.go` to include Design and Plan phases.

2. **non-sdd-subagents (~380 LOC)**: Reviewer, Debugger, Researcher, Learner in new `internal/agent/ops/` package. These are NOT part of the SDD pipeline — they are invoked on-demand by the Brain when it detects review, debug, research, or learning intents. Each has domain-specific tool filters and learning domains.

3. **skills-hub (~380 LOC)**: `internal/skills/` package — `Hub` struct with `Search`, `Install`, `List`, `Activate`, `Deactivate`, `Remove` methods. Skills stored in `skills/{name}/SKILL.md`. CLI: `gaia skills <subcommand>`. Activation loads skill content into a registry that subagent prompts can reference. Wizard flow in `internal/adapters/tui/wizard.go` extended with language selection and skill recommendation.

4. **headless-mode (~350 LOC)**: `gaia exec` subcommand in `cmd/gaia/exec.go`. Parses flags (`--json`, `--quiet`, `--verbose`, `--dry-run`). Runs Brain.ProcessMessage in non-interactive mode. `--json` wraps output in structured envelope. `--dry-run` runs SDD pipeline up to Plan phase without executing Implementer. Output formatter in `internal/adapters/output/`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/sdd/` | Modified | Add designer.go, planner.go, archiver.go (3 new SDD subagents) |
| `internal/agent/ops/` | New | Reviewer, Debugger, Researcher, Learner (4 non-SDD subagents) |
| `internal/skills/` | New | Skills Hub: search, install, activate, deactivate, remove |
| `internal/adapters/output/` | New | Output formatters: JSON, quiet, verbose |
| `cmd/gaia/exec.go` | New | `gaia exec` headless subcommand |
| `cmd/gaia/main.go` | Modified | Register new subagents, wire skills hub, detect exec mode |
| `internal/core/kernel.go` | Modified | Extended SDD pipeline (8 phases), non-SDD subagent dispatch |
| `internal/adapters/tui/wizard.go` | Modified | Extended wizard: language selection + skill recommendation |
| `skills/` | Modified | Skills directory becomes runtime-managed (install/activate) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| SDD pipeline too long (8 phases) for simple changes | High | Keep trigger heuristic selective; `/direct` bypass; Planner can collapse phases |
| Skills Hub conflicts with bundled skills in `skills/` | Medium | Namespace separation: bundled skills are read-only; installed skills go to `~/.gaia/skills/` |
| Headless mode bypasses confirmation guard | Medium | `--dry-run` implies no side effects; without `--dry-run`, guard still active unless `--yes` flag |
| Non-SDD subagent intent detection (when to invoke Reviewer vs Debugger) | Medium | Keyword heuristic initially; LLM-classified in Milestone 4 |
| Wizard skill recommendations are irrelevant | Low | Use project language detection + static mapping; user can skip |

## Rollback Plan

Each PR is independently revertable. New subagents are additive — they register into the existing Registry without changing existing subagent behavior. Skills Hub is a new package with no existing dependencies. Headless mode is a new CLI subcommand that does not affect the interactive TUI flow.

## Dependencies

- Milestone 2 complete (subagent infrastructure, 5 SDD subagents, learning loop, memory namespaces)
- Existing `skills/` directory with SKILL.md format
- No external service dependencies (Skills Hub is local filesystem)

## Success Criteria

- [ ] 3 new SDD subagents (Designer, Planner, Archiver) follow existing pattern and pass unit tests
- [ ] 4 non-SDD subagents (Reviewer, Debugger, Researcher, Learner) spawn and return structured results
- [ ] Skills Hub supports search, install, list, activate, deactivate, remove via CLI
- [ ] Wizard detects project type, asks language, recommends and installs skills
- [ ] `gaia exec "task" --json` returns structured JSON envelope
- [ ] `gaia exec "task" --dry-run` runs pipeline without side effects
- [ ] All 12 subagents registered and discoverable via `brain.AvailableSubagents()`
- [ ] `go test ./...` passes, `go build ./cmd/gaia` succeeds
