package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Brain orchestrates the agent loop: receive user input, call LLM,
// execute tool calls, enforce budget, delegate to subagents, and return results.
type Brain struct {
	provider      ports.LLMProvider
	repo          ports.Repository
	registry      *ToolRegistry
	guard         *ConfirmGuard
	ui            ports.UIService
	budget        domain.BudgetConfig
	onToken       func(string)                // streaming callback
	subagentPort  ports.SubagentPort           // subagent delegation (nil if not wired)
	kgStore       ports.KnowledgeGraphStore    // shared knowledge graph (nil if not wired)
	compactedTo   int                          // messages compacted so far (context compaction)
	providerName  string                       // e.g. "openai"
	modelName     string                       // e.g. "gpt-4o"
	costTracker   *CostTracker                 // tracks LLM call costs
}

// BrainOption configures the Brain.
type BrainOption func(*Brain)

// WithTokenCallback sets a function called for each streaming token.
func WithTokenCallback(fn func(string)) BrainOption {
	return func(b *Brain) { b.onToken = fn }
}

// WithModelInfo sets the provider and model name for usage display.
func WithModelInfo(provider, model string) BrainOption {
	return func(b *Brain) {
		b.providerName = provider
		b.modelName = model
	}
}

// WithCostTracker wires a cost tracker into the Brain.
func WithCostTracker(ct *CostTracker) BrainOption {
	return func(b *Brain) {
		b.costTracker = ct
	}
}

// NewBrain creates a new Brain with the given dependencies.
func NewBrain(provider ports.LLMProvider, repo ports.Repository, ui ports.UIService, guard *ConfirmGuard, budget domain.BudgetConfig, opts ...BrainOption) *Brain {
	b := &Brain{
		provider: provider,
		repo:     repo,
		registry: NewToolRegistry(),
		guard:    guard,
		ui:       ui,
		budget:   budget,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// RegisterModule adds a module's tools to the registry.
func (b *Brain) RegisterModule(mod ports.Module) {
	b.registry.Register(mod)
}

// SetSubagentPort wires the subagent infrastructure into the Brain.
// Pass nil to disable subagent delegation.
func (b *Brain) SetSubagentPort(sp ports.SubagentPort) {
	b.subagentPort = sp
}

// SetKnowledgeGraphStore wires the shared knowledge graph into the Brain.
// When set, the Brain queries relevant facts before each turn and auto-populates
// subagent task KGContext fields. Pass nil to disable.
func (b *Brain) SetKnowledgeGraphStore(kg ports.KnowledgeGraphStore) {
	b.kgStore = kg
}

// queryKGContext searches the knowledge graph for facts relevant to the given text.
// Returns formatted KG facts or nil if the store is not wired.
func (b *Brain) queryKGContext(ctx context.Context, text string) []string {
	if b.kgStore == nil {
		return nil
	}

	// Search for facts matching the query — use keywords from the text
	facts, err := b.kgStore.SearchFacts(ctx, text)
	if err != nil || len(facts) == 0 {
		// Fall back to recent facts if search yields nothing
		recent, recentErr := b.kgStore.GetRecentFacts(ctx, 5)
		if recentErr != nil || len(recent) == 0 {
			return nil
		}
		facts = recent
	}

	result := make([]string, 0, len(facts))
	seen := make(map[string]bool)
	for _, f := range facts {
		key := f.Topic + "/" + f.Concept + "/" + f.Fact
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, fmt.Sprintf("[%s/%s] %s (by %s)", f.Topic, f.Concept, f.Fact, f.SourceAgent))
	}
	return result
}

// compactHistory compacts stale messages when the history exceeds the compaction threshold.
// Old messages (beyond keepRecent) are condensed into a single system message, reducing
// token usage on long conversations. Compaction is non-destructive — old messages remain
// in the database but are excluded from subsequent history fetches via compactedTo offset.
func (b *Brain) compactHistory(ctx context.Context) error {
	if b.budget.CompactionThreshold <= 0 {
		return nil // disabled
	}

	count, err := b.repo.GetMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("get message count for compaction: %w", err)
	}

	keepRecent := b.budget.KeepRecentMessages
	if keepRecent <= 0 {
		keepRecent = 20
	}

	// Only compact when history exceeds the threshold
	if count < b.budget.CompactionThreshold {
		return nil
	}

	compactCount := count - keepRecent
	if compactCount <= b.compactedTo {
		return nil // already compacted up to this point
	}

	// Fetch un-compacted old messages
	oldCount := compactCount - b.compactedTo
	oldMsgs, err := b.repo.GetHistoryFrom(ctx, oldCount, b.compactedTo)
	if err != nil {
		return fmt.Errorf("fetch old messages for compaction: %w", err)
	}
	if len(oldMsgs) == 0 {
		return nil
	}

	// Build compacted summary: drop tool outputs, condense user/assistant messages
	var sb strings.Builder
	sb.WriteString("Compacted conversation history (older messages):\n")
	for _, msg := range oldMsgs {
		// Skip tool role messages entirely — they're the longest and least relevant
		if msg.Role == domain.RoleTool {
			continue
		}

		prefix := strings.ToUpper(string(msg.Role))[:4]
		content := msg.Content

		// Truncate long messages
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		// Single-line compact format
		content = strings.ReplaceAll(content, "\n", " ")
		sb.WriteString(fmt.Sprintf("[%s] %s\n", prefix, content))
	}

	summary := sb.String()

	// Cap total compacted size at ~3000 chars (~750 tokens)
	if len(summary) > 3000 {
		summary = summary[:3000] + "\n...[truncated]"
	}

	// Save as a system message so it appears in the conversation context
	summaryMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: summary,
	}
	if err := b.repo.SaveMessage(ctx, summaryMsg); err != nil {
		return fmt.Errorf("save compaction summary: %w", err)
	}

	b.compactedTo = compactCount
	return nil
}

