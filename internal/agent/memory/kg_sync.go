package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gaia/internal/core"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// KnowledgeSync extracts discoveries, mutations, and conversational facts
// and persists them into the shared KnowledgeGraphStore.
type KnowledgeSync struct {
	store ports.KnowledgeGraphStore
	repo  ports.Repository
}

var _ ports.KnowledgeRecorder = (*KnowledgeSync)(nil)

// NewKnowledgeSync creates a new KnowledgeSync instance.
func NewKnowledgeSync(store ports.KnowledgeGraphStore, repo ports.Repository) *KnowledgeSync {
	return &KnowledgeSync{
		store: store,
		repo:  repo,
	}
}

// RecordSubagentResult extracts cross-domain discoveries from a subagent result
// and saves them to the shared knowledge graph.
func (s *KnowledgeSync) RecordSubagentResult(ctx context.Context, name, description string, result *domain.SubagentResult) {
	if s.store == nil || result == nil {
		return
	}

	projectRoot, _ := os.Getwd()
	projectLang := core.DetectLanguage(projectRoot)

	now := time.Now()
	saved := 0

	// 1. Save the subagent summary as a discovery fact
	if result.Summary != "" {
		summary := result.Summary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		id, err := s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       name,
			Concept:     description,
			Fact:        summary,
			SourceAgent: name,
			Labels:      []string{"discovery", "subagent-result"},
			CreatedAt:   now,
		})
		if err == nil && id != "" {
			saved++
		}
	}

	// 2. Save each artifact as a codebase fact
	for _, artifact := range result.Artifacts {
		if artifact == "" {
			continue
		}
		id, err := s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       "Codebase",
			Concept:     artifact,
			Fact:        fmt.Sprintf("Referenced by %s during: %s", name, description),
			SourceAgent: name,
			Labels:      []string{"artifact", "codebase"},
			CreatedAt:   now,
		})
		if err == nil && id != "" {
			saved++
		}
	}

	// 2b. Save user habits & preferences (from orchestrator-level observations)
	if name == "orchestrator" {
		_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       "User Habits",
			Scope:       "user",
			Language:    projectLang,
			Concept:     description,
			Fact:        result.Summary,
			SourceAgent: name,
			Labels:      []string{"preference", "user"},
			CreatedAt:   now,
		})
	}

	// 2c. Save language-level knowledge (applies to all projects in this language)
	if projectLang != "" {
		_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       name,
			Scope:       "language",
			Language:    projectLang,
			Concept:     description,
			Fact:        result.Summary,
			SourceAgent: name,
			Labels:      []string{"language", projectLang},
			CreatedAt:   now,
		})
	}

	// 3. Save each risk as a risk fact
	for _, risk := range result.Risks {
		if risk == "" {
			continue
		}
		riskText := risk
		if len(riskText) > 300 {
			riskText = riskText[:300] + "..."
		}
		id, err := s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       "Risks",
			Concept:     fmt.Sprintf("%s: %s", name, description),
			Fact:        riskText,
			SourceAgent: name,
			Labels:      []string{"risk", "warning"},
			CreatedAt:   now,
		})
		if err == nil && id != "" {
			saved++
		}
	}

	if saved > 0 && s.repo != nil {
		_ = s.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Saved %d knowledge facts from @%s.", saved, name),
		})
	}
}

// RecordToolExecution records direct inline tool mutations (file writes, git commits, configuration changes)
// into the knowledge graph so direct parent decisions and actions are not lost during compactions or session restarts.
func (s *KnowledgeSync) RecordToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result *domain.ToolResult) {
	if s.store == nil || result == nil || !result.Success {
		return
	}

	now := time.Now()
	switch toolName {
	case "file_write", "file_edit", "write_file", "edit_file", "file_patch", "apply_patch":
		filePath := ""
		if p, ok := args["path"].(string); ok {
			filePath = p
		} else if f, ok := args["file"].(string); ok {
			filePath = f
		}
		if filePath == "" {
			return
		}

		_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       "DirectAction",
			Concept:     filePath,
			Fact:        fmt.Sprintf("Directly modified file %s via %s", filePath, toolName),
			SourceAgent: "brain",
			Labels:      []string{"direct-action", "mutation", "file-change"},
			CreatedAt:   now,
		})

	case "git_commit", "git_stage", "git_ops":
		action := ""
		if a, ok := args["action"].(string); ok {
			action = a
		}
		_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
			Topic:       "DirectAction",
			Concept:     "git-operation",
			Fact:        fmt.Sprintf("Executed direct git action %q: %s", action, result.Output),
			SourceAgent: "brain",
			Labels:      []string{"direct-action", "git-ops"},
			CreatedAt:   now,
		})

	case "shell_exec":
		cmd := ""
		if c, ok := args["command"].(string); ok {
			cmd = c
		}
		if strings.HasPrefix(cmd, "go test") || strings.HasPrefix(cmd, "go build") || strings.HasPrefix(cmd, "npm ") || strings.HasPrefix(cmd, "cargo ") {
			_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
				Topic:       "DirectAction",
				Concept:     "build-test",
				Fact:        fmt.Sprintf("Direct shell execution %q: %s", cmd, result.Output),
				SourceAgent: "brain",
				Labels:      []string{"direct-action", "shell-exec"},
				CreatedAt:   now,
			})
		}
	}
}

// RecordResponseFacts analyzes a response and extracts key facts to store in the knowledge graph.
func (s *KnowledgeSync) RecordResponseFacts(ctx context.Context, response string) {
	if s.store == nil {
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
				_, _ = s.store.AddFact(ctx, domain.KnowledgeFact{
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
