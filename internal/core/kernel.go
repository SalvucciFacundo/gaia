package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

var (
	// ErrSubagentPortNotWired is returned when subagent delegation is attempted without a wired port.
	ErrSubagentPortNotWired = errors.New("subagent port not wired")
)

const (
	// GAIAVersion is the current version, set at build time via -ldflags.
	GAIAVersion = "development"
)

// AuditEntry records a single policy-audited tool execution.
type AuditEntry struct {
	Time    time.Time
	Tool    string
	Args    string
	Tier    string
	Allowed bool
	Reason  string
}

// Brain orchestrates the agent loop: receive user input, call LLM,
// execute tool calls, enforce budget, delegate to subagents, and return results.
type Brain struct {
	stateMu            sync.RWMutex                    // guards currentSessionID, pendingImages, onToken across concurrent calls
	provider           ports.LLMProvider
	repo               ports.Repository
	registry           *ToolRegistry
	guard              *ConfirmGuard
	policy             *PolicyGuard // policy-based guard (additive, not replacement)
	ui                 ports.UIService
	budget             domain.BudgetConfig
	onToken            func(string)                    // streaming callback
	subagentPort       ports.SubagentPort              // subagent delegation (nil if not wired)
	kgStore            ports.KnowledgeGraphStore       // shared knowledge graph (nil if not wired)
	kgEnabled          bool                            // knowledge graph recall toggle (/kg)
	commandDispatcher  ports.CommandDispatcher         // pluggable slash command dispatcher (nil = use internal)
	workflowRouter     ports.WorkflowRouter            // pluggable workflow router for ODD/SDD methodologies
	mutationVerifier   ports.MutationVerifier          // post-tool mutation compiler/linter verifier
	moaRunner          ports.MoARunner                 // pluggable Mixture-of-Agents consensus runner
	kgRecorder         ports.KnowledgeRecorder         // captures domain facts, mutations, and subagent discoveries
	compactedTo        int                             // messages compacted so far (context compaction)
	providerName       string                          // e.g. "openai"
	modelName          string                          // e.g. "gpt-4o"
	costTracker        *CostTracker                    // tracks LLM call costs
	currentSessionID   string                          // tracked for /title and /resume
	messageQueue       []string                        // queued messages for /queue command
	processingQueue    bool                            // prevents re-entrant queue processing
	steerCh            chan string                     // buffered channel (cap 1) for /steer mid-loop injection
	goal               string                          // active persistent goal (/goal)
	subgoals           []string                        // subgoal criteria (/subgoal)
	goalTurns          int                             // turn counter for active goal auto-continuation
	availableProviders map[string]ports.LLMProvider    // named providers for /model switching
	auditLog           []AuditEntry                    // audit trail for policy-audited tools (/audit)
	sessionMgr         *SessionManager                 // routes messages across platforms
	skillLister        func() []string                 // returns available skill names for progressive loading
	skillPathResolver  func([]string) []string         // resolves skill names to SKILL.md file paths
	skillMatcher       func(string, []string) []string // matches skills by task intent and file context
	mcpMgr             ports.MCPManager                // MCP server connection manager
	personaResolver    ports.PersonaResolver           // resolves persona names to prompt instructions
	pendingImages      []domain.ImageContent           // images queued for the next ProcessMessage
	fastModeEnabled    bool                            // whether /fast mode is active
	originalModel      string                          // model before /fast was enabled
	fastModel          string                          // override fast model name
	busyMode           string                          // input behavior while agent is working: queue, steer, ignore
	reasoningEffort    string                          // "low", "medium", "high"
	personaName        string                          // active personality name
	personaOverlay     string                          // system prompt instructions derived from persona
	pendingConfirm     string                          // tool awaiting interactive confirmation (/approve or /deny)
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

// WithMoATimeout is a legacy option kept for backward compatibility.
func WithMoATimeout(d time.Duration) BrainOption {
	return func(b *Brain) {}
}

type noopUI struct{}

func (n *noopUI) Display(domain.Message) error                       { return nil }
func (n *noopUI) AppendToken(string) error                           { return nil }
func (n *noopUI) PromptConfirmation(string) (bool, error)            { return true, nil }
func (n *noopUI) PromptChoice(domain.ChoiceEnvelope) (string, error) { return "", nil }
func (n *noopUI) Run() error                                         { return nil }

// NewBrain creates a new Brain with the given dependencies.
func NewBrain(provider ports.LLMProvider, repo ports.Repository, ui ports.UIService, guard *ConfirmGuard, budget domain.BudgetConfig, opts ...BrainOption) *Brain {
	if ui == nil {
		ui = &noopUI{}
	}
	b := &Brain{
		provider: provider,
		repo:     repo,
		registry: NewToolRegistry(),
		guard:    guard,
		ui:       ui,
		budget:   budget,
		steerCh:  make(chan string, 1),
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
	b.SetKGEnabled(kg != nil)
}

// SetKGEnabled enables or disables knowledge graph context recall in a thread-safe manner.
func (b *Brain) SetKGEnabled(enabled bool) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.kgEnabled = enabled
}

// KGEnabled returns whether knowledge graph context recall is currently enabled.
func (b *Brain) KGEnabled() bool {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.kgEnabled
}

// SetCommandDispatcher wires an external command dispatcher into the Brain.
func (b *Brain) SetCommandDispatcher(cd ports.CommandDispatcher) {
	b.commandDispatcher = cd
}

// DispatchCommand delegates to commandDispatcher if set.
func (b *Brain) DispatchCommand(ctx context.Context, content string) (bool, error) {
	if b.commandDispatcher != nil {
		return b.commandDispatcher.Dispatch(ctx, content)
	}
	return false, nil
}

// SetWorkflowRouter wires an external workflow router into the Brain.
func (b *Brain) SetWorkflowRouter(wr ports.WorkflowRouter) {
	b.workflowRouter = wr
}

// SetMutationVerifier wires an external mutation compiler/linter verifier into the Brain.
func (b *Brain) SetMutationVerifier(mv ports.MutationVerifier) {
	b.mutationVerifier = mv
}

// SetPolicyGuard wires the policy guard into the Brain for tool call evaluation.
// Pass nil to disable policy enforcement (guard-only mode).
func (b *Brain) SetPolicyGuard(pg *PolicyGuard) {
	b.policy = pg
}

// SetMoARunner wires the Mixture-of-Agents consensus runner into the Brain.
func (b *Brain) SetMoARunner(r ports.MoARunner) {
	b.moaRunner = r
}

// MoARunner returns the wired Mixture-of-Agents runner, if any.
func (b *Brain) MoARunner() ports.MoARunner {
	return b.moaRunner
}

// SetKnowledgeRecorder wires an external knowledge fact recorder into the Brain.
func (b *Brain) SetKnowledgeRecorder(kr ports.KnowledgeRecorder) {
	b.kgRecorder = kr
}

// KnowledgeRecorder returns the wired knowledge fact recorder, if any.
func (b *Brain) KnowledgeRecorder() ports.KnowledgeRecorder {
	return b.kgRecorder
}

// SetSessionManager wires the session manager into the Brain.
func (b *Brain) SetSessionManager(sm *SessionManager) {
	b.sessionMgr = sm
}

// SetSkillLister wires a function that returns available skill names.
func (b *Brain) SetSkillLister(lister func() []string) {
	b.skillLister = lister
}

// SetSkillPathResolver wires a function that resolves skill names to file paths.
func (b *Brain) SetSkillPathResolver(resolver func([]string) []string) {
	b.skillPathResolver = resolver
}

// SetSkillMatcher wires a function that matches skills by task intent and target files.
func (b *Brain) SetSkillMatcher(matcher func(string, []string) []string) {
	b.skillMatcher = matcher
}

// SetReasoningEffort changes the reasoning effort level for the current provider.
// Supported values: "low", "medium", "high". Passed to the provider on next Chat/Stream calls.
func (b *Brain) SetReasoningEffort(level string) {
	clean := strings.ToLower(strings.TrimSpace(level))
	b.stateMu.Lock()
	b.reasoningEffort = clean
	b.stateMu.Unlock()
	fmt.Printf("Reasoning effort set to: %s (provider support may vary)\n", clean)
}

// ReasoningEffort returns the current reasoning effort level.
func (b *Brain) ReasoningEffort() string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.reasoningEffort
}

