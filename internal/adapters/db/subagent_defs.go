package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gaia/internal/agent"
)

// DefRepo implements agent.DefRepository against the subagent_defs SQLite table.
type DefRepo struct {
	db *sql.DB
}

// NewDefRepo creates a DefRepo with the given database connection.
func NewDefRepo(db *sql.DB) *DefRepo {
	return &DefRepo{db: db}
}

// CreateDef inserts a new SubagentDef into the subagent_defs table.
// AllowedTools and Skills are serialized as JSON arrays.
func (r *DefRepo) CreateDef(ctx context.Context, def agent.SubagentDef) error {
	toolsJSON, err := json.Marshal(def.AllowedTools)
	if err != nil {
		return fmt.Errorf("marshal allowed_tools: %w", err)
	}
	skillsJSON, err := json.Marshal(def.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}

	now := time.Now()
	query := `INSERT INTO subagent_defs (name, description, allowed_tools, skills, system_prompt, personality, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query,
		def.Name, def.Description,
		string(toolsJSON), string(skillsJSON),
		def.SystemPrompt, def.Personality,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("insert subagent def %q: %w", def.Name, err)
	}
	return nil
}

// GetDef retrieves a single SubagentDef by name.
// Returns nil, nil if not found.
func (r *DefRepo) GetDef(ctx context.Context, name string) (*agent.SubagentDef, error) {
	query := `SELECT name, description, allowed_tools, skills, system_prompt, personality, created_at
		FROM subagent_defs WHERE name = ?`
	row := r.db.QueryRowContext(ctx, query, name)

	var def agent.SubagentDef
	var toolsJSON, skillsJSON string
	var createdAt time.Time

	err := row.Scan(&def.Name, &def.Description, &toolsJSON, &skillsJSON,
		&def.SystemPrompt, &def.Personality, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get subagent def %q: %w", name, err)
	}

	def.CreatedAt = createdAt

	if err := json.Unmarshal([]byte(toolsJSON), &def.AllowedTools); err != nil {
		def.AllowedTools = []string{}
	}
	if err := json.Unmarshal([]byte(skillsJSON), &def.Skills); err != nil {
		def.Skills = []string{}
	}

	return &def, nil
}

// ListDefs retrieves all SubagentDefs ordered by creation time.
func (r *DefRepo) ListDefs(ctx context.Context) ([]agent.SubagentDef, error) {
	query := `SELECT name, description, allowed_tools, skills, system_prompt, personality, created_at
		FROM subagent_defs ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list subagent defs: %w", err)
	}
	defer rows.Close()

	var defs []agent.SubagentDef
	for rows.Next() {
		var def agent.SubagentDef
		var toolsJSON, skillsJSON string
		var createdAt time.Time

		if err := rows.Scan(&def.Name, &def.Description, &toolsJSON, &skillsJSON,
			&def.SystemPrompt, &def.Personality, &createdAt); err != nil {
			return nil, fmt.Errorf("scan subagent def: %w", err)
		}

		def.CreatedAt = createdAt

		if err := json.Unmarshal([]byte(toolsJSON), &def.AllowedTools); err != nil {
			def.AllowedTools = []string{}
		}
		if err := json.Unmarshal([]byte(skillsJSON), &def.Skills); err != nil {
			def.Skills = []string{}
		}

		defs = append(defs, def)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subagent defs: %w", err)
	}

	if defs == nil {
		defs = []agent.SubagentDef{}
	}
	return defs, nil
}

// UpdateDef updates an existing SubagentDef by name.
// AllowedTools and Skills are re-serialized as JSON arrays.
func (r *DefRepo) UpdateDef(ctx context.Context, def agent.SubagentDef) error {
	toolsJSON, err := json.Marshal(def.AllowedTools)
	if err != nil {
		return fmt.Errorf("marshal allowed_tools: %w", err)
	}
	skillsJSON, err := json.Marshal(def.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}

	now := time.Now()
	query := `UPDATE subagent_defs SET description = ?, allowed_tools = ?, skills = ?,
		system_prompt = ?, personality = ?, updated_at = ? WHERE name = ?`
	result, err := r.db.ExecContext(ctx, query,
		def.Description, string(toolsJSON), string(skillsJSON),
		def.SystemPrompt, def.Personality, now, def.Name,
	)
	if err != nil {
		return fmt.Errorf("update subagent def %q: %w", def.Name, err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subagent def %q not found", def.Name)
	}
	return nil
}

// DeleteDef removes a SubagentDef by name.
func (r *DefRepo) DeleteDef(ctx context.Context, name string) error {
	query := `DELETE FROM subagent_defs WHERE name = ?`
	result, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("delete subagent def %q: %w", name, err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subagent def %q not found", name)
	}
	return nil
}
