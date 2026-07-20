package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gaia/internal/core/domain"
	_ "modernc.org/sqlite"
)

type SQLiteRepo struct {
	db *sql.DB
}

// NewSQLiteRepo creates a new SQLite repository at the default path (~/.config/gaia/gaia.db).
func NewSQLiteRepo() (*SQLiteRepo, error) {
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".config/gaia")
	dbPath := filepath.Join(dbDir, "gaia.db")
	return newRepoAt(dbPath)
}

// NewSQLiteRepoWithPath creates a new SQLite repository at the given path.
func NewSQLiteRepoWithPath(dbPath string) (*SQLiteRepo, error) {
	return newRepoAt(dbPath)
}

func newRepoAt(dbPath string) (*SQLiteRepo, error) {

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	repo := &SQLiteRepo{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}

	return repo, nil
}

// DB returns the underlying database connection for use by other repositories.
func (r *SQLiteRepo) DB() *sql.DB {
	return r.db
}

func (r *SQLiteRepo) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);

	-- Dynamic subagent definitions
	CREATE TABLE IF NOT EXISTS subagent_defs (
		name TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		allowed_tools TEXT NOT NULL DEFAULT '[]',
		skills TEXT NOT NULL DEFAULT '[]',
		system_prompt TEXT NOT NULL DEFAULT '',
		personality TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tracker cached state (ETag, last_checked)
	CREATE TABLE IF NOT EXISTS tracker_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Tracker cached releases
	CREATE TABLE IF NOT EXISTS tracker_releases (
		tag TEXT PRIMARY KEY,
		body TEXT NOT NULL DEFAULT '',
		published_at DATETIME NOT NULL,
		html_url TEXT NOT NULL DEFAULT '',
		cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Async tasks (persisted across restarts)
	CREATE TABLE IF NOT EXISTS tasks (
		task_id TEXT PRIMARY KEY,
		subagent_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		result_json TEXT,
		error_text TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		completed_at DATETIME
	);

	-- Knowledge graph: shared cross-domain facts
	CREATE TABLE IF NOT EXISTS knowledge_facts (
		id TEXT PRIMARY KEY,
		topic TEXT NOT NULL,
		concept TEXT NOT NULL,
		fact TEXT NOT NULL,
		source_agent TEXT NOT NULL,
		labels TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_knowledge_topic ON knowledge_facts(topic);
	CREATE INDEX IF NOT EXISTS idx_knowledge_topic_concept ON knowledge_facts(topic, concept);
	CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_facts_fts USING fts5(
		topic, concept, fact, labels,
		content='knowledge_facts',
		content_rowid='rowid'
	);
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *SQLiteRepo) CreateSession(ctx context.Context, name string) (string, error) {
	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	query := `INSERT INTO sessions (id, name, created_at) VALUES (?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, id, name, time.Now())
	return id, err
}

func (r *SQLiteRepo) SaveMessage(ctx context.Context, msg domain.Message) error {
	id := msg.ID
	if id == "" {
		id = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	
	// Use a default session if none provided; Brain sets the real session ID.
	sessID := "default"
	query := `INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, id, sessID, string(msg.Role), msg.Content, time.Now())
	return err
}

func (r *SQLiteRepo) GetHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	return r.GetMessages(ctx, "default", limit)
}

func (r *SQLiteRepo) GetMessageCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE session_id = 'default'`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *SQLiteRepo) GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error) {
	query := `SELECT id, role, content, created_at FROM messages WHERE session_id = 'default' ORDER BY created_at ASC LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []domain.Message
	for rows.Next() {
		var msg domain.Message
		var roleStr string
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &roleStr, &msg.Content, &createdAt); err != nil {
			return nil, err
		}
		msg.Role = domain.Role(roleStr)
		msg.CreatedAt = createdAt
		history = append(history, msg)
	}
	return history, rows.Err()
}

func (r *SQLiteRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error) {
	query := `SELECT id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []domain.Message
	for rows.Next() {
		var msg domain.Message
		var roleStr string
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &roleStr, &msg.Content, &createdAt); err != nil {
			return nil, err
		}
		msg.Role = domain.Role(roleStr)
		msg.CreatedAt = createdAt
		history = append(history, msg)
	}
	return history, nil
}
