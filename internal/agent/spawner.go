package agent

import (
	"context"
	"fmt"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// SpawnerConfig holds the dependencies needed to spawn subagent executions.
type SpawnerConfig struct {
	Provider     ports.LLMProvider
	Tools        *core.ToolRegistry // Full tool registry; filtered per subagent
	Budget       domain.BudgetConfig
	Namespace    *memory.NamespaceManager // Engram namespace wrapper
	TaskManager  *TaskManager             // Async task lifecycle tracking (nil = sync-only mode)
	MoAProviders map[string]ports.LLMProvider // providers for MoA fan-out, keyed by name
}

// Spawner implements ports.SubagentPort. It creates isolated execution
// contexts for subagents: filtered tools, scoped system prompts, and
// fresh message history. Tool filtering is enforced at the Spawner level,
// not self-reported by subagents.
type Spawner struct {
	cfg        SpawnerConfig
	registry   *Registry
	learnLoop  *learn.LearningLoop // Tracks per-subagent execution counts
}

// NewSpawner creates a Spawner with the given configuration and subagent registry.
func NewSpawner(cfg SpawnerConfig, registry *Registry) *Spawner {
	if cfg.Budget.MaxIterations <= 0 {
		cfg.Budget = domain.DefaultBudget()
	}
	return &Spawner{
		cfg:       cfg,
		registry:  registry,
		learnLoop: learn.NewLearningLoop(5),
	}
}

// SetLearningLoop replaces the default learning loop with a custom one.
// Pass nil to disable learning nudges.
func (s *Spawner) SetLearningLoop(l *learn.LearningLoop) {
	s.learnLoop = l
}

// LearningLoop returns the spawner's learning loop (may be nil if disabled).
func (s *Spawner) LearningLoop() *learn.LearningLoop {
	return s.learnLoop
}

// Namespace returns the Engram namespace manager (may be nil if not configured).
func (s *Spawner) Namespace() *memory.NamespaceManager {
	return s.cfg.Namespace
}

// TaskManager returns the async task manager (may be nil if not configured).
func (s *Spawner) TaskManager() *TaskManager {
	return s.cfg.TaskManager
}

// SpawnAsync launches a subagent execution in a goroutine and returns immediately.
// The goroutine wraps the existing synchronous Spawn logic with panic recovery.
// Returns the TaskID immediately. The caller can track completion via TaskManager.
//
// If SpawnerConfig.TaskManager is nil, returns an error — async mode is disabled.
func (s *Spawner) SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error) {
	if s.cfg.TaskManager == nil {
		return "", fmt.Errorf("async spawning is disabled: TaskManager not configured")
	}

	if _, ok := s.registry.Get(name); !ok {
		return "", fmt.Errorf("unknown subagent: %s", name)
	}

	taskID, taskCtx := s.cfg.TaskManager.CreateTask(name, task)
	s.cfg.TaskManager.UpdateStatus(taskID, TaskRunning, nil, nil)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in subagent %q: %v", name, r)
				s.cfg.TaskManager.UpdateStatus(taskID, TaskFailed, nil, err)
			}
		}()

		result, err := s.Spawn(taskCtx, name, task)
		if err != nil {
			s.cfg.TaskManager.UpdateStatus(taskID, TaskFailed, nil, err)
			return
		}

		s.cfg.TaskManager.UpdateStatus(taskID, TaskCompleted, result, nil)
	}()

	return taskID, nil
}

// Spawn executes a named subagent with the given task.
// It looks up the subagent factory, creates the subagent, calls Execute,
// and returns the structured result. It also records the execution in
// the learning loop for nudge detection.
func (s *Spawner) Spawn(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	factory, ok := s.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown subagent: %s", name)
	}

	sub := factory(s)
	result := sub.Execute(ctx, task)
	if result == nil {
		return &domain.SubagentResult{
			Status:  domain.SubagentBlocked,
			Summary: fmt.Sprintf("subagent %q returned nil result", name),
		}, nil
	}

	// Record execution for learning loop
	if s.learnLoop != nil {
		s.learnLoop.RecordExecution(name)
	}

	return result, nil
}

// Available returns the list of registered subagent names.
func (s *Spawner) Available() []string {
	return s.registry.Available()
}

