package memory

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

type mockKGStore struct {
	mu    sync.Mutex
	facts []domain.KnowledgeFact
}

func (m *mockKGStore) AddFact(ctx context.Context, fact domain.KnowledgeFact) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = append(m.facts, fact)
	return "fact-1", nil
}

func (m *mockKGStore) GetFactsByTopic(ctx context.Context, topic string) ([]domain.KnowledgeFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.KnowledgeFact
	for _, f := range m.facts {
		if f.Topic == topic {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *mockKGStore) SearchFacts(ctx context.Context, query string) ([]domain.KnowledgeFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.facts, nil
}

func (m *mockKGStore) GetFactsByConcept(ctx context.Context, topic, concept string) ([]domain.KnowledgeFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.facts, nil
}

func (m *mockKGStore) ListAllFacts(ctx context.Context) ([]domain.KnowledgeFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.facts, nil
}

func (m *mockKGStore) GetFactCount(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.facts), nil
}
func (m *mockKGStore) GetRecentFacts(ctx context.Context, limit int) ([]domain.KnowledgeFact, error) { return m.facts, nil }
func (m *mockKGStore) GetAllTopics(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockKGStore) GetRecentTopics(ctx context.Context, since time.Duration) ([]string, error) { return nil, nil }
func (m *mockKGStore) Clear(ctx context.Context) error { m.facts = nil; return nil }

type mockRepo struct {
	messages []domain.Message
}

func (r *mockRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	r.messages = append(r.messages, msg)
	return nil
}
func (r *mockRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) { return nil, nil }
func (r *mockRepo) CreateSession(ctx context.Context, name string) (string, error) { return "s-1", nil }
func (r *mockRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) { return nil, nil }
func (r *mockRepo) GetMessageCount(ctx context.Context) (int, error) { return 0, nil }
func (r *mockRepo) GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error) { return nil, nil }
func (r *mockRepo) GetLastMessages(ctx context.Context, n int) ([]domain.Message, error) { return nil, nil }
func (r *mockRepo) DeleteMessagesAfter(ctx context.Context, afterID string) error { return nil }
func (r *mockRepo) ClearMessages(ctx context.Context) error { r.messages = nil; return nil }
func (r *mockRepo) ListSessions(ctx context.Context) ([]domain.SessionInfo, error) { return nil, nil }
func (r *mockRepo) RenameSession(ctx context.Context, sessionID, name string) error { return nil }

func TestKnowledgeSync_RecordSubagentResult(t *testing.T) {
	store := &mockKGStore{}
	repo := &mockRepo{}
	sync := NewKnowledgeSync(store, repo)

	result := &domain.SubagentResult{
		Status:    domain.SubagentSuccess,
		Summary:   "Explored architecture and found components",
		Artifacts: []string{"internal/agent/moa.go"},
		Risks:     []string{"High concurrency load on LLM provider"},
	}

	sync.RecordSubagentResult(context.Background(), "explorer", "explore project structure", result)

	if len(store.facts) == 0 {
		t.Fatal("expected facts to be recorded in knowledge graph")
	}

	var foundSummary, foundArtifact, foundRisk bool
	for _, f := range store.facts {
		if f.Topic == "explorer" && strings.Contains(f.Fact, "Explored architecture") {
			foundSummary = true
		}
		if f.Topic == "Codebase" && f.Concept == "internal/agent/moa.go" {
			foundArtifact = true
		}
		if f.Topic == "Risks" && strings.Contains(f.Fact, "High concurrency") {
			foundRisk = true
		}
	}

	if !foundSummary {
		t.Error("missing subagent summary fact")
	}
	if !foundArtifact {
		t.Error("missing artifact fact")
	}
	if !foundRisk {
		t.Error("missing risk fact")
	}
	if len(repo.messages) == 0 {
		t.Error("expected notification message saved to repo")
	}
}

func TestKnowledgeSync_RecordToolExecution(t *testing.T) {
	store := &mockKGStore{}
	sync := NewKnowledgeSync(store, nil)

	// File mutation
	sync.RecordToolExecution(context.Background(), "file_write", map[string]interface{}{
		"path": "test.go",
	}, &domain.ToolResult{Success: true})

	// Git commit
	sync.RecordToolExecution(context.Background(), "git_commit", map[string]interface{}{
		"action": "commit message",
	}, &domain.ToolResult{Success: true, Output: "committed abc1234"})

	// Shell test
	sync.RecordToolExecution(context.Background(), "shell_exec", map[string]interface{}{
		"command": "go test ./...",
	}, &domain.ToolResult{Success: true, Output: "PASS"})

	if len(store.facts) != 3 {
		t.Fatalf("expected 3 direct action facts, got %d", len(store.facts))
	}
}

func TestKnowledgeSync_RecordResponseFacts(t *testing.T) {
	store := &mockKGStore{}
	sync := NewKnowledgeSync(store, nil)

	response := `
Here is what we did:
This system implements the WorkflowRouter port for ODD and SDD workflows.
We also refactored the entire package structure to be clean.
Normal conversation line without keywords.
`

	sync.RecordResponseFacts(context.Background(), response)

	if len(store.facts) < 2 {
		t.Fatalf("expected at least 2 extracted facts, got %d", len(store.facts))
	}
}
