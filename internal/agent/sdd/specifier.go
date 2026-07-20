package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// specifier produces delta specs with RFC 2119 requirements and
// Given/When/Then scenarios. It reads existing artifacts and produces
// structured spec documents. It MUST NOT write code or execute shell
// commands.
type specifier struct {
	spawner *agent.Spawner
}

// NewSpecifier creates the Specifier subagent.
func NewSpecifier(spawner *agent.Spawner) agent.Subagent {
	return &specifier{spawner: spawner}
}

func (s *specifier) Name() string        { return "specifier" }
func (s *specifier) Description() string { return "Produces SDD delta specs with requirements and scenarios" }

func (s *specifier) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := specifierPrompt(task)
	resp, err := s.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Specifier execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "sdd-design")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"specs-generated"}
	}
	return result
}

func specifierPrompt(task domain.SubagentTask) string {
	p := `You are the Specifier subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to produce delta specs: requirements written with RFC 2119 keywords
(MUST, SHALL, SHOULD, MAY) and Given/When/Then scenarios.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.
Your specs will be persisted by the orchestrator after you return them.

RULES:
1. Every requirement MUST use RFC 2119 keywords (MUST, SHALL, SHOULD, MAY).
2. Every requirement MUST include at least one happy-path and one edge-case scenario.
3. Use Given/When/Then format for all scenarios.
4. Requirements are categorized: ADDED, MODIFIED, REMOVED, RENAMED.
5. Group requirements by domain/area.
6. MODIFIED requirements MUST include the complete updated requirement block.
7. REMOVED requirements MUST include a reason and migration path.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (specs complete), "partial" (some specs done), or "blocked"
- ExecutiveSummary: 2-4 sentence summary of the delta specs, including requirement count
- Artifacts: "specs" (always include this) — list specific spec file paths if known
- Observations: notable spec decisions or tradeoffs made
- NextRecommended: "sdd-design" or "sdd-tasks" if specs are ready
- Risks: ambiguous requirements or coverage gaps, or "none"
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

var _ agent.Subagent = (*specifier)(nil)
