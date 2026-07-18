// Package agent provides the subagent system for GAIA.
// Subagents are specialized autonomous agents with filtered tool sets,
// isolated context, and structured return envelopes.
package agent

import (
	"context"

	"gaia/internal/core/domain"
)

// Subagent defines the contract for a domain-specific subagent.
// Each implementation has a Name, Description, and an Execute method
// that processes a task and returns a structured result.
type Subagent interface {
	Name() string
	Description() string
	Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult
}