// RunLoop is a helper that subagent implementations can use to run a
// simplified agent loop with filtered tools. It handles LLM calls,
// tool execution via the filtered registry, and budget enforcement.
//
// The caller provides a system prompt and the initial task message.
// RunLoop returns the final assistant message (typically the subagent's summary).
//
// RunLoop applies message redaction to all tool outputs before feeding
// them back to the LLM.
//
// When task.MoA.Enabled is true and MoAProviders are configured, the first
// iteration uses MoA (fan-out to multiple models + synthesis) instead of
// a single provider call. Subsequent iterations (tool call loops) always
// use the single provider for consistency.
func (s *Spawner) RunLoop(ctx context.Context, task domain.SubagentTask, systemPrompt string) (*domain.Message, error) {
	filtered := s.cfg.Tools.Filtered(task.AllowedTools)

	messages := []domain.Message{
		{Role: domain.RoleSystem, Content: systemPrompt},
		{Role: domain.RoleUser, Content: task.Description},
	}

	for iter := 0; iter < s.cfg.Budget.MaxIterations; iter++ {
		// Check for cancellation before each LLM call
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var resp *domain.Message
		var err error

		// First iteration + MoA enabled → use MoA fan-out
		if iter == 0 && task.MoA.Enabled && len(s.cfg.MoAProviders) > 0 {
			runner, moaErr := newMoARunner(s.cfg.Provider, s.cfg.MoAProviders, task.MoA)
			if moaErr == nil && runner != nil {
				resp, err = runner.run(ctx, messages)
			}
			// If MoA failed or returned nothing, fall back to single provider
			if resp == nil || err != nil {
				resp, err = s.cfg.Provider.Chat(ctx, messages)
			}
		} else {
			resp, err = s.cfg.Provider.Chat(ctx, messages)
		}

		if err != nil {
			return nil, fmt.Errorf("llm chat: %w", err)
		}

		// If the model returns tool calls, execute them and feed results back
		if len(resp.ToolCalls) > 0 {
			messages = append(messages, *resp)
			for _, tc := range resp.ToolCalls {
				// Check cancellation between tool executions
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				toolResult, execErr := filtered.Execute(ctx, tc.Name, tc.Arguments)
				if execErr != nil {
					toolResult = &domain.ToolResult{
						Success: false,
						Error:   execErr.Error(),
					}
				}
				output := toolResult.Output
				if !toolResult.Success {
					output = fmt.Sprintf("Error: %s", toolResult.Error)
				}

				// Redact sensitive content from tool output
				output, _ = RedactContent(output)

				messages = append(messages, domain.Message{
					Role:    domain.RoleTool,
					Content: output,
				})
			}
			continue
		}

		// No tool calls — this is the final response
		return resp, nil
	}

	return &domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("Subagent budget exhausted after %d iterations.", s.cfg.Budget.MaxIterations),
	}, nil
}

// BuildSystemPrompt constructs a system prompt for a subagent from the task context.
// Exported for use by subagent implementations.
func BuildSystemPrompt(role, description string, task domain.SubagentTask) string {
	prompt := fmt.Sprintf("You are the %s subagent. %s\n\n", role, description)
	prompt += fmt.Sprintf("Task ID: %s\n", task.ID)
	prompt += fmt.Sprintf("Task: %s\n", task.Description)

	if len(task.KGContext) > 0 {
		prompt += "\nRelevant knowledge:\n"
		for _, fact := range task.KGContext {
			prompt += fmt.Sprintf("- %s\n", fact)
		}
	}

	if len(task.Skills) > 0 {
		prompt += "\nSkills to load: "
		for i, s := range task.Skills {
			if i > 0 {
				prompt += ", "
			}
			prompt += s
		}
		prompt += "\n"
	}

	if task.Mode != "" {
		prompt += fmt.Sprintf("\nExecution mode: %s\n", task.Mode)
	}

	prompt += "\nReturn your result as a structured summary with these sections:\n"
	prompt += "- Status (success, partial, or blocked)\n"
	prompt += "- Artifacts produced (list)\n"
	prompt += "- Next recommended phase (or none)\n"
	prompt += "- Risks discovered (list, or none)\n"
	prompt += "- Skill resolution (how skills were loaded)\n"

	return prompt
}
