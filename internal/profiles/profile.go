package profiles

import (
	"context"

	"gaia/internal/core/domain"
)

// Profile represents a modular persona and capability set in GAIA.
type Profile interface {
	ID() string
	Name() string
	Description() string
	SystemPrompt() string
	Tools() []domain.ToolDef
	ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error)
}

// ProfileInfo represents a summary view of a profile.
type ProfileInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	Enabled     bool   `json:"enabled"`
	ToolCount   int    `json:"tool_count"`
}