// UndoLastTurn removes the last user message and everything the AI generated
// in response, effectively rewinding the conversation by one turn.
func (b *Brain) UndoLastTurn(ctx context.Context) error {
	lastMsgs, err := b.repo.GetLastMessages(ctx, 2)
	if err != nil {
		return fmt.Errorf("undo: get last messages: %w", err)
	}

	// Find the last user message to know what to delete
	var lastUserID string
	for _, msg := range lastMsgs {
		if msg.Role == domain.RoleUser {
			lastUserID = msg.ID
			break
		}
	}
	if lastUserID == "" {
		undoMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Nothing to undo — no user message found.",
		}
		b.repo.SaveMessage(ctx, undoMsg)
		return b.ui.Display(undoMsg)
	}

	if err := b.repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("undo: delete messages: %w", err)
	}

	// Reset compaction state since history changed
	b.compactedTo = 0

	undoMsg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Last turn undone.",
	}
	b.repo.SaveMessage(ctx, undoMsg)
	return b.ui.Display(undoMsg)
}

// RetryLastTurn removes the last AI response and re-runs the last user message
// through the full agent loop.
func (b *Brain) RetryLastTurn(ctx context.Context) error {
	lastMsgs, err := b.repo.GetLastMessages(ctx, 2)
	if err != nil {
		return fmt.Errorf("retry: get last messages: %w", err)
	}

	// Find the last user message content
	var lastUserContent string
	var lastUserID string
	for _, msg := range lastMsgs {
		if msg.Role == domain.RoleUser {
			lastUserContent = msg.Content
			lastUserID = msg.ID
			break
		}
	}
	if lastUserContent == "" {
		errMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Nothing to retry — no previous user message.",
		}
		b.repo.SaveMessage(ctx, errMsg)
		return b.ui.Display(errMsg)
	}

	// Delete everything after the last user message
	if err := b.repo.DeleteMessagesAfter(ctx, lastUserID); err != nil {
		return fmt.Errorf("retry: delete messages: %w", err)
	}

	// Reset compaction state
	b.compactedTo = 0

	// Re-process the user message
	return b.ProcessMessage(ctx, lastUserContent)
}

