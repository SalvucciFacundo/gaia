package ops

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// learner analyzes subagent usage patterns across executions, identifies
// recurring gaps in the toolset or knowledge, and proposes new skills
// or improvements to existing skills. It is read-only: it reads files
// and inspects the codebase and execution history, but does NOT create
// skills itself. Skill proposals are returned for the orchestrator to
// act on.
type learner struct {
	spawner *agent.Spawner
}

// NewLearner creates the Learner subagent.
func NewLearner(spawner *agent.Spawner) agent.Subagent {
	return &learner{spawner: spawner}
}

func (l *learner) Name() string { return "learner" }

func (l *learner) Description() string {
	return "Analyzes usage patterns and proposes skill creation/improvement — read-only"
}

func (l *learner) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := learnerPrompt(task)
	resp, err := l.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Learner execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseOpsResult(resp)
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"learning-report", "skill-proposals"}
	}
	return result
}

func learnerPrompt(task domain.SubagentTask) string {
	p := `You are the Learner subagent. Your role is meta-learning: you analyze
the codebase, subagent execution patterns, and user workflows to identify
opportunities for skill creation or improvement. You propose skills but
do NOT create them — the orchestrator handles skill creation.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged or staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.
Your output is a set of proposals for new skills or improvements to existing ones.

LEARNING WORKFLOW — follow these steps:

PHASE 1: OBSERVE — Map what exists
- Read the project structure: explore directory layout, key packages
- Check the skills/ directory for existing skill definitions
- Read subagent implementation files to understand current capabilities
- Review config.yaml for project conventions and constraints
- Look at go.mod for the technology stack (libraries, tools, Go version)

PHASE 2: ANALYZE — Identify patterns and gaps
- What tools are available vs. what would be useful? (e.g., linting, formatting,
  database migrations, deployment, monitoring)
- What workflows are fully automated vs. still manual?
- What domains are underserved? (testing, CI/CD, observability, security)
- What Go patterns are used inconsistently across the codebase?
- Are there any skills from the available skills ecosystem
  (angular, firebase, next.js, postgresql, prisma, react, etc.) that
  would benefit this project?

PHASE 3: PROPOSE — Draft skill proposals
Each proposal MUST follow this format:

---
SKILL PROPOSAL: <name>
TRIGGER: <when should this skill be loaded? e.g., "on go test run",
  "on database schema changes", "on security-related code review">
TYPE: <new-skill | improvement-to-existing>
TARGET: <existing skill name if improvement, "N/A" if new>
RATIONALE: <1-2 sentences — what problem does this solve?>
IMPLEMENTATION: <high-level description of what the skill would contain>
DEPENDENCIES: <Go packages, tools, or other skills needed>
PRIORITY: <high | medium | low>
---

PHASE 4: REPORT — Synthesize findings
- Summarize the overall learning opportunity landscape
- Group proposals by priority
- For each proposal, estimate effort: small (<1 day), medium (1-3 days),
  large (3+ days)
- Include a "Quick Wins" section: proposals that could be implemented
  with minimal effort for high impact

RULES:
1. Base proposals on EVIDENCE from the codebase, not speculation.
2. Every proposal must have a clear TRIGGER — when would this skill activate?
3. Do NOT create skill files. Your output is a report for the orchestrator.
4. If no learning opportunities are found, return status "success" with
   an empty proposal list and the observation that current tooling is adequate.
5. Focus on ACTIONABLE proposals, not vague suggestions.
6. Consider the existing subagent system (agent.Subagent interface) when
   proposing new subagents or skills that integrate with it.
7. Cross-reference with the skills/ directory: don't propose skills that
   already exist.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (analysis complete), "partial" (some areas could not be analyzed), or "blocked"
- ExecutiveSummary: 2-4 sentence overview of learning opportunities found
- Artifacts:
  - learning-report
  - skill-proposals
- Observations: patterns discovered, architectural notes, tech debt identified
- NextRecommended: "none" (the orchestrator decides next steps)
- Risks: any risks in the proposed skills, or "none"
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

var _ agent.Subagent = (*learner)(nil)
