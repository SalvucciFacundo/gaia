package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/autofix"
	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/mcp"
	"gaia/internal/persona"
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
	compactedTo        int                             // messages compacted so far (context compaction)
	providerName       string                          // e.g. "openai"
	modelName          string                          // e.g. "gpt-4o"
	costTracker        *CostTracker                    // tracks LLM call costs
	currentSessionID   string                          // tracked for /title and /resume
	moaProviders       map[string]ports.LLMProvider    // extra providers for MoA (/moa command)
	messageQueue       []string                        // queued messages for /queue command
	processingQueue    bool                            // prevents re-entrant queue processing
	steerCh            chan string                     // buffered channel (cap 1) for /steer mid-loop injection
	goal               string                          // active persistent goal (/goal)
	subgoals           []string                        // subgoal criteria (/subgoal)
	availableProviders map[string]ports.LLMProvider    // named providers for /model switching
	auditLog           []AuditEntry                    // audit trail for policy-audited tools (/audit)
	sessionMgr         *SessionManager                 // routes messages across platforms
	skillLister        func() []string                 // returns available skill names for progressive loading
	skillPathResolver  func([]string) []string         // resolves skill names to SKILL.md file paths
	skillMatcher       func(string, []string) []string // matches skills by task intent and file context
	mcpMgr             *mcp.Manager                    // MCP server connection manager
	pendingImages      []domain.ImageContent           // images queued for the next ProcessMessage
	fastModeEnabled    bool                            // whether /fast mode is active
	originalModel      string                          // model before /fast was enabled
	fastModel          string                          // override fast model name
	busyMode           string                          // input behavior while agent is working: queue, steer, ignore
	reasoningEffort    string                          // "low", "medium", "high"
	personaName        string                          // active personality name
	personaOverlay     string                          // system prompt instructions derived from persona
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
	b.kgEnabled = (kg != nil)
}

// SetPolicyGuard wires the policy guard into the Brain for tool call evaluation.
// Pass nil to disable policy enforcement (guard-only mode).
func (b *Brain) SetPolicyGuard(pg *PolicyGuard) {
	b.policy = pg
}

// SetMoAProviders configures extra providers for the /moa command.
func (b *Brain) SetMoAProviders(providers map[string]ports.LLMProvider) {
	b.moaProviders = providers
	b.availableProviders = providers
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
	b.reasoningEffort = strings.ToLower(strings.TrimSpace(level))
	fmt.Printf("Reasoning effort set to: %s (provider support may vary)\n", b.reasoningEffort)
}

// ReasoningEffort returns the current reasoning effort level.
func (b *Brain) ReasoningEffort() string {
	return b.reasoningEffort
}

// SetPersona changes the agent personality/behavior overlay.
func (b *Brain) SetPersona(name string) {
	b.personaName = strings.ToLower(strings.TrimSpace(name))
	overlays := persona.NewOverlays(nil)
	text, err := overlays.Resolve(b.personaName)
	if err == nil {
		b.personaOverlay = text
	} else {
		b.personaOverlay = ""
	}
	fmt.Printf("Persona set to: %s (will apply to next interactions)\n", b.personaName)
}

// Persona returns the active persona name and overlay prompt.
func (b *Brain) Persona() (string, string) {
	return b.personaName, b.personaOverlay
}

// queryKGContext searches the knowledge graph for facts relevant to the given text.
// Returns formatted KG facts or nil if the store is not wired.
func (b *Brain) queryKGContext(ctx context.Context, text string) []string {
	return b.recallKnowledgeGraph(ctx, text)
}

