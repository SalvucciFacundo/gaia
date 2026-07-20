package judgment

import (
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

// CompareFindings merges findings from two judges using a deterministic
// algorithm with fallback for ambiguous cases.
//
// Rules:
//   1. Same file+line+severity → merge messages (deduplication)
//   2. Same file+line, different severity → take HIGHER severity
//   3. Unique findings → kept as-is
func CompareFindings(a, b []domain.ReviewFinding) ([]domain.ReviewFinding, error) {
	// severities ordered from highest to lowest.
	severityRank := map[string]int{
		"BLOCKER":    3,
		"WARNING":    2,
		"SUGGESTION": 1,
	}

	// Build a combined map keyed by file+line.
	type key struct {
		file string
		line int
	}

	merged := make(map[key]*domain.ReviewFinding)
	var result []domain.ReviewFinding

	// Process judge A findings.
	for _, f := range a {
		k := key{file: f.File, line: f.Line}
		existing, ok := merged[k]
		if !ok {
			cp := f
			merged[k] = &cp
			continue
		}

		// Same file+line: resolve conflict.
		resolveConflict(existing, &f, severityRank)
	}

	// Process judge B findings.
	for _, f := range b {
		k := key{file: f.File, line: f.Line}
		existing, ok := merged[k]
		if !ok {
			cp := f
			merged[k] = &cp
			continue
		}

		// Same file+line: resolve conflict.
		resolveConflict(existing, &f, severityRank)
	}

	// Collect results in a stable order.
	for _, v := range merged {
		result = append(result, *v)
	}

	return result, nil
}

// resolveConflict merges two findings at the same file+line.
// - Higher severity wins
// - Messages are merged (combined with " | ")
// - Suggestions are merged or the higher-severity suggestion wins
func resolveConflict(existing *domain.ReviewFinding, incoming *domain.ReviewFinding, severityRank map[string]int) {
	existingRank := severityRank[existing.Severity]
	incomingRank := severityRank[incoming.Severity]

	if incomingRank > existingRank {
		// Incoming has higher severity → use its severity.
		existing.Severity = incoming.Severity
		existing.Lens = existing.Lens + "+" + incoming.Lens
	} else if incomingRank == existingRank {
		// Same severity → merge messages and suggestions.
		if existing.Lens != incoming.Lens {
			existing.Lens = existing.Lens + "+" + incoming.Lens
		}
	}

	// Merge messages if they differ.
	if incoming.Message != "" && !strings.Contains(existing.Message, incoming.Message) {
		existing.Message = existing.Message + " | " + incoming.Message
	}

	// Merge suggestions if they differ.
	if incoming.Suggestion != "" && !strings.Contains(existing.Suggestion, incoming.Suggestion) {
		if existing.Suggestion != "" {
			existing.Suggestion = existing.Suggestion + "; also: " + incoming.Suggestion
		} else {
			existing.Suggestion = incoming.Suggestion
		}
	}
}

// parseJudgeResponse extracts ReviewFinding entries from a judge's LLM response.
// Uses the same format as review.parseFindings (FINDING: severity file:line message).
func parseJudgeResponse(response, judgeName string) []domain.ReviewFinding {
	var findings []domain.ReviewFinding
	lines := strings.Split(response, "\n")

	// Check for FINDINGS: NONE sentinel.
	for _, line := range lines {
		if strings.TrimSpace(line) == "FINDINGS: NONE" {
			return findings
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "FINDING:") {
			// Also try severity-prefixed format.
			if !strings.HasPrefix(trimmed, "BLOCKER:") &&
				!strings.HasPrefix(trimmed, "WARNING:") &&
				!strings.HasPrefix(trimmed, "SUGGESTION:") {
				continue
			}
		}

		// Remove the FINDING prefix.
		content := trimmed
		content = strings.TrimPrefix(content, "FINDING:")
		content = strings.TrimSpace(content)

		// Extract severity.
		severity := "SUGGESTION"
		for _, sev := range []string{"BLOCKER", "WARNING", "SUGGESTION"} {
			if strings.HasPrefix(content, sev+":") || strings.HasPrefix(content, sev+" ") {
				severity = sev
				content = strings.TrimPrefix(content, sev+":")
				content = strings.TrimPrefix(content, sev+" ")
				content = strings.TrimSpace(content)
				break
			}
		}

		// Parse file:line
		file := ""
		lineNum := 0
		message := content
		suggestion := ""

		// Check for "| SUGGESTION:" separator.
		if idx := strings.Index(content, "| SUGGESTION:"); idx >= 0 {
			suggestion = strings.TrimSpace(content[idx+len("| SUGGESTION:"):])
			message = strings.TrimSpace(content[:idx])
		}

		// Parse file:line prefix.
		parts := strings.SplitN(message, " ", 2)
		if len(parts) >= 1 {
			fileLine := parts[0]
			if colonIdx := strings.LastIndex(fileLine, ":"); colonIdx >= 0 {
				file = fileLine[:colonIdx]
				if n, err := fmt.Sscanf(fileLine[colonIdx+1:], "%d", &lineNum); err == nil && n == 1 {
					if len(parts) > 1 {
						message = parts[1]
					}
				} else {
					lineNum = 0
				}
			}
		}

		if message == "" {
			continue
		}

		findings = append(findings, domain.ReviewFinding{
			Lens:       judgeName,
			Severity:   severity,
			File:       file,
			Line:       lineNum,
			Message:    message,
			Suggestion: suggestion,
		})
	}

	return findings
}
