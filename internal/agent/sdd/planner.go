package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// planner breaks the design into implementation tasks with workload forecasts.
// It sits between Designer and Implementer in the SDD pipeline. It has read
// access plus shell_exec for validation (e.g., checking file existence).
// It MUST NOT write files.
type planner struct {
	spawner *agent.Spawner
}

// NewPlanner creates the Planner subagent.
func NewPlanner(spawner *agent.Spawner) agent.Subagent {
	return &planner{spawner: spawner}
}

func (p *planner) Name() string        { return "planner" }
func (p *planner) Description() string { return "Creates task breakdowns and workload forecasts (read + shell)" }

func (p *planner) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"shell_exec",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := plannerPrompt(task)
	resp, err := p.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Planner execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "sdd-implement")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"task-breakdown"}
	}
	return result
}

func plannerPrompt(task domain.SubagentTask) string {
	p := `You are the Planner subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to break the design into concrete, ordered implementation tasks.
You produce a tasks.md document with workload forecasting for PR boundaries.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (e.g., wc -l for counting lines)
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ access plus shell validation. You CANNOT write files.
Your task plan will be persisted by the orchestrator after you return it.

RULES:
1. Group tasks into phases using hierarchical numbering (e.g., 1.1, 1.2, 2.1).
2. Each task MUST be completable in one session.
3. Each task MUST have a clear deliverable and acceptance criteria.
4. Tasks MUST be ordered by dependency — earlier tasks unblock later ones.
5. Include a REVIEW WORKLOAD FORECAST section:
   - Decision needed before apply: Yes/No
   - Chained PRs recommended: Yes/No (recommend if over 400 changed lines)
   - 400-line budget risk: Low/Medium/High
   - Chain strategy: stacked-to-main or feature-branch-chain (if chained)
6. Each PR slice must have autonomous scope, verification, and rollback.
7. Number tasks consistently: PR1.x, PR2.x, PR3.x, PR4.x or 1.x, 2.x.
8. Mark task dependencies explicitly when a task depends on another.
9. Use the project's existing conventions for file paths and naming.
10. Keep task descriptions actionable: start with a verb (Create, Add, Update, etc.).

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (plan complete), "partial" (some tasks need clarification), or "blocked"
- ExecutiveSummary: 2-4 sentence summary including task count and PR structure
- Artifacts:
  - tasks (always include this)
  - task-breakdown
- Observations: any risks or assumptions in the task plan
- NextRecommended: "sdd-implement" if plan is ready
- Risks: dependency bottlenecks, scope creep risks, or "none"
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

var _ agent.Subagent = (*planner)(nil)
