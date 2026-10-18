package overnight

import (
	"time"
)

// PlanStatus represents the lifecycle state of an autonomous overnight task.
type PlanStatus string

const (
	PlanStatusPlanning  PlanStatus = "planning"
	PlanStatusReady     PlanStatus = "ready"
	PlanStatusRunning   PlanStatus = "running"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
	PlanStatusMerged    PlanStatus = "merged"
	PlanStatusDiscarded PlanStatus = "discarded"
)

// SandboxMode determines the isolation mechanism for autonomous execution.
type SandboxMode string

const (
	SandboxGitWorktree SandboxMode = "git_worktree"
	SandboxStaging     SandboxMode = "staging"
)

// PreflightQuestion represents an alignment decision surfaced to the user before sleep.
type PreflightQuestion struct {
	ID            string   `json:"id"`
	Question      string   `json:"question"`
	Options       []string `json:"options,omitempty"`
	DefaultAnswer string   `json:"default_answer"`
	UserAnswer    string   `json:"user_answer"`
	Criticality   string   `json:"criticality"` // "high" or "medium"
}

// DeferredAction tracks a sensitive operation intercepted during autonomous execution.
type DeferredAction struct {
	ID        string                 `json:"id"`
	PlanID    string                 `json:"plan_id"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Reason    string                 `json:"reason"`
	Status    string                 `json:"status"` // "pending", "approved", "rejected"
	CreatedAt time.Time              `json:"created_at"`
}

// ActionPlan encapsulates the pre-flight alignment, sandbox context, and execution status.
type ActionPlan struct {
	ID          string              `json:"id"`
	Goal        string              `json:"goal"`
	Profile     string              `json:"profile"`
	Directives  []string            `json:"directives"`
	Questions   []PreflightQuestion `json:"questions"`
	SandboxMode SandboxMode         `json:"sandbox_mode"`
	SandboxPath string              `json:"sandbox_path"`
	BranchName  string              `json:"branch_name,omitempty"`
	Status      PlanStatus          `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
}

// ReviewPack contains the consolidated artifacts, test results, and diffs for morning review.
type ReviewPack struct {
	PlanID          string           `json:"plan_id"`
	Status          PlanStatus       `json:"status"`
	Goal            string           `json:"goal"`
	Diff            string           `json:"diff,omitempty"`
	TestResults     string           `json:"test_results,omitempty"`
	GeneratedFiles  []string         `json:"generated_files,omitempty"`
	DeferredActions []DeferredAction `json:"deferred_actions,omitempty"`
	Summary         string           `json:"summary"`
	CreatedAt       time.Time        `json:"created_at"`
}
