package agent

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// handleSDDTrigger routes a detected SDD-triggering message through the
// SDD pipeline asynchronously via PipelineRunner or synchronously as fallback.
func (r *WorkflowRouter) handleSDDTrigger(ctx context.Context, content string, trigger TriggerResult) error {
	// If /direct was used, bypass SDD pipeline and run inline via ProcessMessage
	if trigger.ForceDirect {
		clean := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(content, DirectCommandPrefix), InlineCommandPrefix))
		return r.brain.ProcessMessage(ctx, clean)
	}

	subagentPort := r.brain.SubagentPort()
	ui := r.brain.UI()

	// Check if subagent port is wired
	if subagentPort == nil {
		msg := &domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD trigger detected (%s), but subagent system is not available.", trigger.Reason),
		}
		return ui.Display(*msg)
	}

	// Strip command prefix if present
	taskDesc := content
	if strings.HasPrefix(taskDesc, ODDCommandPrefix) {
		taskDesc = strings.TrimSpace(taskDesc[len(ODDCommandPrefix):])
	} else if strings.HasPrefix(taskDesc, LegacySDDCommandPrefix) {
		taskDesc = strings.TrimSpace(taskDesc[len(LegacySDDCommandPrefix):])
	}

	// Build the SDD pipeline phases
	phases := domain.SDDPhases(taskDesc)
	baseTask := domain.SubagentTask{
		ID:        "sdd-pipeline",
		Mode:      "plan",
		KGContext: r.brain.QueryKGContext(ctx, taskDesc),
	}

	// Try async pipeline via AsyncSpawner
	if asyncPort, isAsync := subagentPort.(ports.AsyncSpawner); isAsync {
		// Display trigger notification
		_ = ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD pipeline triggered (%s). Running 7 phases asynchronously...", trigger.Reason),
		})

		results, err := asyncPort.RunPipeline(ctx, phases, baseTask)
		if err != nil {
			return ui.Display(domain.Message{
				Role:    domain.RoleAssistant,
				Content: fmt.Sprintf("SDD pipeline failed: %v", err),
			})
		}

		// Save discoveries from all pipeline phases
		for i, phase := range phases {
			if i < len(results) && results[i] != nil {
				r.brain.SaveSubagentDiscoveries(ctx, phase.SubagentName, taskDesc, results[i])
			}
		}

		finalMsg := buildAsyncSDDPipelineSummary(trigger.Reason, results)
		return ui.Display(finalMsg)
	}

	// Fall back to synchronous path
	_ = ui.Display(domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("SDD pipeline triggered (%s). Delegating to Explorer...", trigger.Reason),
	})

	explorerTask := domain.SubagentTask{
		ID:          "sdd-explore-001",
		Description: taskDesc,
		Mode:        "plan",
	}
	exploreResult, err := r.brain.Delegate(ctx, "explorer", explorerTask)
	if err != nil {
		return fmt.Errorf("explorer phase: %w", err)
	}
	if exploreResult.Status == domain.SubagentBlocked {
		return ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD Explorer blocked: %s", exploreResult.Summary),
		})
	}

	proposerTask := domain.SubagentTask{
		ID:          "sdd-propose-001",
		Description: fmt.Sprintf("Create SDD proposal for: %s\nExplorer findings: %s", taskDesc, exploreResult.Summary),
		KGContext:   exploreResult.Artifacts,
		Mode:        "plan",
	}
	propResult, err := r.brain.Delegate(ctx, "proposer", proposerTask)
	if err != nil {
		return fmt.Errorf("proposer phase: %w", err)
	}
	if propResult.Status == domain.SubagentBlocked {
		return ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD Proposer blocked: %s", propResult.Summary),
		})
	}

	specifierTask := domain.SubagentTask{
		ID:          "sdd-spec-001",
		Description: fmt.Sprintf("Write delta specs based on proposal: %s", propResult.Summary),
		KGContext:   propResult.Artifacts,
		Mode:        "plan",
	}
	specResult, err := r.brain.Delegate(ctx, "specifier", specifierTask)
	if err != nil {
		return fmt.Errorf("specifier phase: %w", err)
	}
	if specResult.Status == domain.SubagentBlocked {
		return ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD Specifier blocked: %s", specResult.Summary),
		})
	}

	implementerTask := domain.SubagentTask{
		ID:          "sdd-impl-001",
		Description: fmt.Sprintf("Implement from specs: %s", specResult.Summary),
		KGContext:   specResult.Artifacts,
		Mode:        "build",
	}
	implResult, err := r.brain.Delegate(ctx, "implementer", implementerTask)
	if err != nil {
		return fmt.Errorf("implementer phase: %w", err)
	}

	verifierTask := domain.SubagentTask{
		ID:          "sdd-verify-001",
		Description: fmt.Sprintf("Verify implementation: %s", implResult.Summary),
		KGContext:   implResult.Artifacts,
		Mode:        "plan",
	}
	verResult, err := r.brain.Delegate(ctx, "verifier", verifierTask)
	if err != nil {
		return fmt.Errorf("verifier phase: %w", err)
	}

	finalMsg := buildSDDPipelineSummary(trigger.Reason, exploreResult, propResult, specResult, implResult, verResult)
	return ui.Display(finalMsg)
}

