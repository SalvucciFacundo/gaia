package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gaia/internal/codegraph/domain"
	_ "modernc.org/sqlite"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS files (
    path TEXT PRIMARY KEY,
    hash TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    package TEXT NOT NULL,
    file TEXT NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER NOT NULL,
    signature TEXT,
    doc TEXT,
    is_exported BOOLEAN NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_nodes_kind_name ON nodes(kind, name);
CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package);
CREATE INDEX IF NOT EXISTS idx_nodes_file ON nodes(file);

CREATE TABLE IF NOT EXISTS edges (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    file TEXT,
    line INTEGER
);

CREATE INDEX IF NOT EXISTS idx_edges_source_target ON edges(source_id, target_id);
CREATE INDEX IF NOT EXISTS idx_edges_target_kind ON edges(target_id, kind);
CREATE INDEX IF NOT EXISTS idx_edges_source_kind ON edges(source_id, kind);
CREATE INDEX IF NOT EXISTS idx_edges_file ON edges(file);
`

// Store handles SQLite persistence for the AST index.
type Store struct {
	db *sql.DB
}

// NewStore initializes a SQLite store at the given database path.
func NewStore(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	dsn := dbPath
	if !strings.Contains(dsn, "?") && dsn != ":memory:" {
		dsn += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	s := &Store{db: db}
	if err := s.InitSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// NewStoreWithDB creates a Store using an existing *sql.DB connection.
func NewStoreWithDB(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, domain.ErrDatabaseNotInitialized
	}
	s := &Store{db: db}
	if err := s.InitSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return s, nil
}

// DB returns the underlying sql.DB instance.
func (s *Store) DB() *sql.DB {
	return s.db
}

// InitSchema executes DDL tables and indexes.
func (s *Store) InitSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaDDL)
	return err
}

// GetFileHashes loads all recorded files and their respective hashes.
func (s *Store) GetFileHashes(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT path, hash FROM files")
	if err != nil {
		return nil, fmt.Errorf("failed to query file hashes: %w", err)
	}
	defer rows.Close()

	hashes := make(map[string]string)
	for rows.Next() {
		var p, h string
		if err := rows.Scan(&p, &h); err != nil {
			return nil, fmt.Errorf("failed to scan file hash: %w", err)
		}
		hashes[p] = h
	}
	return hashes, rows.Err()
}

// DeleteStaleFile removes file entries, associated nodes, and edges.
func (s *Store) DeleteStaleFile(ctx context.Context, filePath string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Delete edges originating or targeting nodes in this file, or where edge.file = filePath
	deleteEdgesQuery := `
		DELETE FROM edges 
		WHERE file = ? 
		   OR source_id IN (SELECT id FROM nodes WHERE file = ?) 
		   OR target_id IN (SELECT id FROM nodes WHERE file = ?)
	`
	if _, err := tx.ExecContext(ctx, deleteEdgesQuery, filePath, filePath, filePath); err != nil {
		return fmt.Errorf("failed to delete edges for file %s: %w", filePath, err)
	}

	// 2. Delete nodes in this file
	if _, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE file = ?", filePath); err != nil {
		return fmt.Errorf("failed to delete nodes for file %s: %w", filePath, err)
	}

	// 3. Delete from files table
	if _, err := tx.ExecContext(ctx, "DELETE FROM files WHERE path = ?", filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return tx.Commit()
}

// SaveBatch executes batch insert of files, nodes, and edges in a single transaction.
func (s *Store) SaveBatch(ctx context.Context, files map[string]string, nodes []domain.SymbolNode, edges []domain.Edge) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin batch tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert/Update files
	stmtFile, err := tx.PrepareContext(ctx, `
		INSERT INTO files (path, hash, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET hash = excluded.hash, updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare file statement: %w", err)
	}
	defer stmtFile.Close()

	for path, hash := range files {
		if _, err := stmtFile.ExecContext(ctx, path, hash); err != nil {
			return fmt.Errorf("failed to insert file %s: %w", path, err)
		}
	}

	// 2. Insert/Update nodes
	stmtNode, err := tx.PrepareContext(ctx, `
		INSERT INTO nodes (id, kind, name, package, file, line_start, line_end, signature, doc, is_exported)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind = excluded.kind,
			name = excluded.name,
			package = excluded.package,
			file = excluded.file,
			line_start = excluded.line_start,
			line_end = excluded.line_end,
			signature = excluded.signature,
			doc = excluded.doc,
			is_exported = excluded.is_exported
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare node statement: %w", err)
	}
	defer stmtNode.Close()

	for _, n := range nodes {
		if _, err := stmtNode.ExecContext(ctx, string(n.ID), n.Kind, n.Name, n.Package, n.File, n.LineStart, n.LineEnd, n.Signature, n.Doc, n.IsExported); err != nil {
			return fmt.Errorf("failed to insert node %s: %w", n.ID, err)
		}
	}

	// 3. Insert edges
	stmtEdge, err := tx.PrepareContext(ctx, `
		INSERT INTO edges (id, source_id, target_id, kind, file, line)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_id = excluded.source_id,
			target_id = excluded.target_id,
			kind = excluded.kind,
			file = excluded.file,
			line = excluded.line
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare edge statement: %w", err)
	}
	defer stmtEdge.Close()

	for _, e := range edges {
		if _, err := stmtEdge.ExecContext(ctx, e.ID, string(e.SourceID), string(e.TargetID), e.Kind, e.File, e.Line); err != nil {
			return fmt.Errorf("failed to insert edge %s: %w", e.ID, err)
		}
	}

	return tx.Commit()
}

// GetNode retrieves a symbol node by its ID.
func (s *Store) GetNode(ctx context.Context, id domain.SymbolRef) (*domain.SymbolNode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, kind, name, package, file, line_start, line_end, signature, doc, is_exported
		FROM nodes WHERE id = ?
	`, string(id))

	var n domain.SymbolNode
	var idStr string
	var sig, doc sql.NullString
	err := row.Scan(&idStr, &n.Kind, &n.Name, &n.Package, &n.File, &n.LineStart, &n.LineEnd, &sig, &doc, &n.IsExported)
	if err != nil {
		if errorsIs(err, sql.ErrNoRows) {
			return nil, domain.ErrSymbolNotFound
		}
		return nil, fmt.Errorf("failed to scan node: %w", err)
	}
	n.ID = domain.SymbolRef(idStr)
	if sig.Valid {
		n.Signature = sig.String
	}
	if doc.Valid {
		n.Doc = doc.String
	}
	return &n, nil
}

// Close closes the underlying SQLite database.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func errorsIs(err, target error) bool {
	return err == target || (err != nil && err.Error() == target.Error())
}
