package mneme

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

type mockKGStore struct {
	ports.KnowledgeGraphStore
	topics []string
	facts  map[string][]domain.KnowledgeFact
}

func (m *mockKGStore) GetAllTopics(ctx context.Context) ([]string, error) {
	return m.topics, nil
}

func (m *mockKGStore) GetFactsByTopic(ctx context.Context, topic string) ([]domain.KnowledgeFact, error) {
	return m.facts[topic], nil
}

func TestExportKnowledgeGraph(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gaia-export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	kg := &mockKGStore{
		topics: []string{"architecture", "security"},
		facts: map[string][]domain.KnowledgeFact{
			"architecture": {
				{Topic: "architecture", Concept: "kernel", Fact: "Core is decoupled via ports", SourceAgent: "agent-1"},
			},
			"security": {
				{Topic: "security", Concept: "guard", Fact: "Tool execution requires confirmation", SourceAgent: "agent-2"},
			},
		},
	}

	count, exportDir, err := ExportKnowledgeGraph(context.Background(), kg, tempDir)
	if err != nil {
		t.Fatalf("ExportKnowledgeGraph failed: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 topics exported, got %d", count)
	}

	archFile := filepath.Join(exportDir, "architecture.md")
	content, err := os.ReadFile(archFile)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("expected content in %s, got empty", archFile)
	}
}
