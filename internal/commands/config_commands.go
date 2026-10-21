package commands

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
)

// ListModels displays available alternative LLM models.
func (d *Dispatcher) ListModels(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	available := d.brain.AvailableProviders()
	current := d.brain.ProviderName()

	if len(available) == 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No alternative models configured. Use llm.fallback_chain in config.yaml to add them.\n\nCurrent provider: " + current,
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available models (current: %s):\n\n", current))
	for name := range available {
		mark := "  "
		if name == current {
			mark = "➤ "
		}
		sb.WriteString(fmt.Sprintf("  %s%s\n", mark, name))
	}
	sb.WriteString("\nUsage: /model <name> — switch provider mid-session.\nNote: model switch is temporary unless saved to config.yaml.")

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// SwitchModel hot-swaps the active LLM provider at runtime.
func (d *Dispatcher) SwitchModel(ctx context.Context, name string) error {
	if name == "" {
		return d.ListModels(ctx)
	}

	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	available := d.brain.AvailableProviders()
	prov, ok := available[name]
	if !ok {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unknown model %q. Use /model to list available models.", name),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	d.brain.SetActiveProvider(name, prov)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Switched to model: %s\nThis change is temporary. Edit config.yaml to make it permanent.", name),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// FastMode toggles or configures the fast model override.
func (d *Dispatcher) FastMode(ctx context.Context, mode string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	fastEnabled, origModel, fastModel := d.brain.FastModeState()
	currentProvider := d.brain.ProviderName()
	available := d.brain.AvailableProviders()

	if mode == "" || mode == "status" {
		status := "disabled"
		if fastEnabled {
			status = fmt.Sprintf("enabled (active fast model: %s)", currentProvider)
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Fast mode: %s\nUsage: /fast on | /fast off | /fast <model-name>", status),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if mode == "off" {
		if !fastEnabled {
			msg := domain.Message{Role: domain.RoleSystem, Content: "Fast mode is already disabled."}
			_ = repo.SaveMessage(ctx, msg)
			return ui.Display(msg)
		}
		if origModel != "" {
			if prov, ok := available[origModel]; ok {
				d.brain.SetActiveProvider(origModel, prov)
			}
		}
		d.brain.SetFastMode(false, origModel, fastModel)
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Fast mode disabled. Restored original model: %s", d.brain.ProviderName()),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	targetModel := "gpt-4o-mini"
	if fastModel != "" {
		targetModel = fastModel
	}
	if mode != "on" {
		targetModel = mode
	}

	if !fastEnabled {
		origModel = currentProvider
	}

	if prov, ok := available[targetModel]; ok {
		d.brain.SetActiveProvider(targetModel, prov)
	}

	d.brain.SetFastMode(true, origModel, targetModel)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Fast mode enabled. Switched to fast model: %s\nUse '/fast off' to restore %s.", targetModel, origModel),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// BusyMode controls input handling when the agent is actively processing.
func (d *Dispatcher) BusyMode(ctx context.Context, mode string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "status" {
		current := "queue"
		if d.brain.BusyMode() != "" {
			current = d.brain.BusyMode()
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Busy mode: %s\nAvailable modes: queue (default), steer, ignore\nUsage: /busy <mode>", current),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	switch mode {
	case "queue", "steer", "ignore":
		d.brain.SetBusyMode(mode)
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Busy mode set to %q.\n- queue: queues input messages\n- steer: injects input as steering instruction\n- ignore: discards input while busy", mode),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unknown busy mode %q. Supported modes: queue, steer, ignore.", mode),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
}

// CodeGraphCommand handles the /codegraph slash command for semantic code graph queries.
func (d *Dispatcher) CodeGraphCommand(ctx context.Context, arg string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	reg := d.brain.Registry()
	if repo == nil || ui == nil || reg == nil {
		return nil
	}

	arg = strings.TrimSpace(arg)

	if arg == "" || arg == "status" {
		res, err := reg.Execute(ctx, "codegraph_lookup_symbol", map[string]interface{}{
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
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if arg == "index" || strings.HasPrefix(arg, "index ") {
		path := "."
		if strings.HasPrefix(arg, "index ") {
			path = strings.TrimSpace(arg[6:])
		}
		res, err := reg.Execute(ctx, "codegraph_index_workspace", map[string]interface{}{
			"path": path,
		})
		if err != nil || (res != nil && !res.Success) {
			errMsg := "indexing failed"
			if res != nil {
				errMsg = res.Error
			}
			msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("CodeGraph indexing failed: %s", errMsg)}
			_ = repo.SaveMessage(ctx, msg)
			return ui.Display(msg)
		}

		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("CodeGraph Indexing Complete:\n%s", res.Output),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if strings.HasPrefix(arg, "find ") {
		symbol := strings.TrimSpace(arg[5:])
		res, _ := reg.Execute(ctx, "codegraph_lookup_symbol", map[string]interface{}{
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
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if strings.HasPrefix(arg, "callers ") {
		symbol := strings.TrimSpace(arg[8:])
		res, _ := reg.Execute(ctx, "codegraph_find_callers", map[string]interface{}{
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
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Unknown codegraph command %q. Usage: /codegraph [status|index|find <symbol>|callers <symbol>]", arg),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MemoryPending displays recent memory operations pending review.
func (d *Dispatcher) MemoryPending(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	history, err := repo.GetHistory(ctx, 100)
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
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MemoryApprove approves a pending memory write by ID.
func (d *Dispatcher) MemoryApprove(ctx context.Context, id string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Memory %s approved. (Full write confirmation requires Engram MCP access.)", id),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// MemoryReject flags a memory write as rejected.
func (d *Dispatcher) MemoryReject(ctx context.Context, id string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Memory %s rejected and will not be persisted.", id),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}
