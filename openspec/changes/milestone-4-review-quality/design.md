# Design: Milestone 4 — Review & Quality

## Technical Approach

Implement a formal review system in three layers: (1) a deterministic review engine with risk taxonomy, lens selection, and content-bound receipts; (2) gate validators and CLI for review lifecycle management; (3) Judgment Day adversarial review for high-risk changes. The engine is a pure Go package (`internal/review/`) with no LLM dependency for receipt/state-machine logic — LLM is only used inside individual lens implementations for qualitative analysis. The existing `reviewer.go` stub is upgraded to delegate to the engine while keeping the subagent interface contract.

## Architecture Decisions

### Decision: Review engine as pure Go package (no LLM in core)

**Choice**: `internal/review/` contains deterministic logic (risk classification, lens selection, receipt generation, state machine). LLM calls happen only inside lens implementations.
**Alternatives considered**: Fully LLM-driven review (prompt-only, like the current stub); hybrid where engine calls LLM directly.
**Rationale**: Deterministic core is testable without mocks. Lens implementations can be swapped (LLM-based today, rule-based tomorrow for specific checks). Receipt SHA256 is computed over file contents, not LLM output — it must be reproducible.

### Decision: Receipt storage in Engram (not filesystem or database)

**Choice**: Receipts stored as Engram observations under `gaia/review/{change-name}/{transaction-id}`.
**Alternatives considered**: SQLite table; JSON files in `.gaia/reviews/`; in-memory only.
**Rationale**: Engram already provides per-project persistence, cross-session recall, and conflict detection. Receipts are small (~500 bytes JSON). Using Engram avoids a new storage mechanism and integrates with the existing mutation journal pattern from SPEC.md 9.4.4.

### Decision: Gate hooks as optional shell scripts (not native git integration)

**Choice**: `gaia review install-hooks` writes `.git/hooks/pre-commit` and `.git/hooks/pre-push` shell scripts that call `gaia review validate`. Gates also callable directly via CLI.
**Alternatives considered**: Native git hook via libgit2; pre-commit framework plugin; always-on file watcher.
**Rationale**: Shell scripts are universal (work on any OS with git). No new dependency. Users who don't want hooks can still use `gaia review validate` manually or in CI. The gate logic itself is pure Go — the hook is just a thin shell wrapper.

### Decision: Judgment Day judges as isolated Spawner invocations (not separate processes)

**Choice**: judge-a and judge-b are two independent `Spawner.Spawn("reviewer", ...)` calls with different system prompts and isolated message histories. Comparison runs in a third LLM call.
**Alternatives considered**: Separate OS processes; single LLM call with "imagine two judges" prompt; external service.
**Rationale**: Isolation at the Spawner level ensures judges cannot see each other's findings (blind review). Different system prompts ("You are judge-a, focus on security and data flow" vs "You are judge-b, focus on error handling and edge cases") produce genuinely independent analysis. Same infrastructure as existing subagents — no new mechanism.

### Decision: AGENTS.md parser follows SKILL.md pattern (YAML frontmatter + markdown body)

**Choice**: Parse AGENTS.md with YAML frontmatter delimited by `---` and markdown body. Frontmatter contains structured rules (forbidden patterns, naming conventions, required tests). Body contains prose guidelines.
**Alternatives considered**: Pure YAML; pure markdown; TOML; custom DSL.
**Rationale**: SKILL.md already established this pattern in the codebase (`internal/skills/hub.go`). Consistency reduces cognitive load. YAML frontmatter is machine-parseable; markdown body is human-readable. Same `gopkg.in/yaml.v3` dependency already in go.mod.

### Decision: Snapshot hash uses normalized file content (LF line endings)

**Choice**: Before SHA256 hashing, normalize all file content to LF line endings. Hash is computed over `path + "\n" + normalized_content` for each file, sorted by path, then hashed as a single stream.
**Alternatives considered**: Raw file bytes; git tree hash; per-file hashes stored separately.
**Rationale**: Windows uses CRLF; Linux uses LF. Without normalization, the same code produces different hashes on different OSes. Sorting by path ensures deterministic ordering. Single stream hash is simpler to compare than a map of per-file hashes.

## Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        GAIA Review System                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  User: "review this PR"  OR  git hook triggers  OR  gaia review start  │
│           │                        │                      │             │
│           └────────────────────────┼──────────────────────┘             │
│                                    │                                    │
│                                    ▼                                    │
│                    ┌───────────────────────────┐                        │
│                    │    Review Engine           │                        │
│                    │  ┌─────────────────────┐  │                        │
│                    │  │  1. Snapshot Files   │  │                        │
│                    │  │  2. Classify Risk    │  │                        │
│                    │  │  3. Select Lenses    │  │                        │
│                    │  │  4. Run Lenses       │  │                        │
│                    │  │  5. Classify Findings│  │                        │
│                    │  │  6. Generate Receipt │  │                        │
│                    │  └─────────────────────┘  │                        │
│                    └───────────┬───────────────┘                        │
│                                │                                        │
│              ┌─────────────────┼──────────────────┐                     │
│              │                 │                   │                     │
│              ▼                 ▼                   ▼                     │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐            │
│    │  Risk Level  │  │   Lenses     │  │    Receipt        │            │
│    │              │  │              │  │                    │            │
│    │ Low → none   │  │ Risk         │  │ lineage_id         │            │
│    │ Med → 1 lens │  │ Resilience   │  │ snapshot_hash      │            │
│    │ High → all 4 │  │ Readability  │  │ selected_lenses    │            │
│    │              │  │ Reliability  │  │ risk_level         │            │
│    └──────────────┘  └──────────────┘  │ correction_budget  │            │
│                                         │ state              │            │
│                                         └────────┬───────────┘            │
│                                                  │                        │
│                                                  ▼                        │
│                                    ┌──────────────────────────┐          │
│                                    │   Gate Validators         │          │
│                                    │                           │          │
│                                    │  pre-commit → validate    │          │
│                                    │  pre-push   → validate    │          │
│                                    │  pre-pr     → validate    │          │
│                                    └──────────────────────────┘          │
│                                                                          │
│    HIGH-RISK CHANGES (>400 lines, auth, security, payments):             │
│                                                                          │
│                    ┌───────────────────────────┐                         │
│                    │    Judgment Day            │                         │
│                    │                           │                         │
│                    │  judge-a ──┐              │                         │
│                    │            ├─ compare ──► fix ──► re-judge (max 2) │
│                    │  judge-b ──┘              │                         │
│                    └───────────────────────────┘                         │
│                                                                          │
│    ┌──────────────────────────────────────────────────────────┐          │
│    │  AGENTS.md Parser                                        │          │
│    │  Reads team standards → injects into review prompts      │          │
│    └──────────────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Review State Machine (Formal)

```
unreviewed
    │
    ▼ (review started)
reviewing
    │
    ├──► judges_confirmed  [Judgment Day only]
    │        │
    │        ▼
    │   findings_frozen
    │
    ▼ (lenses complete)
findings_frozen
    │
    ▼ (findings classified)
evidence_classified
    │
    ├──► fix_required ──► fixing ──► fix_validating ──► evidence_classified
    │     [max 1 round normal, 2 rounds Judgment Day]
    │
    ▼ (no fixes needed or fixes validated)
ready_final_verification
    │
    ▼ (tests + build pass)
final_verifying
    │
    ▼ (verification passes)
approved  ──► [receipt issued]
    │
    ├──► escalated  [unresolvable — human intervention]
    └──► invalidated [content changed since receipt — new review needed]
```

State transitions enforced by `Transition(from, to State) error` — invalid transitions return error.

## Risk Taxonomy & Lens Selection

