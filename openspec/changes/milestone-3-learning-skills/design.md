# Design: Milestone 3 — Learning & Skills

## Technical Approach

Extend the existing subagent infrastructure with 7 new subagents (3 SDD + 4 non-SDD), introduce a Skills Hub for runtime skill management, add a headless execution mode, and extend the first-run wizard. All new subagents follow the established pattern from M2: struct implementing `agent.Subagent`, constructor receiving `*agent.Spawner`, `Execute()` with domain-specific tool filters, and a prompt builder function. The Skills Hub is a new `internal/skills/` package that manages skills on the local filesystem. Headless mode adds a `gaia exec` subcommand.

## Architecture Decisions

### Decision: Non-SDD subagent package location

**Choice**: `internal/agent/ops/` (separate from `internal/agent/sdd/`)
**Alternatives considered**: Put all 12 subagents in `internal/agent/sdd/`; put non-SDD in `internal/agent/general/`
**Rationale**: SDD subagents are pipeline-coupled (sequential phases). Non-SDD subagents are on-demand (invoked by intent detection). Separate packages reflect this architectural difference and prevent the `sdd/` package from growing beyond its domain.

### Decision: Skills storage — bundled vs user-installed

**Choice**: Bundled skills remain in `skills/` (read-only); user-installed skills go to `~/.gaia/skills/`
**Alternatives considered**: Single `skills/` directory for both; database-backed skill store
**Rationale**: Bundled skills ship with the binary and are version-locked. User-installed skills are mutable and project-specific. Separation prevents `go build` from clobbering user skills and allows independent lifecycle.

### Decision: Headless mode architecture

**Choice**: New `cmd/gaia/exec.go` that creates a Brain without TUI, uses a `NullUI` adapter
**Alternatives considered**: Reuse TUI in hidden mode; add headless flag to existing main
**Rationale**: Clean separation — `exec` creates its own dependency graph with a `NullUI` that captures output instead of rendering it. No Bubbletea dependency in headless mode. `--json` flag wraps the captured output in a structured envelope.

### Decision: Non-SDD subagent dispatch mechanism

**Choice**: Keyword heuristic in Brain (like SDD trigger), with explicit `/review`, `/debug`, `/research` commands
**Alternatives considered**: LLM-classified intent; always-on subagent selection
**Rationale**: Avoids extra LLM call for routing. Explicit commands give users deterministic control. Heuristic handles natural language ("review this PR", "debug this test").

### Decision: Skills Hub activation model

**Choice**: Active skills are loaded into a `SkillRegistry` that injects content into subagent prompts via `task.Skills`
**Alternatives considered**: All skills always available; skills as separate LLM context window
**Rationale**: Selective activation keeps prompt size manageable. The `task.Skills` field already exists in `SubagentTask` — we wire it to the Hub's active set.

## Data Flow

```
                          ┌──────────────────────────────────────┐
                          │           GAIA Entry Points           │
                          ├──────────────────┬───────────────────┤
                          │  TUI (interactive)│  exec (headless)  │
                          └────────┬─────────┴─────────┬─────────┘
                                   │                    │
                                   ▼                    ▼
                          ┌──────────────────────────────────────┐
                          │            Brain (orchestrator)        │
                          │  ┌─────────┐  ┌───────────────────┐  │
                          │  │  SDD    │  │  Non-SDD Dispatch  │  │
                          │  │  Trigger│  │  (intent heuristic)│  │
                          │  └────┬────┘  └────────┬──────────┘  │
                          └───────┼────────────────┼─────────────┘
                                  │                │
              ┌───────────────────┼────────────────┼───────────────────┐
              │                   │                │                    │
              ▼                   ▼                ▼                    ▼
    ┌─────────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │  SDD Pipeline   │  │   Reviewer   │  │   Debugger   │  │  Researcher  │
    │  (8 phases)     │  │   (on-demand)│  │   (on-demand)│  │  (on-demand) │
    │                 │  └──────────────┘  └──────────────┘  └──────────────┘
    │  Explorer       │
    │  Proposer       │          ┌──────────────┐
    │  Specifier      │          │   Learner    │
    │  Designer  (NEW)│          │  (cross-agent│
    │  Planner   (NEW)│          │   patterns)  │
    │  Implementer    │          └──────────────┘
    │  Archiver  (NEW)│
    │  Verifier       │          ┌──────────────────────────────┐
    └─────────────────┘          │        Skills Hub             │
                                 │  search │ install │ activate  │
                                 │  list   │ remove  │ deactivate│
                                 └──────────────────────────────┘
                                          │
                                          ▼
                                 ┌──────────────────┐
                                 │  ~/.gaia/skills/  │
                                 │  (user-installed) │
                                 └──────────────────┘
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/agent/sdd/designer.go` | Create | Designer subagent — architecture decisions + design docs (read-only + Engram) |
| `internal/agent/sdd/planner.go` | Create | Planner subagent — task breakdown + sequencing (read + Engram) |
| `internal/agent/sdd/archiver.go` | Create | Archiver subagent — delta spec sync + change archival (read + write) |
| `internal/agent/ops/reviewer.go` | Create | Reviewer subagent — code review + quality gates (read-only) |
| `internal/agent/ops/debugger.go` | Create | Debugger subagent — root cause analysis (read + shell for test execution) |
| `internal/agent/ops/researcher.go` | Create | Researcher subagent — knowledge synthesis (read + file_list) |
| `internal/agent/ops/learner.go` | Create | Learner subagent — cross-subagent pattern extraction (read + Engram) |
| `internal/skills/hub.go` | Create | Skills Hub: Search, Install, List, Activate, Deactivate, Remove |
| `internal/skills/registry.go` | Create | Active skill registry: loads SKILL.md content for prompt injection |
| `internal/skills/hub_test.go` | Create | Unit tests for Hub operations |
| `internal/adapters/output/formatter.go` | Create | Output formatters: JSON, text (quiet/verbose/normal) |
| `cmd/gaia/exec.go` | Create | `gaia exec` headless subcommand with flag parsing |
| `cmd/gaia/main.go` | Modify | Register 7 new subagents, wire Skills Hub, detect exec vs interactive mode |
| `internal/core/kernel.go` | Modify | Extended SDD pipeline (add Design + Plan phases), non-SDD dispatch |
| `internal/adapters/tui/wizard.go` | Modify | Extended wizard: language selection + skill recommendation step |
| `internal/core/domain/models.go` | Modify | Add `SkillContent` field to `SubagentTask` for injected skill text |

