package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

func setupTestKG(t *testing.T) (*SQLiteKnowledgeGraph, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "gaia-kg-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	dbPath := filepath.Join(dir, "test.db")
	repo, err := NewSQLiteRepoWithPath(dbPath)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("new repo: %v", err)
	}

	kg := NewKnowledgeGraph(repo.DB())
	cleanup := func() { repo.db.Close(); os.RemoveAll(dir) }
	return kg, cleanup
}

func TestKG_AddAndGetByTopic(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	id, err := kg.AddFact(ctx, domain.KnowledgeFact{
		Topic:       "Authentication",
		Concept:     "JWT in this project",
		Fact:        "Tokens expire in 24h, refresh in 7d",
		SourceAgent: "designer",
		Labels:      []string{"security", "jwt"},
	})
	if err != nil {
		t.Fatalf("AddFact: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	facts, err := kg.GetFactsByTopic(ctx, "Authentication")
	if err != nil {
		t.Fatalf("GetFactsByTopic: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Fact != "Tokens expire in 24h, refresh in 7d" {
		t.Errorf("unexpected fact: %q", facts[0].Fact)
	}
	if facts[0].SourceAgent != "designer" {
		t.Errorf("unexpected source: %q", facts[0].SourceAgent)
	}
	if len(facts[0].Labels) != 2 || facts[0].Labels[0] != "security" {
		t.Errorf("unexpected labels: %v", facts[0].Labels)
	}
}

func TestKG_GetFactsByConcept(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Auth", Concept: "JWT", Fact: "Token expires in 24h", SourceAgent: "designer",
	})
	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Auth", Concept: "JWT", Fact: "Refresh token in 7d", SourceAgent: "debugger",
	})
	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Auth", Concept: "OAuth", Fact: "Google OAuth flow", SourceAgent: "explorer",
	})

	facts, err := kg.GetFactsByConcept(ctx, "Auth", "JWT")
	if err != nil {
		t.Fatalf("GetFactsByConcept: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
}

func TestKG_SearchFacts(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Auth", Concept: "JWT", Fact: "Tokens expire in 24 hours", SourceAgent: "designer",
	})
	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Database", Concept: "SQLite", Fact: "Uses WAL mode for concurrency", SourceAgent: "implementer",
	})
	kg.AddFact(ctx, domain.KnowledgeFact{
		Topic: "Auth", Concept: "API Keys", Fact: "Stored in OS keychain", SourceAgent: "reviewer",
		Labels: []string{"security", "credentials"},
	})

	tests := []struct {
		name   string
		query  string
		expect int
	}{
		// FTS5 is case-insensitive for ASCII, but does NOT do stemming.
		// "hours" matches the exact word "hours" in the fact text.
		{"search by word", "hours", 1},
		// "auth" (lowercase) matches "Auth" topic via case-insensitive FTS5.
		{"search by topic", "auth", 2},
		// "keychain" is an exact word match in the fact text.
		{"search by fact word", "keychain", 1},
		// "concurrency" is an exact word match.
		{"search by fact", "concurrency", 1},
		// "jwt" matches the concept field (case-insensitive).
		{"search by concept", "jwt", 1},
		// Empty query returns no results.
		{"empty query", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts, err := kg.SearchFacts(ctx, tt.query)
			if err != nil {
				t.Fatalf("SearchFacts(%q): %v", tt.query, err)
			}
			if len(facts) != tt.expect {
				t.Errorf("expected %d results for %q, got %d", tt.expect, tt.query, len(facts))
				for _, f := range facts {
					t.Logf("  match: [%s] %s — %s", f.Topic, f.Concept, f.Fact)
				}
			}
		})
	}
}

func TestKG_GetRecentFacts(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		kg.AddFact(ctx, domain.KnowledgeFact{
			Topic: "Test", Concept: "Item", Fact: "Fact #", SourceAgent: "tester",
		})
		time.Sleep(time.Millisecond) // ensure different timestamps
	}

	recent, err := kg.GetRecentFacts(ctx, 3)
	if err != nil {
		t.Fatalf("GetRecentFacts: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent facts, got %d", len(recent))
	}
}

func TestKG_GetAllTopics(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	kg.AddFact(ctx, domain.KnowledgeFact{Topic: "Auth", Concept: "JWT", Fact: "x", SourceAgent: "a"})
	kg.AddFact(ctx, domain.KnowledgeFact{Topic: "Database", Concept: "SQL", Fact: "y", SourceAgent: "b"})
	kg.AddFact(ctx, domain.KnowledgeFact{Topic: "Auth", Concept: "OAuth", Fact: "z", SourceAgent: "c"})

	topics, err := kg.GetAllTopics(ctx)
	if err != nil {
		t.Fatalf("GetAllTopics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 distinct topics, got %d: %v", len(topics), topics)
	}
}

func TestKG_GetRecentTopics(t *testing.T) {
	kg, cleanup := setupTestKG(t)
	defer cleanup()
	ctx := context.Background()

	kg.AddFact(ctx, domain.KnowledgeFact{Topic: "Auth", Concept: "JWT", Fact: "x", SourceAgent: "a"})
	kg.AddFact(ctx, domain.KnowledgeFact{Topic: "Database", Concept: "SQL", Fact: "y", SourceAgent: "b"})

	// Get topics from the last hour
	topics, err := kg.GetRecentTopics(ctx, time.Hour)
	if err != nil {
		t.Fatalf("GetRecentTopics: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics in last hour, got %d: %v", len(topics), topics)
	}

	// Get topics from the last nanosecond — should be empty
	topics, err = kg.GetRecentTopics(ctx, time.Nanosecond)
	if err != nil {
		t.Fatalf("GetRecentTopics: %v", err)
	}
	if len(topics) != 0 {
		t.Fatalf("expected 0 topics in last nanosecond, got %d: %v", len(topics), topics)
	}
}
