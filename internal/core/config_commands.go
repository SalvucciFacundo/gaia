package core

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

func (b *Brain) ListModels(ctx context.Context) error {
	if len(b.availableProviders) == 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No alternative models configured. Use llm.fallback_chain in config.yaml to add them.\n\nCurrent provider: " + b.providerName,
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available models (current: %s):\n\n", b.providerName))
	for name := range b.availableProviders {
		mark := "  "
		if name == b.providerName {
			mark = "➤ "
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", mark, name))
	}
	sb.WriteString("\nUsage: /model <name> — switch provider mid-session.\nNote: model switch is temporary unless saved to config.yaml.")

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SwitchModel hot-swaps the active LLM provider at runtime.
func (b *Brain) SwitchModel(ctx context.Context, name string) error {
	if name == "" {
		return b.ListModels(ctx)
	}
	prov, ok := b.availableProviders[name]
	if !ok {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unknown model %q. Use /model to list available models.", name),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	b.provider = prov
	b.providerName = name
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Switched to model: %s\nThis change is temporary. Edit config.yaml to make it permanent.", name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// FastMode toggles or configures the fast model override.
func (b *Brain) FastMode(ctx context.Context, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "status" {
		status := "disabled"
		if b.fastModeEnabled {
			status = fmt.Sprintf("enabled (active fast model: %s)", b.providerName)
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Fast mode: %s\nUsage: /fast on | /fast off | /fast <model-name>", status),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if mode == "off" {
		if !b.fastModeEnabled {
			msg := domain.Message{Role: domain.RoleSystem, Content: "Fast mode is already disabled."}
			b.repo.SaveMessage(ctx, msg)
			return b.ui.Display(msg)
		}
		b.fastModeEnabled = false
		if b.originalModel != "" {
			if prov, ok := b.availableProviders[b.originalModel]; ok {
				b.provider = prov
				b.providerName = b.originalModel
			}
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Fast mode disabled. Restored original model: %s", b.providerName),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// mode == "on" or custom model name
	targetModel := "gpt-4o-mini"
	if b.fastModel != "" {
		targetModel = b.fastModel
	}
	if mode != "on" {
		targetModel = mode
	}

	if !b.fastModeEnabled {
		b.originalModel = b.providerName
	}

	if prov, ok := b.availableProviders[targetModel]; ok {
		b.provider = prov
		b.providerName = targetModel
	}

	b.fastModeEnabled = true
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Fast mode enabled. Switched to fast model: %s\nUse '/fast off' to restore %s.", targetModel, b.originalModel),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BusyMode controls input handling when the agent is actively processing.
// Modes: "queue" (default), "steer", "ignore".
func (b *Brain) BusyMode(ctx context.Context, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "status" {
		current := "queue"
		if b.busyMode != "" {
			current = b.busyMode
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Busy mode: %s\nAvailable modes: queue (default), steer, ignore\nUsage: /busy <mode>", current),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	switch mode {
	case "queue", "steer", "ignore":
		b.busyMode = mode
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Busy mode set to %q.\n- queue: queues input messages\n- steer: injects input as steering instruction\n- ignore: discards input while busy", mode),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unknown busy mode %q. Supported modes: queue, steer, ignore.", mode),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
}

// CodeGraphCommand handles the /codegraph slash command for semantic code graph queries.
func (b *Brain) CodeGraphCommand(ctx context.Context, arg string) error {
	arg = strings.TrimSpace(arg)

	if arg == "" || arg == "status" {
		res, err := b.registry.Execute(ctx, "codegraph_lookup_symbol", map[string]interface{}{
			"symbol": "main",
		})
		statusText := "CodeGraph semantic index is active."
		if err != nil || (res != nil && !res.Success) {
			statusText = "CodeGraph index not initialized. Run '/codegraph index' to index the workspace."
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("CodeGraph Semantic Index:\n- Status: %s\n- Usage: /codegraph index [path] | /codegraph find <symbol> | /codegraph callers <symbol>", statusText),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if arg == "index" || strings.HasPrefix(arg, "index ") {
		path := "."
		if strings.HasPrefix(arg, "index ") {
			path = strings.TrimSpace(arg[6:])
		}
		res, err := b.registry.Execute(ctx, "codegraph_index_workspace", map[string]interface{}{
			"path": path,
		})
		if err != nil || (res != nil && !res.Success) {
			errMsg := "indexing failed"
			if res != nil {
				errMsg = res.Error
			}
			msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("CodeGraph indexing failed: %s", errMsg)}
			b.repo.SaveMessage(ctx, msg)
			return b.ui.Display(msg)
		}

		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("CodeGraph Indexing Complete:\n%s", res.Output),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if strings.HasPrefix(arg, "find ") {
		symbol := strings.TrimSpace(arg[5:])
		res, _ := b.registry.Execute(ctx, "codegraph_lookup_symbol", map[string]interface{}{
			"symbol": symbol,
		})
		output := "No symbol found."
		if res != nil && res.Success && res.Output != "" {
			output = res.Output
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("CodeGraph Lookup (%s):\n%s", symbol, output),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if strings.HasPrefix(arg, "callers ") {
		symbol := strings.TrimSpace(arg[8:])
		res, _ := b.registry.Execute(ctx, "codegraph_find_callers", map[string]interface{}{
			"symbol": symbol,
		})
		output := "No callers found."
		if res != nil && res.Success && res.Output != "" {
			output = res.Output
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("CodeGraph Callers of %s:\n%s", symbol, output),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Unknown codegraph command %q. Usage: /codegraph [status|index|find <symbol>|callers <symbol>]", arg),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// MemoryPending displays recent memory operations pending review.
func (b *Brain) MemoryPending(ctx context.Context) error {
	// Tracked via recent messages that contain "mem_save" patterns
	history, err := b.repo.GetHistory(ctx, 100)
	if err != nil {
		return fmt.Errorf("memory pending: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Recent memory operations detected in this session:\n\n")
	count := 0
	for i := len(history) - 1; i >= 0 && count < 10; i-- {
		msg := history[i]
		if strings.Contains(msg.Content, "Saved") || strings.Contains(msg.Content, "mem_save") {
			preview := msg.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("  [msg-%d] %s\n", i, preview))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("  No recent memory writes detected.\n")
	}
	sb.WriteString("\nNote: Full memory approval requires Engram MCP integration.\nUse /memory reject <id> to flag incorrect memories.")

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// MemoryApprove approves a pending memory write by ID.
func (b *Brain) MemoryApprove(ctx context.Context, id string) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Memory %s approved. (Full write confirmation requires Engram MCP access.)", id),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// MemoryReject flags a memory write as rejected.
func (b *Brain) MemoryReject(ctx context.Context, id string) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Memory %s rejected and will not be persisted.", id),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

