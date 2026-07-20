package db

import (
	"context"
	"fmt"
	"time"

	"gaia/internal/core/domain"
)

// Cron table migration adds the cron_jobs table for scheduled task persistence.
const cronMigration = `
CREATE TABLE IF NOT EXISTS cron_jobs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	schedule TEXT NOT NULL,
	task TEXT NOT NULL,
	deliver_to TEXT NOT NULL DEFAULT 'terminal',
	deliver_target TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	last_run DATETIME,
	next_run DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// MigrateCron adds the cron_jobs table. Safe to call multiple times.
func (r *SQLiteRepo) MigrateCron() error {
	_, err := r.db.Exec(cronMigration)
	return err
}

// ListJobs returns all cron jobs ordered by creation time.
func (r *SQLiteRepo) ListJobs(ctx context.Context) ([]domain.CronJob, error) {
	query := `SELECT id, name, schedule, task, deliver_to, deliver_target, enabled,
		COALESCE(last_run, '') as last_run, COALESCE(next_run, '') as next_run,
		COALESCE(created_at, '') as created_at
		FROM cron_jobs ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.CronJob
	for rows.Next() {
		var j domain.CronJob
		var lastRun, nextRun, createdAt string
		if err := rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Task,
			&j.DeliverTo, &j.DeliverTarget, &j.Enabled,
			&lastRun, &nextRun, &createdAt); err != nil {
			return nil, fmt.Errorf("scan cron job: %w", err)
		}
		if lastRun != "" {
			j.LastRun, _ = time.Parse(time.RFC3339, lastRun)
		}
		if nextRun != "" {
			j.NextRun, _ = time.Parse(time.RFC3339, nextRun)
		}
		if createdAt != "" {
			j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// CreateJob inserts a new cron job and returns its ID.
func (r *SQLiteRepo) CreateJob(ctx context.Context, job domain.CronJob) (string, error) {
	job.ID = fmt.Sprintf("cron-%d", time.Now().UnixNano())
	query := `INSERT INTO cron_jobs (id, name, schedule, task, deliver_to, deliver_target, enabled, next_run)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	enabled := 0
	if job.Enabled {
		enabled = 1
	}

	nextRun := ""
	if !job.NextRun.IsZero() {
		nextRun = job.NextRun.Format(time.RFC3339)
	}

	_, err := r.db.ExecContext(ctx, query, job.ID, job.Name, job.Schedule, job.Task,
		job.DeliverTo, job.DeliverTarget, enabled, nextRun)
	if err != nil {
		return "", fmt.Errorf("create cron job: %w", err)
	}
	return job.ID, nil
}

// UpdateJob updates an existing cron job.
func (r *SQLiteRepo) UpdateJob(ctx context.Context, job domain.CronJob) error {
	enabled := 0
	if job.Enabled {
		enabled = 1
	}

	query := `UPDATE cron_jobs SET name=?, schedule=?, task=?, deliver_to=?, deliver_target=?, enabled=?
		WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, job.Name, job.Schedule, job.Task,
		job.DeliverTo, job.DeliverTarget, enabled, job.ID)
	if err != nil {
		return fmt.Errorf("update cron job: %w", err)
	}
	return nil
}

// DeleteJob removes a cron job by ID.
func (r *SQLiteRepo) DeleteJob(ctx context.Context, id string) error {
	query := `DELETE FROM cron_jobs WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}
	return nil
}

// GetDueJobs returns jobs whose next_run is before now.
func (r *SQLiteRepo) GetDueJobs(ctx context.Context) ([]domain.CronJob, error) {
	query := `SELECT id, name, schedule, task, deliver_to, deliver_target, enabled,
		COALESCE(last_run, '') as last_run, COALESCE(next_run, '') as next_run,
		COALESCE(created_at, '') as created_at
		FROM cron_jobs WHERE enabled = 1 ORDER BY next_run ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get due jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.CronJob
	now := time.Now()
	for rows.Next() {
		var j domain.CronJob
		var lastRun, nextRun, createdAt string
		if err := rows.Scan(&j.ID, &j.Name, &j.Schedule, &j.Task,
			&j.DeliverTo, &j.DeliverTarget, &j.Enabled,
			&lastRun, &nextRun, &createdAt); err != nil {
			return nil, fmt.Errorf("scan due job: %w", err)
		}
		if nextRun != "" {
			j.NextRun, _ = time.Parse(time.RFC3339, nextRun)
		}
		if lastRun != "" {
			j.LastRun, _ = time.Parse(time.RFC3339, lastRun)
		}
		if createdAt != "" {
			j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}
		// Only include if next_run has passed
		if !j.NextRun.IsZero() && j.NextRun.Before(now) {
			jobs = append(jobs, j)
		}
	}
	return jobs, nil
}

// MarkRun updates last_run and next_run for a cron job.
func (r *SQLiteRepo) MarkRun(ctx context.Context, id string, lastRun, nextRun time.Time) error {
	query := `UPDATE cron_jobs SET last_run=?, next_run=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, lastRun.Format(time.RFC3339), nextRun.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("mark run: %w", err)
	}
	return nil
}
