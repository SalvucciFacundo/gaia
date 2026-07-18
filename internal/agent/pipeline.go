package agent

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
)

// PipelinePhase represents one step in an async SDD pipeline.
type PipelinePhase struct {
	SubagentName string // Name of the subagent to spawn
	Description  string // Task description for this phase
}

// RunPipeline executes phases sequentially via SpawnAsync. Each phase's
// result feeds into the next as KGContext (artifacts from prior phases).
// Cancelling the context cancels the current and all subsequent phases.
//
// Returns the collected results from all completed phases, or an error
// if any phase fails or is cancelled.
func RunPipeline(ctx context.Context, spawner *Spawner, phases []PipelinePhase, baseTask domain.SubagentTask) ([]*domain.SubagentResult, error) {
	if spawner.TaskManager() == nil {
		return nil, fmt.Errorf("pipeline requires TaskManager to be configured")
	}

	results := make([]*domain.SubagentResult, 0, len(phases))
	kgContext := baseTask.KGContext

	for i, phase := range phases {
		// Check for cancellation before starting each phase
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		task := baseTask
		task.ID = fmt.Sprintf("pipeline-%s-%d", phase.SubagentName, i)
		task.Description = phase.Description
		task.KGContext = kgContext

		taskID, err := spawner.SpawnAsync(ctx, phase.SubagentName, task)
		if err != nil {
			return results, fmt.Errorf("phase %q spawn: %w", phase.SubagentName, err)
		}

		state, err := spawner.TaskManager().WaitTask(ctx, taskID)
		if err != nil {
			return results, fmt.Errorf("phase %q wait: %w", phase.SubagentName, err)
		}

		if state.Status == TaskCancelled {
			return results, fmt.Errorf("phase %q cancelled", phase.SubagentName)
		}
		if state.Status == TaskFailed {
			return results, fmt.Errorf("phase %q failed: %s", phase.SubagentName, state.Error)
		}

		results = append(results, state.Result)

		// Feed artifacts forward as knowledge for the next phase
		if state.Result != nil {
			kgContext = append(kgContext, state.Result.Artifacts...)
		}
	}

	return results, nil
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
