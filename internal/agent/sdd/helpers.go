package sdd

import (
	"strings"

	"gaia/internal/core/domain"
)

// parseSDDResult interprets the LLM response and extracts a structured
// SubagentResult envelope. It uses a simple heuristic: each line prefixed
// with a known section name is captured.
func parseSDDResult(resp *domain.Message, defaultNext string) *domain.SubagentResult {
	if resp == nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "No response from LLM.",
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := &domain.SubagentResult{
		Status:          domain.SubagentSuccess,
		NextRecommended: defaultNext,
		SkillResolution: "none",
	}

	content := resp.Content
	lines := strings.Split(content, "\n")

	var currentSection string
	var summaryLines []string
	var artifactLines []string
	var riskLines []string
	var observationLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowerTrimmed := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lowerTrimmed, "status:"):
			currentSection = "status"
			val := strings.TrimPrefix(lowerTrimmed, "status:")
			val = strings.TrimSpace(val)
			switch strings.ToLower(val) {
			case "partial":
				result.Status = domain.SubagentPartial
			case "blocked":
				result.Status = domain.SubagentBlocked
			default:
				result.Status = domain.SubagentSuccess
			}

		case strings.HasPrefix(lowerTrimmed, "executivesummary:") ||
			strings.HasPrefix(lowerTrimmed, "executive summary:"):
			currentSection = "summary"
			val := trimAfter(lowerTrimmed, "executivesummary:", "executive summary:")
			summaryLines = append(summaryLines, val)

		case strings.HasPrefix(lowerTrimmed, "artifacts:"):
			currentSection = "artifacts"
			val := trimAfter(lowerTrimmed, "artifacts:")
			if val != "" {
				artifactLines = append(artifactLines, val)
			}

		case strings.HasPrefix(lowerTrimmed, "observations:"):
			currentSection = "observations"
			val := trimAfter(lowerTrimmed, "observations:")
			if val != "" {
				observationLines = append(observationLines, val)
			}

		case strings.HasPrefix(lowerTrimmed, "nextrecommended:") ||
			strings.HasPrefix(lowerTrimmed, "next recommended:"):
			currentSection = "next"
			val := trimAfter(lowerTrimmed, "nextrecommended:", "next recommended:")
			if v := strings.TrimSpace(val); v != "" {
				result.NextRecommended = v
			}

		case strings.HasPrefix(lowerTrimmed, "risks:"):
			currentSection = "risks"
			val := trimAfter(lowerTrimmed, "risks:")
			if val != "" && !strings.EqualFold(strings.TrimSpace(val), "none") {
				riskLines = append(riskLines, val)
			}

		case strings.HasPrefix(lowerTrimmed, "skillresolution:") ||
			strings.HasPrefix(lowerTrimmed, "skill resolution:"):
			currentSection = "skills"
			val := trimAfter(lowerTrimmed, "skillresolution:", "skill resolution:")
			if v := strings.TrimSpace(val); v != "" {
				result.SkillResolution = v
			}

		default:
			// Accumulate into current section
			if trimmed != "" {
				switch currentSection {
				case "summary":
					summaryLines = append(summaryLines, trimmed)
				case "artifacts":
					artifactLines = append(artifactLines, trimmed)
				case "observations":
					observationLines = append(observationLines, trimmed)
				case "risks":
					if !strings.EqualFold(trimmed, "none") {
						riskLines = append(riskLines, trimmed)
					}
				}
			}
		}
	}

	// Build summary from parsed lines; fall back to full content.
	result.Summary = strings.Join(summaryLines, " ")
	if result.Summary == "" {
		result.Summary = firstNLines(content, 3)
	}

	// Deduplicate and clean artifacts
	seen := make(map[string]bool)
	for _, a := range artifactLines {
		a = strings.TrimPrefix(a, "-")
		a = strings.TrimPrefix(a, "*")
		a = strings.TrimSpace(a)
		if a != "" && !seen[a] {
			result.Artifacts = append(result.Artifacts, a)
			seen[a] = true
		}
	}

	// If no explicit artifacts but we have observations, note them.
	if len(result.Artifacts) == 0 && len(observationLines) > 0 {
		result.Artifacts = append(result.Artifacts, "structured-observations")
	}

	// Clean risks
	for _, r := range riskLines {
		r = strings.TrimPrefix(r, "-")
		r = strings.TrimPrefix(r, "*")
		r = strings.TrimSpace(r)
		if r != "" && !strings.EqualFold(r, "none") {
			result.Risks = append(result.Risks, r)
		}
	}

	return result
}

// trimAfter strips the prefix from the line. Handles multiple prefix variants.
func trimAfter(line string, prefixes ...string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(line), p) {
			return strings.TrimSpace(line[len(p):])
		}
	}
	return strings.TrimSpace(line)
}

// firstNLines truncates text to the first n lines, joined by spaces.
func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}
