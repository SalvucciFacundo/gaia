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
	NextSteps     []string `json:"next_steps"`
	RelevantFiles []string `json:"relevant_files"`
}

// BuildSessionSummary analyzes conversation messages to build a structured session summary.
func BuildSessionSummary(messages []domain.Message) SessionSummary {
	var summary SessionSummary
	seenFiles := make(map[string]bool)

	for _, msg := range messages {
		content := msg.Content
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Extract file references
			if strings.Contains(trimmed, ".go") || strings.Contains(trimmed, ".md") || strings.Contains(trimmed, ".yaml") {
				for _, word := range strings.Fields(trimmed) {
					cleaned := strings.Trim(word, "`,:\"'()")
					if (strings.HasSuffix(cleaned, ".go") || strings.HasSuffix(cleaned, ".md") || strings.HasSuffix(cleaned, ".yaml")) && !seenFiles[cleaned] {
						seenFiles[cleaned] = true
						summary.RelevantFiles = append(summary.RelevantFiles, cleaned)
					}
				}
			}

			// Extract goal from the earliest user message
			if summary.Goal == "" && msg.Role == domain.RoleUser && trimmed != "" {
				summary.Goal = trimmed
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
	if len(summary.RelevantFiles) > 0 {
		sb.WriteString(fmt.Sprintf("Relevant Files: %s\n", strings.Join(summary.RelevantFiles, ", ")))
	}
	if len(summary.Accomplished) > 0 {
		sb.WriteString("Accomplished:\n")
		for _, acc := range summary.Accomplished {
			sb.WriteString(fmt.Sprintf("- %s\n", acc))
		}
	}
	return sb.String()
}

// recallKnowledgeGraph searches the knowledge graph for facts relevant to text.
// Returns a slice of formatted fact strings to append to system prompt.
// Uses keyword search; falls back to recent facts if search yields nothing.
func (b *Brain) recallKnowledgeGraph(ctx context.Context, text string) []string {
	if !b.kgEnabled || b.kgStore == nil {
		return nil
	}

	// Search for facts matching the query — use keywords from the text
	facts, err := b.kgStore.SearchFacts(ctx, text)
	if err != nil || len(facts) == 0 {
		// Fall back to recent facts if search yields nothing
		recent, recentErr := b.kgStore.GetRecentFacts(ctx, 5)
		if recentErr != nil || len(recent) == 0 {
			return nil
		}
		facts = recent
	}

	result := make([]string, 0, len(facts))
	seen := make(map[string]bool)
	for _, f := range facts {
		key := f.Topic + "/" + f.Concept + "/" + f.Fact
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, fmt.Sprintf("[%s/%s] %s (by %s)", f.Topic, f.Concept, f.Fact, f.SourceAgent))
	}
	return result
}

// compactHistory compacts stale messages when the history exceeds the compaction threshold.
// Old messages (beyond keepRecent) are condensed into a structured session summary message, reducing
// token usage on long conversations while preventing post-compaction amnesia.
func (b *Brain) compactHistory(ctx context.Context) error {
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
	if compactCount <= b.compactedTo {
		return nil // already compacted up to this point
	}

	// Fetch un-compacted old messages
	oldCount := compactCount - b.compactedTo
	oldMsgs, err := b.repo.GetHistoryFrom(ctx, oldCount, b.compactedTo)
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
		if msg.Role == domain.RoleTool {
			continue
		}

		prefix := strings.ToUpper(string(msg.Role))[:4]
		content := msg.Content

		if len(content) > 300 {
			content = content[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", prefix, content))
	}

	b.compactedTo = compactCount

	// Save compacted summary message
	compactMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	return b.repo.SaveMessage(ctx, compactMsg)
}
