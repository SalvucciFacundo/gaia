package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gaia/internal/agent/learn"
	"gaia/internal/agent/memory"
	"gaia/internal/blackboard"
	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// SpawnerConfig holds the dependencies needed to spawn subagent executions.
type SpawnerConfig struct {
	Provider       ports.LLMProvider
	Tools          *core.ToolRegistry // Full tool registry; filtered per subagent
	Budget         domain.BudgetConfig
	Namespace      *memory.NamespaceManager                                     // Engram namespace wrapper
	TaskManager    *TaskManager                                                 // Async task lifecycle tracking (nil = sync-only mode)
	MoAProviders   map[string]ports.LLMProvider                                 // providers for MoA fan-out, keyed by name
	Policy         *core.PolicyGuard                                            // policy evaluation guard (nil = no policy enforcement)
	AuditFunc      func(name string, args map[string]interface{}, allowed bool) // audit trail callback
	DefaultTimeout time.Duration                                                // optional timeout for Spawn execution
	WorktreeMgr    *WorktreeManager                                             // Git worktree isolation for async tasks
	Blackboard     *blackboard.Blackboard                                       // L1 working memory ledger (nil = disabled)
}

// Spawner implements ports.SubagentPort. It creates isolated execution
// contexts for subagents: filtered tools, scoped system prompts, and
// fresh message history. Tool filtering is enforced at the Spawner level,
// not self-reported by subagents.
type Spawner struct {
	cfg       SpawnerConfig
	registry  *Registry
	learnLoop           *learn.LearningLoop // Tracks per-subagent execution counts
	onLearningThreshold func(string)        // Called when a subagent reaches the learning threshold
}

// NewSpawner creates a Spawner with the given configuration and subagent registry.
func NewSpawner(cfg SpawnerConfig, registry *Registry) *Spawner {
	if cfg.Budget.MaxIterations <= 0 {
		cfg.Budget = domain.DefaultBudget()
	}
	if cfg.WorktreeMgr == nil {
		cfg.WorktreeMgr = NewWorktreeManager()
	}
	return &Spawner{
		cfg:       cfg,
		registry:  registry,
		learnLoop: learn.NewLearningLoop(5),
	}
}

