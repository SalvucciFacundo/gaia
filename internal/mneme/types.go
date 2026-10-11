// Package mneme implements the long-term memory engine (Mnemosyne) for GAIA.
package mneme

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Role identifies the author of a Message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Observation lifecycle status constants.
const (
	StatusActive      = "active"
	StatusNeedsReview = "needs_review"
)

// Skill source constants.
const (
	SourceImported = "imported"
	SourceAgent    = "agent"
)

// Attachment represents a non-text media payload (image, audio, document).
type Attachment struct {
	Type     string `json:"Type"`
	MimeType string `json:"MimeType"`
	Data     []byte `json:"Data"`
	URL      string `json:"URL"`
	Name     string `json:"Name"`
}

// ToolCall represents a model-requested tool invocation.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef advertises one callable tool to the provider.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Message is a single turn in a Conversation.
type Message struct {
	ID             int64        `json:"id"`
	ConversationID string       `json:"conversation_id"`
	Role           Role         `json:"role"`
	Content        string       `json:"content"`
	CreatedAt      time.Time    `json:"created_at"`
	ToolCalls      []ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID     string       `json:"tool_call_id,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
}

// Conversation is a persisted session.
type Conversation struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Summary       string    `json:"summary"`
	MessageCount  int       `json:"message_count"`
	Goal          string    `json:"goal,omitempty"`
	NextSteps     string    `json:"next_steps,omitempty"`
	RelevantFiles string    `json:"relevant_files,omitempty"`
}

// Observation is a single long-term memory fact. It is keyed by TopicKey:
// re-saving an observation with the same TopicKey updates the existing row
// instead of duplicating it.
type Observation struct {
	ID           string    `json:"id"`
	TopicKey     string    `json:"topic_key"`
	Type         string    `json:"type"`
	Content      string    `json:"content"`
	Importance   int       `json:"importance"`
	Project      string    `json:"project"`
	WorkspaceDir string    `json:"workspace_dir"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SourceRef    string    `json:"source_ref"`
}

// UserModel is one aggregated fact about the user, keyed by Key. Confidence is in [0,1].
type UserModel struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Confidence float64   `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SearchResult is a single full-text or vector match over persisted messages or observations.
type SearchResult struct {
	DocType string `json:"doc_type"`
	DocID   string `json:"doc_id"`
	Content string `json:"content"`
}

// Skill is one unit of procedural memory: reusable instructions for accomplishing a task.
type Skill struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Trigger     string    `json:"trigger"`
	Content     string    `json:"content"`
	Source      string    `json:"source"`
	UsageCount  int       `json:"usage_count"`
	LastUsed    time.Time `json:"last_used"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditEntry is one audited security-relevant event.
type AuditEntry struct {
	ID       int64     `json:"id"`
	Time     time.Time `json:"time"`
	Backend  string    `json:"backend"`
	Category string    `json:"category"`
	Subject  string    `json:"subject"`
	Decision string    `json:"decision"`
	Scope    string    `json:"scope"`
}

// Snapshot is a point-in-time copy of a conversation and its messages.
type Snapshot struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	MessagesJSON   string    `json:"messages_json"`
	CreatedAt      time.Time `json:"created_at"`
}

// Embedder abstracts dense vector embedding generation for text inputs.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// ModelInfo describes one available model.
type ModelInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

// ChatRequest is the input to a Provider.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
}

// ChatResponse is the complete Provider reply.
type ChatResponse struct {
	Content string `json:"content"`
}

// StreamEvent is one event of a streaming response.
type StreamEvent struct {
	Text     string
	ToolCall *ToolCall
	Err      error
}

// Provider is the LLM port.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	Models() []ModelInfo
}

// Nudger is the consumer-side interface for periodic curation.
type Nudger interface {
	Nudge(ctx context.Context, convID string, msgs []Message) ([]Observation, error)
}

// SessionCloser is the consumer-side interface for close-time summarization.
type SessionCloser interface {
	Close(ctx context.Context, convID string, msgs []Message) error
}

// Store is the persistence port interface for Mnemosyne.
type Store interface {
	CreateConversation(ctx context.Context, title string) (*Conversation, error)
	LatestConversation(ctx context.Context) (*Conversation, error)
	AppendMessage(ctx context.Context, convID string, msg Message) error
	Messages(ctx context.Context, convID string, limit int) ([]Message, error)
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	SaveObservations(ctx context.Context, convID string, obs []Observation) error
	Observations(ctx context.Context, limit int) ([]Observation, error)
	ObservationsByProject(ctx context.Context, project string, limit int) ([]Observation, error)

	UpdateConversationSummary(ctx context.Context, convID, summary string) error
	UpsertUserModel(ctx context.Context, rows []UserModel) error
	UserModelRows(ctx context.Context, limit int) ([]UserModel, error)
	ClearUserModel(ctx context.Context) error
	RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error

	SaveSkill(ctx context.Context, skill Skill) error
	ListSkills(ctx context.Context) ([]Skill, error)
	RecordSkillUsage(ctx context.Context, name string) error
	DeleteSkill(ctx context.Context, name string) error

	AppendAudit(ctx context.Context, entry AuditEntry) error
	AuditTail(ctx context.Context, n int) ([]AuditEntry, error)

	ListConversations(ctx context.Context, limit, offset int) ([]Conversation, error)
	GetConversation(ctx context.Context, id string) (*Conversation, error)
	RenameConversation(ctx context.Context, id, title string) error
	DeleteConversation(ctx context.Context, id string) error

	CreateSnapshot(ctx context.Context, convID string) (*Snapshot, error)
	ListSnapshots(ctx context.Context, convID string) ([]Snapshot, error)

	Close() error
}

// Repo is an alias for Store.
type Repo = Store
