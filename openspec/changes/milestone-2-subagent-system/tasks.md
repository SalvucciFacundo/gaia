# Tasks: Milestone 2 — Subagent System

## Review Workload Forecast

| Metric | Value |
|---|---|
| Estimated changed lines (Phase 1) | ~380 |
| Estimated changed lines (Phase 2) | ~380 |
| Estimated changed lines (Phase 3) | ~380 |
| Decision needed before apply | No |
| Chained PRs recommended | Yes |
| Chain strategy | stacked-to-main |
| 400-line budget risk | Medium |

---

## Phase 1: Subagent Base (~380 lines)

- [x] 1.1 Create `internal/agent/` package with `Subagent` interface: `Name() string`, `Description() string`, `Execute(ctx, task SubagentTask) *SubagentResult`
- [x] 1.2 Create `SubagentTask` and `SubagentResult` types in `internal/core/domain/models.go`
- [x] 1.3 SubagentTask: ID, Description, KGContext, Skills, AllowedTools, Mode
- [x] 1.4 SubagentResult: Status, Summary, Artifacts, NextRecommended, Risks, SkillResolution
- [x] 1.5 Create `Spawner` in `internal/agent/spawner.go` — runs subagent in isolated context with filtered tools, returns SubagentResult
- [x] 1.6 Create `Registry` in `internal/agent/registry.go` — maps subagent name to factory function
- [x] 1.7 Add `SubagentPort` to ports, wire delegation step to Brain: `SubagentPort` field + `Delegate` method
- [x] 1.8 Register subagents in `cmd/gaia/main.go` (start with Explorer stub in `internal/agent/sdd/explorer.go`)
- [x] 1.9 Tests: subagent contract, spawner isolation with mock provider, registry lookup

## Phase 2: SDD Subagents (~380 lines)

- [x] 2.1 Create Explorer subagent in `internal/agent/sdd/explorer.go` — read-only tools, structured Observations
- [x] 2.2 Create Proposer subagent in `internal/agent/sdd/proposer.go` — read + Engram, proposal generation
- [x] 2.3 Create Specifier subagent in `internal/agent/sdd/specifier.go` — read + Engram, delta specs
- [x] 2.4 Create Implementer subagent in `internal/agent/sdd/implementer.go` — write + shell, code generation
- [x] 2.5 Create Verifier subagent in `internal/agent/sdd/verifier.go` — shell + read, test execution
- [x] 2.6 Wire system prompts per subagent (domain-specific instructions)
- [x] 2.7 Add tool filtering per subagent (Explorer: read-only, Implementer: write+shell, Verifier: shell+read)
- [x] 2.8 Integration tests with mock LLM for pipeline flow

## Phase 3: Learning & Memory (~380 lines)

- [x] 3.1 Create Engram namespace wrapper in `internal/agent/memory/namespace.go` — `gaia/{subagent}/{project}` prefix
- [x] 3.2 Implement scoped save/search/get per namespace
- [x] 3.3 Create learning loop in `internal/agent/learn/loop.go` — counter-based nudge (N=5)
- [x] 3.4 Implement session summary generation per subagent
- [x] 3.5 Add SDD trigger heuristic to Brain (keyword + scope detection)
- [x] 3.6 Wire `/direct` override and `/sdd` force commands
- [x] 3.7 Complete message redaction in tool output pipeline
- [x] 3.8 Tests: namespace format, learning counter, trigger heuristics
