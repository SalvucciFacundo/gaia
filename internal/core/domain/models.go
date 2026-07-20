package domain

import (
	"fmt"
	"time"
)

// Role defines the sender of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a single chat message.
type Message struct {
	ID        string     `json:"id"`
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a request from the LLM to execute a tool.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolDef defines a tool schema the LLM provider can expose.
type ToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema object
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// TokenChunk represents a streaming response fragment.
type TokenChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// TrustMode defines confirmation behavior for tool execution.
type TrustMode string

const (
	TrustAlways    TrustMode = "always"
	TrustPerSession TrustMode = "per-session"
	TrustPerAction  TrustMode = "per-action"
	TrustNever      TrustMode = "never"
)

// PipelinePhase represents one step in an async SDD pipeline.
type PipelinePhase struct {
	SubagentName string // Name of the subagent to spawn
	Description  string // Task description for this phase
}

// SDDPhases returns the standard 7-phase SDD pipeline definition.
func SDDPhases(taskDesc string) []PipelinePhase {
	return []PipelinePhase{
		{SubagentName: "explorer", Description: taskDesc},
		{SubagentName: "proposer", Description: fmt.Sprintf("Create SDD proposal for: %s", taskDesc)},
		{SubagentName: "specifier", Description: fmt.Sprintf("Write delta specs based on proposal for: %s", taskDesc)},
		{SubagentName: "designer", Description: fmt.Sprintf("Create technical design for: %s", taskDesc)},
		{SubagentName: "planner", Description: fmt.Sprintf("Break into implementation tasks for: %s", taskDesc)},
		{SubagentName: "implementer", Description: fmt.Sprintf("Implement from specs and tasks for: %s", taskDesc)},
		{SubagentName: "verifier", Description: fmt.Sprintf("Verify implementation for: %s", taskDesc)},
	}
}

// BudgetConfig defines iteration limits for the agent loop.
type BudgetConfig struct {
	MaxIterations int `yaml:"max_iterations"`
}

// Config represents the application configuration.
type Config struct {
	APIKeys map[string]string `yaml:"api_keys"`
	LLM     struct {
		Provider      string   `yaml:"provider"`
		Model         string   `yaml:"model"`
		FallbackChain []string `yaml:"fallback_chain"`
		TrustMode     string   `yaml:"trust_mode"`
	} `yaml:"llm"`
	Budget   BudgetConfig   `yaml:"budget"`
	Telegram struct {
		Token          string  `yaml:"token"`
		AllowedUserIDs []int64 `yaml:"allowed_user_ids"`
	} `yaml:"telegram"`
	System struct {
		RequiresConfirmation bool   `yaml:"requires_confirmation"`
		Language             string `yaml:"language"` // User's preferred language (en, es, pt)
	} `yaml:"system"`
	Terminal TerminalConfig `yaml:"terminal"`
	MCP     MCPConfig     `yaml:"mcp"`
}

// MCPConfig defines MCP (Model Context Protocol) client settings.
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig defines connection settings for an MCP server.
type MCPServerConfig struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// TerminalConfig defines the execution backend for shell commands.
type TerminalConfig struct {
	Backend string       `yaml:"backend"` // "local", "docker", "ssh"
	Docker  DockerConfig `yaml:"docker"`
	SSH     SSHConfig    `yaml:"ssh"`
}

// DockerConfig holds container settings for the Docker executor.
type DockerConfig struct {
	Container string `yaml:"container"` // container name or ID
	WorkDir   string `yaml:"workdir"`   // working dir inside container
}

// SSHConfig holds connection settings for the SSH executor.
type SSHConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	User       string `yaml:"user"`
	KeyPath    string `yaml:"key_path"`
	KnownHosts string `yaml:"known_hosts"`
}

// DefaultBudget returns a sensible default budget config.
func DefaultBudget() BudgetConfig {
	return BudgetConfig{MaxIterations: 25}
}

// SubagentStatus represents the outcome of a subagent execution.
type SubagentStatus string

const (
	SubagentSuccess SubagentStatus = "success"
	SubagentPartial SubagentStatus = "partial"
	SubagentBlocked SubagentStatus = "blocked"
)

// SubagentTask is a self-contained work unit sent to a subagent.
type SubagentTask struct {
	ID           string   // Unique identifier for this task
	Description  string   // Human-readable instruction for the subagent
	KGContext    []string // Relevant knowledge graph facts
	Skills       []string // Skill names to load before execution
	AllowedTools []string // Tool names allowed for this subagent; empty = all
	Mode         string   // Execution mode: "plan" or "build"
	IsDirectChat bool     // True when routed via @name syntax; subagent responds directly to user
}

// ReviewState represents the state of a review transaction in the formal state machine.
type ReviewState string

