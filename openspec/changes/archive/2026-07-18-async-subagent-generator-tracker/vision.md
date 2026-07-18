# Vision: Async + Dynamic + Tracker — The GAIA Evolution

## The Three Pillars

These three changes are independent but mutually reinforcing. Together they evolve GAIA from a **synchronous, hardcoded, isolated orchestrator** into an **asynchronous, extensible, upstream-aware agent platform**.

| Pillar | Transforms | Into |
|--------|-----------|------|
| **Async Delegation** | Blocking Spawner, orchestrator-as-gatekeeper | Background tasks, direct subagent chat, responsive TUI |
| **Dynamic Generator** | 12 hardcoded subagents, recompile-to-extend | Runtime subagent creation, user-defined specialists |
| **Upstream Tracker** | Manual drift tracking against Gentle AI | Automated release monitoring, port manifest, coverage reports |

## Why Together

1. **Async unlocks Dynamic's UX**: a user-created subagent running a 5-minute research task must not freeze the TUI. Async is the runtime foundation Dynamic needs to be usable.
2. **Dynamic unlocks Tracker's actionability**: when the tracker identifies an unported upstream feature, the user can spin up a `gentle-ai-porter` dynamic subagent specialized for that port — without waiting for a GAIA release.
3. **Tracker informs Async's SDD pipeline**: the SDD phases (propose → spec → design → tasks → apply → verify) are the primary async workload. Knowing which upstream SDD features GAIA lacks (via tracker) prioritizes which async behaviors to build first.

## Shared Infrastructure

- **TaskManager** (from Async) is reused by Dynamic to track dynamic-subagent executions and by Tracker to run `check` as a cancellable background task.
- **Registry** (existing) gains `RegisterDynamic` (from Dynamic) and becomes the single source of truth for both compiled and runtime subagents — Async's `@name` routing works for both.
- **SQLite** (existing adapter) gains three additive tables: `tasks` (Async), `subagents` (Dynamic), `tracker_releases` (Tracker). One migration, one connection pool.
- **Engram namespaces** extend naturally: `gaia/subagent/<name>/` (Dynamic), `gaia/tracker/` (Tracker), `gaia/tasks/<id>/` (Async).

## Implementation Order

Recommended sequencing based on dependency graph:

1. **Async Delegation** (foundation — unlocks responsive UX for everything else)
2. **Dynamic Generator** (leverages Async; extends the platform)
3. **Upstream Tracker** (standalone; can ship in parallel with Dynamic)

## Combined Risk

The compound risk is **scope creep across three large changes**. Mitigation: each proposal has explicit non-goals and a feature flag. Ship Async first, validate, then Dynamic, then Tracker. No cross-feature coupling in v1.

## Combined Success

GAIA becomes a system where:
- Users chat with specific subagents directly while other tasks run in the background
- New subagent specialists are created in minutes, not release cycles
- The project's parity with Gentle AI is visible, measurable, and actionable
