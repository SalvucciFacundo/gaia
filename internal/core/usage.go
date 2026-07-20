package core

import (
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

var modelContextWindows = map[string]int{
	"gpt-4o":               128000,
	"gpt-4o-mini":          128000,
	"gpt-4-turbo":          128000,
	"claude-sonnet-4":      200000,
	"claude-haiku-3":       200000,
	"claude-opus-4":        200000,
	"claude-3-5-sonnet":    200000,
	"claude-3-haiku":       200000,
	"gemini-2.5-pro":       1000000,
	"gemini-2.0-flash":     1000000,
	"gemini-1.5-pro":       2000000,
	"llama3":               8192,
	"llama3.1":             128000,
	"mistral":              8192,
	"mixtral":              32768,
	"codellama":            16384,
	"qwen2.5":              32768,
	"deepseek-coder":       16384,
	"o1":                   200000,
	"o3-mini":              200000,
}

type UsageCategory struct {
	Label  string
	Tokens int
}

type UsageStats struct {
	Provider     string
	Model        string
	ContextLimit int
	Categories   []UsageCategory
	TotalTokens  int
	BudgetLimit  int
}

func estimateTokens(text string) int {
	return len([]rune(text)) / 4
}

func (b *Brain) GetUsageStats(toolNames []string, skills []string, kgFacts []string, history []domain.Message) UsageStats {
	stats := UsageStats{
		Provider:     b.providerName,
		Model:        b.modelName,
		ContextLimit: resolveContextLimit(b.modelName),
		BudgetLimit:  b.budget.MaxIterations,
	}

	systemTokens := estimateTokens("GAIA programming agent with subagents, tools, and memory")
	stats.Categories = append(stats.Categories, UsageCategory{Label: "System Prompt", Tokens: systemTokens})

	var toolTok int
	for _, n := range toolNames {
		toolTok += estimateTokens(n) + 30
	}
	stats.Categories = append(stats.Categories, UsageCategory{Label: "Tools", Tokens: toolTok})

	var skillTok int
	for _, s := range skills {
		skillTok += estimateTokens(s)
	}
	stats.Categories = append(stats.Categories, UsageCategory{Label: "Skills", Tokens: skillTok})

	var kgTok int
	for _, f := range kgFacts {
		kgTok += estimateTokens(f)
	}
	stats.Categories = append(stats.Categories, UsageCategory{Label: "KG Context", Tokens: kgTok})

	var convTok int
	for _, msg := range history {
		convTok += estimateTokens(fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	stats.Categories = append(stats.Categories, UsageCategory{Label: "Conversation", Tokens: convTok})

	for _, c := range stats.Categories {
		stats.TotalTokens += c.Tokens
	}
	return stats
}

func FormatUsage(stats UsageStats) string {
	var sb strings.Builder
	sb.WriteString("── Context Usage ──────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Model:    %s / %s\n", stats.Provider, stats.Model))
	sb.WriteString(fmt.Sprintf("  Window:   %d tokens\n", stats.ContextLimit))
	sb.WriteString(fmt.Sprintf("  Budget:   %d max iterations\n", stats.BudgetLimit))

	pct := float64(stats.TotalTokens) * 100 / float64(stats.ContextLimit)
	barLen := 30
	filled := int(pct * float64(barLen) / 100)
	if filled > barLen {
		filled = barLen
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	sb.WriteString(fmt.Sprintf("  Usage:    %s %d / %d tok (%.0f%%)\n", bar, stats.TotalTokens, stats.ContextLimit, pct))

	sb.WriteString("\n  Breakdown:\n")
	for _, c := range stats.Categories {
		if c.Tokens == 0 {
			continue
		}
		catPct := float64(c.Tokens) * 100 / float64(stats.ContextLimit)
		sb.WriteString(fmt.Sprintf("    %-20s %6d tok  %.0f%%\n", c.Label, c.Tokens, catPct))
	}
	sb.WriteString("────────────────────────────────────────\n")
	return sb.String()
}

func resolveContextLimit(model string) int {
	lower := strings.ToLower(model)
	if limit, ok := modelContextWindows[lower]; ok {
		return limit
	}
	for key, limit := range modelContextWindows {
		if strings.Contains(lower, key) {
			return limit
		}
	}
	return 128000
}
