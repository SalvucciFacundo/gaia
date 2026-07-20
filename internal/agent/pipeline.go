package agent

import (
	"context"
	"fmt"

	"gaia/internal/core/domain"
)

// RunPipeline executes phases sequentially via SpawnAsync. Each phase's
// result feeds into the next as KGContext (artifacts from prior phases).
// Cancelling the context cancels the current and all subsequent phases.
//
// Returns the collected results from all completed phases, or an error
// if any phase fails or is cancelled.
func (s *Spawner) RunPipeline(ctx context.Context, phases []domain.PipelinePhase, baseTask domain.SubagentTask) ([]*domain.SubagentResult, error) {
	if s.cfg.TaskManager == nil {
		return nil, fmt.Errorf("pipeline requires TaskManager to be configured")
	}
	tm := s.cfg.TaskManager

	results := make([]*domain.SubagentResult, 0, len(phases))
	kgContext := make([]string, len(baseTask.KGContext))
	copy(kgContext, baseTask.KGContext)

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

		// Merge KGContext from previous phases
		task.KGContext = kgContext

		taskID, err := s.SpawnAsync(ctx, phase.SubagentName, task)
		if err != nil {
			return results, fmt.Errorf("phase %q spawn: %w", phase.SubagentName, err)
		}

		state, err := tm.WaitTask(ctx, taskID)
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
