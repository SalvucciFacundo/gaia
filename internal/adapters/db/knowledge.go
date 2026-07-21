package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"

	"github.com/google/uuid"
)

// SQLiteKnowledgeGraph implements ports.KnowledgeGraphStore using SQLite.
type SQLiteKnowledgeGraph struct {
	db *sql.DB
}

// NewKnowledgeGraph creates a new SQLiteKnowledgeGraph backed by the given DB.
func NewKnowledgeGraph(db *sql.DB) *SQLiteKnowledgeGraph {
	return &SQLiteKnowledgeGraph{db: db}
}

// AddFact stores a new knowledge fact. ID is auto-generated if empty.
func (k *SQLiteKnowledgeGraph) AddFact(ctx context.Context, fact domain.KnowledgeFact) (string, error) {
	if fact.ID == "" {
		fact.ID = uuid.New().String()
	}
	if fact.CreatedAt.IsZero() {
		fact.CreatedAt = time.Now()
	}

	labelsJSON, err := json.Marshal(fact.Labels)
	if err != nil {
		return "", fmt.Errorf("marshal labels: %w", err)
	}

	query := `INSERT INTO knowledge_facts (id, topic, concept, fact, source_agent, labels, created_at, project, language, scope)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = k.db.ExecContext(ctx, query,
		fact.ID, fact.Topic, fact.Concept, fact.Fact,
		fact.SourceAgent, string(labelsJSON), fact.CreatedAt, fact.Project, fact.Language, fact.Scope)
	if err != nil {
		return "", fmt.Errorf("insert knowledge fact: %w", err)
	}

	// Sync the FTS index manually (content table = external content)
	syncQuery := `INSERT INTO knowledge_facts_fts (rowid, topic, concept, fact, labels)
		VALUES (last_insert_rowid(), ?, ?, ?, ?)`
	_, err = k.db.ExecContext(ctx, syncQuery, fact.Topic, fact.Concept, fact.Fact, string(labelsJSON))
	if err != nil {
		return "", fmt.Errorf("sync fts index: %w", err)
	}

	return fact.ID, nil
}

// GetFactsByTopic returns all facts for a given topic, newest first.
func (k *SQLiteKnowledgeGraph) GetFactsByTopic(ctx context.Context, topic string) ([]domain.KnowledgeFact, error) {
	query := `SELECT id, topic, concept, fact, source_agent, labels, created_at
		FROM knowledge_facts WHERE topic = ? ORDER BY created_at DESC`
	return k.queryFacts(ctx, query, topic)
}

// GetFactsByConcept returns all facts under a specific topic+concept, newest first.
func (k *SQLiteKnowledgeGraph) GetFactsByConcept(ctx context.Context, topic, concept string) ([]domain.KnowledgeFact, error) {
	query := `SELECT id, topic, concept, fact, source_agent, labels, created_at
		FROM knowledge_facts WHERE topic = ? AND concept = ? ORDER BY created_at DESC`
	return k.queryFacts(ctx, query, topic, concept)
}

// SearchFacts performs full-text search across topic, concept, fact, and labels.
func (k *SQLiteKnowledgeGraph) SearchFacts(ctx context.Context, queryStr string) ([]domain.KnowledgeFact, error) {
	// Sanitize: FTS5 doesn't like special chars as bare tokens
	cleaned := sanitizeFTS(queryStr)
	if cleaned == "" {
		return nil, nil
	}

	ftsQuery := `SELECT k.id, k.topic, k.concept, k.fact, k.source_agent, k.labels, k.created_at
		FROM knowledge_facts k
		JOIN knowledge_facts_fts fts ON k.rowid = fts.rowid
		WHERE knowledge_facts_fts MATCH ?
		ORDER BY rank
		LIMIT 50`
	return k.queryFacts(ctx, ftsQuery, cleaned)
}

// GetRecentFacts returns the most recent N facts across all topics.
func (k *SQLiteKnowledgeGraph) GetRecentFacts(ctx context.Context, limit int) ([]domain.KnowledgeFact, error) {
	query := `SELECT id, topic, concept, fact, source_agent, labels, created_at
		FROM knowledge_facts ORDER BY created_at DESC LIMIT ?`
	return k.queryFacts(ctx, query, limit)
}

// GetAllTopics returns all distinct topic names.
func (k *SQLiteKnowledgeGraph) GetAllTopics(ctx context.Context) ([]string, error) {
	rows, err := k.db.QueryContext(ctx, "SELECT DISTINCT topic FROM knowledge_facts ORDER BY topic")
	if err != nil {
		return nil, fmt.Errorf("query topics: %w", err)
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

// GetRecentTopics returns topics that have facts newer than the given duration.
func (k *SQLiteKnowledgeGraph) GetRecentTopics(ctx context.Context, since time.Duration) ([]string, error) {
	sinceTime := time.Now().Add(-since)
	rows, err := k.db.QueryContext(ctx,
		"SELECT DISTINCT topic FROM knowledge_facts WHERE created_at >= ? ORDER BY topic",
		sinceTime)
	if err != nil {
		return nil, fmt.Errorf("query recent topics: %w", err)
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

// queryFacts is an internal helper that scans rows into KnowledgeFact slices.
func (k *SQLiteKnowledgeGraph) queryFacts(ctx context.Context, query string, args ...interface{}) ([]domain.KnowledgeFact, error) {
	rows, err := k.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query knowledge facts: %w", err)
	}
	defer rows.Close()

	var facts []domain.KnowledgeFact
	for rows.Next() {
		var fact domain.KnowledgeFact
		var labelsJSON string
		var createdAt time.Time

		if err := rows.Scan(&fact.ID, &fact.Topic, &fact.Concept, &fact.Fact,
			&fact.SourceAgent, &labelsJSON, &fact.Project, &fact.Language, &fact.Scope, &createdAt); err != nil {
			return nil, fmt.Errorf("scan fact: %w", err)
		}

		if labelsJSON != "" {
			if err := json.Unmarshal([]byte(labelsJSON), &fact.Labels); err != nil {
				fact.Labels = nil
			}
		}
		fact.CreatedAt = createdAt
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

// sanitizeFTS removes characters that break FTS5 match syntax.
func sanitizeFTS(s string) string {
	// Wrap each word in quotes to treat as literal terms
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		// Remove special FTS5 chars
		cleaned := strings.NewReplacer(
			"\"", "", "*", "", "^", "", "(", "", ")", "",
			"NEAR", "", "NOT", "", "AND", "", "OR", "",
		).Replace(p)
		if cleaned != "" {
			quoted[i] = "\"" + cleaned + "\""
		}
	}
	return strings.Join(quoted, " ")
}

