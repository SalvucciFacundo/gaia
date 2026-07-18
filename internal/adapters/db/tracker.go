package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gaia/internal/tracker"
)

// TrackerRepo implements tracker.ReleaseStore using the SQLite database.
type TrackerRepo struct {
	db *sql.DB
}

// NewTrackerRepo creates a new TrackerRepo backed by the given DB connection.
func NewTrackerRepo(db *sql.DB) *TrackerRepo {
	return &TrackerRepo{db: db}
}

// GetLastCheck returns the last check time and cached ETag from tracker_state.
func (r *TrackerRepo) GetLastCheck(ctx context.Context) (*time.Time, string, error) {
	var tStr, etag string
	query := `SELECT value FROM tracker_state WHERE key = ?`
	err := r.db.QueryRowContext(ctx, query, "last_checked").Scan(&tStr)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("get last_checked: %w", err)
	}

	t, err := time.Parse(time.RFC3339, tStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, tStr)
		if err != nil {
			return nil, "", fmt.Errorf("parse last_checked time: %w", err)
		}
	}

	err = r.db.QueryRowContext(ctx, query, "etag").Scan(&etag)
	if err == sql.ErrNoRows {
		etag = ""
	} else if err != nil {
		return nil, "", fmt.Errorf("get etag: %w", err)
	}

	return &t, etag, nil
}

// SetLastCheck updates the last check time and ETag in tracker_state.
func (r *TrackerRepo) SetLastCheck(ctx context.Context, t time.Time, etag string) error {
	now := time.Now().UTC()
	tStr := t.Format(time.RFC3339)

	// Upsert last_checked.
	query := `INSERT INTO tracker_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	if _, err := r.db.ExecContext(ctx, query, "last_checked", tStr, now); err != nil {
		return fmt.Errorf("set last_checked: %w", err)
	}

	// Upsert etag.
	if etag != "" {
		if _, err := r.db.ExecContext(ctx, query, "etag", etag, now); err != nil {
			return fmt.Errorf("set etag: %w", err)
		}
	}
	return nil
}

// SaveRelease inserts or updates a release in tracker_releases.
func (r *TrackerRepo) SaveRelease(ctx context.Context, release tracker.Release) error {
	now := time.Now().UTC()
	query := `INSERT INTO tracker_releases (tag, body, published_at, html_url, cached_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(tag) DO UPDATE SET body = excluded.body, published_at = excluded.published_at,
		html_url = excluded.html_url, cached_at = excluded.cached_at`
	_, err := r.db.ExecContext(ctx, query,
		release.Tag,
		release.Body,
		release.PublishedAt,
		release.HTMLURL,
		now,
	)
	if err != nil {
		return fmt.Errorf("save release %q: %w", release.Tag, err)
	}
	return nil
}

// ListReleases returns all cached releases ordered by published date descending.
func (r *TrackerRepo) ListReleases(ctx context.Context) ([]tracker.Release, error) {
	query := `SELECT tag, body, published_at, html_url FROM tracker_releases ORDER BY published_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	defer rows.Close()

	var releases []tracker.Release
	for rows.Next() {
		var rel tracker.Release
		if err := rows.Scan(&rel.Tag, &rel.Body, &rel.PublishedAt, &rel.HTMLURL); err != nil {
			return nil, fmt.Errorf("scan release: %w", err)
		}
		releases = append(releases, rel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate releases: %w", err)
	}

	if releases == nil {
		releases = []tracker.Release{}
	}
	return releases, nil
}
