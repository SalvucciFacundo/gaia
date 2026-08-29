package learn

import (
	"context"
	"fmt"
	"strings"
)

// ExtractLearnings parses a message or task output for structured learning/discovery sections.
// Supported headers: "## Key Learnings", "## Discoveries", "Observations:", "## Learned", "Learned:".
func ExtractLearnings(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var inLearningSection bool
	var learnings []string
	seen := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Check for learning section headers
		if strings.HasPrefix(lower, "## key learnings") ||
			strings.HasPrefix(lower, "## discoveries") ||
			strings.HasPrefix(lower, "## learned") ||
			strings.HasPrefix(lower, "observations:") ||
			strings.HasPrefix(lower, "learned:") {
			inLearningSection = true
			// Check if there's text on the same line after colon
			if idx := strings.Index(trimmed, ":"); idx != -1 && idx < len(trimmed)-1 {
				afterColon := strings.TrimSpace(trimmed[idx+1:])
				if afterColon != "" && !strings.EqualFold(afterColon, "none") {
					if !seen[afterColon] {
						seen[afterColon] = true
						learnings = append(learnings, afterColon)
					}
				}
			}
			continue
		}

		// Check for exit from learning section (new markdown header or SDD envelope section header)
		if inLearningSection {
			if strings.HasPrefix(trimmed, "#") ||
				strings.HasPrefix(lower, "risks:") ||
				strings.HasPrefix(lower, "status:") ||
				strings.HasPrefix(lower, "executivesummary:") ||
				strings.HasPrefix(lower, "artifacts:") ||
				strings.HasPrefix(lower, "nextrecommended:") ||
				strings.HasPrefix(lower, "skillresolution:") {
				inLearningSection = false
			}
		}

		// Collect bullet points or numbered items inside learning section
		if inLearningSection && trimmed != "" {
			item := strings.TrimLeft(trimmed, "-*0123456789. ")
			item = strings.TrimSpace(item)
			if item != "" && !strings.EqualFold(item, "none") && !seen[item] {
				seen[item] = true
				learnings = append(learnings, item)
			}
		}
	}

	return learnings
}

// PassiveLearner automatically extracts and records insights from subagent executions.
type PassiveLearner struct {
	loop *LearningLoop
}

// NewPassiveLearner creates a new PassiveLearner.
func NewPassiveLearner(loop *LearningLoop) *PassiveLearner {
	return &PassiveLearner{loop: loop}
}

// CaptureExtract scans output, extracts learnings, and increments the learning loop.
func (p *PassiveLearner) CaptureExtract(ctx context.Context, subagentName, outputContent string) []string {
	insights := ExtractLearnings(outputContent)
	if len(insights) > 0 && p.loop != nil {
		p.loop.RecordExecution(subagentName)
	}
	return insights
}

// FormatLearningsPrompt formats a list of insights into markdown bullet points.
func FormatLearningsPrompt(insights []string) string {
	if len(insights) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Domain Insights (Learned from past runs):\n")
	for _, ins := range insights {
		b.WriteString(fmt.Sprintf("- %s\n", ins))
	}
	return b.String()
}
