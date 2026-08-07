package core

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

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
// Old messages (beyond keepRecent) are condensed into a single system message, reducing
// token usage on long conversations. Compaction is non-destructive — old messages remain
// in the database but are excluded from subsequent history fetches via compactedTo offset.
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

	// Build compacted summary: drop tool outputs, condense user/assistant messages
	var sb strings.Builder
	sb.WriteString("Compacted conversation history (older messages):\n")
	for _, msg := range oldMsgs {
		// Skip tool role messages entirely — they're the longest and least relevant
		if msg.Role == domain.RoleTool {
			continue
		}

		prefix := strings.ToUpper(string(msg.Role))[:4]
		content := msg.Content

		// Truncate long messages
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