// SetPersonaResolver wires an external personality overlay resolver into the Brain.
func (b *Brain) SetPersonaResolver(pr ports.PersonaResolver) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.personaResolver = pr
}

// SetPersona changes the agent personality/behavior overlay.
func (b *Brain) SetPersona(name string) {
	clean := strings.ToLower(strings.TrimSpace(name))
	var text string
	b.stateMu.RLock()
	resolver := b.personaResolver
	b.stateMu.RUnlock()

	if resolver != nil {
		if resolved, err := resolver.Resolve(clean); err == nil {
			text = resolved
		}
	}

	b.stateMu.Lock()
	b.personaName = clean
	b.personaOverlay = text
	b.stateMu.Unlock()
	fmt.Printf("Persona set to: %s (will apply to next interactions)\n", clean)
}

// Persona returns the active persona name and overlay prompt.
func (b *Brain) Persona() (string, string) {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.personaName, b.personaOverlay
}



// getHistory returns conversation history, filtering out compacted messages.
// If compaction has occurred (compactedTo > 0), returns only the recent messages
// (oldest compacted messages are excluded but their compaction summary exists).
func (b *Brain) getHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	b.stateMu.RLock()
	compactedTo := b.compactedTo
	personaOverlay := b.personaOverlay
	personaName := b.personaName
	b.stateMu.RUnlock()

	var msgs []domain.Message
	if compactedTo > 0 {
		// Messages before compactedTo have been compacted into a summary.
		// Return the summary (most recent system message) + recent messages.
		history, err := b.repo.GetHistoryFrom(ctx, b.budget.KeepRecentMessages, compactedTo)
		if err != nil {
			return nil, err
		}
		// Also try to include the compaction summary (last system message)
		recent, err := b.repo.GetLastMessages(ctx, b.budget.KeepRecentMessages+5)
		if err != nil {
			msgs = history
		} else {
			foundSummary := false
			// GetLastMessages returns newest first
			for _, m := range recent {
				if m.Role == domain.RoleSystem && strings.Contains(m.Content, "Compacted conversation") {
					msgs = append([]domain.Message{m}, history...)
					foundSummary = true
					break
				}
			}
			if !foundSummary {
				msgs = history
			}
		}
	} else {
		h, err := b.repo.GetHistory(ctx, limit)
		if err != nil {
			return nil, err
		}
		msgs = h
	}

	if personaOverlay != "" {
		personaMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("PERSONALITY OVERLAY [%s]:\n%s", strings.ToUpper(personaName), personaOverlay),
		}
		msgs = append([]domain.Message{personaMsg}, msgs...)
	}

	return msgs, nil
}