const (
	ReviewStateUnreviewed            ReviewState = "unreviewed"
	ReviewStateReviewing             ReviewState = "reviewing"
	ReviewStateJudgesConfirmed       ReviewState = "judges_confirmed"
	ReviewStateFindingsFrozen        ReviewState = "findings_frozen"
	ReviewStateEvidenceClassified    ReviewState = "evidence_classified"
	ReviewStateFixRequired           ReviewState = "fix_required"
	ReviewStateFixing                ReviewState = "fixing"
	ReviewStateFixValidating         ReviewState = "fix_validating"
	ReviewStateReadyFinalVerification ReviewState = "ready_final_verification"
	ReviewStateFinalVerifying        ReviewState = "final_verifying"
	ReviewStateApproved              ReviewState = "approved"
	ReviewStateEscalated             ReviewState = "escalated"
	ReviewStateInvalidated           ReviewState = "invalidated"
)

// ReviewFinding represents a classified finding from a review lens.
type ReviewFinding struct {
	Lens       string `json:"lens"`       // "review-risk", "review-resilience", etc.
	Severity   string `json:"severity"`   // "BLOCKER", "WARNING", "SUGGESTION"
	File       string `json:"file"`       // path to the file
	Line       int    `json:"line"`       // line number (0 if file-level)
	Message    string `json:"message"`    // human-readable finding
	Suggestion string `json:"suggestion"` // concrete fix suggestion
}

// ReviewReceipt is the content-bound receipt produced by a review.
type ReviewReceipt struct {
	Schema                string         `json:"schema"`                  // "gentle-ai.review-receipt/v2"
	LineageID             string         `json:"lineage_id"`              // SHA256 of review transaction chain
	SnapshotHash          string         `json:"snapshot_hash"`           // "sha256:{hash of all reviewed files}"
	SelectedLenses        []string       `json:"selected_lenses"`         // ["review-risk", "review-readability"]
	RiskLevel             string         `json:"risk_level"`              // "low", "medium", "high"
	RiskReasons           []string       `json:"risk_reasons"`            // ["hot_path", "large_change"]
	CorrectionBudget      int            `json:"correction_budget"`       // max correction tokens (85 default)
	CorrectionUsed        int            `json:"correction_used"`         // tokens used so far
	State                 ReviewState    `json:"state"`                   // state machine state
	FinalVerificationHash string         `json:"final_verification_hash"` // "sha256:{verification evidence}"
	Findings              []ReviewFinding `json:"findings"`               // classified findings
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// SubagentResult is the structured envelope returned by a subagent.
type SubagentResult struct {
	Status          SubagentStatus // "success", "partial", or "blocked"
	Summary         string         // Human-readable summary of what happened
	Artifacts       []string       // Artifact keys or paths produced
	NextRecommended string         // Next recommended phase, or "none"
	Risks           []string       // Risks discovered during execution
	SkillResolution string         // How skills were resolved (paths-injected, fallback-registry, none)
}

// CronJob represents a scheduled task definition.
type CronJob struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Schedule      string    `json:"schedule"`       // cron expression (e.g. "0 2 * * *")
	Task          string    `json:"task"`            // description sent to the Brain
	DeliverTo     string    `json:"deliver_to"`      // "terminal", "telegram", "gateway"
	DeliverTarget string    `json:"deliver_target"`  // chat ID for telegram, channel for gateway
	Enabled       bool      `json:"enabled"`
	LastRun       time.Time `json:"last_run"`
	NextRun       time.Time `json:"next_run"`
	CreatedAt     time.Time `json:"created_at"`
}

// KnowledgeFact is a single atomic fact in the shared knowledge graph.
// Facts are organized as Topic → Concept → Fact with source attribution.
type KnowledgeFact struct {
	ID          string    `json:"id"`
	Topic       string    `json:"topic"`       // e.g. "Authentication"
	Concept     string    `json:"concept"`     // e.g. "JWT in this project"
	Fact        string    `json:"fact"`        // e.g. "Tokens expire in 24h, refresh in 7d"
	SourceAgent string    `json:"source_agent"` // e.g. "designer"
	Labels      []string  `json:"labels"`       // e.g. ["security", "auth", "jwt"]
	CreatedAt   time.Time `json:"created_at"`
}

// GatewayConfig defines messaging gateway settings.
type GatewayConfig struct {
	Enabled   bool                   `yaml:"enabled"`
	Telegram  TelegramGatewayConfig  `yaml:"telegram"`
	Discord   DiscordGatewayConfig   `yaml:"discord"`
	BrowserTools BrowserToolsConfig  `yaml:"browser_tools"`
}

// TelegramGatewayConfig holds Telegram gateway adapter settings.
type TelegramGatewayConfig struct {
	Enabled        bool    `yaml:"enabled"`
	Token          string  `yaml:"token"`
	AllowedUserIDs []int64 `yaml:"allowed_user_ids"`
}

// DiscordGatewayConfig holds Discord gateway adapter settings via MCP.
type DiscordGatewayConfig struct {
	Enabled bool   `yaml:"enabled"`
	Command string `yaml:"command"` // path to discord-mcp binary
}

// BrowserToolsConfig holds optional browser automation MCP settings.
type BrowserToolsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Command string `yaml:"command"` // path to browser MCP server
}

