// Package sdd contains the five SDD-phase subagent implementations:
// Explorer, Proposer, Specifier, Implementer, and Verifier.
package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// explorer investigates codebase structure, patterns, and dependencies.
// It is restricted to read-only tools: file_read, file_list, and git
// operations. It MUST NOT write files or execute arbitrary shell commands.
type explorer struct {
	spawner *agent.Spawner
}

// NewExplorer creates the Explorer subagent.
func NewExplorer(spawner *agent.Spawner) agent.Subagent {
	return &explorer{spawner: spawner}
}

func (e *explorer) Name() string        { return "explorer" }
func (e *explorer) Description() string { return "Investigates codebase structure, patterns, and dependencies (read-only)" }

func (e *explorer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	// Enforce read-only tool filter at the task level so the Spawner's
	// ToolRegistry.Filtered() has an authoritative list.
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := explorerPrompt(task)
	resp, err := e.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Explorer execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	return parseSDDResult(resp, "explorer")
}

// explorerPrompt builds the system prompt for the Explorer subagent.
func explorerPrompt(task domain.SubagentTask) string {
	p := `You are the Explorer subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to investigate the codebase and return structured Observations.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ-ONLY access. You CANNOT write files or execute shell commands.

RULES:
1. Focus on relevant files, patterns, dependencies, and architecture.
2. Be thorough but concise — list file paths, key symbols, and notable patterns.
3. If you find nothing relevant, say so clearly.
4. Do NOT hallucinate file paths or code that you have not actually read.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (investigation complete), "partial" (partial findings), or "blocked" (could not proceed)
- ExecutiveSummary: 1-3 sentence summary of what was found
- Artifacts: list of key files or paths discovered
- Observations: bullet list of findings (patterns, architecture, dependencies, risks)
- NextRecommended: "sdd-propose" if investigation is complete, "none" if blocked
- Risks: any risks discovered (list, or "none")
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

// Compile-time check that explorer satisfies the Subagent interface.
var _ agent.Subagent = (*explorer)(nil)
