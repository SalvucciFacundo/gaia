package core

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

// SessionSummary holds structured session context for post-compaction rehydration.
type SessionSummary struct {
	Goal          string   `json:"goal"`
	Instructions  []string `json:"instructions"`
	Discoveries   []string `json:"discoveries"`
	Accomplished  []string `json:"accomplished"`
	KeyDecisions  []string `json:"key_decisions"`
	ModifiedFiles []string `json:"modified_files"`
	RelevantFiles []string `json:"relevant_files"`
	NextSteps     []string `json:"next_steps"`
}

// BuildSessionSummary analyzes conversation messages to build a structured session summary.
func BuildSessionSummary(messages []domain.Message) SessionSummary {
	var summary SessionSummary
	seenFiles := make(map[string]bool)
	seenModified := make(map[string]bool)
	seenDecisions := make(map[string]bool)
	seenAccomplished := make(map[string]bool)
	seenInstructions := make(map[string]bool)

	for _, msg := range messages {
		content := msg.Content

		// Inspect tool calls for direct mutations
		for _, tc := range msg.ToolCalls {
			toolLower := strings.ToLower(tc.Name)
			switch toolLower {
			case "file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch":
				if p, ok := tc.Arguments["path"].(string); ok && p != "" && !seenModified[p] {
					seenModified[p] = true
					summary.ModifiedFiles = append(summary.ModifiedFiles, p)
				} else if f, ok := tc.Arguments["file"].(string); ok && f != "" && !seenModified[f] {
					seenModified[f] = true
					summary.ModifiedFiles = append(summary.ModifiedFiles, f)
				}
			}
		}

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Extract file references
			if strings.Contains(trimmed, ".go") || strings.Contains(trimmed, ".md") || strings.Contains(trimmed, ".yaml") || strings.Contains(trimmed, ".json") || strings.Contains(trimmed, ".ts") || strings.Contains(trimmed, ".rs") {
				for _, word := range strings.Fields(trimmed) {
					cleaned := strings.Trim(word, "`,:\"'()[]{}*")
					if (strings.HasSuffix(cleaned, ".go") || strings.HasSuffix(cleaned, ".md") || strings.HasSuffix(cleaned, ".yaml") || strings.HasSuffix(cleaned, ".json") || strings.HasSuffix(cleaned, ".ts") || strings.HasSuffix(cleaned, ".rs")) && !seenFiles[cleaned] {
						seenFiles[cleaned] = true
						summary.RelevantFiles = append(summary.RelevantFiles, cleaned)
					}
				}
			}

			// Extract goal from the earliest user message
			if summary.Goal == "" && msg.Role == domain.RoleUser && trimmed != "" {
				summary.Goal = trimmed
			}

			// Extract user instructions or constraints
			if msg.Role == domain.RoleUser {
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "must ") || strings.HasPrefix(lower, "never ") || strings.HasPrefix(lower, "always ") || strings.HasPrefix(lower, "don't ") || strings.HasPrefix(lower, "do not ") {
					if !seenInstructions[trimmed] && len(trimmed) < 200 {
						seenInstructions[trimmed] = true
						summary.Instructions = append(summary.Instructions, trimmed)
					}
				}
			}

			// Extract architectural decisions and accomplishments from assistant messages
			if msg.Role == domain.RoleAssistant {
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "decided to") || strings.HasPrefix(lower, "decision:") || strings.HasPrefix(lower, "we chose") {
					dec := strings.TrimPrefix(trimmed, "- ")
					if !seenDecisions[dec] && len(dec) < 200 {
						seenDecisions[dec] = true
						summary.KeyDecisions = append(summary.KeyDecisions, dec)
					}
				}
				if strings.HasPrefix(lower, "implemented") || strings.HasPrefix(lower, "fixed") || strings.HasPrefix(lower, "created") || strings.HasPrefix(lower, "refactored") {
					acc := strings.TrimPrefix(trimmed, "- ")
					if !seenAccomplished[acc] && len(acc) < 200 {
						seenAccomplished[acc] = true
						summary.Accomplished = append(summary.Accomplished, acc)
					}
				}
			}
		}
	}

	return summary
}

// FormatRehydrationPrompt builds the system injection string to rehydrate post-compaction context.
func FormatRehydrationPrompt(summary SessionSummary) string {
	var sb strings.Builder
	sb.WriteString("[REHYDRATED SESSION CONTEXT AFTER COMPACTION]\n")
	if summary.Goal != "" {
		sb.WriteString(fmt.Sprintf("Active Goal: %s\n", summary.Goal))
	}
	if len(summary.Instructions) > 0 {
		sb.WriteString("User Constraints & Instructions:\n")
		for _, inst := range summary.Instructions {
			sb.WriteString(fmt.Sprintf("- %s\n", inst))
		}
	}
	if len(summary.KeyDecisions) > 0 {
		sb.WriteString("Key Architectural Decisions:\n")
		for _, dec := range summary.KeyDecisions {
			sb.WriteString(fmt.Sprintf("- %s\n", dec))
		}
	}
	if len(summary.ModifiedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("Modified Files: %s\n", strings.Join(summary.ModifiedFiles, ", ")))
	}
	if len(summary.Accomplished) > 0 {
		sb.WriteString("Accomplished Items:\n")
		for _, acc := range summary.Accomplished {
			sb.WriteString(fmt.Sprintf("- %s\n", acc))
		}
	}
	if len(summary.RelevantFiles) > 0 {
		sb.WriteString(fmt.Sprintf("Relevant Files Referenced: %s\n", strings.Join(summary.RelevantFiles, ", ")))
	}
	return sb.String()
}

