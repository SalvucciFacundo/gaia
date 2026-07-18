package ports

import (
	"context"
	"io"
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