// Registry returns the brain's tool registry for use by subagent infrastructure.
func (b *Brain) Registry() *ToolRegistry {
	return b.registry
}

// PolicyGuard returns the brain's policy guard (nil if not configured).
func (b *Brain) PolicyGuard() *PolicyGuard {
	return b.policy
}

// Step executes a single turn through the full agent loop (implements gateway.BrainRunner).
func (b *Brain) Step(ctx context.Context, input string) error {
	return b.ProcessMessage(ctx, input)
}

// SetActiveConversation updates the current active session/conversation id.
func (b *Brain) SetActiveConversation(id string) {
	b.stateMu.Lock()
	b.currentSessionID = id
	b.stateMu.Unlock()
	if sr, ok := b.repo.(interface{ SetSessionID(string) }); ok {
		sr.SetSessionID(id)
	}
}

// CurrentSessionID returns the active session ID safely under lock.
func (b *Brain) CurrentSessionID() string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.currentSessionID
}

// StepWithSessionAndAttachments processes one user turn bound to a specific session and token sink with attachments.
func (b *Brain) StepWithSessionAndAttachments(ctx context.Context, sessionID string, input string, attachments []Attachment, sink func(string)) error {
	if sessionID != "" {
		b.SetActiveConversation(sessionID)
	}
	if sink != nil {
		b.stateMu.Lock()
		oldSink := b.onToken
		b.onToken = sink
		b.stateMu.Unlock()
		defer func() {
			b.stateMu.Lock()
			b.onToken = oldSink
			b.stateMu.Unlock()
		}()
	}
	if len(attachments) > 0 {
		for _, att := range attachments {
			if strings.HasPrefix(att.MimeType, "image/") || att.Type == "image" {
				mime := att.MimeType
				if mime == "" {
					mime = "image/jpeg"
				}
				data := ""
				if len(att.Data) > 0 {
					data = base64.StdEncoding.EncodeToString(att.Data)
				} else if att.URL != "" {
					if strings.HasPrefix(att.URL, "data:") {
						parts := strings.SplitN(att.URL, ",", 2)
						if len(parts) == 2 {
							data = parts[1]
						}
					} else {
						data = att.URL
					}
				}
				if data != "" {
					b.stateMu.Lock()
					b.pendingImages = append(b.pendingImages, domain.ImageContent{
						MIMEType: mime,
						Data:     data,
					})
					b.stateMu.Unlock()
				}
			}
		}
	}
	return b.ProcessMessage(ctx, input)
}

