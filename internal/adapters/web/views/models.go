package views

import (
	"gaia/internal/core/domain"
)

// ProjectViewModel represents a project item in the sidebar.
type ProjectViewModel struct {
	ID          string
	Name        string
	Path        string
	Active      bool
	TaskCount   int
	StatusText  string
}

// SubagentStateViewModel represents a subagent in the execution pipeline visualization.
type SubagentStateViewModel struct {
	Name        string
	Role        string
	Active      bool
	Completed   bool
	Description string
}

// WebDashboardData contains all data needed for the Web UI rendering.
type WebDashboardData struct {
	Projects       []ProjectViewModel
	ActiveProject  ProjectViewModel
	Messages       []domain.Message
	Subagents      []SubagentStateViewModel
	Tasks          []string
	ProviderName   string
	ModelName      string
}
