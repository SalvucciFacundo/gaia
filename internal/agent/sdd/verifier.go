package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// verifier runs tests, checks spec compliance, and validates that the
// implementation matches the requirements. It has shell access (test
// execution) and read tools. It MUST NOT write or modify code.
type verifier struct {
	spawner *agent.Spawner
}

// NewVerifier creates the Verifier subagent.
func NewVerifier(spawner *agent.Spawner) agent.Subagent {
	return &verifier{spawner: spawner}
}

func (v *verifier) Name() string        { return "verifier" }
func (v *verifier) Description() string { return "Executes tests and validates implementation against specs" }

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
Your role is to execute tests and validate the implementation against the specs.
You MUST NOT write or modify code — your job is verification only.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (e.g., go test, go build)
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

RULES:
1. Run the test suite: "go test ./..." and report results.
2. Run the build: "go build ./cmd/gaia" and confirm it succeeds.
3. Compare implementation files against spec requirements.
4. Report any requirement not satisfied by the implementation.
5. Check that ALL spec scenarios have corresponding test coverage.
6. DO NOT modify files — if a test fails, report it; do not fix it.
7. Be thorough: check edge cases, error paths, and boundary conditions.

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
		p += "\nRELEVANT CONTEXT:\n"
		for _, fact := range task.KGContext {
			p += "- " + fact + "\n"
		}
	}

	return p
}

var _ agent.Subagent = (*verifier)(nil)
