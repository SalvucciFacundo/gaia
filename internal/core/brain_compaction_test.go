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
