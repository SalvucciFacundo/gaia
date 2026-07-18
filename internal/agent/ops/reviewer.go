package ops

import (
	"context"
	"fmt"
	"strings"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
	"gaia/internal/review"
	"gaia/internal/review/gates"
)

// reviewer analyzes code through GGA's 4 lenses — risk, resilience,
// readability, and reliability — and returns a bounded receipt.
// It has read-only access: read files, list directories, and inspect
// git history, but CANNOT write code or execute shell commands.
type reviewer struct {
	spawner *agent.Spawner
}

// NewReviewer creates the Reviewer subagent.
func NewReviewer(spawner *agent.Spawner) agent.Subagent {
	return &reviewer{spawner: spawner}
}

func (r *reviewer) Name() string { return "reviewer" }

func (r *reviewer) Description() string {
	return "Reviews code through GGA 4 lenses (risk, resilience, readability, reliability) — read-only"
}

func (r *reviewer) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"git_status",
		"git_log",
		"git_diff",
	}

	// Create the review engine.
	// The SpawnerLLM bridges the review.LensLLM interface to the Spawner.
	llm := &spawnerLLM{spawner: r.spawner, allowedTools: task.AllowedTools}
	engine := review.NewEngine(".", llm)

	// Extract files from task context or default to scanning.
	files := extractFiles(task)
	if len(files) == 0 {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "No files specified for review.",
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	// Phase 1: Start review — snapshot files.
	tx, err := engine.Start(files)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         fmt.Sprintf("Failed to start review: %s", err),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	// Phase 2: Classify risk.
	diff := task.Description // Use task description as diff context
	riskCodes, riskLevel := engine.ClassifyRisk(diff)

	// Phase 3: Select and run lenses.
	lensNames := engine.SelectLenses(riskLevel)

	var findings []domain.ReviewFinding
	if len(lensNames) > 0 {
		findings, err = engine.RunLenses(ctx, tx, lensNames)
		if err != nil {
			return &domain.SubagentResult{
				Status:          domain.SubagentPartial,
				Summary:         fmt.Sprintf("Review lenses partially failed: %s", err),
				NextRecommended: "none",
				Risks:           riskCodeStrings(riskCodes),
				SkillResolution: "none",
			}
		}
	}

	// Phase 4: Generate bounded receipt.
	receipt, err := engine.GenerateReceipt(tx, findings, riskCodes, riskLevel)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentPartial,
			Summary:         fmt.Sprintf("Receipt generation failed: %s", err),
			NextRecommended: "none",
			Risks:           riskCodeStrings(riskCodes),
			SkillResolution: "none",
		}
	}

	// Build summary.
	summary := fmt.Sprintf(
		"Review complete. Risk: %s. Lenses: %s. Findings: %d BLOCKER, %d WARNING, %d SUGGESTION.",
		riskLevel,
		strings.Join(lensNames, ", "),
		countBySeverity(findings, "BLOCKER"),
		countBySeverity(findings, "WARNING"),
		countBySeverity(findings, "SUGGESTION"),
	)

	result := &domain.SubagentResult{
		Status:          domain.SubagentSuccess,
		Summary:         summary,
		Artifacts:       []string{"review-receipt"},
		NextRecommended: "none",
		Risks:           riskCodeStrings(riskCodes),
		SkillResolution: "none",
	}

	// Store receipt reference in Engram if namespace is available.
	if ns := r.spawner.Namespace(); ns != nil {
		// The receipt is serialized and stored by the caller.
		_ = ns // placeholder — actual storage is done by the Engine caller
	}

	// Save receipt to filesystem for gate validation.
	store := gates.NewFSReceiptStore(".")
	if saveErr := store.SaveReceipt(receipt, tx.ChangeName); saveErr != nil {
		// Non-fatal: gate validation won't work, but review still succeeded.
		_ = saveErr
	}

	return result
}

// spawnerLLM adapts the agent.Spawner to the review.LensLLM interface.
type spawnerLLM struct {
	spawner      *agent.Spawner
	allowedTools []string
}

func (s *spawnerLLM) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	task := domain.SubagentTask{
		ID:           "lens-analysis",
		Description:  userMessage,
		AllowedTools: s.allowedTools,
	}

	resp, err := s.spawner.RunLoop(ctx, task, systemPrompt)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// extractFiles extracts file paths from the task context.
func extractFiles(task domain.SubagentTask) []string {
	var files []string
	if task.KGContext != nil {
		for _, ctx := range task.KGContext {
			// KGContext entries are formatted as "file: path/to/file" or similar.
			if strings.HasPrefix(ctx, "file:") {
				f := strings.TrimPrefix(ctx, "file:")
				files = append(files, strings.TrimSpace(f))
			}
		}
	}
	// If no files from context, scan the task description for paths.
	if len(files) == 0 && task.Description != "" {
		// Simple heuristic: extract quoted or space-separated paths.
		files = extractPathsFromText(task.Description)
	}
	return files
}

// extractPathsFromText extracts file-like tokens from a description.
func extractPathsFromText(text string) []string {
	var paths []string
	// Common file extensions to look for.
	extensions := []string{".go", ".md", ".yaml", ".yml", ".json", ".toml", ".sh"}
	for _, ext := range extensions {
		textLower := strings.ToLower(text)
		idx := 0
		for {
			pos := strings.Index(textLower[idx:], ext)
			if pos < 0 {
				break
			}
			pos += idx
			// Extract the token containing this extension.
			start := pos
			for start > 0 && text[start-1] != ' ' && text[start-1] != '\n' && text[start-1] != '"' && text[start-1] != '\'' {
				start--
			}
			end := pos + len(ext)
			if end < len(text) && text[end] != ' ' && text[end] != '\n' && text[end] != '"' && text[end] != '\'' {
				// Extension is part of a longer token; skip.
				idx = pos + 1
				continue
			}
			paths = append(paths, strings.TrimSpace(text[start:end]))
			idx = pos + 1
		}
	}
	return paths
}

// riskCodeStrings converts risk codes to their string representations.
func riskCodeStrings(codes []review.RiskCode) []string {
	strs := make([]string, len(codes))
	for i, c := range codes {
		strs[i] = string(c)
	}
	return strs
}

// countBySeverity counts findings at a given severity level.
func countBySeverity(findings []domain.ReviewFinding, severity string) int {
	count := 0
	for _, f := range findings {
		if f.Severity == severity {
			count++
		}
	}
	return count
}

var _ agent.Subagent = (*reviewer)(nil)