// StepWithAttachments processes one user turn with attachments.
func (b *Brain) StepWithAttachments(ctx context.Context, input string, attachments []Attachment) error {
	return b.StepWithSessionAndAttachments(ctx, "", input, attachments, nil)
}

// ProcessMessage handles a user input through the full agent loop.
// Before the standard iteration loop, it checks for @name direct routing,
// then SDD trigger keywords, /undo, /retry, and routes accordingly.
func (b *Brain) ProcessMessage(ctx context.Context, content string) error {
	// 0. Dispatch slash commands & subagent mentions (@name)
	if b.commandDispatcher != nil {
		if handled, err := b.commandDispatcher.Dispatch(ctx, content); handled {
			return err
		}
	}

	// 1. Workflow router interception (ODD / SDD / delegated workers)
	if b.workflowRouter != nil {
		b.workflowRouter.ResetTurn()
		if handled, err := b.workflowRouter.RouteTurn(ctx, content); handled {
			return err
		}
	}

	// 2. Create user message (strip direct/inline prefixes if present)
	cleanedContent := content
	if strings.HasPrefix(cleanedContent, "/direct ") {
		cleanedContent = strings.TrimSpace(strings.TrimPrefix(cleanedContent, "/direct "))
	} else if strings.HasPrefix(cleanedContent, "/inline ") {
		cleanedContent = strings.TrimSpace(strings.TrimPrefix(cleanedContent, "/inline "))
	}

	userMsg := domain.Message{
		Role:    domain.RoleUser,
		Content: cleanedContent,
	}
	// Attach any pending images and clear the buffer safely under lock.
	b.stateMu.Lock()
	if len(b.pendingImages) > 0 {
		userMsg.Images = make([]domain.ImageContent, len(b.pendingImages))
		copy(userMsg.Images, b.pendingImages)
		b.pendingImages = nil
	}
	b.stateMu.Unlock()
	if err := b.repo.SaveMessage(ctx, userMsg); err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	// 2b. Inject relevant knowledge graph facts as context
	if kgFacts := b.recallKnowledgeGraph(ctx, content); len(kgFacts) > 0 {
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

		// 2d. Check for steer messages mid-loop (non-blocking)
		select {
		case steer := <-b.steerCh:
			steerMsg := domain.Message{
				Role:    domain.RoleSystem,
				Content: fmt.Sprintf("MID-EXECUTION GUIDANCE: %s\n\nAdjust your approach accordingly.", steer),
			}
			history = append(history, steerMsg)
			// Also persist so it appears in compaction
			b.repo.SaveMessage(ctx, steerMsg)
		default:
		}

		// 3. Call LLM — prefer streaming, fall back to non-streaming
		var chatOpts []ports.ChatOpt
		b.stateMu.RLock()
		effort := b.reasoningEffort
		b.stateMu.RUnlock()
		if effort != "" {
			chatOpts = append(chatOpts, ports.WithReasoningEffort(effort))
		}

		var response *domain.Message
		stream, err := b.provider.Stream(ctx, history, chatOpts...)
		if err != nil {
			// Fall back to non-streaming Chat for this iteration
			resp, chatErr := b.provider.Chat(ctx, history, chatOpts...)
			if chatErr != nil {
				return fmt.Errorf("llm error: %w", chatErr)
			}
			response = resp
		} else {
			response, err = b.readStream(ctx, stream)
			stream.Close()
			if err != nil {
				// Fall back to non-streaming on read failure
				resp, chatErr := b.provider.Chat(ctx, history, chatOpts...)
				if chatErr != nil {
					return fmt.Errorf("llm error: %w", chatErr)
				}
				response = resp
			}
		}

		if response != nil && b.costTracker != nil {
			b.costTracker.Record(b.modelName, history, response)
		}

		// 4. Handle tool calls
		if len(response.ToolCalls) > 0 {
			if err := b.handleToolCalls(ctx, response); err != nil {
				return err
			}

			// In-Flight Mid-Turn Delegation Check (WorkflowRouter):
			if b.workflowRouter != nil {
				if handled, err := b.workflowRouter.CheckMidTurn(ctx, content); handled {
					return err
				}
			}

			continue // Let LLM see results
		}

		// 5. Save and display final response
		if err := b.repo.SaveMessage(ctx, *response); err != nil {
			return fmt.Errorf("save assistant response: %w", err)
		}
		if err := b.ui.Display(*response); err != nil {
			return err
		}
		// Extract knowledge graph facts from the response
		if b.kgRecorder != nil && b.KGEnabled() {
			b.kgRecorder.RecordResponseFacts(ctx, response.Content)
		}
		// Process queued messages after successful response
		b.processQueuedMessages(ctx)
		// Check if goal is complete and auto-continue if needed
		b.checkGoalAfterTurn(ctx)
		return nil
	}

	// Budget exhausted
	errMsg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: fmt.Sprintf("Iteration budget exhausted (%d iterations). Stopping.", b.budget.MaxIterations),
	}
	b.repo.SaveMessage(ctx, *errMsg)
	if err := b.ui.Display(*errMsg); err != nil {
		return err
	}
	if b.kgRecorder != nil && b.KGEnabled() {
		b.kgRecorder.RecordResponseFacts(ctx, errMsg.Content)
	}
	b.processQueuedMessages(ctx)
	b.checkGoalAfterTurn(ctx)
	return nil
}

