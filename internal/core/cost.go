package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
)

// providerCost holds per-token pricing for common models.
type providerCost struct {
	Input  float64 // $ per 1M input tokens
	Output float64 // $ per 1M output tokens
}

var knownCosts = map[string]providerCost{
	"gpt-4o":              {Input: 2.50, Output: 10.00},
	"gpt-4o-mini":         {Input: 0.15, Output: 0.60},
	"gpt-4-turbo":         {Input: 10.00, Output: 30.00},
	"claude-sonnet-4":     {Input: 3.00, Output: 15.00},
	"claude-haiku-3":      {Input: 0.25, Output: 1.25},
	"claude-opus-4":       {Input: 15.00, Output: 75.00},
	"gemini-2.5-pro":      {Input: 1.25, Output: 10.00},
	"gemini-2.0-flash":    {Input: 0.10, Output: 0.40},
}

// CostEntry records a single LLM call cost.
type CostEntry struct {
	Model     string
	InputTok  int
	OutputTok int
	Cost      float64
	Time      time.Time
}

// CostTracker accumulates LLM call costs across the session.
type CostTracker struct {
	mu     sync.Mutex
	entries []CostEntry
	start   time.Time
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{start: time.Now()}
}

// Record adds a cost entry from a chat call.
func (c *CostTracker) Record(model string, messages []domain.Message, response *domain.Message) {
	inputTok := estimateTokenCount(messages)
	outputTok := estimateSingleToken(response.Content)

	cost := resolveCost(model, inputTok, outputTok)

	c.mu.Lock()
	c.entries = append(c.entries, CostEntry{
		Model:     model,
		InputTok:  inputTok,
		OutputTok: outputTok,
		Cost:      cost,
		Time:      time.Now(),
	})
	c.mu.Unlock()
}

// Summary returns the current session cost summary.
type CostSummary struct {
	TotalCost    float64
	TotalInput   int
	TotalOutput  int
	CallCount    int
	SessionStart time.Time
	Entries      []CostEntry
}

// GetSummary returns the cost summary for the session.
func (c *CostTracker) GetSummary() CostSummary {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := CostSummary{SessionStart: c.start}
	s.Entries = make([]CostEntry, len(c.entries))
	copy(s.Entries, c.entries)

	for _, e := range c.entries {
		s.TotalCost += e.Cost
		s.TotalInput += e.InputTok
		s.TotalOutput += e.OutputTok
		s.CallCount++
	}
	return s
}

// FormatCost formats a cost summary as a human-readable string.
func FormatCost(s CostSummary) string {
	var sb strings.Builder
	duration := time.Since(s.SessionStart).Truncate(time.Second)

	sb.WriteString("── LLM Cost ───────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Session: %s\n", duration))
	sb.WriteString(fmt.Sprintf("  Calls:   %d\n", s.CallCount))
	sb.WriteString(fmt.Sprintf("  Input:   %d tokens\n", s.TotalInput))
	sb.WriteString(fmt.Sprintf("  Output:  %d tokens\n", s.TotalOutput))
	sb.WriteString(fmt.Sprintf("  Total:   $%.4f\n", s.TotalCost))
	sb.WriteString("\n  Last calls:\n")

	// Show last 5 calls
	start := len(s.Entries) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(s.Entries); i++ {
		e := s.Entries[i]
		sb.WriteString(fmt.Sprintf("    %s  in=%d out=%d $%.4f\n",
			e.Model, e.InputTok, e.OutputTok, e.Cost))
	}

	sb.WriteString("────────────────────────────────────────\n")
	return sb.String()
}

// estimateTokenCount counts all tokens in a message slice.
func estimateTokenCount(msgs []domain.Message) int {
	var total int
	for _, m := range msgs {
		total += estimateSingleToken(m.Content)
	}
	return total
}

// estimateSingleToken estimates tokens in text (char/4 heuristic).
func estimateSingleToken(text string) int {
	n := len([]rune(text)) / 4
	if n < 1 && len(text) > 0 {
		return 1
	}
	return n
}

// resolveCost estimates the cost for the given model and token counts.
func resolveCost(model string, inTok, outTok int) float64 {
	cost, ok := knownCosts[strings.ToLower(model)]
	if !ok {
		// Try suffix matching
		for key, c := range knownCosts {
			if strings.Contains(strings.ToLower(model), key) {
				cost = c
				ok = true
				break
			}
		}
	}
	if !ok {
		cost = providerCost{Input: 3.00, Output: 15.00} // default: ~Sonnet pricing
	}
	inCost := float64(inTok) / 1_000_000 * cost.Input
	outCost := float64(outTok) / 1_000_000 * cost.Output
	return inCost + outCost
}

// Ensure domain import is used
var _ = domain.Message{}
