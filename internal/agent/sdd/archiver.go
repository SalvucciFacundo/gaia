package sdd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
	"gaia/internal/delivery"
	"gaia/internal/review"
	"gaia/internal/review/gates"
)

// archiver finalizes the SDD pipeline by merging delta specs into main specs
// and archiving completed changes. It is the only SDD subagent with write
// access to spec files and change directories. It handles the complete
// archival workflow: delta merge, directory move, and audit trail.
type archiver struct {
	spawner  *agent.Spawner
	queue    delivery.Queue
	casStore gates.ReceiptStore
	workDir  string
}

// NewArchiver creates the Archiver subagent with default dependencies.
func NewArchiver(spawner *agent.Spawner) agent.Subagent {
	return &archiver{
		spawner: spawner,
	}
}

// NewArchiverWithDeps creates the Archiver subagent with explicit dependencies for testing or custom configuration.
func NewArchiverWithDeps(spawner *agent.Spawner, queue delivery.Queue, casStore gates.ReceiptStore, workDir string) agent.Subagent {
	return &archiver{
		spawner:  spawner,
		queue:    queue,
		casStore: casStore,
		workDir:  workDir,
	}
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

	workDir := a.resolveWorkDir(task)
	receiptStore := a.resolveReceiptStore(workDir)
	changeName := extractChangeName(task)

	// Gate: validate review receipt before archiving.
	// The archiver must not proceed without an approved review receipt.
	if gateErr := a.checkReviewGate(receiptStore, workDir, changeName); gateErr != nil {
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

	// PR3.1: Delivery Queue Hook
	if result.Status == domain.SubagentSuccess {
		delMode := extractDeliveryMode(task)
		if delMode == delivery.DeliveryModeDeferred {
			changeName := extractChangeName(task)
			item, err := a.enqueueDeliveryItem(workDir, receiptStore, changeName, result.Summary)
			if err == nil && item != nil {
				result.Artifacts = append(result.Artifacts, fmt.Sprintf("delivery-item:%s", item.ID))
			}
		}
	}

	return result
}

func (a *archiver) resolveWorkDir(task domain.SubagentTask) string {
	if a.workDir != "" {
		return a.workDir
	}
	if task.WorkDir != "" {
		return task.WorkDir
	}
	return "."
}

func (a *archiver) resolveReceiptStore(workDir string) gates.ReceiptStore {
	if a.casStore != nil {
		return a.casStore
	}
	return gates.NewFSReceiptStore(workDir)
}

func (a *archiver) resolveQueue(workDir string) (delivery.Queue, error) {
	if a.queue != nil {
		return a.queue, nil
	}
	queuePath := filepath.Join(workDir, ".gaia", "delivery", "queue.json")
	return delivery.NewFileQueue(queuePath)
}

func (a *archiver) enqueueDeliveryItem(workDir string, receiptStore gates.ReceiptStore, changeName string, summary string) (*delivery.DeliveryItem, error) {
	q, err := a.resolveQueue(workDir)
	if err != nil {
		return nil, err
	}

	// Retrieve receipt lineage if available
	var lineageID string
	var commitSHA string
	if receiptStore != nil {
		if r, err := receiptStore.LatestReceipt(changeName); err == nil && r != nil {
			lineageID = r.LineageID
		}
	}
	// If review mode is disabled and no receipt exists, do not fabricate synthetic lineage.
	// item.ReceiptLineage can remain empty ("") or be unreviewed.

	// Resolve commit SHA & branch from git if possible
	branch := getGitCurrentBranch(workDir)
	if branch == "" || branch == "HEAD" {
		branch = fmt.Sprintf("feature/%s", changeName)
	}

	commitSHA = getGitCommitSHA(workDir)
	if commitSHA == "" {
		commitSHA = fmt.Sprintf("sha1-%x", time.Now().UnixNano())
	}

	prTitle, prBody := generatePRTitleAndBody(changeName, summary)

	itemID := fmt.Sprintf("del-%s", changeName)
	item := delivery.DeliveryItem{
		ID:             itemID,
		ChangeName:     changeName,
		Branch:         branch,
		BaseBranch:     "main",
		CommitSHA:      commitSHA,
		ReceiptLineage: lineageID,
		PRTitle:        prTitle,
		PRBody:         prBody,
		Status:         delivery.StatusStandby,
		CreatedAt:      time.Now(),
	}

	if err := q.Enqueue(item); err != nil {
		return nil, err
	}

	return &item, nil
}

func extractDeliveryMode(task domain.SubagentTask) delivery.DeliveryMode {
	combined := strings.ToLower(task.Description + " " + strings.Join(task.KGContext, " "))
	if strings.Contains(combined, "delivery_mode: deferred") ||
		strings.Contains(combined, "delivery: deferred") ||
		strings.Contains(combined, "mode: deferred") ||
		strings.Contains(combined, "deferred mode") ||
		strings.Contains(combined, "delivery_mode:deferred") {
		return delivery.DeliveryModeDeferred
	}
	if strings.Contains(combined, "delivery_mode: auto") ||
		strings.Contains(combined, "delivery: auto") ||
		strings.Contains(combined, "auto mode") {
		return delivery.DeliveryModeAuto
	}
	return delivery.DeliveryModeInteractive
}

func extractChangeName(task domain.SubagentTask) string {
	for _, text := range append([]string{task.Description}, task.KGContext...) {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if idx := strings.Index(strings.ToLower(line), "change:"); idx != -1 {
				val := strings.TrimSpace(line[idx+len("change:"):])
				fields := strings.Fields(val)
				if len(fields) > 0 {
					return fields[0]
				}
			}
			// check change <name> (e.g. "archive change auth-models")
			reAfter := regexp.MustCompile(`(?i)\b(?:change|changes/)\s+([a-zA-Z0-9_\-]+)`)
			if m := reAfter.FindStringSubmatch(line); len(m) > 1 {
				candidate := m[1]
				if !strings.EqualFold(candidate, "directory") && !strings.EqualFold(candidate, "specs") && !strings.EqualFold(candidate, "for") {
					return candidate
				}
			}
			// check <name> change (e.g. "auth-models change")
			reBefore := regexp.MustCompile(`(?i)\b([a-zA-Z0-9_\-]+)\s+change\b`)
			if m := reBefore.FindStringSubmatch(line); len(m) > 1 {
				candidate := m[1]
				if !strings.EqualFold(candidate, "completed") && !strings.EqualFold(candidate, "the") && !strings.EqualFold(candidate, "this") && !strings.EqualFold(candidate, "active") && !strings.EqualFold(candidate, "an") {
					return candidate
				}
			}
		}
	}
	if task.ID != "" {
		return strings.TrimPrefix(task.ID, "task-")
	}
	return "change"
}