// readStream reads token chunks from the stream and builds a final message.
// It actively listens for ctx.Done() and closes reader if it implements io.Closer.
func (b *Brain) readStream(ctx context.Context, reader io.Reader) (*domain.Message, error) {
	if closer, ok := reader.(io.Closer); ok {
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-ctx.Done():
				_ = closer.Close()
			case <-done:
			}
		}()
	}

	response := &domain.Message{Role: domain.RoleAssistant}
	var sb strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var chunk domain.TokenChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("stream error: %s", chunk.Error)
		}
		sb.WriteString(chunk.Content)

		b.stateMu.RLock()
		fn := b.onToken
		b.stateMu.RUnlock()
		if fn != nil {
			fn(chunk.Content)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	response.Content = sb.String()
	return response, scanner.Err()
}

// checkGoalAfterTurn evaluates goal completion and auto-continues if needed.
// Called after ProcessMessage completes (not for goal commands themselves).
func (b *Brain) checkGoalAfterTurn(ctx context.Context) {
	b.stateMu.RLock()
	goal := b.goal
	subgoals := b.subgoals
	turns := b.goalTurns
	b.stateMu.RUnlock()

	if goal == "" {
		return
	}

	history, err := b.getHistory(ctx, 20)
	if err != nil || len(history) == 0 {
		return
	}

	var evalPrompt strings.Builder
	evalPrompt.WriteString("Evaluate whether the following goal is COMPLETE or INCOMPLETE.\n")
	evalPrompt.WriteString("Answer with exactly one word: YES or NO.\n\n")
	evalPrompt.WriteString(fmt.Sprintf("Goal: %s\n", goal))
	if len(subgoals) > 0 {
		evalPrompt.WriteString("Subgoals:\n")
		for _, sg := range subgoals {
			evalPrompt.WriteString(fmt.Sprintf("  - %s\n", sg))
		}
	}
	evalPrompt.WriteString("\nRecent conversation:\n")
	for _, m := range history {
		preview := m.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		evalPrompt.WriteString(fmt.Sprintf("[%s] %s\n", strings.ToUpper(string(m.Role[:1])), preview))
	}
	evalPrompt.WriteString("\nIs the goal complete? Answer YES or NO:")

	evalMsg, err := b.provider.Chat(ctx, []domain.Message{
		{Role: domain.RoleSystem, Content: "You are a goal evaluation judge. Be strict — only YES if the goal is genuinely accomplished."},
		{Role: domain.RoleUser, Content: evalPrompt.String()},
	})
	if err != nil || evalMsg == nil {
		return
	}

	answer := strings.TrimSpace(strings.ToUpper(evalMsg.Content))
	if strings.HasPrefix(answer, "YES") {
		_ = b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Goal complete: %s ✓", goal),
		})
		b.ClearGoal()
		return
	}

	turns++
	b.stateMu.Lock()
	b.goalTurns = turns
	b.stateMu.Unlock()

	const maxGoalTurns = 25
	if turns >= maxGoalTurns {
		_ = b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Goal paused: reached maximum automatic turn limit (%d turns) without completion. Use /goal to resume or inspect.", maxGoalTurns),
		})
		b.ClearGoal()
		return
	}

	contPrompt := fmt.Sprintf("Continue working on the goal: %s (Turn %d/%d)", goal, turns+1, maxGoalTurns)
	if len(subgoals) > 0 {
		contPrompt += "\nRequirements:\n"
		for _, sg := range subgoals {
			contPrompt += fmt.Sprintf("- %s\n", sg)
		}
	}
	_ = b.ProcessMessage(ctx, contPrompt)
}

// processQueuedMessages checks the queue and processes the next item.
func (b *Brain) processQueuedMessages(ctx context.Context) {
	b.stateMu.Lock()
	if b.processingQueue || len(b.messageQueue) == 0 {
		b.stateMu.Unlock()
		return
	}
	b.processingQueue = true
	prompt := b.messageQueue[0]
	b.messageQueue = b.messageQueue[1:]
	remaining := len(b.messageQueue)
	b.stateMu.Unlock()

	defer func() {
		b.stateMu.Lock()
		b.processingQueue = false
		b.stateMu.Unlock()
	}()

	_ = b.repo.SaveMessage(ctx, domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Processing queued message (%d remaining): %s", remaining, prompt),
	})
	_ = b.ProcessMessage(ctx, prompt)
}

// recallKnowledgeGraph searches the knowledge graph for facts relevant to text.
// Returns a slice of formatted fact strings to append to system prompt.
// Uses keyword search; falls back to recent facts if search yields nothing.
func (b *Brain) recallKnowledgeGraph(ctx context.Context, text string) []string {
	if !b.KGEnabled() || b.kgStore == nil {
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


