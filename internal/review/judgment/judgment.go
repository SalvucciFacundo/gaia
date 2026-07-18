// Package judgment implements GAIA's Judgment Day adversarial review protocol.
// Two independent judges (judge-a, judge-b) analyze changes with different
// focus areas, their findings are compared and merged, then a fix agent
// applies surgical corrections up to 2 rounds until both judges agree.
package judgment

import (
	"context"
	"fmt"
	"time"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
	"gaia/internal/review"
)

// MaxRounds is the default maximum number of fix-judge cycles.
const MaxRounds = 2

// JudgmentDay orchestrates blind dual-review with fix cycles.
type JudgmentDay struct {
	spawner   *agent.Spawner
	maxRounds int
}

// JudgmentResult holds the complete output of a Judgment Day review.
type JudgmentResult struct {
	JudgeAFindings []domain.ReviewFinding `json:"judge_a_findings"`
	JudgeBFindings []domain.ReviewFinding `json:"judge_b_findings"`
	MergedFindings []domain.ReviewFinding `json:"merged_findings"`
	Rounds         int                    `json:"rounds"`
	Approved       bool                   `json:"approved"`
	StartedAt      time.Time              `json:"started_at"`
	CompletedAt    time.Time              `json:"completed_at"`
}

// NewJudgmentDay creates a Judgment Day orchestrator with the given spawner.
// maxRounds defaults to 2 if <= 0.
func NewJudgmentDay(spawner *agent.Spawner) *JudgmentDay {
	return &JudgmentDay{
		spawner:   spawner,
		maxRounds: MaxRounds,
	}
}

// SetMaxRounds overrides the default round limit.
func (jd *JudgmentDay) SetMaxRounds(n int) {
	if n > 0 {
		jd.maxRounds = n
	}
}

// Run executes the full Judgment Day protocol: judge-a → judge-b → compare → fix → re-judge.
// It advances through the review state machine and returns merged findings with approval status.
func (jd *JudgmentDay) Run(ctx context.Context, tx *review.Transaction) (*JudgmentResult, error) {
	startedAt := time.Now()
	result := &JudgmentResult{StartedAt: startedAt}

	// Transition to judges_confirmed.
	if err := review.Transition(tx.State, review.StateJudgesConfirmed); err != nil {
		return nil, fmt.Errorf("judgment day: %w", err)
	}
	tx.State = review.StateJudgesConfirmed

	// Re-snapshot files for judges to analyze.
	snapshots, err := review.SnapshotFiles("", tx.Files) // projectRoot via current dir
	if err != nil {
		return nil, fmt.Errorf("judgment day snapshot: %w", err)
	}

	// Re-snapshot at current working directory (archiver/reviewer provide ".").
	snapshots, _ = review.SnapshotFiles(".", tx.Files)
	if snapshots == nil {
		snapshots = make([]review.FileSnapshot, 0)
	}

	for round := 1; round <= jd.maxRounds; round++ {
		result.Rounds = round

		// Phase 1: Blind judge analysis.
		judgeA, judgeB, err := jd.runBlindJudges(ctx, tx, snapshots, round)
		if err != nil {
			return nil, fmt.Errorf("round %d judges: %w", round, err)
		}
		result.JudgeAFindings = judgeA
		result.JudgeBFindings = judgeB

		// Phase 2: Compare and merge findings.
		merged, err := CompareFindings(judgeA, judgeB)
		if err != nil {
			return nil, fmt.Errorf("round %d compare: %w", round, err)
		}
		result.MergedFindings = merged

		// Phase 3: Classify merged findings.
		blockers, warnings := classifyFindings(merged)

		if len(blockers) == 0 && len(warnings) == 0 {
			// Both judges agree: no issues. Approve.
			result.Approved = true
			break
		}

		// Phase 4: Apply fixes (only blockers and warnings).
		if len(blockers) > 0 || len(warnings) > 0 {
			fixable := filterFixable(merged)
			if len(fixable) > 0 {
				if _, err := ApplyFixes(ctx, tx, fixable, 85); err != nil {
					return nil, fmt.Errorf("round %d fix: %w", round, err)
				}
			}

			// After fix, re-snapshot to get updated files.
			snapshots, _ = review.SnapshotFiles(".", tx.Files)
			if snapshots == nil {
				snapshots = make([]review.FileSnapshot, 0)
			}
		}

		// If this was the last round and not approved, escalate.
		if round == jd.maxRounds && !result.Approved {
			break
		}
	}

	result.CompletedAt = time.Now()

	// Final state transition.
	if result.Approved {
		if err := review.Transition(tx.State, review.StateApproved); err == nil {
			tx.State = review.StateApproved
		}
	} else {
		if err := review.Transition(tx.State, review.StateEscalated); err == nil {
			tx.State = review.StateEscalated
		}
	}

	return result, nil
}

