package subagents

import (
	"context"
	"strings"
	"testing"

	"gaia/internal/mneme"
)

type mockParentStore struct {
	mneme.Store
	savedObs []mneme.Observation
}

func (m *mockParentStore) SaveObservations(ctx context.Context, convID string, obs []mneme.Observation) error {
	m.savedObs = append(m.savedObs, obs...)
	return nil
}

func TestEphemeralRepository_Isolation(t *testing.T) {
	parent := &mockParentStore{}
	repo := NewEphemeralRepository(parent)
	ctx := context.Background()

	// Create ephemeral conversation
	conv, err := repo.CreateConversation(ctx, "Child research")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if !strings.HasPrefix(conv.ID, "subagent-") {
		t.Errorf("expected subagent prefix in ID, got %s", conv.ID)
	}

	// Append message in ephemeral repo
	msg := mneme.Message{Role: mneme.RoleUser, Content: "Researching topic X"}
	if err := repo.AppendMessage(ctx, conv.ID, msg); err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}

	msgs, err := repo.Messages(ctx, conv.ID, 0)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("expected 1 message in ephemeral repo, got %d", len(msgs))
	}

	// Save observation proxies to parent
	obs := []mneme.Observation{{Content: "Topic X has property Y"}}
	if err := repo.SaveObservations(ctx, conv.ID, obs); err != nil {
		t.Fatalf("SaveObservations failed: %v", err)
	}
	if len(parent.savedObs) != 1 {
		t.Fatalf("expected observation proxied to parent, got %d", len(parent.savedObs))
	}
}

func TestExtractKeyLearnings(t *testing.T) {
	output := `Here is the comprehensive report.

## Key Learnings
- Go interfaces are satisfied implicitly without implements keyword.
- FTS5 in SQLite enables sub-millisecond lexical queries.

## Next Steps
- Continue implementing Phase 5.
`
	obs := ExtractKeyLearnings("explore-go-interfaces", output, 3, "conv-1")
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations extracted, got %d", len(obs))
	}
	if obs[0].Type != "discovery" {
		t.Errorf("expected type 'discovery', got %q", obs[0].Type)
	}
	if !strings.Contains(obs[0].Content, "Go interfaces") {
		t.Errorf("expected content to contain Go interfaces, got: %q", obs[0].Content)
	}
}

func TestExtractKeyLearnings_Decisions(t *testing.T) {
	output := `
## Decisions
1. We will use DuckDuckGo as default zero-config search provider.
2. WhatsApp integration will keep session isolation.
`
	obs := ExtractKeyLearnings("architecture decisions", output, 5, "conv-2")
	if len(obs) != 2 {
		t.Fatalf("expected 2 decisions extracted, got %d", len(obs))
	}
	if obs[0].Type != "decision" {
		t.Errorf("expected type 'decision', got %q", obs[0].Type)
	}
	if !strings.Contains(obs[0].Content, "DuckDuckGo") {
		t.Errorf("expected content to contain DuckDuckGo, got: %q", obs[0].Content)
	}
}
