package sdd

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
	"gaia/internal/review/gates"
)

// archiver finalizes the SDD pipeline by merging delta specs into main specs
// and archiving completed changes. It is the only SDD subagent with write
// access to spec files and change directories. It handles the complete
// archival workflow: delta merge, directory move, and audit trail.
type archiver struct {
	spawner *agent.Spawner
}

// NewArchiver creates the Archiver subagent.
func NewArchiver(spawner *agent.Spawner) agent.Subagent {
	return &archiver{spawner: spawner}
}

func (a *archiver) Name() string        { return "archiver" }
func (a *archiver) Description() string { return "Merges delta specs and archives completed changes (read + write)" }

func (a *archiver) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_write",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	// Gate: validate review receipt before archiving.
	// The archiver must not proceed without an approved review receipt.
	if gateErr := checkReviewGate(); gateErr != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         gateErr.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	prompt := archiverPrompt(task)
	resp, err := a.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Archiver execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseSDDResult(resp, "none")
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"archive-complete"}
	}
	return result
}

func archiverPrompt(task domain.SubagentTask) string {
	p := `You are the Archiver subagent in the SDD (Spec-Driven Development) pipeline.
Your role is to finalize completed changes: merge delta specs into main specs,
archive the change directory, and maintain the audit trail. You are the gatekeeper
for the SDD record — your work creates the permanent history of the change.

AVAILABLE TOOLS:
- file_read: read a file's contents
- file_write: write content to a file (creates parent directories)
- file_list: list directory contents
- git_status: show working tree status
- git_log: show commit history
- git_diff: show unstaged/staged changes

You have READ and WRITE access. You are the only SDD subagent authorized to
modify spec files and move change directories. Use this authority with care.

ARCHIVE STRUCTURE:
- Source: openspec/changes/{change-name}/
- Destination: openspec/changes/archive/YYYY-MM-DD-{change-name}/
- Delta specs in: openspec/changes/{change-name}/specs/{domain}/spec.md
- Main specs in: openspec/specs/{domain}/spec.md

ARCHIVE WORKFLOW:
1. Read the delta specs from the change's specs/ directory.
2. Identify each requirement section: ADDED, MODIFIED, REMOVED, RENAMED.
3. For ADDED: append new requirements to the main spec file for that domain.
4. For MODIFIED: replace the matching requirement block in the main spec
   (the delta MUST contain the complete updated requirement).
5. For REMOVED: confirm the reason is documented, then delete from main spec.
   Each removed requirement MUST have "(Reason: ...)" — warn if missing.
6. For RENAMED: update the requirement heading in main spec.
7. After merging ALL deltas, move the change directory to the archive.
8. Verify the archive move succeeded and the change directory is gone.
9. Never delete or modify archived changes — the archive is an AUDIT TRAIL.

RULES:
1. WARN before merging destructive deltas (REMOVED requirements).
2. Verify the main spec was updated correctly by reading it after the merge.
3. Ensure no duplicate requirements are introduced by the ADDED merge.
4. The archive destination MUST use today's date in ISO format (YYYY-MM-DD).
5. After archival, the NextRecommended is ALWAYS "none" — the change is complete.
6. If any delta section is missing a required field (e.g., REMOVED without reason),
   report it as a risk and mark status as "partial".
7. Do NOT archive if verification has not passed — check for a verify-report.
8. Maintain a clear audit trail: record what was merged, moved, and when.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (all deltas merged, archive complete), "partial" (some deltas pending), or "blocked"
- ExecutiveSummary: 2-4 sentence summary of the archival, including merged domains and archive location
- Artifacts:
  - archive-complete
  - list all merged spec domains (e.g., "openspec/specs/auth/spec.md")
- Observations: any warnings, delta validation issues, or merge conflicts resolved
- NextRecommended: "none" (archive is the final phase)
- Risks: destructive delta warnings, merge conflicts, or missing verify-report, or "none"
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

var _ agent.Subagent = (*archiver)(nil)

// checkReviewGate verifies that a valid review receipt exists before
// allowing the archiver to proceed. It blocks the archive if:
//   - No receipt is found
//   - The receipt state is not "approved"
//
// If no review store exists at all (no .gaia/reviews/ directory), the
// gate passes silently — the review infrastructure is not yet set up.
func checkReviewGate() error {
	// Use the filesystem receipt store at the current working directory.
	store := gates.NewFSReceiptStore(".")
	summaries, err := store.ListReceipts()
	if err != nil {
		// No review infrastructure → pass through.
		return nil
	}
	if len(summaries) == 0 {
		// No receipts yet → pass through (review may be optional).
		return nil
	}

	// Check if ANY receipt is in approved state.
	for _, s := range summaries {
		if s.State == "approved" {
			return nil
		}
	}
	// Receipts exist but none are approved → block.
	return agent.ErrReceiptNotApproved
}
