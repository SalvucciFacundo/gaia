// Package learn provides the per-subagent learning loop:
// counter-based nudges, session summary generation, and skill creation prompts.
package learn

import (
	"fmt"
	"strings"
	"sync"
)

// LearningLoop tracks subagent executions and generates reflection nudges.
// After N executions (default 5), it suggests a memory save and pattern reflection.
// The nudge is non-blocking — subagents may decline.
type LearningLoop struct {
	mu        sync.Mutex
	counters  map[string]int
	threshold int
}

// NewLearningLoop creates a new learning loop with the given nudge threshold.
// If threshold <= 0, defaults to 5.
func NewLearningLoop(threshold int) *LearningLoop {
	if threshold <= 0 {
		threshold = 5
	}
	return &LearningLoop{
		counters:  make(map[string]int),
		threshold: threshold,
	}
}

// RecordExecution increments the execution counter for a subagent.
// Returns true if the nudge threshold has been reached (exactly at threshold, not above).
func (l *LearningLoop) RecordExecution(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counters[name]++
	return l.counters[name] == l.threshold
}

// ResetCounter resets the counter for a subagent after a nudge is acknowledged.
func (l *LearningLoop) ResetCounter(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counters[name] = 0
}

// Count returns the current execution count for a subagent.
func (l *LearningLoop) Count(name string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counters[name]
}

// NudgePrompt generates a reflection prompt when the nudge threshold is reached.
// It asks the subagent to consider memory save, pattern recognition, and skill creation.
func (l *LearningLoop) NudgePrompt(name string) string {
	count := l.Count(name)
	return fmt.Sprintf(`LEARNING REFLECTION (execution #%d):
You have completed %d executions. Before proceeding, consider:
1. What patterns have you observed across these tasks?
2. Are there reusable insights worth saving to your Engram memory (mem_save)?
3. Should any repeatable patterns become a skill via skill-creator?
4. Is there cross-domain knowledge for the shared knowledge graph?

This reflection is OPTIONAL — you may continue without saving.`, count, count)
}

// SessionSummaryPrompt generates a domain-specific session summary prompt
// for the subagent to save to its Engram namespace.
func (l *LearningLoop) SessionSummaryPrompt(name, goal string, discoveries, accomplished []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Goal\n%s\n\n", goal))

	if len(discoveries) > 0 {
		b.WriteString("## Discoveries\n")
		for _, d := range discoveries {
			b.WriteString(fmt.Sprintf("- %s\n", d))
		}
		b.WriteString("\n")
	}

	if len(accomplished) > 0 {
		b.WriteString("## Accomplished\n")
		for _, a := range accomplished {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// SkillCheckPrompt generates a prompt asking whether any observed patterns
// warrant skill creation via the skill-creator format.
func (l *LearningLoop) SkillCheckPrompt(name string, patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("SKILL CHECK — the following patterns were observed:\n")
	for _, p := range patterns {
		b.WriteString(fmt.Sprintf("- %s\n", p))
	}
	b.WriteString("\nConsider: should any of these become a reusable skill? Use skill-creator if yes.\n")
	return b.String()
}

// Threshold returns the current nudge threshold.
func (l *LearningLoop) Threshold() int {
	return l.threshold
}
