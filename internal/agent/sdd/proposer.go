package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// proposer generates SDD change proposals: intent, scope, approach, and
// rollback plan. It has read access to files and repos plus the ability
// to search Engram memory for prior context. It MUST NOT write code or
// execute shell commands.
type proposer struct {
	spawner *agent.Spawner
}

// NewProposer creates the Proposer subagent.
func NewProposer(spawner *agent.Spawner) agent.Subagent {
	return &proposer{spawner: spawner}
}

func (p *proposer) Name() string        { return "proposer" }
func (p *proposer) Description() string { return "Creates SDD change proposals (read + Engram)" }

func (p *proposer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := proposerPrompt(task)
	resp, err := p.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Proposer execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "sdd-spec")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"proposal-generated"}
	}
	return result
}

func proposerPrompt(task domain.SubagentTask) string {
	p := `You are the Proposer subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to create a change proposal: intent, scope, approach, and rollback plan.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.
Your proposal will be persisted by the orchestrator after you return it.

RULES:
1. Start with a clear INTENT section — what is the change and why.
2. Define explicit SCOPE: what is in-scope, what is out-of-scope.
3. Describe the technical APPROACH at a high level.
4. Include a ROLLBACK plan for risky changes.
5. List AFFECTED AREAS with impact assessment.
6. Identify RISKS with likelihood and mitigation.
7. Every proposal MUST reference the SDD protocol.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (proposal complete), "partial" (incomplete), or "blocked"
- ExecutiveSummary: 2-4 sentence overview of the proposed change
- Artifacts: "proposal" (always include this)
- Observations: key findings that informed the proposal
- NextRecommended: "sdd-spec" or "sdd-design" if proposal is ready
- Risks: list of risks with likelihood/mitigation, or "none"
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

var _ agent.Subagent = (*proposer)(nil)
