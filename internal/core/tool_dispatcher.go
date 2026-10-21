package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"gaia/internal/core/domain"
)

// toolCallItem tracks execution lifecycle and results for one tool call in a turn.
type toolCallItem struct {
	tc      domain.ToolCall
	result  *domain.ToolResult
	skipped bool
	denied  bool
	errMsg  string
}

// isParallelSafeTool returns true if a tool call is read-only and requires no human confirmation.
func (b *Brain) isParallelSafeTool(tc domain.ToolCall) bool {
	// Guard confirmation check
	if b.guard != nil && b.guard.ShouldConfirm(tc.Name) {
		return false
	}
	// Policy check: must be explicitly allowed without prompt
	if b.policy != nil {
		res := b.policy.Evaluate(tc.Name, tc.Arguments)
		if !res.Allowed {
			return false
		}
	}
	// Tier check: must be read-only tier
	return ClassifyTool(tc.Name) == TierRead
}

func (b *Brain) handleToolCalls(ctx context.Context, msg *domain.Message) error {
	n := len(msg.ToolCalls)
	if n == 0 {
		return nil
	}

	items := make([]toolCallItem, n)
	for i, tc := range msg.ToolCalls {
		items[i].tc = tc
	}

	// Partition and dispatch: contiguous parallel-safe tools run concurrently in goroutines;
	// state-mutating or interactive tools run sequentially.
	i := 0
	for i < n {
		if b.isParallelSafeTool(items[i].tc) {
			start := i
			for i < n && b.isParallelSafeTool(items[i].tc) {
				i++
			}
			end := i

			if end-start == 1 {
				res, err := b.registry.Execute(ctx, items[start].tc.Name, items[start].tc.Arguments)
				if err != nil {
					res = &domain.ToolResult{
						Success: false,
						Error:   err.Error(),
					}
				}
				items[start].result = res
			} else {
				// Execute parallel batch concurrently with bounded worker limit
				g, gCtx := errgroup.WithContext(ctx)
				g.SetLimit(8)
				for j := start; j < end; j++ {
					idx := j
					g.Go(func() error {
						res, err := b.registry.Execute(gCtx, items[idx].tc.Name, items[idx].tc.Arguments)
						if err != nil {
							res = &domain.ToolResult{
								Success: false,
								Error:   err.Error(),
							}
						}
						items[idx].result = res
						return nil
					})
				}
				_ = g.Wait()
			}
		} else {
			// Sequential execution with policy evaluation and confirmation gates
			tc := items[i].tc

			// Policy guard evaluation (additive — runs before ConfirmGuard)
			if b.policy != nil {
				result := b.policy.Evaluate(tc.Name, tc.Arguments)
				if !result.Allowed {
					switch result.SuggestedAction {
					case "skip":
						items[i].skipped = true
						i++
						continue
					case "alternative":
						items[i].denied = true
						items[i].errMsg = fmt.Sprintf("Policy denied: %s (reason: %s). Try an alternative tool.", tc.Name, result.Reason)
						i++
						continue
					case "ask_user":
						b.stateMu.Lock()
						b.pendingConfirm = tc.Name
						b.stateMu.Unlock()

						confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf(
							"Policy blocks %s (reason: %s). Allow anyway?",
							tc.Name, result.Reason))

						b.stateMu.Lock()
						b.pendingConfirm = ""
						b.stateMu.Unlock()

						if err != nil || !confirmed {
							items[i].denied = true
							items[i].errMsg = fmt.Sprintf("User denied policy-blocked tool %s.", tc.Name)
							i++
							continue
						}
						b.policy.Session().ApproveSession(tc.Name)
					default:
						items[i].denied = true
						items[i].errMsg = fmt.Sprintf("Policy blocked: %s (reason: %s)", tc.Name, result.Reason)
						i++
						continue
					}
				}
			}

			// Confirmation gate (legacy ConfirmGuard — kept for backward compat)
			if b.guard != nil && b.guard.ShouldConfirm(tc.Name) {
				b.stateMu.Lock()
				b.pendingConfirm = tc.Name
				b.stateMu.Unlock()

				confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf("Allow tool %s with args %v?", tc.Name, tc.Arguments))

				b.stateMu.Lock()
				b.pendingConfirm = ""
				b.stateMu.Unlock()

				if err != nil || !confirmed {
					items[i].denied = true
					items[i].errMsg = "User denied tool execution."
					i++
					continue
				}
				b.guard.Approve(tc.Name)
			}

			// Execute via registry
			res, err := b.registry.Execute(ctx, tc.Name, tc.Arguments)
			if err != nil {
				res = &domain.ToolResult{
					Success: false,
					Error:   err.Error(),
				}
			}
			items[i].result = res
			i++
		}
	}

	// Save the assistant message that triggered these tool calls BEFORE the tool results,
	// preserving valid OpenAI message sequence (assistant tool_calls -> tool response).
	if err := b.repo.SaveMessage(ctx, *msg); err != nil {
		return fmt.Errorf("save assistant message: %w", err)
	}

	// Post-execution: process audit, facts, budget tracking, and persistence in exact original index order
	for idx := range items {
		item := &items[idx]
		if item.skipped {
			continue
		}
		if item.denied {
			toolMsg := domain.Message{
				Role:    domain.RoleTool,
				Content: item.errMsg,
			}
			b.repo.SaveMessage(ctx, toolMsg)
			continue
		}

		result := item.result
		if result == nil {
			result = &domain.ToolResult{Success: false, Error: "no result returned"}
		}

		// Audit log: record tool execution if policy has audit override
		b.logAudit(item.tc.Name, item.tc.Arguments, result.Success)

		// Parent Hot Memory: persist direct inline tool execution / mutations into KnowledgeGraph
		if b.kgRecorder != nil {
			b.kgRecorder.RecordToolExecution(ctx, item.tc.Name, item.tc.Arguments, result)
		}

		output := result.Output
		if !result.Success {
			output = fmt.Sprintf("Error: %s", result.Error)
		}

		// Apply message redaction to tool output
		output, _ = b.RedactToolOutput(output)

		// Record tool execution into WorkflowRouter if wired
		if b.workflowRouter != nil {
			b.workflowRouter.RecordToolCall(item.tc.Name, item.tc.Arguments, output)
		}

		toolMsg := domain.Message{
			Role:    domain.RoleTool,
			Content: output,
		}
		b.repo.SaveMessage(ctx, toolMsg)
	}

	// Auto-Repair Feedback Loop:
	// If a mutation verifier is wired and any mutation tool was executed successfully,
	// run compilation/lint checks and inject repair feedback into history if broken.
	if b.mutationVerifier != nil {
		modifiedPaths := make(map[string]bool)
		for _, item := range items {
			if !item.skipped && !item.denied && item.result != nil && item.result.Success {
				if b.mutationVerifier.IsMutationTool(item.tc.Name) {
					if p := b.mutationVerifier.ExtractModifiedPaths(item.tc.Arguments); p != "" {
						modifiedPaths[p] = true
					}
				}
			}
		}

		if len(modifiedPaths) > 0 {
			paths := make([]string, 0, len(modifiedPaths))
			for path := range modifiedPaths {
				paths = append(paths, path)
			}
			repairPrompt, err := b.mutationVerifier.VerifyMutations(ctx, paths)
			if err == nil && repairPrompt != "" {
				repairMsg := domain.Message{
					Role:    domain.RoleSystem,
					Content: repairPrompt,
				}
				b.repo.SaveMessage(ctx, repairMsg)
			}
		}
	}

	return nil
}

// logAudit records a tool execution in the audit trail if the tool has an "audit" override.
func (b *Brain) logAudit(toolName string, args map[string]interface{}, success bool) {
	if b.policy == nil {
		return
	}
	ov, ok := b.policy.Override(toolName)
	if !ok || ov != OverrideAudit {
		return
	}
	var argStr string
	if len(args) > 0 {
		parts := make([]string, 0, len(args))
		for k, v := range args {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		argStr = strings.Join(parts, " ")
	}
	entry := AuditEntry{
		Time:    time.Now(),
		Tool:    toolName,
		Args:    argStr,
		Tier:    string(b.policy.Tier()),
		Allowed: success,
	}
	b.stateMu.Lock()
	b.auditLog = append(b.auditLog, entry)
	b.stateMu.Unlock()
}

