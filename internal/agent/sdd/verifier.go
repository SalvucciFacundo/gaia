package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// verifier runs tests, checks spec compliance, validates TDD phases,
// and ensures that implementation matches requirements. It has shell
// access (test execution) and read tools. It MUST NOT write or modify code.
type verifier struct {
	spawner *agent.Spawner
}

// NewVerifier creates the Verifier subagent.
func NewVerifier(spawner *agent.Spawner) agent.Subagent {
	return &verifier{spawner: spawner}
}

func (v *verifier) Name() string        { return "verifier" }
func (v *verifier) Description() string { return "Executes tests and validates implementation against specs and TDD phases" }

func (v *verifier) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"shell_exec",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := verifierPrompt(task)
	resp, err := v.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Verifier execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "none")
	if result.Status == domain.SubagentSuccess {
		result.NextRecommended = "sdd-archive"
	}
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"verification-complete"}
	}
	return result
}

func verifierPrompt(task domain.SubagentTask) string {
	p := `You are the Verifier subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to execute tests, validate TDD phases, and verify implementation compliance against specs.
You MUST NOT write or modify code — your job is verification only.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (e.g., go test, go build)
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

RULES:
1. STRICT TDD VERIFICATION:
   - If evaluating a RED phase: Confirm that the test suite fails due to expected assertion failure (NOT syntax error or broken build).
   - If evaluating a GREEN phase: Confirm that the test suite passes 100% and the build succeeds.
2. Run the test suite: "go test ./..." and report pass/fail/skip counts.
3. Run the build: "go build ./cmd/gaia" and confirm it succeeds.
4. Compare implementation files against spec requirements.
5. Report any requirement not satisfied by the implementation.
6. Check that ALL spec scenarios have corresponding test coverage.
7. DO NOT modify files — if a test fails in GREEN phase, report the exact failure; do not fix it yourself.
8. Be thorough: check edge cases, error paths, and boundary conditions.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (all tests pass, build succeeds), "partial" (tests pass but spec gaps found), or "blocked" (tests or build failed)
- ExecutiveSummary: 2-4 sentence summary of verification results including pass/fail counts
- Artifacts: list of test output summaries or verification reports generated
- Observations: spec compliance gaps, test coverage issues, or notable findings
- NextRecommended: "sdd-archive" if verification passes, "none" if blocked
- Risks: failing tests or missing coverage areas, or "none"
- SkillResolution: "none"
`

	if task.Description != "" {
		p += "\nTASK:\n" + task.Description + "\n"
	}

	if len(task.KGContext) > 0 {
		p += "\nRELEVANT CONTEXT & LEARNED INSIGHTS:\n"
		for _, fact := range task.KGContext {
			p += "- " + fact + "\n"
		}
	}

	return p
}

var _ agent.Subagent = (*verifier)(nil)
