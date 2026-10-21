package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// populateSkillRegistry fills the task's SkillRegistry with available skill names,
// matches relevant skills by task intent and file context, and resolves their paths
// for lazy loading via file_read to avoid context saturation.
func (b *Brain) populateSkillRegistry(task *domain.SubagentTask) {
	if b.skillLister != nil {
		task.SkillRegistry = b.skillLister()
	}
	// If specific skills were not already passed, match skills from task description
	if len(task.Skills) == 0 && b.skillMatcher != nil && task.Description != "" {
		matched := b.skillMatcher(task.Description, nil)
		if len(matched) > 0 {
			task.Skills = matched
		}
	}
	// Resolve file paths for matched skills to enable progressive path-based loading
	if len(task.Skills) > 0 && b.skillPathResolver != nil && len(task.SkillPaths) == 0 {
		task.SkillPaths = b.skillPathResolver(task.Skills)
	}
}

// Delegate dispatches a task to a named subagent and returns the structured result.
// After a successful delegation, it automatically saves subagent discoveries
// to the shared knowledge graph for cross-pollination.
// Returns ErrSubagentPortNotWired if no subagent port is wired.
func (b *Brain) Delegate(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if b.subagentPort == nil {
		return nil, ErrSubagentPortNotWired
	}
	b.populateSkillRegistry(&task)
	result, err := b.subagentPort.Spawn(ctx, name, task)
	if err == nil && result != nil && result.Status != domain.SubagentBlocked {
		if b.kgRecorder != nil {
			b.kgRecorder.RecordSubagentResult(ctx, name, task.Description, result)
		}
	}
	return result, err
}

// AvailableSubagents returns the list of registered subagent names.
func (b *Brain) AvailableSubagents() []string {
	if b.subagentPort == nil {
		return nil
	}
	return b.subagentPort.Available()
}

// HandleDirectSubagent parses @name syntax and routes the message directly to that subagent.
func (b *Brain) HandleDirectSubagent(ctx context.Context, content string) error {
	return b.handleDirectSubagent(ctx, content)
}

// handleDirectSubagent parses @name syntax and routes the message directly
// to the named subagent. If the subagent is unknown, an error is returned with
// the available subagent list. Uses SpawnAsync when available, falls back to
// synchronous Spawn.
func (b *Brain) handleDirectSubagent(ctx context.Context, content string) error {
	// Parse: @name rests of message
	trimmed := strings.TrimPrefix(content, "@")
	parts := strings.SplitN(trimmed, " ", 2)
	name := parts[0]
	message := ""
	if len(parts) > 1 {
		message = parts[1]
	}

	// Validate subagent exists
	available := b.AvailableSubagents()
	found := false
	for _, a := range available {
		if a == name {
			found = true
			break
		}
	}
	if !found {
		errMsg := &domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Unknown subagent: @%s\nAvailable: %s", name, strings.Join(available, ", ")),
		}
		return b.ui.Display(*errMsg)
	}

	task := domain.SubagentTask{
		ID:           fmt.Sprintf("direct-%s-%d", name, time.Now().UnixNano()),
		Description:  message,
		Mode:         "plan",
		IsDirectChat: true,
		KGContext:    b.recallKnowledgeGraph(ctx, message),
	}

	// Try async spawn first
	asyncPort, isAsync := b.subagentPort.(ports.AsyncSpawner)
	if isAsync {
		b.populateSkillRegistry(&task)
		taskID, err := asyncPort.SpawnAsync(ctx, name, task)
		if err != nil {
			return b.ui.Display(domain.Message{
				Role:    domain.RoleAssistant,
				Content: fmt.Sprintf("Error spawning @%s: %v", name, err),
			})
		}
		return b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Dispatched to @%s (task %s)", name, taskID[:8]),
		})
	}

	// Fall back to synchronous spawn
	result, err := b.Delegate(ctx, name, task)
	if err != nil {
		return b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Error running @%s: %v", name, err),
		})
	}

	return b.ui.Display(domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("[@%s] %s: %s", name, result.Status, result.Summary),
	})
}
