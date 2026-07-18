# Gentle AI Upstream Tracker Specification

## Purpose

Automated monitoring of the Gentle AI upstream repository (github.com/Gentleman-Programming/gentle-ai) to track which features GAIA has ported, identify gaps, and produce actionable port-status reports.

## Requirements

### Requirement: Release Monitoring

The system MUST provide a `GitHubReleaseMonitor` that fetches releases from the Gentle AI repository using the GitHub REST API. The monitor MUST support optional `GITHUB_TOKEN` authentication and MUST use ETag caching (`If-None-Match`) to minimize API calls. The monitor MUST cache the last-checked release in a SQLite `tracker_state` table. Methods: `CheckLatest(ctx) (*Release, bool)` returns the latest release and whether it is new; `ListReleases(ctx, since) []Release` returns releases since a given version or date.

#### Scenario: Fetch latest release

- GIVEN the monitor has no cached state
- WHEN CheckLatest is called
- THEN the GitHub API is queried
- AND the latest release is returned with hasNew: true
- AND the release tag is cached in tracker_state

#### Scenario: ETag cache hit

- GIVEN a previous CheckLatest stored an ETag
- WHEN CheckLatest is called again and the upstream has not changed
- THEN the GitHub API returns 304 Not Modified
- AND no new release data is fetched
- AND hasNew returns false

#### Scenario: Authenticated request

- GIVEN the GITHUB_TOKEN environment variable is set
- WHEN CheckLatest is called
- THEN the request includes an Authorization header with the token

#### Scenario: Unauthenticated rate limit

- GIVEN no GITHUB_TOKEN is set
- WHEN CheckLatest is called and the rate limit is exceeded
- THEN the monitor returns a rate-limit error with the reset time

### Requirement: Changelog Analysis

The system MUST provide a `ChangelogAnalyzer` that parses a release body into structured changes. Each change MUST be classified as one of: feature, fix, protocol-change, refactor, docs, other. Classification MUST use rule-based parsing of Conventional Commits format (e.g., "feat:", "fix:", "BREAKING CHANGE:"). The system MAY optionally use an LLM call for complex classification when configured. Methods: `AnalyzeRelease(release) []Change` and `DiffReleases(from, to) []Change`.

#### Scenario: Classify conventional commit entries

- GIVEN a release body containing "feat: add streaming support" and "fix: resolve nil pointer"
- WHEN AnalyzeRelease is called
- THEN two Change entries are returned: one classified as "feature" and one as "fix"

#### Scenario: Unparseable entry

- GIVEN a release body with freeform text not matching Conventional Commits
- WHEN AnalyzeRelease is called
- THEN the entry is classified as "other"

#### Scenario: Diff between releases

- GIVEN release v1.0 and v1.2 with different changelogs
- WHEN DiffReleases(v1.0, v1.2) is called
- THEN only changes introduced in v1.1 and v1.2 are returned

### Requirement: Port Manifest

The system MUST maintain a YAML file `port-manifest.yaml` in the project root. Each entry MUST contain: upstream_feature (string), status (ported | partial | not-ported | not-applicable), gaia_location (string, file or package path), upstream_version (string), and notes (string). The system MUST provide Load() and Save() methods with atomic writes to prevent corruption. The default manifest SHOULD be seeded from the project's SPEC.md Section 9.1 mapping.

#### Scenario: Load manifest

- GIVEN a valid port-manifest.yaml exists
- WHEN Load() is called
- THEN all entries are parsed and returned

#### Scenario: Atomic save

- GIVEN the manifest has been modified
- WHEN Save() is called
- THEN the file is written atomically (write to temp, rename)
- AND concurrent readers see either the old or new version, never a partial write

#### Scenario: Missing manifest

- GIVEN no port-manifest.yaml exists
- WHEN Load() is called
- THEN an empty manifest is returned with no error

### Requirement: Tracker CLI

The system MUST provide CLI subcommands under `gaia tracker`:
- `check`: fetch latest release, compare with manifest, print delta of new/changed upstream features
- `report`: print full port status as a markdown table grouped by status
- `port <feature>`: mark a feature as ported in the manifest

Reports MUST be printed to stdout as markdown tables. The system SHOULD support future JSON output for CI consumption.

#### Scenario: Check for new releases

- GIVEN the manifest shows 10 features and upstream has 12
- WHEN `gaia tracker check` is run
- THEN the output shows 2 new unported features

#### Scenario: Full status report

- GIVEN a manifest with 5 ported, 3 partial, 2 not-ported features
- WHEN `gaia tracker report` is run
- THEN a markdown table is printed with all 10 features grouped by status

#### Scenario: Mark feature as ported

- GIVEN feature "streaming-tools" has status "partial"
- WHEN `gaia tracker port streaming-tools` is run
- THEN the manifest is updated with status "ported"
- AND the current timestamp is recorded
