package commands

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core"
	"gaia/internal/core/domain"
)

// SetHome sets the notification home for this conversation.
func (d *Dispatcher) SetHome(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Home set for this conversation.\n\n" +
			"Cron job results, background task completions, and alerts will be delivered here.\n" +
			"In TUI mode, this is always the terminal.\n" +
			"In gateway mode (Telegram/Discord/Slack), this binds to the current chat.\n\n" +
			"To change, send /sethome from the desired chat.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// ApprovePending approves the last pending dangerous command confirmation.
func (d *Dispatcher) ApprovePending(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	tool := d.brain.PendingConfirm()
	d.brain.SetPendingConfirm("")

	if tool != "" {
		if p := d.brain.Policy(); p != nil && p.Session() != nil {
			p.Session().ApproveSession(tool)
		}
		if g := d.brain.Guard(); g != nil {
			g.Approve(tool)
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Pending command for tool %q approved for this session.", tool),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if p := d.brain.Policy(); p != nil {
		p.SetTier(core.TierFull)
	}
	if g := d.brain.Guard(); g != nil {
		g.SetMode(domain.TrustNever)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Autonomy elevated: all tools approved for the remainder of this session.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// DenyPending denies the last pending dangerous command confirmation.
func (d *Dispatcher) DenyPending(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	tool := d.brain.PendingConfirm()
	d.brain.SetPendingConfirm("")

	if tool != "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Pending command for tool %q was denied.", tool),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if p := d.brain.Policy(); p != nil {
		p.SetTier(core.TierRead)
	}
	if g := d.brain.Guard(); g != nil {
		g.SetMode(domain.TrustAlways)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Restrictions enforced: tier set to read-only. All dangerous actions will be prompted or denied.",
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// KGStatus displays the current KG recall state.
func (d *Dispatcher) KGStatus(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	status := "OFF"
	if d.brain.KGEnabled() {
		status = "ON"
	}
	var count int
	if kg := d.brain.KGStore(); kg != nil {
		facts, _ := kg.SearchFacts(ctx, "")
		count = len(facts)
	}
	content := fmt.Sprintf("Knowledge Graph Recall: %s\n\nFacts stored: %d\n\nCommands:\n  /kg on     — Enable KG recall\n  /kg off    — Disable KG recall\n  /kg stats  — Show facts by topic\n  /kg clear  — Clear all facts\n\nKG recall injects relevant facts as extra context.\nIt does NOT replace conversation history.", status, count)
	msg := domain.Message{Role: domain.RoleSystem, Content: content}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// KGOn enables knowledge graph recall.
func (d *Dispatcher) KGOn(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	d.brain.SetKGEnabled(true)
	msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph recall: ON\nWill inject relevant facts as extra context.\nUse /kg off to disable."}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// KGOff disables knowledge graph recall.
func (d *Dispatcher) KGOff(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	d.brain.SetKGEnabled(false)
	msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph recall: OFF\nUsing standard context only."}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// KGStats shows facts grouped by topic.
func (d *Dispatcher) KGStats(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	kg := d.brain.KGStore()
	if repo == nil || ui == nil || kg == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph not configured."}
		if repo != nil {
			_ = repo.SaveMessage(ctx, msg)
		}
		if ui != nil {
			_ = ui.Display(msg)
		}
		return nil
	}

	facts, err := kg.SearchFacts(ctx, "")
	if err != nil || len(facts) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No facts stored yet. Facts are extracted automatically from responses when KG recall is enabled."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	byTopic := make(map[string]int)
	for _, f := range facts {
		byTopic[f.Topic]++
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Knowledge Graph Facts (%d total)\n\n", len(facts)))
	for topic, count := range byTopic {
		sb.WriteString(fmt.Sprintf("  %-25s %d facts\n", topic, count))
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// KGClear removes all knowledge graph facts.
func (d *Dispatcher) KGClear(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	if kg := d.brain.KGStore(); kg != nil {
		if err := kg.Clear(ctx); err != nil {
			msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to clear Knowledge Graph: %v", err)}
			_ = repo.SaveMessage(ctx, msg)
			return ui.Display(msg)
		}
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph facts cleared."}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MCPStatus displays MCP server status.
func (d *Dispatcher) MCPStatus(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	mcpMgr := d.brain.MCPManager()
	if mcpMgr == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "MCP manager not configured. Servers are configured in config.yaml under mcp.servers."}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: mcpMgr.StatusText()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MCPConnect connects to a named MCP server.
func (d *Dispatcher) MCPConnect(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	mcpMgr := d.brain.MCPManager()
	if mcpMgr == nil {
		return d.MCPStatus(ctx)
	}
	if err := mcpMgr.Connect(ctx, name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to connect %q: %v", name, err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q connected", name)}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MCPDisconnect disconnects from a named MCP server.
func (d *Dispatcher) MCPDisconnect(ctx context.Context, name string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	mcpMgr := d.brain.MCPManager()
	if mcpMgr == nil {
		return d.MCPStatus(ctx)
	}
	if err := mcpMgr.Disconnect(name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to disconnect %q: %v", name, err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q disconnected", name)}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}