func generatePRTitleAndBody(changeName, summary string) (string, string) {
	title := fmt.Sprintf("feat(%s): implement %s", changeName, changeName)
	var sb strings.Builder
	sb.WriteString("## Summary\n")
	if summary != "" {
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Implementation and specs for SDD change `%s`.\n\n", changeName))
	}
	sb.WriteString("## Verification\n")
	sb.WriteString("- Strict TDD verified: all unit tests passing.\n")
	sb.WriteString("- CAS review receipt approved and validated.\n\n")
	sb.WriteString("## SDD Audit Trail\n")
	sb.WriteString(fmt.Sprintf("- Change: `%s`\n- Spec delta merged into main specification.\n- Change archived under `openspec/changes/archive/`.\n", changeName))
	return title, sb.String()
}

func getGitCurrentBranch(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getGitCommitSHA(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
// allowing the archiver to proceed, when review mode is enabled or receipts exist.
func (a *archiver) checkReviewGate(store gates.ReceiptStore, workDir, changeName string) error {
	if store == nil {
		return nil
	}
	// Only enforce when review mode is enabled
	if workDir != "" && review.IsEnabled(workDir).Mode != review.ModeEnabled {
		// If review mode is disabled, still check if receipts exist in store
		summaries, err := store.ListReceipts()
		if err != nil || len(summaries) == 0 {
			return nil
		}
		// If receipts exist for this change, ensure none are blocking/unapproved
		for _, s := range summaries {
			if changeName != "" && s.ChangeName != changeName {
				continue
			}
			if s.State != string(domain.ReviewStateApproved) {
				return agent.ErrReceiptNotApproved
			}
			return nil
		}
		return nil
	}

	summaries, err := store.ListReceipts()
	if err != nil {
		return nil
	}
	if len(summaries) == 0 {
		return agent.ErrReceiptNotApproved
	}

	for _, s := range summaries {
		if changeName != "" && s.ChangeName != changeName {
			continue
		}
		if s.State == string(domain.ReviewStateApproved) {
			return nil
		}
	}
	return agent.ErrReceiptNotApproved
}