// getHistory returns conversation history, filtering out compacted messages.
// If compaction has occurred (compactedTo > 0), returns only the recent messages
// (oldest compacted messages are excluded but their compaction summary exists).
func (b *Brain) getHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	if b.compactedTo > 0 {
		// Messages before compactedTo have been compacted into a summary.
		// Return the summary (most recent system message) + recent messages.
		history, err := b.repo.GetHistoryFrom(ctx, b.budget.KeepRecentMessages, b.compactedTo)
		if err != nil {
			return nil, err
		}
		// Also try to include the compaction summary (last system message)
		recent, err := b.repo.GetHistoryFrom(ctx, b.budget.KeepRecentMessages+5, 0)
		if err != nil {
			return history, nil // best-effort
		}
		// Find the compaction summary — it's the most recent system msg with COMPACTED prefix
		for i := len(recent) - 1; i >= 0; i-- {
			if recent[i].Role == domain.RoleSystem && strings.Contains(recent[i].Content, "Compacted conversation") {
				// Prepend summary then recent messages
				return append([]domain.Message{recent[i]}, history...), nil
			}
		}
		return history, nil
	}
	return b.repo.GetHistory(ctx, limit)
}

// Delegate dispatches a task to a named subagent and returns the structured result.
// After a successful delegation, it automatically saves subagent discoveries
// to the shared knowledge graph for cross-pollination.
// Returns nil, error if no subagent port is wired or the subagent is unknown.
func (b *Brain) Delegate(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if b.subagentPort == nil {
		return nil, fmt.Errorf("subagent port not wired")
	}
	result, err := b.subagentPort.Spawn(ctx, name, task)
	if err == nil && result != nil && result.Status != domain.SubagentBlocked {
		b.saveSubagentDiscoveries(ctx, name, task.Description, result)
	}
	return result, err
}

// saveSubagentDiscoveries extracts cross-domain discoveries from a subagent result
// and saves them to the shared knowledge graph. Non-fatal — errors are logged as
// system messages but don't interrupt the flow.
func (b *Brain) saveSubagentDiscoveries(ctx context.Context, name, description string, result *domain.SubagentResult) {
	projectRoot, _ := os.Getwd()
	projectName := DetectProjectName(projectRoot); _ = projectName
	projectLang := DetectLanguage(projectRoot)
	if b.kgStore == nil {
		return
	}

	now := time.Now()
	saved := 0

	// 1. Save the subagent's summary as a discovery fact
	if result.Summary != "" {
		summary := result.Summary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		id, err := b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
		id, err := b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
		b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
		b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
		id, err := b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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

	if saved > 0 {
		b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Saved %d knowledge facts from @%s.", saved, name),
		})
	}
}

// AvailableSubagents returns the list of registered subagent names.
func (b *Brain) AvailableSubagents() []string {
	if b.subagentPort == nil {
		return nil
	}
	return b.subagentPort.Available()
}

// Registry returns the brain's tool registry for use by subagent infrastructure.
func (b *Brain) Registry() *ToolRegistry {
	return b.registry
}

