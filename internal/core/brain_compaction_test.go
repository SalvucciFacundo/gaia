package core

import (
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

func TestBuildSessionSummary(t *testing.T) {
	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Please refactor the auth system in internal/auth/service.go and update docs/sdd.md",
		},
		{
			Role:    domain.RoleAssistant,
			Content: "I will refactor internal/auth/service.go and check config.yaml.",
		},
	}

	summary := BuildSessionSummary(messages)

	if !strings.Contains(summary.Goal, "refactor the auth system") {
		t.Errorf("expected goal to contain 'refactor the auth system', got %q", summary.Goal)
	}

	if len(summary.RelevantFiles) < 2 {
		t.Errorf("expected at least 2 relevant files, got %d: %v", len(summary.RelevantFiles), summary.RelevantFiles)
	}

	rehydration := FormatRehydrationPrompt(summary)
	if !strings.Contains(rehydration, "[REHYDRATED SESSION CONTEXT AFTER COMPACTION]") {
		t.Error("expected rehydration prompt header")
	}
	if !strings.Contains(rehydration, "Active Goal:") {
		t.Error("expected Active Goal in rehydration prompt")
	}
}

func TestFormatEngramSessionSummary(t *testing.T) {
	summary := SessionSummary{
		Goal:          "Implement ODD Router",
		Instructions:  []string{"Use TDD", "Verify edge cases"},
		Discoveries:   []string{"Router should keep /sdd as deprecated alias"},
		Accomplished:  []string{"Built ODD routing and unit tests"},
		NextSteps:     []string{"Align documentation"},
		RelevantFiles: []string{"internal/core/odd_router.go"},
	}

	engramSummary := FormatEngramSessionSummary(summary)
	if !strings.Contains(engramSummary, "## Goal\nImplement ODD Router") {
		t.Error("expected Goal section in Engram summary")
	}
	if !strings.Contains(engramSummary, "## Discoveries\n- Router should keep /sdd as deprecated alias") {
		t.Error("expected Discoveries section in Engram summary")
	}
	if !strings.Contains(engramSummary, "## Accomplished\n- Built ODD routing and unit tests") {
		t.Error("expected Accomplished section in Engram summary")
	}
	if !strings.Contains(engramSummary, "## Relevant Files\n- internal/core/odd_router.go") {
		t.Error("expected Relevant Files section in Engram summary")
	}
}

