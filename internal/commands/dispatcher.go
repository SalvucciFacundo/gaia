package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core"
	"gaia/internal/core/ports"
)

// Dispatcher inspects user inputs for slash commands or subagent mentions
// and dispatches them to the corresponding core agent operations.
type Dispatcher struct {
	brain *core.Brain
}

var _ ports.CommandDispatcher = (*Dispatcher)(nil)

// New creates a new command Dispatcher for the given Brain.
func New(brain *core.Brain) *Dispatcher {
	return &Dispatcher{brain: brain}
}

// Dispatch inspects content for slash commands and executes them.
// Returns (handled, error) where handled is true if content was recognized and processed.
func (d *Dispatcher) Dispatch(ctx context.Context, content string) (bool, error) {
	if d.brain == nil {
		return false, nil
	}

	// 0. Direct subagent routing (@explorer, @implementer, etc.)
	if strings.HasPrefix(content, "@") {
		return true, d.brain.HandleDirectSubagent(ctx, content)
	}

	// Only process slash commands
	if !strings.HasPrefix(content, "/") {
		return false, nil
	}

	// 1. Exact match commands
	switch content {
	case "/undo":
		return true, d.UndoLastTurn(ctx)
	case "/retry":
		return true, d.RetryLastTurn(ctx)
	case "/new", "/reset":
		return true, d.NewSession(ctx)
	case "/save":
		return true, d.SaveSession(ctx, "")
	case "/compress":
		return true, d.ManualCompress(ctx)
	case "/history":
		return true, d.ShowHistory(ctx)
	case "/sessions":
		return true, d.ListSessionsCmd(ctx)
	case "/snapshot", "/snap":
		return true, d.SnapshotHelp(ctx)
	case "/branch":
		return true, d.BranchFork(ctx, fmt.Sprintf("branch-%s", time.Now().Format("2006-01-02-15-04-05")))
	case "/branches":
		return true, d.BranchList(ctx)
	case "/queue", "/q":
		return true, d.QueueShow(ctx)
	case "/queue clear":
		return true, d.QueueClear(ctx)
	case "/handoff":
		return true, d.HandoffHelp(ctx)
	case "/steer", "/steer ":
		return true, d.Steer(ctx, "")
	case "/goal", "/goals":
		return true, d.GoalShow(ctx)
	case "/goal clear":
		return true, d.GoalClear(ctx)
	case "/model", "/models":
		return true, d.ListModels(ctx)
	case "/memory", "/memory pending":
		return true, d.MemoryPending(ctx)
	case "/suggestions":
		return true, d.SkillSuggestions(ctx)
	case "/curator":
		return true, d.SkillCurator(ctx)
	case "/insights":
		return true, d.Insights(ctx)
	case "/debug":
		return true, d.DebugInfo(ctx)
	case "/credits":
		return true, d.CreditsInfo(ctx)
	case "/billing":
		return true, d.BillingInfo(ctx)
	case "/paste":
		return true, d.PasteImage(ctx)
	case "/fast":
		return true, d.FastMode(ctx, "")
	case "/diff":
		return true, d.DiffView(ctx, false)
	case "/busy":
		return true, d.BusyMode(ctx, "")
	case "/codegraph":
		return true, d.CodeGraphCommand(ctx, "")
	case "/help", "/h", "/commands":
		return true, d.HelpCommands(ctx)
	case "/sethome":
		return true, d.SetHome(ctx)
	case "/approve":
		return true, d.ApprovePending(ctx)
	case "/deny":
		return true, d.DenyPending(ctx)
	case "/restart":
		return true, d.RestartGateway(ctx)
	case "/update":
		return true, d.UpdateGAIA(ctx)
	case "/topic":
		return true, d.TopicHelp(ctx)
	case "/audit":
		return true, d.AuditLog(ctx)
	case "/audit clear":
		return true, d.AuditClear(ctx)
	case "/session":
		return true, d.SessionStatus(ctx)
	case "/kg":
		return true, d.KGStatus(ctx)
	case "/kg on":
		return true, d.KGOn(ctx)
	case "/kg off":
		return true, d.KGOff(ctx)
	case "/kg stats":
		return true, d.KGStats(ctx)
	case "/kg clear":
		return true, d.KGClear(ctx)
	}

	// 2. Prefix-based commands
	parts := strings.SplitN(content, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/save":
		return true, d.SaveSession(ctx, arg)
	case "/title":
		return true, d.TitleSession(ctx, arg)
	case "/resume":
		return true, d.ResumeSession(ctx, arg)
	case "/moa":
		return true, d.Moa(ctx, arg)
	case "/background":
		return true, d.BackgroundTask(ctx, arg)
	case "/snapshot", "/snap":
		subParts := strings.SplitN(arg, " ", 2)
		if len(subParts) == 2 && subParts[0] == "save" {
			return true, d.SnapshotSave(ctx, subParts[1])
		}
		if len(subParts) == 2 && subParts[0] == "load" {
			return true, d.SnapshotLoad(ctx, subParts[1])
		}
		return true, d.SnapshotHelp(ctx)
	case "/branch":
		return true, d.BranchFork(ctx, arg)
	case "/queue", "/q":
		return true, d.QueuePush(ctx, arg)
	case "/handoff":
		return true, d.HandoffTo(ctx, arg)
	case "/steer":
		return true, d.Steer(ctx, arg)
	case "/goal":
		return true, d.GoalSet(ctx, arg)
	case "/subgoal":
		return true, d.SubgoalAdd(ctx, arg)
	case "/model", "/models":
		return true, d.SwitchModel(ctx, arg)
	case "/learn":
		return true, d.LearnSkill(ctx, arg)
	case "/blueprint":
		return true, d.BlueprintCreate(ctx, arg)
	case "/image":
		return true, d.AttachImage(ctx, arg)
	case "/fast":
		return true, d.FastMode(ctx, arg)
	case "/diff":
		staged := strings.Contains(content, "--staged") || strings.Contains(content, "--cached")
		return true, d.DiffView(ctx, staged)
	case "/busy":
		return true, d.BusyMode(ctx, arg)
	case "/codegraph":
		return true, d.CodeGraphCommand(ctx, arg)
	case "/topic":
		return true, d.TopicSession(ctx, arg)
	case "/session":
		return true, d.SessionCommand(ctx, arg)
	}

	// Special compound subcommands
	if strings.HasPrefix(content, "/memory approve ") {
		return true, d.MemoryApprove(ctx, strings.TrimSpace(content[15:]))
	}
	if strings.HasPrefix(content, "/memory reject ") {
		return true, d.MemoryReject(ctx, strings.TrimSpace(content[14:]))
	}

	return false, nil
}
