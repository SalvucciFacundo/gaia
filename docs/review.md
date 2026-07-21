# Code Review System

GAIA includes a built-in bounded code review system inspired by Gentle-AI's BR (Gentleman Guardian Angel) and extended with Judgment Day for adversarial review.

---

## Overview

```
gaia review start                    # Start a review
gaia review start --judgment-day     # Adversarial review
gaia review status                   # Check review state
gaia review validate                 # Validate receipt
gaia review list                     # List recent reviews
gaia review install-hooks            # Install git hooks
```

Reviews produce **content-bound receipts** (SHA256). Every delivery gate validates against the same receipt:

```
post-apply → pre-commit → pre-push → pre-PR → release
```

If content changes between gates, the receipt is invalidated and a new review is needed.

---

## Risk Classification

The engine classifies each change using 8 risk codes:

| Risk Code | Meaning | Triggers |
|---|---|---|
| `configuration_change` | Config file changes | YAML, JSON, TOML, env files |
| `executable_change` | Binary output changes | Build outputs, compiled files |
| `executable_mode` | Permission changes | File mode bits (+x) |
| `hot_path` | Auth/security/payments | Routes with auth middleware |
| `large_change` | Many changed lines | >400 changed lines |
| `non_executable_only` | Docs/comments only | Only markdown, comments, formatting |
| `service_token` | Credential changes | New API keys, tokens in code |
| `shell_source` | Subprocess changes | Shell scripts, Makefile, subprocess calls |

Risk level is determined by combining codes:

| Risk Level | Condition | Lenses |
|---|---|---|
| **Low** | Only `non_executable_only` | No lens needed (auto-approve) |
| **Medium** | Any other single reason | 1 dominant lens |
| **High** | `hot_path` OR `large_change` OR `service_token` OR `shell_source` | All 4 lenses |

---

## The 4 Review Lenses

### Risk Lens

Focuses on security, permissions, data exposure, and architecture.

Checks:
- SQL injection and command injection
- Missing authentication/authorization
- Hardcoded secrets
- Insecure cryptography
- Path traversal vulnerabilities
- Dependency vulnerabilities
- Overly permissive permissions

### Resilience Lens

Focuses on fallbacks, retry, graceful degradation, and observability.

Checks:
- Missing error handling
- No retry/backoff for external calls
- Missing fallbacks for degraded dependencies
- No logging/metrics for critical paths
- Resource leaks (connections, file handles)
- Missing circuit breakers

### Readability Lens

Focuses on naming, structure, maintainability, and comments.

Checks:
- Clear naming (no abbreviations)
- Single responsibility per function
- Appropriate abstraction level
- Comments explain WHY, not WHAT
- Consistent project patterns
- No dead code

### Reliability Lens

Focuses on tests, determinism, regressions, and edge cases.

Checks:
- Missing tests for new code
- Flaky test patterns
- Untested edge cases (empty, nil, boundary)
- Non-deterministic test behavior
- Missing test fixtures
- Test coverage gaps

---

## Review State Machine

A review progresses through formal states:

```
unreviewed
    │
    ▼
reviewing ───── Start the review
    │
    ├── judges_confirmed (Judgment Day only — judges have reported)
    │
    ▼
findings_frozen ─── Findings are locked
    │
    ▼
evidence_classified ─── Each finding gets severity
    │                     (BLOCKER / WARNING / SUGGESTION)
    │
    ├── fix_required → fixing → fix_validating
    │     (1 correction round normal, 2 for Judgment Day)
    │
    ▼
ready_final_verification
    │
    ▼
final_verifying ─── Tests + build confirm the fix
    │
    ├── approved ─────── Receipt issued
    ├── escalated ────── Human intervention needed
    └── invalidated ──── Content changed, new review needed
```

---

## Receipt Structure

```json
{
  "schema": "gaia.review-receipt/v1",
  "lineage_id": "sha256:abc123...",
  "snapshot_hash": "sha256:def456...",
  "selected_lenses": ["review-risk", "review-readability"],
  "risk_level": "medium",
  "risk_reasons": ["configuration_change"],
  "correction_budget": 85,
  "correction_used": 0,
  "state": "approved",
  "findings": [
    {
      "severity": "WARNING",
      "lens": "review-risk",
      "file": "src/config.js",
      "line": 42,
      "message": "API key hardcoded — use environment variable"
    }
  ],
  "final_verification_hash": "sha256:789ghi..."
}
```

---

## Git Hooks

Install pre-commit and pre-push hooks:

```bash
gaia review install-hooks
```

This creates:
- `.git/hooks/pre-commit` — Runs `gaia review validate` before each commit
- `.git/hooks/pre-push` — Runs `gaia review validate` before each push

Both hooks validate the content-bound receipt. If the receipt is invalidated (content changed since review), the hook blocks the operation.

---

## AGENTS.md Standards

GAIA reads `AGENTS.md` from the project root as team coding standards. The file uses YAML frontmatter + markdown:

```markdown
---
name: my-project
version: 1.0.0
rules:
  - use-early-returns
  - no-magic-numbers
---

# Team Standards

## Code Style
- Use early returns over nested if/else
- Error messages should be lowercase, no punctuation
- Prefer table-driven tests

## Security
- Never log secrets or API keys
- Use parameterized queries
- Validate all user input

## Architecture
- Follow hexagonal architecture
- Dependencies point inward only
```

The Reviewer subagent automatically discovers `AGENTS.md` in the project root and injects its rules into review prompts.

---

## Judgment Day Protocol

For high-risk changes (auth, security, payments, >400 lines):

```bash
gaia review start --judgment-day
```

1. **Judge A** (blind) — Focuses on security, data flow, correctness
2. **Judge B** (blind) — Focuses on error handling, edge cases, resilience
3. **Comparison** — Merge findings, resolve conflicts by severity
4. **Fix Agent** — Applies surgical corrections (budget-limited)
5. **Re-judgment** — Maximum 2 rounds of fix + re-judgment
6. **Approval** — Both judges must agree, otherwise escalated to human

