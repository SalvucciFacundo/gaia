package judgment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/review"
)

// FixResult holds the outcome of a fix round.
type FixResult struct {
	AppliedFixCount int                    `json:"applied_fix_count"`
	SkippedCount    int                    `json:"skipped_count"`
	BudgetUsed      int                    `json:"budget_used"`
	Changes         []FixChange            `json:"changes,omitempty"`
}

// FixChange records a single surgical correction applied to a file.
type FixChange struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Finding string `json:"finding"`
}

// ApplyFixes applies surgical corrections for the given findings.
// Only BLOCKER and WARNING findings are fixed; SUGGESTION are skipped.
// The correction budget limits the total number of token changes (default 85).
// Fixes are applied directly to the filesystem — the fix agent has write access.
func ApplyFixes(ctx context.Context, tx *review.Transaction, findings []domain.ReviewFinding, budget int) (*FixResult, error) {
	if len(findings) == 0 {
		return &FixResult{}, nil
	}

	// Transition to fixing state.
	if tx != nil {
		if tx.State != review.StateFixRequired && tx.State != review.StateFixing {
			if err := review.Transition(tx.State, review.StateFixRequired); err == nil {
				tx.State = review.StateFixRequired
			}
			if err := review.Transition(tx.State, review.StateFixing); err == nil {
				tx.State = review.StateFixing
			}
		}
	}

	result := &FixResult{}
	budgetRemaining := budget

	for _, finding := range findings {
		// Skip SUGGESTION findings — only fix BLOCKER and WARNING.
		if finding.Severity == "SUGGESTION" {
			result.SkippedCount++
			continue
		}

		if budgetRemaining <= 0 {
			result.SkippedCount++
			continue
		}

		// Apply the fix if the file exists and we can determine the correction.
		fixChange, tokensUsed, err := applySingleFix(finding, budgetRemaining)
		if err != nil {
			// Non-fatal: continue with remaining findings.
			continue
		}

		if fixChange != nil {
			result.Changes = append(result.Changes, *fixChange)
			result.AppliedFixCount++
			budgetRemaining -= tokensUsed
		} else {
			result.SkippedCount++
		}
	}

	result.BudgetUsed = budget - budgetRemaining

	// Transition back through state machine for re-judging.
	if tx != nil {
		if err := review.Transition(tx.State, review.StateFixValidating); err == nil {
			tx.State = review.StateFixValidating
		}
		if err := review.Transition(tx.State, review.StateEvidenceClassified); err == nil {
			tx.State = review.StateEvidenceClassified
		}
	}

	return result, nil
}

// applySingleFix applies one surgical correction for a single finding.
// It reads the target file, applies the suggested fix, and writes back.
// The fix is applied as a line replacement when a line number is specified,
// or as a file-level change otherwise.
func applySingleFix(finding domain.ReviewFinding, budget int) (*FixChange, int, error) {
	if finding.File == "" || finding.Suggestion == "" {
		return nil, 0, nil
	}

	// Read the file.
	data, err := os.ReadFile(finding.File)
	if err != nil {
		// Try resolving from current directory.
		data, err = os.ReadFile(filepath.Base(finding.File))
		if err != nil {
			return nil, 0, fmt.Errorf("fix %s: %w", finding.File, err)
		}
	}

	original := string(data)
	lines := strings.Split(original, "\n")

	// Estimate token cost (rough: 4 chars ≈ 1 token).
	tokensUsed := (len(finding.Suggestion) + 3) / 4
	if tokensUsed > budget {
		return nil, 0, nil // Not enough budget.
	}

	var before string
	var after string

	if finding.Line > 0 && finding.Line <= len(lines) {
		// Line-specific fix: replace the target line with the suggestion.
		before = lines[finding.Line-1]
		lines[finding.Line-1] = findReplacement(before, finding.Suggestion)
		after = lines[finding.Line-1]
	} else {
		// File-level: append the suggestion as a comment.
		before = "(file-level)"
		suggestionLine := "// FIX: " + finding.Suggestion
		lines = append(lines, suggestionLine)
		after = suggestionLine
	}

	fixed := strings.Join(lines, "\n")

	// Write back.
	if err := os.WriteFile(finding.File, []byte(fixed), 0644); err != nil {
		return nil, 0, fmt.Errorf("write fix %s: %w", finding.File, err)
	}

	return &FixChange{
		File:    finding.File,
		Line:    finding.Line,
		Before:  before,
		After:   after,
		Finding: finding.Message,
	}, tokensUsed, nil
}

// findReplacement derives the replacement text from a suggestion.
// It extracts concrete code changes from natural-language suggestions.
func findReplacement(original, suggestion string) string {
	// If the suggestion is pure code (no explanation), use it directly.
	if !strings.Contains(suggestion, "should") &&
		!strings.Contains(suggestion, "consider") &&
		!strings.Contains(suggestion, "add") &&
		!strings.Contains(suggestion, "remove") &&
		!strings.Contains(suggestion, "replace") {
		return suggestion
	}

	// For natural-language suggestions, extract the replacement pattern.
	// Look for patterns like "replace X with Y" or "change to Y".
	suggestion = strings.TrimSpace(suggestion)

	if strings.Contains(suggestion, "replace") && strings.Contains(suggestion, "with") {
		parts := strings.SplitN(suggestion, "with", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	if strings.Contains(suggestion, "change to") {
		parts := strings.SplitN(suggestion, "change to", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	if strings.Contains(suggestion, "should be") {
		parts := strings.SplitN(suggestion, "should be", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Fallback: return the original line unchanged.
	return original
}