## Interfaces / Contracts

```go
// internal/agent/ops/reviewer.go (pattern for all non-SDD subagents)
type reviewer struct {
    spawner *agent.Spawner
}

func NewReviewer(spawner *agent.Spawner) agent.Subagent {
    return &reviewer{spawner: spawner}
}

func (r *reviewer) Name() string        { return "reviewer" }
func (r *reviewer) Description() string { return "Code review and quality gates (read-only)" }

func (r *reviewer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
    task.AllowedTools = []string{
        "file_read", "file_list", "git_status", "git_log", "git_diff",
    }
    // ... prompt + RunLoop pattern
}

// internal/skills/hub.go
type Hub struct {
    bundledDir  string            // "skills/" (read-only, ships with binary)
    installedDir string           // ~/.gaia/skills/ (user-managed)
    active      map[string]bool   // currently active skill names
}

func NewHub(bundledDir, installedDir string) *Hub
func (h *Hub) Search(query string) []SkillMeta
func (h *Hub) Install(name string) error
func (h *Hub) List() []SkillMeta
func (h *Hub) Activate(name string) error
func (h *Hub) Deactivate(name string) error
func (h *Hub) Remove(name string) error
func (h *Hub) ActiveContent() []SkillContent  // for prompt injection

// SkillMeta is the frontmatter parsed from SKILL.md
type SkillMeta struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    Tags        []string `yaml:"tags"`
    Language    string   `yaml:"language"`  // "go", "typescript", etc.
}

// cmd/gaia/exec.go — CLI flags
// gaia exec "task description" [--json] [--quiet] [--verbose] [--dry-run] [--yes]
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Each new subagent | Table-driven: verify Name(), tool filter, prompt structure, result parsing |
| Unit | Skills Hub | Search filtering, install/remove filesystem ops, activate/deactivate state |
| Unit | Output formatters | JSON envelope structure, quiet/verbose filtering |
| Integration | Extended SDD pipeline | Mock LLM through 8 phases with artifact passing |
| Integration | Non-SDD dispatch | Keyword heuristic triggers correct subagent |
| Integration | Headless exec | `gaia exec` with mock provider, verify JSON output |
| E2E | Wizard flow | Simulated first-run: detect project, recommend skills, install |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|----------|--------------|-----------------|-------------------|
| Documentation-like paths | N/A — Skills Hub reads only `SKILL.md` files with known frontmatter | Path validation: reject non-`.md` files and paths with `..` | Test: install with path traversal attempt |
| Git repository selection | N/A — no git operations in new subagents beyond existing read-only tools | Existing gitops module boundaries apply | N/A |
| Push state | N/A — headless mode does not push | `--dry-run` prevents all writes | Test: `--dry-run` does not create files |
| PR commands | N/A — no PR automation in M3 | N/A | N/A |

## Migration / Rollout

No migration required. All changes are additive:
- New subagents register into existing Registry without affecting current 5
- Skills Hub operates on new `~/.gaia/skills/` directory; bundled `skills/` unchanged
- `gaia exec` is a new subcommand; existing `gaia` (interactive) unaffected
- Extended SDD pipeline is backward-compatible: existing 5-phase flow still works when Designer/Planner/Archiver are not triggered

## Open Questions

- [ ] Should the Learner subagent have write access to create skills, or should skill creation remain manual?
- [ ] Should `gaia exec --json` include streaming tokens or only final result?
- [ ] Should the Skills Hub support version pinning for installed skills?
