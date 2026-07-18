package ops

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// debugger follows a structured debugging workflow: analyze symptoms,
// trace to root cause, propose and apply a fix, then verify the result.
// It has full tool access: file read/write, shell execution, and git
// inspection — everything needed to diagnose and fix a bug end-to-end.
type debugger struct {
	spawner *agent.Spawner
}

// NewDebugger creates the Debugger subagent.
func NewDebugger(spawner *agent.Spawner) agent.Subagent {
	return &debugger{spawner: spawner}
}

func (d *debugger) Name() string { return "debugger" }

func (d *debugger) Description() string {
	return "Debugs and fixes issues: analyze → root cause → fix → verify (read + shell + write)"
}

func (d *debugger) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_write",
		"file_list",
		"shell_exec",
		"git_status",
		"git_log",
		"git_diff",
	}

	prompt := debuggerPrompt(task)
	resp, err := d.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Debugger execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseOpsResult(resp)
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"debugger-report"}
	}
	return result
}

func debuggerPrompt(task domain.SubagentTask) string {
	p := `You are the Debugger subagent. Your role is on-demand debugging:
when the user reports a bug, you follow a structured four-phase workflow
to diagnose, fix, and verify the issue end-to-end.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_write: write content to a file (creates parent directories)
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (go build, go test, etc.)
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged or staged changes

You have FULL tool access: read, write, and shell execution.
You are expected to make code changes when you identify the fix.

STRUCTURED DEBUGGING WORKFLOW — follow these four phases in order:

PHASE 1: ANALYZE — Understand the bug
- Reproduce the issue if possible (use shell_exec to run tests, build, etc.)
- Read relevant source files and trace the code path
- Inspect git log for recent changes that may have introduced the bug
- Summarize: what is the observed symptom? What is the expected behavior?

PHASE 2: ROOT CAUSE — Trace to origin
- Follow the data flow and control flow from symptom to source
- Identify the exact line(s) or logic responsible
- Write a clear root-cause statement: "The bug is in <file>:<line> because <reason>"
- Confirm by reading more code or running diagnostic commands

PHASE 3: FIX — Apply the correction
- Write the minimal fix — change only what is necessary
- If multiple bugs are found, fix the root cause first
- Keep the fix consistent with project conventions (Go idioms, existing patterns)
- Use file_write to apply the fix

PHASE 4: VERIFY — Prove the fix works
- Run relevant tests with shell_exec (go test ./affected/package/...)
- Run go build ./... to confirm compilation
- If the bug is a runtime issue, run the binary with the reproduction scenario
- Confirm the fix does not introduce regressions

RULES:
1. NEVER apply a fix YOU HAVE NOT UNDERSTOOD. Phase 2 MUST complete before Phase 3.
2. Write the ROOT CAUSE statement explicitly — do not skip this step.
3. If you cannot reproduce the bug with available tools, explain what you
   need and return status: "partial".
4. The fix MUST be minimal — do not refactor unrelated code.
5. If the fix requires a new dependency or config change, note it in artifacts.
6. Do NOT commit code; the orchestrator handles version control.
7. Run go build ./... AFTER every file_write to catch compilation errors early.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (bug fixed and verified), "partial" (identified but needs more info), or "blocked" (cannot proceed)
- ExecutiveSummary: brief description of the bug, root cause, and fix applied
- Artifacts:
  - debugger-report
  - <list of files modified>
- Observations: notable patterns, edge cases, or alternative fixes considered
- NextRecommended: "none"
- Risks: any risks introduced by the fix, or "none"
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

var _ agent.Subagent = (*debugger)(nil)