| Risk Code | Signal | When It Fires |
|---|---|---|
| `configuration_change` | ⚠ | Changes to config files, env vars, feature flags |
| `executable_change` | ⚠ | Changes to executable binary outputs |
| `executable_mode` | permissions | File permission mode changes (e.g., +x) |
| `hot_path` | 🔴 | Changes to auth, authorization, payments, security-critical paths |
| `large_change` | 🔴 | More than 400 changed lines |
| `non_executable_only` | ✅ | Only docs, comments, formatting, typo fixes |
| `service_token` | 🔴 | New/modified service tokens, API keys in code |
| `shell_source` | 🔴 | New/modified shell scripts, Makefile targets, subprocess invocations |

**Risk level determination:**
- Only `non_executable_only` → **Low** (no lens needed, auto-approve)
- Any non-high reason → **Medium** (select one dominant lens based on file types)
- `hot_path`, `large_change`, `service_token`, or `shell_source` → **High** (all 4 lenses)

**Dominant lens selection for Medium risk:**
- Config files → `review-risk`
- Test files → `review-reliability`
- Documentation only → `review-readability`
- Service/infra code → `review-resilience`
- Default → `review-risk`

## Review Receipt Structure

```go
type Receipt struct {
    Schema              string   `json:"schema"`               // "gentle-ai.review-receipt/v2"
    LineageID           string   `json:"lineage_id"`           // SHA256 of review transaction chain
    SnapshotHash        string   `json:"snapshot_hash"`        // "sha256:{hash of all reviewed files}"
    SelectedLenses      []string `json:"selected_lenses"`      // ["review-risk", "review-readability"]
    RiskLevel           string   `json:"risk_level"`           // "low", "medium", "high"
    RiskReasons         []string `json:"risk_reasons"`         // ["hot_path", "large_change"]
    CorrectionBudget    int      `json:"correction_budget"`    // max correction tokens (85 default)
    CorrectionUsed      int      `json:"correction_used"`      // tokens used so far
    State               string   `json:"state"`                // state machine state
    FinalVerificationHash string `json:"final_verification_hash"` // "sha256:{verification evidence}"
    Findings            []Finding `json:"findings"`            // classified findings
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
}

type Finding struct {
    Lens       string `json:"lens"`        // "review-risk"
    Severity   string `json:"severity"`    // "BLOCKER", "WARNING", "SUGGESTION"
    File       string `json:"file"`        // "internal/auth/login.go"
    Line       int    `json:"line"`        // 42
    Message    string `json:"message"`     // human-readable finding
    Suggestion string `json:"suggestion"`  // concrete fix suggestion
}
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/review/engine.go` | Create | Core Engine: Start(), ClassifyRisk(), SelectLenses(), RunLenses(), GenerateReceipt() |
| `internal/review/risk.go` | Create | Risk taxonomy: 8 risk codes, classification logic, risk level determination |
| `internal/review/lens.go` | Create | Lens interface + 4 implementations: LensRisk, LensResilience, LensReadability, LensReliability |
| `internal/review/receipt.go` | Create | Receipt struct, SHA256 snapshot computation, serialization |
| `internal/review/state.go` | Create | State machine: State enum, Transition(), valid transition table |
| `internal/review/snapshot.go` | Create | File snapshot: collect files, normalize line endings, compute hash |
| `internal/review/engine_test.go` | Create | Unit tests: risk classification, lens selection, receipt generation, state transitions |
| `internal/review/agentsmd/parser.go` | Create | AGENTS.md parser: YAML frontmatter + markdown body → Standards struct |
| `internal/review/agentsmd/parser_test.go` | Create | Unit tests: parse valid AGENTS.md, handle missing frontmatter, extract rules |
| `internal/review/gates/gates.go` | Create | Gate validators: Validate(receipt, currentFiles) → bool; PreCommit, PrePush, PrePR |
| `internal/review/gates/hooks.go` | Create | Git hook installer: WriteHooks(projectRoot) → creates .git/hooks/pre-commit, pre-push |
| `internal/review/gates/gates_test.go` | Create | Unit tests: gate validation (match/stale), hook generation |
| `internal/review/judgment/judgment.go` | Create | JudgmentDay orchestrator: Run(ctx, snapshot) → merged findings |
| `internal/review/judgment/compare.go` | Create | Comparison logic: merge judge-a + judge-b findings, resolve conflicts |
| `internal/review/judgment/fix.go` | Create | Fix-agent: apply surgical corrections within correction_budget |
| `internal/review/judgment/judgment_test.go` | Create | Integration tests: blind review, comparison, fix round, 2-round limit |
| `internal/agent/ops/reviewer.go` | Modify | Upgrade: use review.Engine instead of prompt-only; produce programmatic receipt |
| `cmd/gaia/review.go` | Create | `gaia review` CLI: start, status, validate, list, install-hooks |
| `cmd/gaia/main.go` | Modify | Add "review" case to CLI dispatch switch |
| `internal/agent/sdd/archiver.go` | Modify | Add receipt validation gate before archive proceeds |
| `internal/core/domain/models.go` | Modify | Add ReviewReceipt, ReviewState, ReviewFinding types |

