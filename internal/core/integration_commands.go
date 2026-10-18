package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gaia/internal/core/domain"
)

func (b *Brain) SetHome(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Home set for this conversation.\n\n" +
			"Cron job results, background task completions, and alerts will be delivered here.\n" +
			"In TUI mode, this is always the terminal.\n" +
			"In gateway mode (Telegram/Discord/Slack), this binds to the current chat.\n\n" +
			"To change, send /sethome from the desired chat.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ApprovePending approves the last pending dangerous command confirmation.
func (b *Brain) ApprovePending(ctx context.Context) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Pending command approved. Use /trust full to auto-approve all commands in this session.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// DenyPending denies the last pending dangerous command confirmation.
func (b *Brain) DenyPending(ctx context.Context) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Pending command denied. Use /trust read for strictest restrictions.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// CommandsList shows all available commands (same as /help).

// TopicHelp shows help for the /topic command.

// SessionStatus displays the current session mode and active sessions.
func (b *Brain) SessionStatus(ctx context.Context) error {
	var content string
	if b.sessionMgr != nil {
		content = b.sessionMgr.Status()
	} else {
		content = "Session manager not configured. All messages use the default session."
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: content}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SessionCommand processes /session subcommands.
func (b *Brain) SessionCommand(ctx context.Context, args string) error {
	if b.sessionMgr == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session manager not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	switch args {
	case "unify":
		b.sessionMgr.SetMode(SessionUnify)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: unify\nAll platforms now share one session. Use /session isolate to separate them."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	case "isolate":
		b.sessionMgr.SetMode(SessionIsolate)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: isolate\nEach platform now has its own independent session. Use /session unify to merge them."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	case "ask":
		b.sessionMgr.SetMode(SessionAsk)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: ask\nGAIA will ask how to handle messages when you switch platforms."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	default:
		return b.SessionStatus(ctx)
	}
}

// KGStatus displays the current KG recall state.
func (b *Brain) KGStatus(ctx context.Context) error {
	status := "OFF"
	if b.kgEnabled {
		status = "ON"
	}
	var count int
	if b.kgStore != nil {
		facts, _ := b.kgStore.SearchFacts(ctx, "")
		count = len(facts)
	}
	content := fmt.Sprintf("Knowledge Graph Recall: %s\n\nFacts stored: %d\n\nCommands:\n  /kg on     — Enable KG recall\n  /kg off    — Disable KG recall\n  /kg stats  — Show facts by topic\n  /kg clear  — Clear all facts\n\nKG recall injects relevant facts as extra context.\nIt does NOT replace conversation history.", status, count)
	msg := domain.Message{Role: domain.RoleSystem, Content: content}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// KGStats shows facts grouped by topic.
func (b *Brain) KGStats(ctx context.Context) error {
	if b.kgStore == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	facts, err := b.kgStore.SearchFacts(ctx, "")
	if err != nil || len(facts) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No facts stored yet. Facts are extracted automatically from responses when KG recall is enabled."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
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
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// KGClear removes all knowledge graph facts.
func (b *Brain) KGClear(ctx context.Context) error {
	msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph facts cleared."}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// extractKGFacts analyzes a response and extracts key facts to store in the knowledge graph.
func (b *Brain) extractKGFacts(ctx context.Context, response string) {
	if b.kgStore == nil || !b.kgEnabled {
		return
	}
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 30 {
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indicators := []string{"uses ", "implements ", "migrate ", "changed ", "refactored ", "decision: ", "recommend ", "configured "}
		for _, ind := range indicators {
			if strings.Contains(strings.ToLower(trimmed), ind) {
				b.kgStore.AddFact(ctx, domain.KnowledgeFact{
					Topic:       "Conversation",
					Concept:     "Key facts",
					Fact:        trimmed,
					SourceAgent: "brain",
					Labels:      []string{"auto-extracted", "recall"},
					CreatedAt:   time.Now(),
				})
				break
			}
		}
	}
}


// MCPStatus displays MCP server status.
func (b *Brain) MCPStatus(ctx context.Context) error {
	if b.mcpMgr == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "MCP manager not configured. Servers are configured in config.yaml under mcp.servers."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: b.mcpMgr.StatusText()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

func (b *Brain) MCPConnect(ctx context.Context, name string) error {
	if b.mcpMgr == nil {
		return b.MCPStatus(ctx)
	}
	if err := b.mcpMgr.Connect(ctx, name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to connect %q: %v", name, err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q connected", name)}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

func (b *Brain) MCPDisconnect(ctx context.Context, name string) error {
	if b.mcpMgr == nil {
		return b.MCPStatus(ctx)
	}
	if err := b.mcpMgr.Disconnect(name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to disconnect %q: %v", name, err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q disconnected", name)}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ListProviderModels shows all available models from the current provider.
func (b *Brain) ListProviderModels(ctx context.Context) error {
	models, err := b.provider.ListModels(ctx)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to list models: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	if len(models) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No models available from the current provider."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Models available for %s (%d total):\n\n", b.providerName, len(models)))
	for _, m := range models {
		mark := "  "
		if m == b.modelName {
			mark = "➤ "
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", mark, m))
	}
	sb.WriteString(fmt.Sprintf("\nUse /model <name> to switch, or set llm.model in config.yaml."))
	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}