// ProcessMessage handles a user input through the full agent loop.
// Before the standard iteration loop, it checks for @name direct routing,
// then SDD trigger keywords, /undo, /retry, and routes accordingly.
func (b *Brain) ProcessMessage(ctx context.Context, content string) error {
	// 0. @name direct routing
	if strings.HasPrefix(content, "@") {
		return b.handleDirectSubagent(ctx, content)
	}

	// 0b. /undo — revert the last turn
	if content == "/undo" {
		return b.UndoLastTurn(ctx)
	}

	// 0c. /retry — re-run the last user message
	if content == "/retry" {
		return b.RetryLastTurn(ctx)
	}

	// 1. SDD trigger detection
	trigger := DetectSDDTrigger(content)
	if trigger.ShouldSDD {
		return b.handleSDDTrigger(ctx, content, trigger)
	}

	// 2. Create user message
	userMsg := domain.Message{
		Role:    domain.RoleUser,
		Content: content,
	}
	if err := b.repo.SaveMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	// 2b. Inject relevant knowledge graph facts as context
	if kgFacts := b.queryKGContext(ctx, content); len(kgFacts) > 0 {
		kgMsg := domain.Message{
			Role: domain.RoleSystem,
			Content: "Knowledge graph facts relevant to this task:\n" +
				strings.Join(kgFacts, "\n"),
		}
		b.repo.SaveMessage(ctx, kgMsg)
	}

	// 2c. Context compaction: condense stale history when the conversation is long.
	// This runs before the LLM loop so the compacted summary is available.
	if err := b.compactHistory(ctx); err != nil {
		// Non-fatal — log and continue with full history
		b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Warning: context compaction failed: %v", err),
		})
	}

	// 3. Iteration loop with budget
	for iter := 0; iter < b.budget.MaxIterations; iter++ {
		history, err := b.getHistory(ctx, 50)
		if err != nil {
			return fmt.Errorf("get history: %w", err)
		}

		// 3. Call LLM — prefer streaming, fall back to non-streaming
		var response *domain.Message
		stream, err := b.provider.Stream(ctx, history)
		if err != nil {
			// Fall back to non-streaming Chat for this iteration
			resp, chatErr := b.provider.Chat(ctx, history)
			if chatErr != nil {
				return fmt.Errorf("llm error: %w", chatErr)
			}
			response = resp
		} else {
			response, err = b.readStream(ctx, stream)
			stream.Close()
			if err != nil {
				// Fall back to non-streaming on read failure
				resp, chatErr := b.provider.Chat(ctx, history)
				if chatErr != nil {
					return fmt.Errorf("llm error: %w", chatErr)
				}
				response = resp
			}
		}

		// 4. Handle tool calls
		if len(response.ToolCalls) > 0 {
			if err := b.handleToolCalls(ctx, response); err != nil {
				return err
			}
			continue // Let LLM see results
		}

		// 5. Save and display final response
		if err := b.repo.SaveMessage(ctx, *response); err != nil {
			return fmt.Errorf("save assistant response: %w", err)
		}
		return b.ui.Display(*response)
	}

	// Budget exhausted
	errMsg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("Iteration budget exhausted (%d iterations). Stopping.", b.budget.MaxIterations),
	}
	b.repo.SaveMessage(ctx, *errMsg)
	return b.ui.Display(*errMsg)
}

// readStream reads token chunks from the stream and builds a final message.
func (b *Brain) readStream(ctx context.Context, reader io.Reader) (*domain.Message, error) {
	response := &domain.Message{Role: domain.RoleAssistant}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		var chunk domain.TokenChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("stream error: %s", chunk.Error)
		}
		response.Content += chunk.Content
		if b.onToken != nil {
			b.onToken(chunk.Content)
		}
	}
	return response, scanner.Err()
}

func (b *Brain) handleToolCalls(ctx context.Context, msg *domain.Message) error {
	for _, tc := range msg.ToolCalls {
		// Confirmation gate
		if b.guard != nil && b.guard.ShouldConfirm(tc.Name) {
			confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf("Allow tool %s with args %v?", tc.Name, tc.Arguments))
			if err != nil || !confirmed {
				toolMsg := domain.Message{
					Role:    domain.RoleTool,
					Content: "User denied tool execution.",
				}
				b.repo.SaveMessage(ctx, toolMsg)
				continue
			}
			b.guard.Approve(tc.Name)
		}

		// Execute via registry
		result, execErr := b.registry.Execute(ctx, tc.Name, tc.Arguments)
		if execErr != nil {
			result = &domain.ToolResult{
				Success: false,
				Error:   execErr.Error(),
			}
		}

		output := result.Output
		if !result.Success {
			output = fmt.Sprintf("Error: %s", result.Error)
		}

		// Apply message redaction to tool output
		output, _ = b.RedactToolOutput(output)

		toolMsg := domain.Message{
			Role:    domain.RoleTool,
			Content: output,
		}
		b.repo.SaveMessage(ctx, toolMsg)
	}

	// Save the assistant message that triggered these tool calls
	b.repo.SaveMessage(ctx, *msg)
	return nil
}

