# Proposal: Gentle AI Upstream Tracker

## Intent

GAIA is a from-scratch Go port of Gentle AI (github.com/Gentleman-Programming/gentle-ai, the TypeScript reference). As Gentle AI evolves, GAIA needs to know which upstream features are already ported, which are missing, and which have diverged. Today this tracking is manual and drifts. An automated tracker monitors upstream releases, classifies changes, and produces port-status reports.

## Scope

### In Scope
- `GitHubReleaseMonitor`: polls the Gentle AI repo for new releases/tags via GitHub API (or `gh` CLI)
- `ChangelogAnalyzer`: classifies each release's changes into `feature | fix | protocol-change | breaking`
- `PortManifest`: YAML file (`tracker/manifest.yaml`) listing each upstream feature with status `ported | partial | unported | diverged` and last-synced release
- `ReportGenerator`: produces markdown comparison reports (per-release delta + overall coverage %)
- CLI: `gaia tracker {check,report,port}`
  - `check` — fetch latest releases, update manifest statuses where detectable
  - `report` — render current port-status report
  - `port <feature>` — mark a feature as ported (manual confirmation)
- Storage: manifest in repo; release cache in SQLite (`tracker_releases` table)

### Out of Scope
- Automatic code porting (the tracker reports; humans port)
- Monitoring repos other than Gentle AI (v1 is single-repo)
- CI integration / scheduled runs (v1 is on-demand CLI; cron integration is a follow-up)
- Semantic diff of source code (classification is changelog-based, not AST-based)

## Capabilities

### New Capabilities
- `upstream-release-monitor`: GitHub release polling, caching, changelog parsing
- `port-manifest`: YAML-backed feature tracking with status lifecycle
- `tracker-cli`: `gaia tracker` subcommands and report rendering

### Modified Capabilities
- None — this is a net-new subsystem.

## Approach

`GitHubReleaseMonitor` uses `net/http` + GitHub REST API (`/repos/{owner}/{repo}/releases`) with `If-None-Match` ETag caching. Releases are stored in `tracker_releases` (id, tag, published_at, body, classified_as). `ChangelogAnalyzer` is a rule-based classifier over the release body (regex for "feat:", "fix:", "BREAKING CHANGE:" following Conventional Commits). `PortManifest` is a YAML file loaded via `gopkg.in/yaml.v3`; mutations write back atomically. `ReportGenerator` walks the manifest and produces a markdown table grouped by status. CLI wires into `cmd/gaia/tracker.go` using the existing cobra command tree.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tracker/` | New | Monitor, Analyzer, Manifest, Report packages |
| `internal/adapters/db/` | Modified | `tracker_releases` table |
| `cmd/gaia/tracker.go` | New | CLI subcommands |
| `tracker/manifest.yaml` | New | Port manifest file (repo root) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| GitHub API rate limit unauthenticated | Med | Support `GITHUB_TOKEN` env; cache aggressively with ETags |
| Changelog format drift breaks classifier | Med | Classifier returns `unknown` for unparseable entries; manual triage path |
| Manifest YAML merge conflicts in teams | Low | Atomic writes; document manual edit workflow |

## Rollback Plan

Entire feature is additive under `internal/tracker/` and `cmd/gaia/tracker.go`. Remove directory + command registration to revert. Manifest file is non-blocking if absent.

## Dependencies

- GitHub REST API (no auth required for public repos, but token recommended)
- `gopkg.in/yaml.v3` (already in `go.mod` for other configs)

## Success Criteria

- [ ] `gaia tracker check` fetches and classifies the last 10 Gentle AI releases
- [ ] `gaia tracker report` renders a markdown table with per-feature port status
- [ ] Manifest survives concurrent `check` + `port` invocations (file lock or atomic write)
- [ ] No network call when ETag indicates no change (cache hit)