// runBlindJudges spawns two independent reviewer subagents with different
// focus areas. Each judge sees only the file snapshots — they cannot
// communicate or see each other's findings.
func (jd *JudgmentDay) runBlindJudges(ctx context.Context, tx *review.Transaction, snapshots []review.FileSnapshot, round int) ([]domain.ReviewFinding, []domain.ReviewFinding, error) {
	// Build file content for the judges to analyze.
	fileContent := buildFileContent(snapshots)

	// Judge A: security, data flow, permissions, correctness.
	taskA := domain.SubagentTask{
		ID:          fmt.Sprintf("judge-a-round-%d", round),
		Description: fmt.Sprintf("Analyze these files for SECURITY, DATA FLOW, PERMISSIONS, and CORRECTNESS:\n\n%s", fileContent),
		AllowedTools: []string{
			"file_read",
			"file_list",
			"git_status",
			"git_log",
			"git_diff",
		},
	}

	// Judge B: error handling, edge cases, resource cleanup, resilience.
	taskB := domain.SubagentTask{
		ID:          fmt.Sprintf("judge-b-round-%d", round),
		Description: fmt.Sprintf("Analyze these files for ERROR HANDLING, EDGE CASES, RESOURCE CLEANUP, and RESILIENCE:\n\n%s", fileContent),
		AllowedTools: []string{
			"file_read",
			"file_list",
			"git_status",
			"git_log",
			"git_diff",
		},
	}

	// Spawn both judges independently (blind review).
	// They cannot see each other's context or findings.
	respA, err := jd.spawner.RunLoop(ctx, taskA, judgeASystemPrompt())
	if err != nil {
		return nil, nil, fmt.Errorf("judge-a: %w", err)
	}
	findingsA := parseJudgeFindings(respA.Content, "judge-a")

	respB, err := jd.spawner.RunLoop(ctx, taskB, judgeBSystemPrompt())
	if err != nil {
		return nil, nil, fmt.Errorf("judge-b: %w", err)
	}
	findingsB := parseJudgeFindings(respB.Content, "judge-b")

	return findingsA, findingsB, nil
}

// judgeASystemPrompt returns the system prompt for Judge A.
func judgeASystemPrompt() string {
	return `You are JUDGE-A in the GAIA Judgment Day review protocol.
You are an independent reviewer focused on security, data flow, and correctness.
You have NO knowledge of what judge-b is analyzing or finding.

FOCUS AREAS:
1. SECURITY: injection vulnerabilities, XSS, secrets exposure, weak cryptography,
   missing authentication/authorization checks, unsafe input handling
2. DATA FLOW: nil pointer dereference, race conditions, data corruption,
   unvalidated user input, improper state transitions
3. PERMISSIONS: missing access control, privilege escalation paths,
   unprotected sensitive operations
4. CORRECTNESS: logical errors, off-by-one, broken invariants,
   contract violations, API misuse

For each issue, output a finding line in this exact format:
  FINDING: <SEVERITY> <file>:<line> <message> | SUGGESTION: <fix>

SEVERITY must be one of: BLOCKER, WARNING, SUGGESTION
- BLOCKER: security vulnerability, data loss, crash — MUST fix before merge
- WARNING: potential issue, not immediately dangerous but should be addressed
- SUGGESTION: improvement opportunity, nice to have

If you find NO issues, output: FINDINGS: NONE

Be thorough. You are the security and correctness specialist.`
}

// judgeBSystemPrompt returns the system prompt for Judge B.
func judgeBSystemPrompt() string {
	return `You are JUDGE-B in the GAIA Judgment Day review protocol.
You are an independent reviewer focused on error handling, edge cases, and resilience.
You have NO knowledge of what judge-a is analyzing or finding.

FOCUS AREAS:
1. ERROR HANDLING: swallowed errors, missing error checks, panic-prone paths,
   improper error wrapping, missing context in error messages
2. EDGE CASES: nil/empty inputs, boundary values, concurrent access,
   timeout handling, partial failures
3. RESOURCE CLEANUP: missing defer closes, connection leaks, goroutine leaks,
   context cancellation not honored, memory leaks
4. RESILIENCE: no graceful degradation, missing retries, single points of failure,
   missing observability (logging/metrics) in error paths

For each issue, output a finding line in this exact format:
  FINDING: <SEVERITY> <file>:<line> <message> | SUGGESTION: <fix>

SEVERITY must be one of: BLOCKER, WARNING, SUGGESTION
- BLOCKER: guaranteed crash, resource leak, data loss — MUST fix before merge
- WARNING: potential issue, not immediately dangerous but should be addressed
- SUGGESTION: improvement opportunity, nice to have

If you find NO issues, output: FINDINGS: NONE

Be thorough. You are the resilience and edge-case specialist.`
}

// buildFileContent formats file snapshots for judge analysis.
func buildFileContent(snapshots []review.FileSnapshot) string {
	var content string
	for _, s := range snapshots {
		content += fmt.Sprintf("=== %s ===\n%s\n\n", s.Path, s.Content)
	}
	return content
}

// parseJudgeFindings extracts findings from a judge's LLM response.
func parseJudgeFindings(response, judgeName string) []domain.ReviewFinding {
	// Use the shared parser from the review package.
	return parseJudgeResponse(response, judgeName)
}

// classifyFindings separates findings into blockers, warnings, and suggestions.
func classifyFindings(findings []domain.ReviewFinding) (blockers []domain.ReviewFinding, warnings []domain.ReviewFinding) {
	for _, f := range findings {
		switch f.Severity {
		case "BLOCKER":
			blockers = append(blockers, f)
		case "WARNING":
			warnings = append(warnings, f)
		}
	}
	return
}

// filterFixable returns findings that can be automatically fixed
// (BLOCKER and WARNING; SUGGESTION are skipped by the fix agent).
func filterFixable(findings []domain.ReviewFinding) []domain.ReviewFinding {
	var fixable []domain.ReviewFinding
	for _, f := range findings {
		if f.Severity == "BLOCKER" || f.Severity == "WARNING" {
			fixable = append(fixable, f)
		}
	}
	return fixable
}