## Interfaces / Contracts

```go
// internal/review/engine.go

type Engine struct {
    projectRoot string
    standards   *agentsmd.Standards  // team standards from AGENTS.md (nil if not found)
}

func NewEngine(projectRoot string) *Engine
func (e *Engine) Start(files []string) (*Transaction, error)
func (e *Engine) ClassifyRisk(diff string) ([]string, string)  // returns (risk_reasons, risk_level)
func (e *Engine) SelectLenses(riskLevel string, files []string) []string
func (e *Engine) RunLenses(ctx context.Context, tx *Transaction, lenses []string) ([]Finding, error)
func (e *Engine) GenerateReceipt(tx *Transaction, findings []Finding) (*Receipt, error)

// Transaction tracks a single review lifecycle
type Transaction struct {
    ID           string
    ChangeName   string
    State        State
    SnapshotHash string
    Files        []string
    Receipt      *Receipt   // populated when state reaches "approved"
}

// internal/review/lens.go

type Lens interface {
    Name() string   // "review-risk", "review-resilience", etc.
    Analyze(ctx context.Context, files []FileSnapshot) ([]Finding, error)
}

type FileSnapshot struct {
    Path    string
    Content string  // normalized (LF)
    Hash    string  // SHA256 of content
}

// internal/review/state.go

type State string

const (
    StateUnreviewed           State = "unreviewed"
    StateReviewing            State = "reviewing"
    StateJudgesConfirmed      State = "judges_confirmed"
    StateFindingsFrozen       State = "findings_frozen"
    StateEvidenceClassified   State = "evidence_classified"
    StateFixRequired          State = "fix_required"
    StateFixing               State = "fixing"
    StateFixValidating        State = "fix_validating"
    StateReadyFinalVerification State = "ready_final_verification"
    StateFinalVerifying       State = "final_verifying"
    StateApproved             State = "approved"
    StateEscalated            State = "escalated"
    StateInvalidated          State = "invalidated"
)

func Transition(from, to State) error  // returns error if transition is invalid

// internal/review/gates/gates.go

type Gate struct {
    Name string  // "pre-commit", "pre-push", "pre-pr"
}

func (g *Gate) Validate(projectRoot string) (*GateResult, error)

type GateResult struct {
    Passed    bool
    Receipt   *Receipt  // nil if no receipt found
    Reason    string    // human-readable explanation
}

// internal/review/judgment/judgment.go

type JudgmentDay struct {
    spawner    *agent.Spawner
    maxRounds  int  // default 2
}

func NewJudgmentDay(spawner *agent.Spawner) *JudgmentDay
func (jd *JudgmentDay) Run(ctx context.Context, tx *review.Transaction) (*JudgmentResult, error)

type JudgmentResult struct {
    JudgeAFindings []Finding
    JudgeBFindings []Finding
    MergedFindings []Finding
    Rounds         int
    Approved       bool
}

// internal/review/agentsmd/parser.go

type Standards struct {
    Rules       []Rule      `yaml:"rules"`
    Forbidden   []string    `yaml:"forbidden"`
    Conventions []string    `yaml:"conventions"`
    Prose       string      // markdown body (guidelines, rationale)
}

type Rule struct {
    ID       string `yaml:"id"`
    Pattern  string `yaml:"pattern"`  // regex pattern to check
    Severity string `yaml:"severity"` // "error", "warning"
    Message  string `yaml:"message"`
}

func Parse(path string) (*Standards, error)
func (s *Standards) InjectIntoPrompt(prompt string) string

// cmd/gaia/review.go — CLI commands
// gaia review start [--files <glob>] [--lens <name>] [--judgment-day]
// gaia review status [--change <name>]
// gaia review validate [--gate <pre-commit|pre-push|pre-pr>]
// gaia review list [--state <state>]
// gaia review install-hooks
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Risk classification | Table-driven: each of 8 risk codes → expected risk level |
| Unit | Lens selection | Low → 0 lenses, Medium → 1 dominant, High → all 4 |
| Unit | Receipt SHA256 | Known content → known hash; CRLF normalization → same hash as LF |
| Unit | State machine | All valid transitions succeed; all invalid transitions return error |
| Unit | AGENTS.md parser | Valid frontmatter, missing frontmatter, empty body, malformed YAML |
| Unit | Gate validation | Receipt matches current content → pass; content changed → fail; no receipt → fail |
| Unit | Hook generation | Generated hook script calls `gaia review validate` with correct gate name |
| Integration | Engine flow | Mock LLM provider → full review cycle: start → classify → lenses → receipt |
| Integration | Judgment Day | Two mock judges with different findings → merge → fix → re-judge |
| Integration | CLI commands | Mock engine → verify start/status/validate/list output |
| Integration | Archiver gate | Receipt missing → archive blocked; receipt approved → archive proceeds |
| E2E | Git hook flow | Install hooks → make change → commit → gate validates receipt |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|----------|--------------|-----------------|-------------------|
| Snapshot integrity | Core — receipt depends on hash | Normalize LF, sort by path, SHA256 stream | Test: same files different OS → same hash; one byte change → different hash |
| State machine bypass | Could skip states to approve without review | Transition() enforces valid transitions only | Test: attempt unreviewed→approved → error; attempt reviewing→approved → error |
| Receipt forgery | Receipt JSON could be hand-edited | Receipt stored in Engram (append-only); hash validated against content | Test: modify receipt JSON → gate validation fails on hash mismatch |
| AGENTS.md injection | Malicious AGENTS.md could inject prompt instructions | Parser extracts only YAML fields + prose body; no eval/exec | Test: AGENTS.md with `{{system prompt}}` injection → treated as literal prose |
| Judgment Day cost | 3 LLM calls per judgment day | Only triggered for high-risk; max 2 rounds = 6 LLM calls worst case | Test: 3 rounds attempted → error after 2 |
| Gate bypass | User could commit with `--no-verify` | Document that hooks are optional; gate also enforced at pre-PR level | N/A (documented limitation) |

## Migration / Rollout

No migration required. All changes are additive or upgrade existing stubs:
- `internal/review/` is a new package with no existing dependencies
- `reviewer.go` is upgraded but maintains the same `Subagent` interface contract
- `gaia review` is a new CLI subcommand; existing `gaia skills` and `gaia exec` unaffected
- Git hooks are opt-in via `gaia review install-hooks`; existing hooks preserved (appended to)
- State machine and receipts are new concepts — no existing state to migrate

## Open Questions

- [ ] Should the correction_budget be configurable per-project, or fixed at 85 tokens?
- [ ] Should Judgment Day be automatically triggered for `hot_path` changes, or always manual?
- [ ] Should the pre-commit hook block or warn by default? (Proposal: block for high-risk, warn for medium)
