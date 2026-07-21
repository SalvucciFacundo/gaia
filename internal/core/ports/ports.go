package ports

import (
	"context"
	"io"
	"time"

	"gaia/internal/core/domain"
)

// TokenStream is an io.ReadCloser that yields normalized SSE token chunks.
type TokenStream = io.ReadCloser

// ChatOpt is a functional option for Chat/Stream calls.
type ChatOpt func(*ChatOptions)

// ChatOptions holds parameters for a Chat or Stream call.
type ChatOptions struct {
	Temperature float64
	MaxTokens   int
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) ChatOpt {
	return func(o *ChatOptions) { o.Temperature = t }
}

// WithMaxTokens sets the max output tokens.
func WithMaxTokens(n int) ChatOpt {
	return func(o *ChatOptions) { o.MaxTokens = n }
}

// LLMProvider is the interface for AI model interactions.
type LLMProvider interface {
	Chat(ctx context.Context, messages []domain.Message, opts ...ChatOpt) (*domain.Message, error)
	Stream(ctx context.Context, messages []domain.Message, opts ...ChatOpt) (TokenStream, error)
	Tools() []domain.ToolDef
}

// Repository handles persistence of conversations and brain data.
type Repository interface {
	SaveMessage(ctx context.Context, msg domain.Message) error
	GetHistory(ctx context.Context, limit int) ([]domain.Message, error)
	CreateSession(ctx context.Context, name string) (string, error)
	GetMessages(ctx context.Context, sessionID string, limit int) ([]domain.Message, error)
	// GetMessageCount returns the total number of stored messages.
	GetMessageCount(ctx context.Context) (int, error)
	// GetHistoryFrom returns messages starting at the given offset (0-based, oldest first).
	GetHistoryFrom(ctx context.Context, limit, offset int) ([]domain.Message, error)
	// GetLastMessages returns the most recent N messages, newest first.
	GetLastMessages(ctx context.Context, n int) ([]domain.Message, error)
	// DeleteMessagesAfter deletes all messages with created_at >= the given message's timestamp.
	DeleteMessagesAfter(ctx context.Context, afterID string) error
}

// UIService handles the terminal interaction.
type UIService interface {
	Display(msg domain.Message) error
	AppendToken(content string) error
	PromptConfirmation(prompt string) (bool, error)
	Run() error
}

// MessagingService handles external communication (Telegram).
type MessagingService interface {
	SendMessage(chatID int64, text string) error
	Start() error
}

// Module defines the contract for an on-demand plugin/toolset.
type Module interface {
	Name() string
	Description() string
	GetTools() []domain.ToolCall // Definitions
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (*domain.ToolResult, error)
}

// SubagentPort defines the contract for spawning and managing subagents.
// Implemented by the agent.Spawner; consumed by the Brain for delegation.
type SubagentPort interface {
	Spawn(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error)
	Available() []string
}

// AsyncSpawner extends SubagentPort with async spawning and task management.
// Implemented by agent.Spawner when TaskManager is wired.
type AsyncSpawner interface {
	SubagentPort
	SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error)
	TaskManager() interface{} // *agent.TaskManager (avoiding circular import)
	// RunPipeline executes SDD phases sequentially via SpawnAsync.
	// Each phase's result feeds into the next as KGContext.
	// Returns collected results from all completed phases.
	RunPipeline(ctx context.Context, phases []domain.PipelinePhase, baseTask domain.SubagentTask) ([]*domain.SubagentResult, error)
}

// CronRepository defines the contract for cron job persistence.
type CronRepository interface {
	ListJobs(ctx context.Context) ([]domain.CronJob, error)
	CreateJob(ctx context.Context, job domain.CronJob) (string, error)
	UpdateJob(ctx context.Context, job domain.CronJob) error
	DeleteJob(ctx context.Context, id string) error
	GetDueJobs(ctx context.Context) ([]domain.CronJob, error)
	MarkRun(ctx context.Context, id string, lastRun time.Time, nextRun time.Time) error
}

// GatewayAdapter is the port for a messaging platform adapter.
type GatewayAdapter interface {
	Name() string
	Start(ctx context.Context, handler MessageHandler) error
	Stop() error
	Send(ctx context.Context, target string, content string) error
}

// MessageHandler processes an incoming gateway message and returns a response.
type MessageHandler func(ctx context.Context, msg IncomingMessage) (string, error)

// KnowledgeGraphStore defines storage and retrieval for the shared knowledge graph.
// Facts are organized as Topic → Concept → Fact with source attribution.
type KnowledgeGraphStore interface {
	// AddFact stores a new fact. ID may be auto-generated if empty.
	AddFact(ctx context.Context, fact domain.KnowledgeFact) (string, error)
	// GetFactsByTopic returns all facts for a given topic.
	GetFactsByTopic(ctx context.Context, topic string) ([]domain.KnowledgeFact, error)
	// GetFactsByConcept returns all facts under a specific topic+concept.
	GetFactsByConcept(ctx context.Context, topic, concept string) ([]domain.KnowledgeFact, error)
	// SearchFacts performs full-text search across all facts.
	SearchFacts(ctx context.Context, query string) ([]domain.KnowledgeFact, error)
	// GetRecentFacts returns the most recent N facts across all topics.
	GetRecentFacts(ctx context.Context, limit int) ([]domain.KnowledgeFact, error)
	// GetAllTopics returns all distinct topic names.
	GetAllTopics(ctx context.Context) ([]string, error)
	// GetRecentTopics returns topics that have facts newer than the given duration.
	GetRecentTopics(ctx context.Context, since time.Duration) ([]string, error)
}

// IncomingMessage is a normalized message from any gateway adapter.
type IncomingMessage struct {
	Platform   string
	SenderID   string
	SenderName string
	Content    string
	ChatID     string
	ThreadID   string // optional
}

