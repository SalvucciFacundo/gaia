package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Brain orchestrates the agent loop: receive user input, call LLM,
// execute tool calls, enforce budget, delegate to subagents, and return results.
type Brain struct {
	provider     ports.LLMProvider
	repo         ports.Repository
	registry     *ToolRegistry
	guard        *ConfirmGuard
	ui           ports.UIService
	budget       domain.BudgetConfig
	onToken      func(string)           // streaming callback
	subagentPort ports.SubagentPort      // subagent delegation (nil if not wired)
}

// BrainOption configures the Brain.
type BrainOption func(*Brain)

// WithTokenCallback sets a function called for each streaming token.
func WithTokenCallback(fn func(string)) BrainOption {
	return func(b *Brain) { b.onToken = fn }
}

// NewBrain creates a new Brain with the given dependencies.
func NewBrain(provider ports.LLMProvider, repo ports.Repository, ui ports.UIService, guard *ConfirmGuard, budget domain.BudgetConfig, opts ...BrainOption) *Brain {
	b := &Brain{
		provider: provider,
		repo:     repo,
		registry: NewToolRegistry(),
		guard:    guard,
		ui:       ui,
		budget:   budget,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// RegisterModule adds a module's tools to the registry.
func (b *Brain) RegisterModule(mod ports.Module) {
	b.registry.Register(mod)
}

// SetSubagentPort wires the subagent infrastructure into the Brain.
// Pass nil to disable subagent delegation.
func (b *Brain) SetSubagentPort(sp ports.SubagentPort) {
	b.subagentPort = sp
}

// Delegate dispatches a task to a named subagent and returns the structured result.
// Returns nil, error if no subagent port is wired or the subagent is unknown.
func (b *Brain) Delegate(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if b.subagentPort == nil {
		return nil, fmt.Errorf("subagent port not wired")
	}
	return b.subagentPort.Spawn(ctx, name, task)
}

// AvailableSubagents returns the list of registered subagent names.
func (b *Brain) AvailableSubagents() []string {
	if b.subagentPort == nil {
		return nil
	}
	return b.subagentPort.Available()
}

// Registry returns the brain's tool registry for use by subagent infrastructure.
func (b *Brain) Registry() *ToolRegistry {
	return b.registry
}

// ProcessMessage handles a user input through the full agent loop.
func (b *Brain) ProcessMessage(ctx context.Context, content string) error {
	// 1. Create user message
	userMsg := domain.Message{
		Role:    domain.RoleUser,
		Content: content,
	}
	if err := b.repo.SaveMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	// 2. Iteration loop with budget
	for iter := 0; iter < b.budget.MaxIterations; iter++ {
		history, err := b.repo.GetHistory(ctx, 50)
		if err != nil {
			return fmt.Errorf("get history: %w", err)
		}

		// 3. Call LLM — prefer streaming, fall back to non-streaming
		var response *domain.Message
		stream, err := b.provider.Stream(ctx, history)
		if err != nil {
			// Fall back to non-streaming Chat for this iteration
			resp, chatErr := b.provider.Chat(ctx, history)
			if chatErr != nil {
				return fmt.Errorf("llm error: %w", chatErr)
			}
			response = resp
		} else {
			response, err = b.readStream(ctx, stream)
			stream.Close()
			if err != nil {
				// Fall back to non-streaming on read failure
				resp, chatErr := b.provider.Chat(ctx, history)
				if chatErr != nil {
					return fmt.Errorf("llm error: %w", chatErr)
				}
				response = resp
			}
		}

		// 4. Handle tool calls
		if len(response.ToolCalls) > 0 {
			if err := b.handleToolCalls(ctx, response); err != nil {
				return err
			}
			continue // Let LLM see results
		}

		// 5. Save and display final response
		if err := b.repo.SaveMessage(ctx, *response); err != nil {
			return fmt.Errorf("save assistant response: %w", err)
		}
		return b.ui.Display(*response)
	}

	// Budget exhausted
	errMsg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("Iteration budget exhausted (%d iterations). Stopping.", b.budget.MaxIterations),
	}
	b.repo.SaveMessage(ctx, *errMsg)
	return b.ui.Display(*errMsg)
}

// readStream reads token chunks from the stream and builds a final message.
func (b *Brain) readStream(ctx context.Context, reader io.Reader) (*domain.Message, error) {
	response := &domain.Message{Role: domain.RoleAssistant}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		var chunk domain.TokenChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("stream error: %s", chunk.Error)
		}
		response.Content += chunk.Content
		if b.onToken != nil {
			b.onToken(chunk.Content)
		}
	}
	return response, scanner.Err()
}

func (b *Brain) handleToolCalls(ctx context.Context, msg *domain.Message) error {
	for _, tc := range msg.ToolCalls {
		// Confirmation gate
		if b.guard != nil && b.guard.ShouldConfirm(tc.Name) {
			confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf("Allow tool %s with args %v?", tc.Name, tc.Arguments))
			if err != nil || !confirmed {
				toolMsg := domain.Message{
					Role:    domain.RoleTool,
					Content: "User denied tool execution.",
				}
				b.repo.SaveMessage(ctx, toolMsg)
				continue
			}
			b.guard.Approve(tc.Name)
		}

		// Execute via registry
		result, execErr := b.registry.Execute(ctx, tc.Name, tc.Arguments)
		if execErr != nil {
			result = &domain.ToolResult{
				Success: false,
				Error:   execErr.Error(),
			}
		}

		output := result.Output
		if !result.Success {
			output = fmt.Sprintf("Error: %s", result.Error)
		}

		toolMsg := domain.Message{
			Role:    domain.RoleTool,
			Content: output,
		}
		b.repo.SaveMessage(ctx, toolMsg)
	}

	// Save the assistant message that triggered these tool calls
	b.repo.SaveMessage(ctx, *msg)
	return nil
}