// handleDirectSubagent parses @name syntax and routes the message directly
// to the named subagent. If the subagent is unknown, an error is returned with
// the available subagent list. Uses SpawnAsync when available, falls back to
// synchronous Spawn.
func (b *Brain) handleDirectSubagent(ctx context.Context, content string) error {
	// Parse: @name rests of message
	trimmed := strings.TrimPrefix(content, "@")
	parts := strings.SplitN(trimmed, " ", 2)
	name := parts[0]
	message := ""
	if len(parts) > 1 {
		message = parts[1]
	}

	// Validate subagent exists
	available := b.AvailableSubagents()
	found := false
	for _, a := range available {
		if a == name {
			found = true
			break
		}
	}
	if !found {
		errMsg := &domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Unknown subagent: @%s\nAvailable: %s", name, strings.Join(available, ", ")),
		}
		return b.ui.Display(*errMsg)
	}

	task := domain.SubagentTask{
		ID:           fmt.Sprintf("direct-%s-%d", name, time.Now().UnixNano()),
		Description:  message,
		Mode:         "plan",
		IsDirectChat: true,
		KGContext:    b.queryKGContext(ctx, message),
	}

	// Try async spawn first
	asyncPort, isAsync := b.subagentPort.(ports.AsyncSpawner)
	if isAsync {
		taskID, err := asyncPort.SpawnAsync(ctx, name, task)
		if err != nil {
			return b.ui.Display(domain.Message{
				Role:    domain.RoleAssistant,
				Content: fmt.Sprintf("Error spawning @%s: %v", name, err),
			})
		}
		return b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Dispatched to @%s (task %s)", name, taskID[:8]),
		})
	}

	// Fall back to synchronous spawn
	result, err := b.Delegate(ctx, name, task)
	if err != nil {
		return b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("Error running @%s: %v", name, err),
		})
	}

	return b.ui.Display(domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("[@%s] %s: %s", name, result.Status, result.Summary),
	})
}

