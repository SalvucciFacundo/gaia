package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gaia/internal/agent"
)

// TaskRepo implements agent.TaskRepository against the tasks SQLite table.
type TaskRepo struct {
	db *sql.DB
}

// NewTaskRepo creates a TaskRepo backed by the given database connection.
func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// Create persists a new task in Pending state.
func (r *TaskRepo) Create(ctx context.Context, state agent.TaskState) error {
	query := `INSERT INTO tasks (task_id, subagent_name, status, result_json, error_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		state.TaskID, state.SubagentName, string(state.Status),
		"", "", state.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task %q: %w", state.TaskID, err)
	}
	return nil
}

// UpdateStatus transitions a task to a new status with optional result/error.
func (r *TaskRepo) UpdateStatus(ctx context.Context, taskID string, status agent.TaskStatus, resultJSON, errorText string, completedAt time.Time) error {
	query := `UPDATE tasks SET status = ?, result_json = ?, error_text = ?, completed_at = ? WHERE task_id = ?`
	result, err := r.db.ExecContext(ctx, query,
		string(status), resultJSON, errorText, completedAt, taskID,
	)
	if err != nil {
		return fmt.Errorf("update task %q: %w", taskID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %q not found", taskID)
	}
	return nil
}

// LoadActive returns all tasks in non-terminal states (Pending, Running).
func (r *TaskRepo) LoadActive(ctx context.Context) ([]agent.TaskState, error) {
	query := `SELECT task_id, subagent_name, status, result_json, error_text, created_at, completed_at
		FROM tasks WHERE status IN ('pending', 'running')
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load active tasks: %w", err)
	}
	defer rows.Close()

	var states []agent.TaskState
	for rows.Next() {
		var state agent.TaskState
		var statusStr, resultJSON string
		if err := rows.Scan(
			&state.TaskID, &state.SubagentName, &statusStr, &resultJSON,
			&state.Error, &state.CreatedAt, &state.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		state.Status = agent.TaskStatus(statusStr)

		states = append(states, state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	if states == nil {
		states = []agent.TaskState{}
	}
	return states, nil
}