// FormatSessionSummaryMarkdown formats a structured markdown session summary from SessionSummary.
func FormatSessionSummaryMarkdown(summary SessionSummary) string {
	var sb strings.Builder
	sb.WriteString("## Goal\n")
	if summary.Goal != "" {
		sb.WriteString(summary.Goal)
	} else {
		sb.WriteString("Autonomous coding session")
	}
	sb.WriteString("\n\n## Instructions\n")
	if len(summary.Instructions) > 0 {
		for _, inst := range summary.Instructions {
			sb.WriteString(fmt.Sprintf("- %s\n", inst))
		}
	} else {
		sb.WriteString("- None\n")
	}
	sb.WriteString("\n## Discoveries\n")
	if len(summary.Discoveries) > 0 {
		for _, disc := range summary.Discoveries {
			sb.WriteString(fmt.Sprintf("- %s\n", disc))
		}
	} else {
		sb.WriteString("- None\n")
	}
	sb.WriteString("\n## Accomplished\n")
	if len(summary.Accomplished) > 0 {
		for _, acc := range summary.Accomplished {
			sb.WriteString(fmt.Sprintf("- %s\n", acc))
		}
	} else {
		sb.WriteString("- None\n")
	}
	sb.WriteString("\n## Next Steps\n")
	if len(summary.NextSteps) > 0 {
		for _, next := range summary.NextSteps {
			sb.WriteString(fmt.Sprintf("- %s\n", next))
		}
	} else {
		sb.WriteString("- None\n")
	}
	sb.WriteString("\n## Relevant Files\n")
	if len(summary.RelevantFiles) > 0 {
		for _, f := range summary.RelevantFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	} else {
		sb.WriteString("- None\n")
	}
	return sb.String()
}

// compactHistory compacts stale messages when the history exceeds the compaction threshold.
// Old messages (beyond keepRecent) are condensed into a structured session summary message, reducing
// token usage on long conversations while preventing post-compaction amnesia.
func (b *Brain) compactHistory(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if b.budget.CompactionThreshold <= 0 {
		return nil // disabled
	}

	count, err := b.repo.GetMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("get message count for compaction: %w", err)
	}

	keepRecent := b.budget.KeepRecentMessages
	if keepRecent <= 0 {
		keepRecent = 20
	}

	// Only compact when history exceeds the threshold
	if count < b.budget.CompactionThreshold {
		return nil
	}

	compactCount := count - keepRecent
	b.stateMu.RLock()
	currentCompactedTo := b.compactedTo
	b.stateMu.RUnlock()

	if compactCount <= currentCompactedTo {
		return nil // already compacted up to this point
	}

	// Fetch un-compacted old messages
	oldCount := compactCount - currentCompactedTo
	oldMsgs, err := b.repo.GetHistoryFrom(ctx, oldCount, currentCompactedTo)
	if err != nil {
		return fmt.Errorf("fetch old messages for compaction: %w", err)
	}
	if len(oldMsgs) == 0 {
		return nil
	}

	// Build structured session summary from old messages
	summary := BuildSessionSummary(oldMsgs)

	var sb strings.Builder
	sb.WriteString(FormatRehydrationPrompt(summary))
	sb.WriteString("\nCompacted conversation history (older messages):\n")
	for _, msg := range oldMsgs {
		prefix := strings.ToUpper(string(msg.Role))
		if len(prefix) > 4 {
			prefix = prefix[:4]
		}
		content := strings.TrimSpace(msg.Content)

		// For tool messages, keep them extra concise to avoid bloating compaction summaries
		if msg.Role == domain.RoleTool {
			firstLine := content
			if idx := strings.Index(firstLine, "\n"); idx != -1 {
				firstLine = firstLine[:idx]
			}
			if len(firstLine) > 120 {
				firstLine = firstLine[:120] + "..."
			}
			sb.WriteString(fmt.Sprintf("[TOOL RESULT] %s\n", firstLine))
			continue
		}

		// For assistant messages with tool calls, note the tools called
		if len(msg.ToolCalls) > 0 {
			toolNames := make([]string, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				toolNames = append(toolNames, tc.Name)
			}
			sb.WriteString(fmt.Sprintf("[TOOL CALLS] %s\n", strings.Join(toolNames, ", ")))
		}

		if content != "" {
			if len(content) > 250 {
				content = content[:250] + "..."
			}
			sb.WriteString(fmt.Sprintf("[%s] %s\n", prefix, content))
		}
	}

	b.stateMu.Lock()
	b.compactedTo = compactCount
	b.stateMu.Unlock()

	// Save compacted summary message
	compactMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	return b.repo.SaveMessage(ctx, compactMsg)
}
