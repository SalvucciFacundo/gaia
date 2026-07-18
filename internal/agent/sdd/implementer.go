package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// implementer writes code from tasks and specs, following existing code
// patterns. It has full tool access: file read/write, shell execution,
// and git operations. It MUST respect the tool filter and follow the
// project's architectural conventions.
type implementer struct {
	spawner *agent.Spawner
}

// NewImplementer creates the Implementer subagent.
func NewImplementer(spawner *agent.Spawner) agent.Subagent {
	return &implementer{spawner: spawner}
}

func (i *implementer) Name() string        { return "implementer" }
func (i *implementer) Description() string { return "Writes code from tasks and specs (write + shell)" }

func (i *implementer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_write",
		"file_list",
		"shell_exec",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := implementerPrompt(task)
	resp, err := i.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Implementer execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "sdd-verify")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"implementation-complete"}
	}
	return result
}

func implementerPrompt(task domain.SubagentTask) string {
	p := `You are the Implementer subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to write or modify code according to tasks, specs, and design documents.
You produce working, tested implementations that follow project conventions.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_write: write content to a file (creates parent directories)
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (go test, go build, etc.)
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

RULES:
1. READ existing code FIRST before writing — understand the patterns in the codebase.
2. Follow the project's architecture: hexagonal (ports & adapters), Go conventions.
3. Write idiomatic Go: early returns, table-driven tests, standard library first.
4. Run "go build ./..." after changes to verify compilation.
5. Run relevant tests to verify correctness.
6. After each file write, verify it was written correctly.
7. Never modify generated files or vendor code.
8. Do NOT commit code — the orchestrator handles version control.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (all changes complete), "partial" (some tasks remain), or "blocked" (could not proceed)
- ExecutiveSummary: 2-4 sentence summary of what was implemented
- Artifacts: list of file paths you created or modified
- Observations: notable implementation decisions or tradeoffs
- NextRecommended: "sdd-verify" if implementation is complete
- Risks: any issues found during implementation, or "none"
- SkillResolution: "none"
`

	if task.Mode == "build" {
		p += "\nEXECUTION MODE: build — write code files, verify compilation, and run tests.\n"
	}

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

var _ agent.Subagent = (*implementer)(nil)
