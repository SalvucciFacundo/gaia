package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Brain orchestrates the agent loop: receive user input, call LLM,
// execute tool calls, enforce budget, delegate to subagents, and return results.
type Brain struct {
	provider     ports.LLMProvider
	repo         ports.Repository
	registry     *ToolRegistry
	guard        *ConfirmGuard
	ui           ports.UIService
	budget       domain.BudgetConfig
	onToken      func(string)           // streaming callback
	subagentPort ports.SubagentPort      // subagent delegation (nil if not wired)
	kgStore      ports.KnowledgeGraphStore // shared knowledge graph (nil if not wired)
}

// BrainOption configures the Brain.
type BrainOption func(*Brain)

// WithTokenCallback sets a function called for each streaming token.
func WithTokenCallback(fn func(string)) BrainOption {
	return func(b *Brain) { b.onToken = fn }
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

// Delegate dispatches a task to a named subagent and returns the structured result.
// Returns nil, error if no subagent port is wired or the subagent is unknown.
func (b *Brain) Delegate(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if b.subagentPort == nil {
		return nil, fmt.Errorf("subagent port not wired")
	}
	return b.subagentPort.Spawn(ctx, name, task)
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
// then SDD trigger keywords, and routes through the SDD pipeline when appropriate.
func (b *Brain) ProcessMessage(ctx context.Context, content string) error {
	// 0. @name direct routing
	if strings.HasPrefix(content, "@") {
		return b.handleDirectSubagent(ctx, content)
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

	// 3. Iteration loop with budget
	for iter := 0; iter < b.budget.MaxIterations; iter++ {
		history, err := b.repo.GetHistory(ctx, 50)
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
