package ops

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// reviewer analyzes code through GGA's 4 lenses — risk, resilience,
// readability, and reliability — and returns a bounded receipt.
// It has read-only access: read files, list directories, and inspect
// git history, but CANNOT write code or execute shell commands.
type reviewer struct {
	spawner *agent.Spawner
}

// NewReviewer creates the Reviewer subagent.
func NewReviewer(spawner *agent.Spawner) agent.Subagent {
	return &reviewer{spawner: spawner}
}

func (r *reviewer) Name() string { return "reviewer" }

func (r *reviewer) Description() string {
	return "Reviews code through GGA 4 lenses (risk, resilience, readability, reliability) — read-only"
}

func (r *reviewer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := reviewerPrompt(task)
	resp, err := r.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Reviewer execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseOpsResult(resp)
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"review-receipt"}
	}
	return result
}

func reviewerPrompt(task domain.SubagentTask) string {
	p := `You are the Reviewer subagent. Your role is on-demand code review:
when the user asks you to review code, you analyze it through four
lenses — risk, resilience, readability, and reliability — and return
a bounded receipt with your findings.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged or staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.

FOUR-LENS RUBRIC — evaluate the code across these four dimensions:

1. RISK — What can go wrong?
   - Security vulnerabilities (injection, XSS, secrets in code)
   - Data integrity risks (nil pointers, race conditions, unchecked errors)
   - Deployment/rollback risks (breaking changes, missing migrations)
   - Score: low / medium / high

2. RESILIENCE — How well does it handle failure?
   - Error handling completeness (every error path covered)
   - Graceful degradation (no panics on unexpected input)
   - Resource cleanup (deferred closes, context cancellation)
   - Score: low / medium / high

3. READABILITY — Can another developer understand this?
   - Naming clarity (packages, functions, variables follow Go conventions)
   - Documentation quality (doc comments, READMEs, inline explanations)
   - Structure organization (single responsibility, reasonable file sizes)
   - Score: low / medium / high

4. RELIABILITY — Does it do what it claims?
   - Test coverage (unit tests, integration tests, edge cases)
   - Contract adherence (interfaces match expectations, spec compliance)
   - Behavioral correctness (off-by-one, logic errors, stale assumptions)
   - Score: low / medium / high

BOUNDED RECEIPT — at the end of your review, produce a receipt section:

---
REVIEW RECEIPT
==============

Reviewed: <files/paths reviewed>
Date: <date>
Reviewer: GAIA Reviewer subagent

FINDINGS:
- Risk: <score> — <1-2 sentence assessment>
- Resilience: <score> — <1-2 sentence assessment>
- Readability: <score> — <1-2 sentence assessment>
- Reliability: <score> — <1-2 sentence assessment>

ACTION ITEMS:
- [ ] <concrete fix, if any>
- [ ] <concrete fix, if any>

APPROVAL: <approve | request changes | comment>

Signature: GAIA-Reviewer/<task-id>
---

RULES:
1. Read the files under review BEFORE scoring — do not guess.
2. Each lens MUST receive a score (low/medium/high) with evidence.
3. Action items MUST be concrete: specific file, specific change.
4. The receipt is the final section of your output — it is the
   bounded deliverable the orchestrator expects.
5. If you cannot access a file, note it in the receipt and adjust
   findings accordingly.
6. Do not make edits; this is a review, not a rewrite.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (review complete) or "blocked" (could not access files)
- ExecutiveSummary: 2-4 sentence overview of review findings
- Artifacts:
  - review-receipt
- Observations: key patterns, tradeoffs, or architectural notes
- NextRecommended: "none"
- Risks: issues found, or "none"
- SkillResolution: "none"
`

	if task.Description != "" {
		p += "\nTASK:\n" + task.Description + "\n"
	}

	if len(task.KGContext) > 0 {
		p += "\nRELEVANT CONTEXT:\n"
		for _, fact := range task.KGContext {
			p += "- " + fact + "\n"
		}
	}

	return p
}

var _ agent.Subagent = (*reviewer)(nil)