// MoaOneShot runs a one-shot Mixture of Agents on the given prompt.
func (b *Brain) MoaOneShot(ctx context.Context, prompt string) error {
	if prompt == "" {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /moa <prompt> — run prompt through multiple models and synthesize."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if len(b.moaProviders) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No additional MoA providers configured. Use subagents config to add models."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	messages := []domain.Message{
		{Role: domain.RoleSystem, Content: "You are a helpful AI assistant."},
		{Role: domain.RoleUser, Content: prompt},
	}

	// Fan out to all MoA providers in parallel
	type modelResult struct {
		Label string
		Msg   *domain.Message
		Err   error
	}

	results := make(chan modelResult, len(b.moaProviders))
	var wg sync.WaitGroup

	for name, prov := range b.moaProviders {
		wg.Add(1)
		go func(label string, p ports.LLMProvider) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			msg, err := p.Chat(callCtx, messages)
			results <- modelResult{Label: label, Msg: msg, Err: err}
		}(name, prov)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect responses
	var responses []modelResult
	for r := range results {
		if r.Err == nil && r.Msg != nil {
			responses = append(responses, r)
		}
	}

	if len(responses) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "All MoA models failed. Try with a simpler prompt."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if len(responses) == 1 {
		// Single response — no synthesis needed
		responses[0].Msg.Role = domain.RoleAssistant
		b.repo.SaveMessage(ctx, *responses[0].Msg)
		return b.ui.Display(*responses[0].Msg)
	}

	// Synthesize: build a merge prompt and ask the primary model
	var sb strings.Builder
	sb.WriteString("The following are responses from different AI models for the same task.\n")
	sb.WriteString("Synthesize them into a single, coherent response that captures the best\n")
	sb.WriteString("insights from each. Resolve any contradictions and avoid repetition.\n\n")

	for i, r := range responses {
		sb.WriteString(fmt.Sprintf("=== Response %d (%s) ===\n%s\n\n", i+1, r.Label, r.Msg.Content))
	}
	sb.WriteString("Synthesized response:")

	synthResp, err := b.provider.Chat(ctx, []domain.Message{
		{Role: domain.RoleSystem, Content: "You are a synthesis model. Merge multiple AI responses into one coherent answer."},
		{Role: domain.RoleUser, Content: sb.String()},
	})
	if err != nil {
		// Synthesis failed — return first response as fallback
		responses[0].Msg.Role = domain.RoleAssistant
		b.repo.SaveMessage(ctx, *responses[0].Msg)
		return b.ui.Display(*responses[0].Msg)
	}

	synthResp.Role = domain.RoleAssistant
	b.repo.SaveMessage(ctx, *synthResp)
	return b.ui.Display(*synthResp)
}

// ListModels displays available providers/models.
// LearnSkill creates a skill from a directory, URL, or description.

// Insights shows usage analytics for the current session.

// AttachImage prepares an image for vision processing.
// It validates the file (exists, size ≤ 20MB, supported MIME),
// base64-encodes it, and stores it in pendingImages for the next
// ProcessMessage call.

// getHistory returns conversation history, filtering out compacted messages.
// If compaction has occurred (compactedTo > 0), returns only the recent messages
// (oldest compacted messages are excluded but their compaction summary exists).
func (b *Brain) getHistory(ctx context.Context, limit int) ([]domain.Message, error) {
	var msgs []domain.Message
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
			msgs = history
		} else {
			foundSummary := false
			for i := len(recent) - 1; i >= 0; i-- {
				if recent[i].Role == domain.RoleSystem && strings.Contains(recent[i].Content, "Compacted conversation") {
					msgs = append([]domain.Message{recent[i]}, history...)
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

	if b.personaOverlay != "" {
		personaMsg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("PERSONALITY OVERLAY [%s]:\n%s", strings.ToUpper(b.personaName), b.personaOverlay),
		}
		msgs = append([]domain.Message{personaMsg}, msgs...)
	}

	return msgs, nil
}

// Delegate dispatches a task to a named subagent and returns the structured result.
// After a successful delegation, it automatically saves subagent discoveries
// to the shared knowledge graph for cross-pollination.
// Returns nil, error if no subagent port is wired or the subagent is unknown.
// populateSkillRegistry fills the task's SkillRegistry with available skill names,
// matches relevant skills by task intent and file context, and resolves their paths
// for lazy loading via file_read to avoid context saturation.
func (b *Brain) populateSkillRegistry(task *domain.SubagentTask) {
	if b.skillLister != nil {
		task.SkillRegistry = b.skillLister()
	}
	// If specific skills were not already passed, match skills from task description
	if len(task.Skills) == 0 && b.skillMatcher != nil && task.Description != "" {
		matched := b.skillMatcher(task.Description, nil)
		if len(matched) > 0 {
			task.Skills = matched
		}
	}
	// Resolve file paths for matched skills to enable progressive path-based loading
	if len(task.Skills) > 0 && b.skillPathResolver != nil && len(task.SkillPaths) == 0 {
		task.SkillPaths = b.skillPathResolver(task.Skills)
	}
}

func (b *Brain) Delegate(ctx context.Context, name string, task domain.SubagentTask) (*domain.SubagentResult, error) {
	if b.subagentPort == nil {
		return nil, fmt.Errorf("subagent port not wired")
	}
	b.populateSkillRegistry(&task)
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
	projectName := DetectProjectName(projectRoot)
	_ = projectName
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

// saveDirectActionFact records direct inline tool mutations (file writes, git commits, configuration changes)
// into the knowledge graph so direct parent decisions and actions are not lost during compactions or session restarts.
func (b *Brain) saveDirectActionFact(ctx context.Context, toolName string, args map[string]interface{}, result *domain.ToolResult) {
	if b.kgStore == nil || result == nil || !result.Success {
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

		_, _ = b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
		_, _ = b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
			_, _ = b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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
	b.currentSessionID = id
	if sr, ok := b.repo.(interface{ SetSessionID(string) }); ok {
		sr.SetSessionID(id)
	}
}

// StepWithSessionAndAttachments processes one user turn bound to a specific session and token sink with attachments.
func (b *Brain) StepWithSessionAndAttachments(ctx context.Context, sessionID string, input string, attachments []Attachment, sink func(string)) error {
	if sessionID != "" {
		b.SetActiveConversation(sessionID)
	}
	if sink != nil {
		oldSink := b.onToken
		b.onToken = sink
		defer func() { b.onToken = oldSink }()
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
					b.pendingImages = append(b.pendingImages, domain.ImageContent{
						MIMEType: mime,
						Data:     data,
					})
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

	// 0d. /new — start a fresh conversation
	if content == "/new" || content == "/reset" {
		return b.NewSession(ctx)
	}

	// 0e. /save <name> — explicitly save the current session
	if content == "/save" {
		return b.SaveSession(ctx, "")
	}
	if strings.HasPrefix(content, "/save ") {
		return b.SaveSession(ctx, strings.TrimSpace(content[6:]))
	}

	// 0f. /compress — force manual context compaction
	if content == "/compress" {
		return b.ManualCompress(ctx)
	}

	// 0g. /history — show full conversation history
	if content == "/history" {
		return b.ShowHistory(ctx)
	}

	// 0h. /sessions — list saved sessions
	if content == "/sessions" {
		return b.ListSessionsCmd(ctx)
	}

	// 0i. /title <name> — rename current session
	if strings.HasPrefix(content, "/title ") {
		return b.TitleSession(ctx, strings.TrimSpace(content[7:]))
	}

	// 0j. /resume <id> — load a saved session's messages
	if strings.HasPrefix(content, "/resume ") {
		return b.ResumeSession(ctx, strings.TrimSpace(content[8:]))
	}

	// 0k. /moa <prompt> — one-shot Mixture of Agents
	if strings.HasPrefix(content, "/moa ") {
		return b.MoaOneShot(ctx, strings.TrimSpace(content[5:]))
	}

	// 0l. /background <prompt> — run in background
	if strings.HasPrefix(content, "/background ") {
		return b.BackgroundTask(ctx, strings.TrimSpace(content[12:]))
	}

	// 0m. /snapshot — save/load conversation snapshots
	if content == "/snapshot" || content == "/snap" {
		return b.SnapshotHelp(ctx)
	}
	if strings.HasPrefix(content, "/snapshot ") || strings.HasPrefix(content, "/snap ") {
		rest := content
		if strings.HasPrefix(rest, "/snap ") {
			rest = "/snapshot " + rest[6:]
		}
		parts := strings.SplitN(strings.TrimSpace(rest[10:]), " ", 2)
		if len(parts) == 2 && parts[0] == "save" {
			return b.SnapshotSave(ctx, parts[1])
		}
		if len(parts) == 2 && parts[0] == "load" {
			return b.SnapshotLoad(ctx, parts[1])
		}
		return b.SnapshotHelp(ctx)
	}

	// 0n. /branch — fork the conversation at this point
	if content == "/branch" {
		return b.BranchFork(ctx, fmt.Sprintf("branch-%s", time.Now().Format("2006-01-02-15-04-05")))
	}
	if strings.HasPrefix(content, "/branch ") {
		return b.BranchFork(ctx, strings.TrimSpace(content[8:]))
	}
	if content == "/branches" {
		return b.BranchList(ctx)
	}

	// 0o. /queue — queue a message for later processing
	if content == "/queue" || content == "/q" {
		return b.QueueShow(ctx)
	}
	if strings.HasPrefix(content, "/queue ") {
		return b.QueuePush(ctx, strings.TrimSpace(content[7:]))
	}
	if strings.HasPrefix(content, "/q ") {
		return b.QueuePush(ctx, strings.TrimSpace(content[3:]))
	}
	if content == "/queue clear" {
		return b.QueueClear(ctx)
	}

	// 0p. /handoff <platform> — save session and show resume instructions
	if content == "/handoff" {
		return b.HandoffHelp(ctx)
	}
	if strings.HasPrefix(content, "/handoff ") {
		return b.HandoffTo(ctx, strings.TrimSpace(content[9:]))
	}

	// 0q. /steer <msg> — inject mid-loop guidance
	if content == "/steer" || content == "/steer " {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /steer <message> — inject guidance mid-execution. It will be processed after the next tool call."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	if strings.HasPrefix(content, "/steer ") {
		guidance := strings.TrimSpace(content[7:])
		if guidance == "" {
			msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /steer <message>"}
			b.repo.SaveMessage(ctx, msg)
			return b.ui.Display(msg)
		}
		return b.Steer(ctx, guidance)
	}

	// 0r. /goal — persistent goal management
	if content == "/goal" {
		return b.GoalShow(ctx)
	}
	if strings.HasPrefix(content, "/goal ") {
		return b.GoalSet(ctx, strings.TrimSpace(content[6:]))
	}
	if content == "/goals" {
		return b.GoalShow(ctx)
	}
	if content == "/goal clear" {
		return b.GoalClear(ctx)
	}
	if strings.HasPrefix(content, "/subgoal ") {
		return b.SubgoalAdd(ctx, strings.TrimSpace(content[9:]))
	}

	// 0s. /model — switch LLM model/provider mid-session
	if content == "/model" || content == "/models" {
		return b.ListModels(ctx)
	}
	if strings.HasPrefix(content, "/model ") {
		return b.SwitchModel(ctx, strings.TrimSpace(content[7:]))
	}
	if strings.HasPrefix(content, "/models ") {
		return b.SwitchModel(ctx, strings.TrimSpace(content[8:]))
	}

	// 0t. /memory approve|reject — review pending memory writes
	if content == "/memory" || content == "/memory pending" {
		return b.MemoryPending(ctx)
	}
	if strings.HasPrefix(content, "/memory approve ") {
		return b.MemoryApprove(ctx, strings.TrimSpace(content[15:]))
	}
	if strings.HasPrefix(content, "/memory reject ") {
		return b.MemoryReject(ctx, strings.TrimSpace(content[14:]))
	}

	// 0u. /learn — create a skill from directory, URL, or description
	if strings.HasPrefix(content, "/learn ") {
		return b.LearnSkill(ctx, strings.TrimSpace(content[7:]))
	}

	// 0v. /suggestions — review agent-suggested automations
	if content == "/suggestions" {
		return b.SkillSuggestions(ctx)
	}

	// 0w. /blueprint — create automation from template
	if strings.HasPrefix(content, "/blueprint ") {
		return b.BlueprintCreate(ctx, strings.TrimSpace(content[11:]))
	}

	// 0x. /curator — background skill maintenance
	if content == "/curator" {
		return b.SkillCurator(ctx)
	}

	// 0y. /insights — usage analytics
	if content == "/insights" {
		return b.Insights(ctx)
	}

	// 0z. /debug — collect debug info
	if content == "/debug" {
		return b.DebugInfo(ctx)
	}

	// 0aa. /credits — show credit/usage balance
	if content == "/credits" {
		return b.CreditsInfo(ctx)
	}

	// 0ab. /billing — billing management
	if content == "/billing" {
		return b.BillingInfo(ctx)
	}

	// 0ac. /image <path> — attach image for vision
	if strings.HasPrefix(content, "/image ") {
		return b.AttachImage(ctx, strings.TrimSpace(content[7:]))
	}

	// 0ad. /paste — attach clipboard image
	if content == "/paste" {
		return b.PasteImage(ctx)
	}

	// 0ae. /fast — toggle fast model override
	if content == "/fast" || strings.HasPrefix(content, "/fast ") {
		arg := ""
		if strings.HasPrefix(content, "/fast ") {
			arg = strings.TrimSpace(content[6:])
		}
		return b.FastMode(ctx, arg)
	}

	// 0af. /diff — inspect git diff
	if content == "/diff" {
		return b.DiffView(ctx, false)
	}
	if strings.HasPrefix(content, "/diff ") {
		staged := strings.Contains(content, "--staged") || strings.Contains(content, "--cached")
		return b.DiffView(ctx, staged)
	}

	// 0af. /busy — control Enter key behavior
	if content == "/busy" || strings.HasPrefix(content, "/busy ") {
		arg := ""
		if strings.HasPrefix(content, "/busy ") {
			arg = strings.TrimSpace(content[6:])
		}
		return b.BusyMode(ctx, arg)
	}

	// 0ag. /codegraph — semantic code graph index and query
	if content == "/codegraph" || strings.HasPrefix(content, "/codegraph ") {
		arg := ""
		if strings.HasPrefix(content, "/codegraph ") {
			arg = strings.TrimSpace(content[11:])
		}
		return b.CodeGraphCommand(ctx, arg)
	}

	// Messaging-only commands (work in both TUI and gateway)
	// Also handle /help here for gateway mode
	if content == "/help" || content == "/h" || content == "/commands" {
		return b.HelpCommands(ctx)
	}

	// 0ae. /sethome — set current chat as delivery home
	if content == "/sethome" {
		return b.SetHome(ctx)
	}

	// 0af. /approve — approve pending dangerous command
	if content == "/approve" {
		return b.ApprovePending(ctx)
	}

	// 0ag. /deny — deny pending dangerous command
	if content == "/deny" {
		return b.DenyPending(ctx)
	}

	// 0ah. /restart — graceful gateway restart
	if content == "/restart" {
		return b.RestartGateway(ctx)
	}

	// 0aj. /update — update GAIA to latest version
	if content == "/update" {
		return b.UpdateGAIA(ctx)
	}

	// 0ak. /topic — multi-session DM mode
	if strings.HasPrefix(content, "/topic ") {
		return b.TopicSession(ctx, strings.TrimSpace(content[7:]))
	}
	if content == "/topic" {
		return b.TopicHelp(ctx)
	}

	// 0al. /audit - view policy audit log
	if content == "/audit" {
		return b.AuditLog(ctx)
	}
	if content == "/audit clear" {
		return b.AuditClear(ctx)
	}

	// 0am. /session — session mode management
	if content == "/session" {
		return b.SessionStatus(ctx)
	}
	if strings.HasPrefix(content, "/session ") {
		return b.SessionCommand(ctx, strings.TrimSpace(content[9:]))
	}

	// 0an. /kg — knowledge graph recall toggle
	if content == "/kg" {
		return b.KGStatus(ctx)
	}
	if content == "/kg on" {
		b.kgEnabled = true
		msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph recall: ON\nWill inject relevant facts as extra context.\nUse /kg off to disable."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	if content == "/kg off" {
		b.kgEnabled = false
		msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph recall: OFF\nUsing standard context only."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	if content == "/kg stats" {
		return b.KGStats(ctx)
	}
	if content == "/kg clear" {
		return b.KGClear(ctx)
	}

	// 1. ODD / SDD trigger detection
	odd := DetectODDRoute(content)
	trigger := DetectSDDTrigger(content)

	if odd.Route == RouteFeatureTracking {
		return b.handleSDDTrigger(ctx, content, trigger)
	}

	if odd.SuggestFeature && !trigger.ForceDirect {
		suggestion := fmt.Sprintf(
			"💡 **Architectural Scope Detected** (%s).\n"+
				"This change involves multi-module scope or architectural decisions where durable specs and tasks protect quality.\n\n"+
				"Would you like to run the formal feature tracking workflow? Use **`/odd %s`** (or `/sdd`) or continue inline with **`/inline %s`** (or `/direct`).",
			odd.Reason, content, content)
		msg := domain.Message{Role: domain.RoleAssistant, Content: suggestion}
		_ = b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if odd.Route == RouteDelegatedWorker && b.subagentPort != nil && !trigger.ForceDirect {
		targetAgent := odd.WorkerRole
		if targetAgent == "writer" {
			targetAgent = "implementer"
		}
		isAvailable := false
		for _, a := range b.subagentPort.Available() {
			if a == targetAgent {
				isAvailable = true
				break
			}
		}
		if isAvailable {
			task := domain.SubagentTask{
				ID:               "odd-worker",
				Description:      content,
				Mode:             "execute",
				KGContext:        b.queryKGContext(ctx, content),
				AllowedEditRoots: []string{"."},
			}
			result, err := b.Delegate(ctx, targetAgent, task)
			if err == nil && result != nil {
				workerMsg := domain.Message{
					Role:    domain.RoleAssistant,
					Content: fmt.Sprintf("🤖 **ODD Delegated Worker (%s)** completed:\n\n%s", targetAgent, result.Summary),
				}
				_ = b.repo.SaveMessage(ctx, workerMsg)
				return b.ui.Display(workerMsg)
			}
		}
	}

	// 2. Create user message
	userMsg := domain.Message{
		Role:    domain.RoleUser,
		Content: content,
	}
	// Attach any pending images and clear the buffer.
	if len(b.pendingImages) > 0 {
		userMsg.Images = make([]domain.ImageContent, len(b.pendingImages))
		copy(userMsg.Images, b.pendingImages)
		b.pendingImages = nil
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

	// 3. Iteration loop with budget & in-flight ODD delegation monitor
	midTurnTracker := NewMidTurnBudgetTracker()
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
		if b.reasoningEffort != "" {
			chatOpts = append(chatOpts, ports.WithReasoningEffort(b.reasoningEffort))
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

		// 4. Handle tool calls
		if len(response.ToolCalls) > 0 {
			if err := b.handleToolCalls(ctx, response, midTurnTracker); err != nil {
				return err
			}

			// In-Flight Mid-Turn Delegation Check (ODD Contract):
			// If inline evidence budget or write boundaries are crossed mid-turn,
			// dynamically delegate the continuation to a specialized subagent worker.
			if handled, err := b.delegateMidTurnODD(ctx, content, midTurnTracker, trigger); handled {
				return err
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
		b.extractKGFacts(ctx, response.Content)
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
	b.extractKGFacts(ctx, errMsg.Content)
	b.processQueuedMessages(ctx)
	b.checkGoalAfterTurn(ctx)
	return nil
}

// delegateMidTurnODD checks and performs mid-turn dynamic delegation under the ODD contract.
// If delegation is triggered and succeeds, it displays the message, executes post-turn hooks
// (extractKGFacts, processQueuedMessages, checkGoalAfterTurn), and returns (true, nil).
// If delegation was not triggered or was skipped/failed, it returns (false, nil).
func (b *Brain) delegateMidTurnODD(ctx context.Context, content string, midTurnTracker *MidTurnBudgetTracker, trigger TriggerResult) (bool, error) {
	if midTurnTracker == nil || b.subagentPort == nil || trigger.ForceDirect {
		return false, nil
	}

	targetRole, reason, shouldDelegate := midTurnTracker.CheckDelegationTrigger()
	if !shouldDelegate {
		return false, nil
	}

	agentName := targetRole
	if agentName == "writer" {
		agentName = "implementer"
	}
	isAvailable := false
	for _, a := range b.subagentPort.Available() {
		if a == agentName {
			isAvailable = true
			break
		}
	}
	if !isAvailable {
		midTurnTracker.Reset()
		return false, nil
	}

	intermediateContext := ""
	if len(midTurnTracker.ToolHistory) > 0 {
		intermediateContext = fmt.Sprintf("\n\n### Intermediate Execution Progress:\nTools executed so far in parent turn:\n- %s\n",
			strings.Join(midTurnTracker.ToolHistory, "\n- "))
	}
	if len(midTurnTracker.FilesWritten) > 0 {
		files := make([]string, 0, len(midTurnTracker.FilesWritten))
		for f := range midTurnTracker.FilesWritten {
			files = append(files, f)
		}
		intermediateContext += fmt.Sprintf("Files touched: %s\n", strings.Join(files, ", "))
	}

	delegateTask := domain.SubagentTask{
		ID:               fmt.Sprintf("odd-midturn-%s", targetRole),
		Description:      fmt.Sprintf("Continue execution (%s). Original request: %s%s", reason, content, intermediateContext),
		Mode:             "execute",
		KGContext:        b.queryKGContext(ctx, content),
		AllowedEditRoots: []string{"."},
	}
	delRes, delErr := b.Delegate(ctx, agentName, delegateTask)
	if delErr == nil && delRes != nil {
		delMsg := domain.Message{
			Role:    domain.RoleAssistant,
			Content: fmt.Sprintf("🤖 **ODD Mid-Turn Dynamic Delegation (%s)** [%s]:\n\n%s", agentName, reason, delRes.Summary),
		}
		_ = b.repo.SaveMessage(ctx, delMsg)
		if err := b.ui.Display(delMsg); err != nil {
			return true, err
		}
		b.extractKGFacts(ctx, delMsg.Content)
		b.processQueuedMessages(ctx)
		b.checkGoalAfterTurn(ctx)
		return true, nil
	}
	// If delegation failed or was unavailable, reset tracker counters to avoid busy loop
	midTurnTracker.Reset()
	return false, nil
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

func (b *Brain) handleToolCalls(ctx context.Context, msg *domain.Message, tracker ...*MidTurnBudgetTracker) error {
	var tr *MidTurnBudgetTracker
	if len(tracker) > 0 {
		tr = tracker[0]
	}

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
				// Execute parallel batch concurrently
				var wg sync.WaitGroup
				for j := start; j < end; j++ {
					j := j
					wg.Add(1)
					go func() {
						defer wg.Done()
						res, err := b.registry.Execute(ctx, items[j].tc.Name, items[j].tc.Arguments)
						if err != nil {
							res = &domain.ToolResult{
								Success: false,
								Error:   err.Error(),
							}
						}
						items[j].result = res
					}()
				}
				wg.Wait()
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
					default:
						if result.SuggestedAction == "ask_user" {
							confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf(
								"Policy blocks %s (reason: %s). Allow anyway?",
								tc.Name, result.Reason))
							if err != nil || !confirmed {
								items[i].denied = true
								items[i].errMsg = fmt.Sprintf("User denied policy-blocked tool %s.", tc.Name)
								i++
								continue
							}
							b.policy.Session().ApproveSession(tc.Name)
						} else {
							items[i].denied = true
							items[i].errMsg = fmt.Sprintf("Policy blocked: %s (reason: %s)", tc.Name, result.Reason)
							i++
							continue
						}
					}
				}
			}

			// Confirmation gate (legacy ConfirmGuard — kept for backward compat)
			if b.guard != nil && b.guard.ShouldConfirm(tc.Name) {
				confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf("Allow tool %s with args %v?", tc.Name, tc.Arguments))
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
		b.saveDirectActionFact(ctx, item.tc.Name, item.tc.Arguments, result)

		output := result.Output
		if !result.Success {
			output = fmt.Sprintf("Error: %s", result.Error)
		}

		// Apply message redaction to tool output
		output, _ = b.RedactToolOutput(output)

		// Record tool execution into MidTurnBudgetTracker if wired
		if tr != nil {
			tr.RecordToolCall(item.tc.Name, item.tc.Arguments, output)
		}

		toolMsg := domain.Message{
			Role:    domain.RoleTool,
			Content: output,
		}
		b.repo.SaveMessage(ctx, toolMsg)
	}

	// Save the assistant message that triggered these tool calls
	b.repo.SaveMessage(ctx, *msg)

	// Pillar 2 Auto-Repair Feedback Loop:
	// If any mutation tool was executed successfully, run compiler/lint check
	// and inject compilation errors into the turn if broken.
	var lastModifiedPath string
	for _, item := range items {
		if !item.skipped && !item.denied && item.result != nil && item.result.Success {
			if autofix.IsMutationTool(item.tc.Name) {
				if p := autofix.ExtractModifiedPaths(item.tc.Arguments); p != "" {
					lastModifiedPath = p
				}
			}
		}
	}

	if lastModifiedPath != "" {
		checker := autofix.NewChecker()
		res, err := checker.Check(ctx, lastModifiedPath)
		if err == nil && res != nil && !res.Clean {
			repairPrompt := autofix.FormatAutoRepairPrompt(res)
			if repairPrompt != "" {
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
		b.populateSkillRegistry(&task)
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
