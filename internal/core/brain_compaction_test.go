package core

import (
	"fmt"
	"strings"
	"testing"

	"gaia/internal/core/domain"
)

func TestBuildSessionSummary(t *testing.T) {
	messages := []domain.Message{
		{
			Role:    domain.RoleUser,
			Content: "Please refactor the auth system in internal/auth/service.go and update docs/sdd.md\nNever break backward compatibility",
		},
		{
			Role:    domain.RoleAssistant,
			Content: "I will refactor internal/auth/service.go and check config.yaml.\nDecided to use Argon2 for hashing\nImplemented JWT refresh token rotation",
			ToolCalls: []domain.ToolCall{
				{
					Name: "file_patch",
					Arguments: map[string]interface{}{
						"path": "internal/auth/service.go",
					},
				},
			},
		},
	}

	summary := BuildSessionSummary(messages)

	if !strings.Contains(summary.Goal, "refactor the auth system") {
		t.Errorf("expected goal to contain 'refactor the auth system', got %q", summary.Goal)
	}

	if len(summary.RelevantFiles) < 2 {
		t.Errorf("expected at least 2 relevant files, got %d: %v", len(summary.RelevantFiles), summary.RelevantFiles)
	}

	if len(summary.ModifiedFiles) != 1 || summary.ModifiedFiles[0] != "internal/auth/service.go" {
		t.Errorf("expected ModifiedFiles to contain internal/auth/service.go, got: %v", summary.ModifiedFiles)
	}

	if len(summary.Instructions) != 1 || !strings.Contains(summary.Instructions[0], "backward compatibility") {
		t.Errorf("expected Instructions to contain backward compatibility constraint, got: %v", summary.Instructions)
	}

	if len(summary.KeyDecisions) != 1 || !strings.Contains(summary.KeyDecisions[0], "Argon2") {
		t.Errorf("expected KeyDecisions to contain Argon2, got: %v", summary.KeyDecisions)
	}

	if len(summary.Accomplished) != 1 || !strings.Contains(summary.Accomplished[0], "JWT refresh token") {
		t.Errorf("expected Accomplished to contain JWT refresh token, got: %v", summary.Accomplished)
	}

	rehydration := FormatRehydrationPrompt(summary)
	if !strings.Contains(rehydration, "[REHYDRATED SESSION CONTEXT AFTER COMPACTION]") {
		t.Error("expected rehydration prompt header")
	}
	if !strings.Contains(rehydration, "Active Goal:") {
		t.Error("expected Active Goal in rehydration prompt")
	}
	if !strings.Contains(rehydration, "Key Architectural Decisions:") {
		t.Error("expected Key Architectural Decisions in rehydration prompt")
	}
	if !strings.Contains(rehydration, "Modified Files: internal/auth/service.go") {
		t.Error("expected Modified Files in rehydration prompt")
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

func TestBrain_CompactHistory_MicroCompaction(t *testing.T) {
	repo := &stubRepo{}
	// Pre-populate with 25 messages to exceed CompactionThreshold (20)
	for i := 0; i < 25; i++ {
		var role domain.Role
		var content string
		if i == 0 {
			role = domain.RoleUser
			content = "Build high-throughput event router in pkg/router.go\nMust handle 100k events/sec"
		} else if i%2 == 1 {
			role = domain.RoleAssistant
			content = fmt.Sprintf("Working on step %d\nDecided to use lock-free ring buffer", i)
		} else {
			role = domain.RoleUser
			content = fmt.Sprintf("Feedback for step %d", i)
		}
		_ = repo.SaveMessage(nil, domain.Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Role:    role,
			Content: content,
		})
	}

	budget := domain.BudgetConfig{
		CompactionThreshold: 20,
		KeepRecentMessages:  10,
	}

	brain := NewBrain(&stubProvider{}, repo, &stubUI{}, nil, budget)

	err := brain.compactHistory(nil)
	if err != nil {
		t.Fatalf("compactHistory failed: %v", err)
	}

	if brain.compactedTo != 15 {
		t.Errorf("expected compactedTo to be 15, got %d", brain.compactedTo)
	}

	// Verify that the saved compacted summary contains executive fields
	var foundSummary bool
	for _, m := range repo.messages {
		if m.Role == domain.RoleSystem && strings.Contains(m.Content, "[REHYDRATED SESSION CONTEXT AFTER COMPACTION]") {
			foundSummary = true
			if !strings.Contains(m.Content, "Active Goal:") {
				t.Error("expected Active Goal in summary")
			}
			if !strings.Contains(m.Content, "User Constraints & Instructions:") {
				t.Error("expected User Constraints in summary")
			}
			if !strings.Contains(m.Content, "Key Architectural Decisions:") {
				t.Error("expected Key Architectural Decisions in summary")
			}
			break
		}
	}

	if !foundSummary {
		t.Fatal("expected compacted summary message to be saved in repository")
	}
}
