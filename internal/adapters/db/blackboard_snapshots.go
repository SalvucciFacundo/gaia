package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gaia/internal/blackboard"
)

var (
	// ErrSnapshotTampered is returned when SHA-256 verification of the snapshot fails.
	ErrSnapshotTampered = errors.New("snapshot integrity check failed: SHA-256 hash mismatch")
	// ErrSnapshotNotFound is returned when the specified snapshot ID does not exist.
	ErrSnapshotNotFound = errors.New("snapshot not found")
)

// SnapshotMeta represents metadata of a stored blackboard snapshot.
type SnapshotMeta struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Profile    string    `json:"profile"`
	Label      string    `json:"label"`
	SHA256     string    `json:"sha256"`
	EntryCount int       `json:"entry_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// SaveBlackboardSnapshot serializes working memory entries, computes an immutable SHA-256 hash, and stores it in SQLite.
func (r *SQLiteRepo) SaveBlackboardSnapshot(ctx context.Context, sessionID, profile, label string, entries map[string]blackboard.MemoryEntry) (string, error) {
	if entries == nil {
		entries = make(map[string]blackboard.MemoryEntry)
	}

	normalized := make(map[string]blackboard.MemoryEntry, len(entries))
	for k, v := range entries {
		if !v.IsSpilled && v.Data != nil && !json.Valid(v.Data) {
			encoded, _ := json.Marshal(string(v.Data))
			v.Data = encoded
		}
		normalized[k] = v
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to marshal blackboard entries: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	now := time.Now()
	if sessionID == "" {
		sessionID = "default"
	}
	if profile == "" {
		profile = "dev"
	}
	if label == "" {
		label = "checkpoint"
	}

	id := fmt.Sprintf("snap-%d-%s", now.Unix(), hashHex[:8])

	query := `
	INSERT INTO blackboard_snapshots (id, session_id, profile, label, snapshot_json, sha256, entry_count, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.ExecContext(ctx, query, id, sessionID, profile, label, string(data), hashHex, len(entries), now)
	if err != nil {
		return "", fmt.Errorf("failed to insert blackboard snapshot: %w", err)
	}

	return id, nil
}

// GetBlackboardSnapshot retrieves and validates the cryptographic integrity of a blackboard snapshot.
func (r *SQLiteRepo) GetBlackboardSnapshot(ctx context.Context, id string) (map[string]blackboard.MemoryEntry, *SnapshotMeta, error) {
	query := `
	SELECT id, session_id, profile, label, snapshot_json, sha256, entry_count, created_at
	FROM blackboard_snapshots
	WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var meta SnapshotMeta
	var snapshotJSON string
	err := row.Scan(&meta.ID, &meta.SessionID, &meta.Profile, &meta.Label, &snapshotJSON, &meta.SHA256, &meta.EntryCount, &meta.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	} else if err != nil {
		return nil, nil, fmt.Errorf("query snapshot: %w", err)
	}

	// Cryptographic integrity validation
	computed := sha256.Sum256([]byte(snapshotJSON))
	computedHex := hex.EncodeToString(computed[:])
	if computedHex != meta.SHA256 {
		return nil, nil, fmt.Errorf("%w: stored=%s, computed=%s", ErrSnapshotTampered, meta.SHA256, computedHex)
	}

	var entries map[string]blackboard.MemoryEntry
	if err := json.Unmarshal([]byte(snapshotJSON), &entries); err != nil {
		return nil, nil, fmt.Errorf("corrupted snapshot json: %w", err)
	}

	return entries, &meta, nil
}

// ListBlackboardSnapshots queries stored checkpoints filtered optionally by session or profile.
func (r *SQLiteRepo) ListBlackboardSnapshots(ctx context.Context, sessionID, profile string) ([]SnapshotMeta, error) {
	query := `
	SELECT id, session_id, profile, label, sha256, entry_count, created_at
	FROM blackboard_snapshots
	WHERE 1=1
	`
	var args []interface{}
	if sessionID != "" {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	if profile != "" {
		query += " AND profile = ?"
		args = append(args, profile)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query snapshots: %w", err)
	}
	defer rows.Close()

	var metas []SnapshotMeta
	for rows.Next() {
		var meta SnapshotMeta
		if err := rows.Scan(&meta.ID, &meta.SessionID, &meta.Profile, &meta.Label, &meta.SHA256, &meta.EntryCount, &meta.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		metas = append(metas, meta)
	}

	return metas, rows.Err()
}
