package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// designer creates architecture decisions, design documents, data flow
// diagrams, and file change plans. It follows the SDD pipeline between
// Specifier and Planner. Access is read-only: it MUST NOT write code
// or execute shell commands.
type designer struct {
	spawner *agent.Spawner
}

// NewDesigner creates the Designer subagent.
func NewDesigner(spawner *agent.Spawner) agent.Subagent {
	return &designer{spawner: spawner}
}

func (d *designer) Name() string        { return "designer" }
func (d *designer) Description() string { return "Creates architecture decisions and design documents (read + Engram)" }

func (d *designer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := designerPrompt(task)
	resp, err := d.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Designer execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "sdd-tasks")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"design-document"}
	}
	return result
}

func designerPrompt(task domain.SubagentTask) string {
	p := `You are the Designer subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to create the technical design and architecture approach for an SDD change.
You receive delta specs and produce a design document with architecture decisions,
data flow descriptions, file change plans, and interface/contract definitions.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.
Your design will be persisted by the orchestrator after you return it.

RULES:
1. Start with a TECHNICAL APPROACH section — high-level strategy for the change.
2. Document ARCHITECTURE DECISIONS with rationale and alternatives considered.
   Use the format: "Decision: {name}\nChoice: {selection}\nAlternatives: {list}\nRationale: {why}"
3. Include a DATA FLOW section — how data moves between components.
   Use sequence diagrams (ASCII text) for complex flows.
4. List FILE CHANGES in a table: File | Action | Description.
5. Define INTERFACES / CONTRACTS — new or modified function signatures, types, etc.
6. Include a TESTING STRATEGY — unit, integration, and E2E approach.
7. Add a THREAT MATRIX for risky operations (path traversal, shell injection, etc.).
8. Document MIGRATION / ROLLOUT steps if applicable.
9. Follow existing architecture patterns: hexagonal, ports & adapters, Go conventions.
10. Every architecture decision MUST have a clear rationale.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (design complete), "partial" (some decisions pending), or "blocked"
- ExecutiveSummary: 2-4 sentence overview of the design approach
- Artifacts:
  - design (always include this)
  - design-document
- Observations: key design tradeoffs, assumptions, or constraints
- NextRecommended: "sdd-tasks" if design is ready
- Risks: design risks with likelihood and mitigation, or "none"
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

var _ agent.Subagent = (*designer)(nil)