// handleSDDTrigger routes a detected SDD-triggering message through the
// SDD pipeline asynchronously via PipelineRunner.
func (b *Brain) handleSDDTrigger(ctx context.Context, content string, trigger TriggerResult) error {
	// If /direct was used, process normally
	if trigger.ForceDirect {
		return b.processDirect(ctx, content)
	}

	// Check if subagent port is wired
	if b.subagentPort == nil {
		msg := &domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD trigger detected (%s), but subagent system is not available.", trigger.Reason),
		}
		return b.ui.Display(*msg)
	}

	// Strip command prefix if present
	taskDesc := content
	if trigger.ForceSDD {
		taskDesc = content[len("+/sdd"):]
	}

	// Build the SDD pipeline phases
	phases := domain.SDDPhases(taskDesc)
	baseTask := domain.SubagentTask{
		ID:        "sdd-pipeline",
		Mode:      "plan",
		KGContext: b.queryKGContext(ctx, taskDesc),
	}

	// Try async pipeline via AsyncSpawner
	if asyncPort, isAsync := b.subagentPort.(ports.AsyncSpawner); isAsync {
		// Display trigger notification
		b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("SDD pipeline triggered (%s). Running 7 phases asynchronously...", trigger.Reason),
		})

		results, err := asyncPort.RunPipeline(ctx, phases, baseTask)
		if err != nil {
			return b.ui.Display(domain.Message{
				Role:    domain.RoleAssistant,
				Content: fmt.Sprintf("SDD pipeline failed: %v", err),
			})
		}

		// Save discoveries from all pipeline phases
		for i, phase := range phases {
			if i < len(results) && results[i] != nil {
				b.saveSubagentDiscoveries(ctx, phase.SubagentName, taskDesc, results[i])
			}
		}

		finalMsg := buildAsyncSDDPipelineSummary(trigger.Reason, results)
		return b.ui.Display(finalMsg)
	}

	// Fall back to synchronous path
	b.ui.Display(domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("SDD pipeline triggered (%s). Delegating to Explorer...", trigger.Reason),
	})

	explorerTask := domain.SubagentTask{
		ID:          "sdd-explore-001",
		Description: taskDesc,
		Mode:        "plan",
	}
	exploreResult, err := b.Delegate(ctx, "explorer", explorerTask)
	if err != nil {
		return fmt.Errorf("explorer phase: %w", err)
	}
	if exploreResult.Status == domain.SubagentBlocked {
		return b.ui.Display(domain.Message{
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
	propResult, err := b.Delegate(ctx, "proposer", proposerTask)
	if err != nil {
		return fmt.Errorf("proposer phase: %w", err)
	}
	if propResult.Status == domain.SubagentBlocked {
		return b.ui.Display(domain.Message{
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
	specResult, err := b.Delegate(ctx, "specifier", specifierTask)
	if err != nil {
		return fmt.Errorf("specifier phase: %w", err)
	}
	if specResult.Status == domain.SubagentBlocked {
		return b.ui.Display(domain.Message{
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
	implResult, err := b.Delegate(ctx, "implementer", implementerTask)
	if err != nil {
		return fmt.Errorf("implementer phase: %w", err)
	}

	verifierTask := domain.SubagentTask{
		ID:          "sdd-verify-001",
		Description: fmt.Sprintf("Verify implementation: %s", implResult.Summary),
		KGContext:   implResult.Artifacts,
		Mode:        "plan",
	}
	verResult, err := b.Delegate(ctx, "verifier", verifierTask)
	if err != nil {
		return fmt.Errorf("verifier phase: %w", err)
	}

	finalMsg := buildSDDPipelineSummary(trigger.Reason, exploreResult, propResult, specResult, implResult, verResult)
	return b.ui.Display(finalMsg)
}

// buildAsyncSDDPipelineSummary creates a final summary from the async pipeline results.
func buildAsyncSDDPipelineSummary(reason string, results []*domain.SubagentResult) domain.Message {
	content := fmt.Sprintf("## SDD Pipeline Complete\n\n**Trigger**: %s\n\n", reason)

	content += "### Pipeline Results\n"
	for i, r := range results {
		phaseName := domain.SDDPhases("")[i].SubagentName
		content += fmt.Sprintf("- **%s**: %s — %s\n", phaseName, r.Status, r.Summary)
	}

	content += "\n### Artifacts\n"
	for _, r := range results {
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

// processDirect handles /direct messages by stripping the command prefix
// and processing normally.
func (b *Brain) processDirect(ctx context.Context, content string) error {
	desc := strings.TrimSpace(strings.TrimPrefix(content, DirectCommandPrefix+" "))
	userMsg := domain.Message{
		Role:    domain.RoleUser,
		Content: desc,
	}
	return b.repo.SaveMessage(ctx, userMsg)
}

// RedactToolOutput applies message redaction to tool outputs before
// they are fed back to the LLM.
func (b *Brain) RedactToolOutput(output string) (string, int) {
	return redactToolOutput(output)
}

// buildSDDPipelineSummary creates a final summary message from all
// five SDD pipeline phases.
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

// --- Redaction helpers (local to avoid circular import from agent package) ---

var redactPatterns = []struct {
	pat     *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`sk-[a-zA-Z0-9\-]{20,}`), "[REDACTED:API_KEY]"},
	{regexp.MustCompile(`ghp_[a-zA-Z0-9]{36,}`), "[REDACTED:GITHUB_TOKEN]"},
	{regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{20,}`), "[REDACTED:GITHUB_TOKEN]"},
	{regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[^-]*-----END [A-Z ]+ PRIVATE KEY-----`), "[REDACTED:PRIVATE_KEY]"},
	{regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`), "Bearer [REDACTED:TOKEN]"},
	{regexp.MustCompile(`eyJ[a-zA-Z0-9\-_]+\.eyJ[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`), "[REDACTED:JWT]"},
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "[REDACTED:AWS_KEY]"},
	{regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api[_-]?key)\s*[:=]\s*\S+`), "${1}=[REDACTED:SECRET]"},
}

func redactToolOutput(output string) (string, int) {
	result := output
	count := 0
	for _, rp := range redactPatterns {
		matches := rp.pat.FindAllString(result, -1)
		if len(matches) > 0 {
			result = rp.pat.ReplaceAllString(result, rp.replace)
			count += len(matches)
		}
	}
	return result, count
}
