// buildAsyncSDDPipelineSummary creates a final summary from the async pipeline results.
func buildAsyncSDDPipelineSummary(reason string, results []*domain.SubagentResult) domain.Message {
	content := fmt.Sprintf("## SDD Pipeline Complete\n\n**Trigger**: %s\n\n", reason)

	content += "### Pipeline Results\n"
	phases := domain.SDDPhases("")
	for i, r := range results {
		phaseName := fmt.Sprintf("Phase %d", i+1)
		if i < len(phases) && phases[i].SubagentName != "" {
			phaseName = phases[i].SubagentName
		}
		status := "unknown"
		summary := "no summary"
		if r != nil {
			status = string(r.Status)
			summary = r.Summary
		}
		content += fmt.Sprintf("- **%s**: %s — %s\n", phaseName, status, summary)
	}

	content += "\n### Artifacts\n"
	for _, r := range results {
		if r == nil {
			continue
		}
		for _, a := range r.Artifacts {
			content += fmt.Sprintf("- %s\n", a)
		}
	}

	content += "\n### Risks\n"
	allRisks := collectRisks(results...)
	if len(allRisks) == 0 {
		content += "- None\n"
	} else {
		for _, r := range allRisks {
			content += fmt.Sprintf("- %s\n", r)
		}
	}

	return domain.Message{
		Role:    domain.RoleAssistant,
		Content: content,
	}
}

// buildSDDPipelineSummary creates a final summary message from all five SDD pipeline phases.
func buildSDDPipelineSummary(reason string, explore, prop, spec, impl, ver *domain.SubagentResult) domain.Message {
	content := fmt.Sprintf("## SDD Pipeline Complete\n\n**Trigger**: %s\n\n", reason)

	content += "### Pipeline Results\n"
	content += fmt.Sprintf("- **Explorer**: %s — %s\n", explore.Status, explore.Summary)
	content += fmt.Sprintf("- **Proposer**: %s — %s\n", prop.Status, prop.Summary)
	content += fmt.Sprintf("- **Specifier**: %s — %s\n", spec.Status, spec.Summary)
	content += fmt.Sprintf("- **Implementer**: %s — %s\n", impl.Status, impl.Summary)
	content += fmt.Sprintf("- **Verifier**: %s — %s\n", ver.Status, ver.Summary)

	content += "\n### Artifacts\n"
	allArtifacts := collectArtifacts(explore, prop, spec, impl, ver)
	for _, a := range allArtifacts {
		content += fmt.Sprintf("- %s\n", a)
	}

	content += "\n### Risks\n"
	allRisks := collectRisks(explore, prop, spec, impl, ver)
	if len(allRisks) == 0 {
		content += "- None\n"
	} else {
		for _, r := range allRisks {
			content += fmt.Sprintf("- %s\n", r)
		}
	}

	return domain.Message{
		Role:    domain.RoleAssistant,
		Content: content,
	}
}

// collectArtifacts gathers unique artifacts from all pipeline phases.
func collectArtifacts(results ...*domain.SubagentResult) []string {
	seen := make(map[string]bool)
	var all []string
	for _, r := range results {
		if r == nil {
			continue
		}
		for _, a := range r.Artifacts {
			if !seen[a] {
				seen[a] = true
				all = append(all, a)
			}
		}
	}
	return all
}

// collectRisks gathers unique risks from all pipeline phases.
func collectRisks(results ...*domain.SubagentResult) []string {
	seen := make(map[string]bool)
	var all []string
	for _, r := range results {
		if r == nil {
			continue
		}
		for _, risk := range r.Risks {
			if !seen[risk] {
				seen[risk] = true
				all = append(all, risk)
			}
		}
	}
	return all
}
