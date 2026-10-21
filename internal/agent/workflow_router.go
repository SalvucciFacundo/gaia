package agent

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// WorkflowRouter coordinates ODD and SDD workflows, handling feature tracking,
// proactive architecture suggestions, delegated workers, and mid-turn budget triggers.
type WorkflowRouter struct {
	brain          *core.Brain
	midTurnTracker *MidTurnBudgetTracker
}

var _ ports.WorkflowRouter = (*WorkflowRouter)(nil)

// NewWorkflowRouter creates a new WorkflowRouter wired to the given Brain.
func NewWorkflowRouter(brain *core.Brain) *WorkflowRouter {
	return &WorkflowRouter{
		brain:          brain,
		midTurnTracker: NewMidTurnBudgetTracker(),
	}
}

// ResetTurn clears per-turn tracking state.
func (r *WorkflowRouter) ResetTurn() {
	if r.midTurnTracker != nil {
		r.midTurnTracker.Reset()
	}
}

// RouteTurn inspects user input before standard turn execution to determine
// if an ODD/SDD route or delegated worker should handle the turn.
// Returns (true, err) if handled, or (false, nil) to let the core engine proceed inline.
func (r *WorkflowRouter) RouteTurn(ctx context.Context, content string) (bool, error) {
	if r.brain == nil {
		return false, nil
	}

	odd := DetectODDRoute(content)
	trigger := DetectSDDTrigger(content)

	if odd.Route == RouteFeatureTracking {
		return true, r.handleSDDTrigger(ctx, content, trigger)
	}

	if odd.SuggestFeature && !trigger.ForceDirect {
		suggestion := fmt.Sprintf(
			"💡 **Architectural Scope Detected** (%s).\n"+
				"This change involves multi-module scope or architectural decisions where durable specs and tasks protect quality.\n\n"+
				"Would you like to run the formal feature tracking workflow? Use **`/odd %s`** (or `/sdd`) or continue inline with **`/inline %s`** (or `/direct`).",
			odd.Reason, content, content)
		msg := domain.Message{Role: domain.RoleAssistant, Content: suggestion}
		_ = r.brain.Repo().SaveMessage(ctx, msg)
		return true, r.brain.UI().Display(msg)
	}

	if odd.Route == RouteDelegatedWorker && r.brain.SubagentPort() != nil && !trigger.ForceDirect {
		targetAgent := odd.WorkerRole
		if targetAgent == "writer" {
			targetAgent = "implementer"
		}
		isAvailable := false
		for _, a := range r.brain.SubagentPort().Available() {
			if a == targetAgent {
				isAvailable = true
				break
			}
		}
		if isAvailable {
			task := domain.SubagentTask{
				ID:               "odd-worker",
				Description:      content,
				Mode:             "execute",
				KGContext:        r.brain.QueryKGContext(ctx, content),
				AllowedEditRoots: []string{"."},
			}
			result, err := r.brain.Delegate(ctx, targetAgent, task)
			if err == nil && result != nil {
				workerMsg := domain.Message{
					Role:    domain.RoleAssistant,
					Content: fmt.Sprintf("🤖 **ODD Delegated Worker (%s)** completed:\n\n%s", targetAgent, result.Summary),
				}
				_ = r.brain.Repo().SaveMessage(ctx, workerMsg)
				return true, r.brain.UI().Display(workerMsg)
			}
		}
	}

	return false, nil
}

// RecordToolCall updates tracker state based on tool execution.
func (r *WorkflowRouter) RecordToolCall(name string, args map[string]interface{}, output string) {
	if r.midTurnTracker != nil {
		r.midTurnTracker.RecordToolCall(name, args, output)
	}
}

// CheckMidTurn checks in-flight execution progress after each tool call to determine
// if mid-turn delegation to a subagent (e.g. explorer, implementer, verifier) is triggered.
func (r *WorkflowRouter) CheckMidTurn(ctx context.Context, content string) (bool, error) {
	if r.brain == nil || r.midTurnTracker == nil || r.brain.SubagentPort() == nil {
		return false, nil
	}

	trigger := DetectSDDTrigger(content)
	if trigger.ForceDirect {
		return false, nil
	}

	targetRole, reason, shouldDelegate := r.midTurnTracker.CheckDelegationTrigger()
	if !shouldDelegate {
		return false, nil
	}

	agentName := targetRole
	if agentName == "writer" {
		agentName = "implementer"
	}
	isAvailable := false
	for _, a := range r.brain.SubagentPort().Available() {
		if a == agentName {
			isAvailable = true
			break
		}
	}
	if !isAvailable {
		r.midTurnTracker.Reset()
		return false, nil
	}

	intermediateContext := ""
	if len(r.midTurnTracker.ToolHistory) > 0 {
		intermediateContext = fmt.Sprintf("\n\n### Intermediate Execution Progress:\nTools executed so far in parent turn:\n- %s\n",
			strings.Join(r.midTurnTracker.ToolHistory, "\n- "))
	}
	if len(r.midTurnTracker.FilesWritten) > 0 {
		files := make([]string, 0, len(r.midTurnTracker.FilesWritten))
		for f := range r.midTurnTracker.FilesWritten {
			files = append(files, f)
		}
		intermediateContext += fmt.Sprintf("Files touched: %s\n", strings.Join(files, ", "))
	}

	delegateTask := domain.SubagentTask{
		ID:               fmt.Sprintf("odd-midturn-%s", targetRole),
		Description:      fmt.Sprintf("Continue execution (%s). Original request: %s%s", reason, content, intermediateContext),
		Mode:             "execute",
		KGContext:        r.brain.QueryKGContext(ctx, content),
		AllowedEditRoots: []string{"."},
	}
	delRes, delErr := r.brain.Delegate(ctx, agentName, delegateTask)
	if delErr == nil && delRes != nil {
		delMsg := domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("🤖 **ODD Mid-Turn Dynamic Delegation (%s)** [%s]:\n\n%s", agentName, reason, delRes.Summary),
		}
		_ = r.brain.Repo().SaveMessage(ctx, delMsg)
		if err := r.brain.UI().Display(delMsg); err != nil {
			return true, err
		}
		r.brain.PostTurnHooks(ctx, delMsg.Content)
		return true, nil
	}

	r.midTurnTracker.Reset()
	return false, nil
}
