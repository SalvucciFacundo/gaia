package learn

import (
	"strings"
	"testing"
)

// TestNewLearningLoop_DefaultThreshold verifies the default threshold is 5.
func TestNewLearningLoop_DefaultThreshold(t *testing.T) {
	l := NewLearningLoop(0)
	if l.Threshold() != 5 {
		t.Errorf("default threshold: want 5, got %d", l.Threshold())
	}
}

// TestNewLearningLoop_CustomThreshold verifies custom thresholds.
func TestNewLearningLoop_CustomThreshold(t *testing.T) {
	l := NewLearningLoop(10)
	if l.Threshold() != 10 {
		t.Errorf("custom threshold: want 10, got %d", l.Threshold())
	}
}

// TestRecordExecution_BelowThreshold verifies that below-threshold
// executions do not trigger a nudge.
func TestRecordExecution_BelowThreshold(t *testing.T) {
	l := NewLearningLoop(5)

	// 3 executions — no nudge
	for i := 0; i < 3; i++ {
		nudge := l.RecordExecution("explorer")
		if nudge {
			t.Errorf("execution %d should not trigger nudge (threshold=5)", i+1)
		}
	}
	if l.Count("explorer") != 3 {
		t.Errorf("count: want 3, got %d", l.Count("explorer"))
	}
}

// TestRecordExecution_AtThreshold verifies that exactly at threshold
// the nudge triggers.
func TestRecordExecution_AtThreshold(t *testing.T) {
	l := NewLearningLoop(3)

	// First 2 should not nudge
	for i := 0; i < 2; i++ {
		if l.RecordExecution("proposer") {
			t.Errorf("execution %d should not trigger nudge", i+1)
		}
	}

	// 3rd execution should trigger
	if !l.RecordExecution("proposer") {
		t.Error("3rd execution should trigger nudge at threshold=3")
	}
}

// TestRecordExecution_AboveThreshold verifies no double-nudge above threshold.
func TestRecordExecution_AboveThreshold(t *testing.T) {
	l := NewLearningLoop(3)

	// Hit threshold
	for i := 0; i < 3; i++ {
		l.RecordExecution("specifier")
	}

	// 4th execution should NOT trigger again (only at exact threshold)
	if l.RecordExecution("specifier") {
		t.Error("4th execution should not trigger nudge again")
	}
	if l.Count("specifier") != 4 {
		t.Errorf("count: want 4, got %d", l.Count("specifier"))
	}
}

// TestResetCounter verifies counter reset works.
func TestResetCounter(t *testing.T) {
	l := NewLearningLoop(5)

	l.RecordExecution("verifier")
	l.RecordExecution("verifier")
	if l.Count("verifier") != 2 {
		t.Errorf("pre-reset count: want 2, got %d", l.Count("verifier"))
	}

	l.ResetCounter("verifier")
	if l.Count("verifier") != 0 {
		t.Errorf("post-reset count: want 0, got %d", l.Count("verifier"))
	}
}

// TestMultipleSubagents_IndependentCounters verifies each subagent
// has its own independent counter.
func TestMultipleSubagents_IndependentCounters(t *testing.T) {
	l := NewLearningLoop(3)

	// Explorer: 2 executions
	l.RecordExecution("explorer")
	l.RecordExecution("explorer")

	// Proposer: 3 executions (should trigger)
	l.RecordExecution("proposer")
	l.RecordExecution("proposer")
	nudge := l.RecordExecution("proposer")

	if !nudge {
		t.Error("proposer should nudge at 3rd execution")
	}

	// Explorer should still be at 2
	if l.Count("explorer") != 2 {
		t.Errorf("explorer count: want 2, got %d", l.Count("explorer"))
	}

	// Proposer should be at 3
	if l.Count("proposer") != 3 {
		t.Errorf("proposer count: want 3, got %d", l.Count("proposer"))
	}
}

// TestNudgePrompt_ContainsCount verifies the nudge prompt includes
// the current execution count.
func TestNudgePrompt_ContainsCount(t *testing.T) {
	l := NewLearningLoop(5)
	for i := 0; i < 5; i++ {
		l.RecordExecution("implementer")
	}

	prompt := l.NudgePrompt("implementer")
	if !strings.Contains(prompt, "5") {
		t.Error("nudge prompt should contain execution count")
	}
	if !strings.Contains(prompt, "mem_save") {
		t.Error("nudge prompt should mention mem_save")
	}
	if !strings.Contains(prompt, "skill-creator") {
		t.Error("nudge prompt should mention skill-creator")
	}
	if !strings.Contains(prompt, "OPTIONAL") {
		t.Error("nudge prompt should mention it's optional")
	}
}

// TestSessionSummaryPrompt_Format verifies the summary prompt structure.
func TestSessionSummaryPrompt_Format(t *testing.T) {
	l := NewLearningLoop(5)

	summary := l.SessionSummaryPrompt("explorer",
		"Investigate auth module",
		[]string{"JWT uses RS256", "No refresh token rotation"},
		[]string{"Mapped auth flow", "Found 3 endpoints"},
	)

	if !strings.Contains(summary, "## Goal") {
		t.Error("summary should have Goal section")
	}
	if !strings.Contains(summary, "## Discoveries") {
		t.Error("summary should have Discoveries section")
	}
	if !strings.Contains(summary, "## Accomplished") {
		t.Error("summary should have Accomplished section")
	}
	if !strings.Contains(summary, "JWT uses RS256") {
		t.Error("summary should include discoveries")
	}
	if !strings.Contains(summary, "Mapped auth flow") {
		t.Error("summary should include accomplishments")
	}
}

// TestSkillCheckPrompt_EmptyPatterns verifies empty patterns produce no prompt.
func TestSkillCheckPrompt_EmptyPatterns(t *testing.T) {
	l := NewLearningLoop(5)
	prompt := l.SkillCheckPrompt("verifier", nil)
	if prompt != "" {
		t.Error("skill check with nil patterns should be empty")
	}
	prompt = l.SkillCheckPrompt("verifier", []string{})
	if prompt != "" {
		t.Error("skill check with empty patterns should be empty")
	}
}

// TestSkillCheckPrompt_WithPatterns verifies skill check prompt content.
func TestSkillCheckPrompt_WithPatterns(t *testing.T) {
	l := NewLearningLoop(5)
	patterns := []string{"Always check nil before dereferencing", "Use context deadline for timeouts"}

	prompt := l.SkillCheckPrompt("implementer", patterns)
	if !strings.Contains(prompt, "skill-creator") {
		t.Error("skill check prompt should mention skill-creator")
	}
	if !strings.Contains(prompt, "nil before dereferencing") {
		t.Error("skill check prompt should include patterns")
	}
}