// SetOnLearningThreshold sets the callback invoked when a subagent hits the learning threshold.
func (s *Spawner) SetOnLearningThreshold(fn func(string)) {
	s.onLearningThreshold = fn
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

// Blackboard returns the spawner's Blackboard working memory (may be nil if not configured).
func (s *Spawner) Blackboard() *blackboard.Blackboard {
	return s.cfg.Blackboard
}

// SpawnAsync launches a subagent execution in a goroutine and returns immediately.
// The goroutine provisions an isolated git worktree for write-capable subagents,
// executes the subagent, attaches the diff/patch to the result, and cleans up the worktree.
// Returns the TaskID immediately. The caller can track completion via TaskManager.
//
// If SpawnerConfig.TaskManager is nil, returns an error — async mode is disabled.
func (s *Spawner) SpawnAsync(ctx context.Context, name string, task domain.SubagentTask) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	if s.cfg.TaskManager == nil {
		return "", fmt.Errorf("async spawning is disabled: TaskManager not configured")
	}

	if _, ok := s.registry.Get(name); !ok {
		return "", fmt.Errorf("unknown subagent: %s", name)
	}

	taskID, taskCtx := s.cfg.TaskManager.CreateTask(name, task)
	s.cfg.TaskManager.UpdateStatus(taskID, TaskRunning, nil, nil)

	go func() {
		var wt *WorktreeContext
		if s.cfg.WorktreeMgr != nil && (name == "implementer" || name == "debugger") {
			if createdWT, err := s.cfg.WorktreeMgr.Create(taskCtx, ".", taskID); err == nil {
				wt = createdWT
				task.WorkDir = wt.WorktreeDir
				defer func() {
					_ = s.cfg.WorktreeMgr.Remove(context.Background(), wt)
				}()
			}
		}

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

		// If worktree was used, extract diff and attach to artifacts
		if wt != nil && s.cfg.WorktreeMgr != nil && result != nil {
			if diff, diffErr := s.cfg.WorktreeMgr.GetDiff(taskCtx, wt); diffErr == nil && diff != "" {
				result.Artifacts = append(result.Artifacts, fmt.Sprintf("worktree-diff (branch: %s)", wt.BranchName))
			}
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
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	factory, ok := s.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown subagent: %s", name)
	}

	if s.cfg.DefaultTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.DefaultTimeout)
			defer cancel()
		}
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
		if thresholdReached := s.learnLoop.RecordExecution(name); thresholdReached && s.onLearningThreshold != nil {
			s.onLearningThreshold(name)
		}
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
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	filtered := s.cfg.Tools.Filtered(task.AllowedTools)

	if len(task.SkillPaths) > 0 {
		systemPrompt += "\n## Skills to load before work\n" +
			"Read these skill files using file_read before performing task work:\n"
		for _, p := range task.SkillPaths {
			systemPrompt += fmt.Sprintf("- %s\n", p)
		}
	}

	// Resolve and inject Blackboard input keys if specified
	if s.cfg.Blackboard != nil && len(task.InputKeys) > 0 {
		var bbFacts []string
		for _, key := range task.InputKeys {
			if entry, err := s.cfg.Blackboard.Get(key); err == nil {
				payload, _ := blackboard.ReadPayload(entry)
				bbFacts = append(bbFacts, fmt.Sprintf("- Key %q: %s", key, string(payload)))
			}
		}
		if len(bbFacts) > 0 {
			systemPrompt += "\n## Working Memory Context (Blackboard)\n" + strings.Join(bbFacts, "\n") + "\n"
		}
	}

	messages := []domain.Message{
		{Role: domain.RoleSystem, Content: systemPrompt},
		{Role: domain.RoleUser, Content: task.Description},
	}

	// Ephemeral worker mode: discard conversation context completely upon return
	if task.Ephemeral {
		defer func() {
			messages = nil
		}()
	}

	maxIterations := s.cfg.Budget.MaxIterations
	if task.Ephemeral {
		// Bounded turns for ephemeral workers (Reminis standard: max 3 turns)
		if maxIterations <= 0 || maxIterations > 3 {
			maxIterations = 3
		}
	}

	for iter := 0; iter < maxIterations; iter++ {
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

			type subagentToolCallItem struct {
				tc         domain.ToolCall
				toolResult *domain.ToolResult
				deniedMsg  string
			}

			n := len(resp.ToolCalls)
			items := make([]subagentToolCallItem, n)
			for idx, tc := range resp.ToolCalls {
				items[idx].tc = tc
			}

			isParallelSafe := func(tc domain.ToolCall) bool {
				if len(task.AllowedEditRoots) > 0 {
					lowerName := strings.ToLower(tc.Name)
					if strings.Contains(lowerName, "write") || strings.Contains(lowerName, "edit") ||
						strings.Contains(lowerName, "patch") || strings.Contains(lowerName, "create") {
						return false
					}
				}
				if s.cfg.Policy != nil {
					res := s.cfg.Policy.Evaluate(tc.Name, tc.Arguments)
					if !res.Allowed {
						return false
					}
				}
				return core.ClassifyTool(tc.Name) == core.TierRead
			}

			i := 0
			for i < n {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				if isParallelSafe(items[i].tc) {
					start := i
					for i < n && isParallelSafe(items[i].tc) {
						i++
					}
					end := i

					if end-start == 1 {
						tr, execErr := filtered.Execute(ctx, items[start].tc.Name, items[start].tc.Arguments)
						if execErr != nil {
							tr = &domain.ToolResult{Success: false, Error: execErr.Error()}
						}
						items[start].toolResult = tr
					} else {
						var wg sync.WaitGroup
						for j := start; j < end; j++ {
							j := j
							wg.Add(1)
							go func() {
								defer wg.Done()
								tr, execErr := filtered.Execute(ctx, items[j].tc.Name, items[j].tc.Arguments)
								if execErr != nil {
									tr = &domain.ToolResult{Success: false, Error: execErr.Error()}
								}
								items[j].toolResult = tr
							}()
						}
						wg.Wait()
					}
				} else {
					tc := items[i].tc
					// Mechanical Edit Authority Enforcement
					if len(task.AllowedEditRoots) > 0 {
						lowerName := strings.ToLower(tc.Name)
						if strings.Contains(lowerName, "write") || strings.Contains(lowerName, "edit") ||
							strings.Contains(lowerName, "patch") || strings.Contains(lowerName, "create") {
							targetPath := ""
							if p, ok := tc.Arguments["path"].(string); ok && p != "" {
								targetPath = p
							} else if f, ok := tc.Arguments["file"].(string); ok && f != "" {
								targetPath = f
							} else if t, ok := tc.Arguments["target"].(string); ok && t != "" {
								targetPath = t
							}

							if targetPath != "" {
								if !checkEditAuthority(targetPath, task.AllowedEditRoots) {
									items[i].deniedMsg = fmt.Sprintf("Edit Authority Denied: target path %q is outside allowed edit roots (%v)",
										targetPath, task.AllowedEditRoots)
									i++
									continue
								}
							}
						}
					}

					// Evaluate policy before execution
					if s.cfg.Policy != nil {
						result := s.cfg.Policy.Evaluate(tc.Name, tc.Arguments)
						if !result.Allowed {
							items[i].deniedMsg = fmt.Sprintf("Policy denied: %s (reason: %s, action: %s)",
								tc.Name, result.Reason, result.SuggestedAction)
							i++
							continue
						}
					}

					toolResult, execErr := filtered.Execute(ctx, tc.Name, tc.Arguments)
					if execErr != nil {
						toolResult = &domain.ToolResult{
							Success: false,
							Error:   execErr.Error(),
						}
					}
					items[i].toolResult = toolResult
					i++
				}
			}

			// Assemble results in exact original order
			for idx := range items {
				item := &items[idx]
				if item.deniedMsg != "" {
					messages = append(messages, domain.Message{
						Role:    domain.RoleTool,
						Content: item.deniedMsg,
					})
					continue
				}

				toolResult := item.toolResult
				if toolResult == nil {
					toolResult = &domain.ToolResult{Success: false, Error: "no result returned"}
				}

				// Audit trail
				if s.cfg.AuditFunc != nil {
					s.cfg.AuditFunc(item.tc.Name, item.tc.Arguments, toolResult.Success)
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
		if s.cfg.Blackboard != nil {
			producerID := task.ID
			if producerID == "" {
				producerID = "subagent"
			}
			if len(task.OutputKeys) > 0 {
				for _, outKey := range task.OutputKeys {
					_, _ = s.cfg.Blackboard.Set(producerID, outKey, []byte(resp.Content))
				}
			} else if task.Ephemeral && task.ID != "" {
				_, _ = s.cfg.Blackboard.Set(producerID, fmt.Sprintf("tasks.%s.output", producerID), []byte(resp.Content))
			}
		}
		return resp, nil
	}

	budgetMsg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("Subagent budget exhausted after %d iterations.", maxIterations),
	}
	if s.cfg.Blackboard != nil && task.Ephemeral && task.ID != "" {
		producerID := task.ID
		_, _ = s.cfg.Blackboard.Set(producerID, fmt.Sprintf("tasks.%s.output", producerID), []byte(budgetMsg.Content))
	}
	return budgetMsg, nil
}

// checkEditAuthority validates whether targetPath falls within any of the allowedRoots.
// Resolves both root and target path to absolute cleaned paths using filepath.Abs and filepath.Clean.
// Uses filepath.Rel(absRoot, absTarget) to verify the target does NOT start with ".." and is not an escape.
func checkEditAuthority(targetPath string, allowedRoots []string) bool {
	if len(allowedRoots) == 0 {
		return false
	}
	cleanTarget := filepath.Clean(targetPath)
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return false
	}

	for _, root := range allowedRoots {
		cleanRoot := filepath.Clean(root)
		absRoot, err := filepath.Abs(cleanRoot)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, absTarget)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
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

	// Skill registry: all available skills the subagent can request
	if len(task.SkillRegistry) > 0 {
		prompt += "\nAvailable skill catalog:\n"
		for _, s := range task.SkillRegistry {
			prompt += fmt.Sprintf("  - %s\n", s)
		}
		prompt += "\nLoad any skill during execution by naming it in your summary as:\n"
		prompt += "  SkillsLoaded: [skill-name, skill-name]\n"
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
