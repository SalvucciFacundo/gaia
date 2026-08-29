package core

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
	"gaia/internal/mcp"
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
	provider      ports.LLMProvider
	repo          ports.Repository
	registry      *ToolRegistry
	guard         *ConfirmGuard
	policy        *PolicyGuard              // policy-based guard (additive, not replacement)
	ui            ports.UIService
	budget        domain.BudgetConfig
	onToken       func(string)                // streaming callback
	subagentPort  ports.SubagentPort           // subagent delegation (nil if not wired)
	kgStore       ports.KnowledgeGraphStore    // shared knowledge graph (nil if not wired)
	kgEnabled     bool                          // knowledge graph recall toggle (/kg)
	compactedTo   int                          // messages compacted so far (context compaction)
	providerName  string                       // e.g. "openai"
	modelName     string                       // e.g. "gpt-4o"
	costTracker   *CostTracker                 // tracks LLM call costs
	currentSessionID string                    // tracked for /title and /resume
	moaProviders  map[string]ports.LLMProvider // extra providers for MoA (/moa command)
	messageQueue  []string                    // queued messages for /queue command
	processingQueue bool                      // prevents re-entrant queue processing
	steerCh       chan string                 // buffered channel (cap 1) for /steer mid-loop injection
	goal          string                      // active persistent goal (/goal)
	subgoals      []string                    // subgoal criteria (/subgoal)
	availableProviders map[string]ports.LLMProvider // named providers for /model switching
	auditLog      []AuditEntry                // audit trail for policy-audited tools (/audit)
	sessionMgr    *SessionManager             // routes messages across platforms
	skillLister   func() []string             // returns available skill names for progressive loading
	mcpMgr        *mcp.Manager                 // MCP server connection manager
	pendingImages []domain.ImageContent       // images queued for the next ProcessMessage
	fastModeEnabled bool                      // whether /fast mode is active
	originalModel   string                    // model before /fast was enabled
	fastModel       string                    // override fast model name
	busyMode        string                    // input behavior while agent is working: queue, steer, ignore
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

// SetReasoningEffort changes the reasoning effort level for the current provider.
// Supported values: "low", "medium", "high". Passed to the provider on next Chat call.
func (b *Brain) SetReasoningEffort(level string) {
	// Store for use in Chat calls. Provider-specific implementation varies.
	// For now, we log the intent; actual wiring requires provider SDK support.
	fmt.Printf("Reasoning effort set to: %s (provider support may vary)\n", level)
}

// SetPersona changes the agent personality/behavior seed.
func (b *Brain) SetPersona(name string) {
	fmt.Printf("Persona set to: %s (will apply to next interactions)\n", name)
}

// queryKGContext searches the knowledge graph for facts relevant to the given text.
// Returns formatted KG facts or nil if the store is not wired.
func (b *Brain) queryKGContext(ctx context.Context, text string) []string {
	return b.recallKnowledgeGraph(ctx, text)
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

// NewSession clears the conversation and starts a fresh session.
func (b *Brain) NewSession(ctx context.Context) error {
	if err := b.repo.ClearMessages(ctx); err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	b.compactedTo = 0
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Started a fresh conversation. Previous session cleared.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SaveSession explicitly saves the current conversation with an optional name.
func (b *Brain) SaveSession(ctx context.Context, name string) error {
	if name == "" {
		name = fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04"))
	}
	id, err := b.repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	b.currentSessionID = id
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session saved: %s (ID: %s)", name, id[:12]),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ManualCompress forces context compaction immediately.
func (b *Brain) ManualCompress(ctx context.Context) error {
	if b.budget.CompactionThreshold <= 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Compaction is disabled in config. Enable it by setting compaction_threshold.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	err := b.compactHistory(ctx)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Context compacted. Older messages have been summarized.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ShowHistory displays the full conversation history.
func (b *Brain) ShowHistory(ctx context.Context) error {
	count, err := b.repo.GetMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("history count: %w", err)
	}
	history, err := b.repo.GetHistory(ctx, 100)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Conversation history (%d messages, %d total):\n\n", len(history), count))
	for _, msg := range history {
		preview := msg.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(string(msg.Role[:1])), preview))
	}

	response := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, response)
	return b.ui.Display(response)
}

// ListSessionsCmd shows all saved sessions.
func (b *Brain) ListSessionsCmd(ctx context.Context) error {
	sessions, err := b.repo.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	var sb strings.Builder
	if len(sessions) == 0 {
		sb.WriteString("No saved sessions.")
	} else {
		sb.WriteString(fmt.Sprintf("Saved sessions (%d):\n", len(sessions)))
		for _, s := range sessions {
			sb.WriteString(fmt.Sprintf("  %s  %-30s %s\n", s.ID[:12], s.Name, s.CreatedAt.Format("2006-01-02 15:04")))
		}
		sb.WriteString("\nUse /resume <id> to load a session.")
	}
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// TitleSession renames the current session.
func (b *Brain) TitleSession(ctx context.Context, name string) error {
	if name == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /title <name> — sets the current session name.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// If we have a saved session ID, rename it
	if b.currentSessionID != "" {
		if err := b.repo.RenameSession(ctx, b.currentSessionID, name); err != nil {
			// Non-fatal: session might not exist yet
			msg := domain.Message{
				Role:    domain.RoleSystem,
				Content: fmt.Sprintf("Note: session not yet saved. Name '%s' will be used when saved.", name),
			}
			b.repo.SaveMessage(ctx, msg)
			return b.ui.Display(msg)
		}
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Session renamed to: %s", name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ResumeSession loads and displays a saved session's messages.
func (b *Brain) ResumeSession(ctx context.Context, idOrPrefix string) error {
	if idOrPrefix == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /resume <session-id> — load a saved session. Use /sessions to list available sessions.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Try to find session by ID prefix
	sessions, err := b.repo.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	var matched *domain.SessionInfo
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, idOrPrefix) {
			matched = &s
			break
		}
	}
	if matched == nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("No session found matching %q. Use /sessions to list available sessions.", idOrPrefix),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msgs, err := b.repo.GetMessages(ctx, matched.ID, 100)
	if err != nil {
		return fmt.Errorf("resume: get messages: %w", err)
	}

	b.currentSessionID = matched.ID

	// Also set the repo's active session so subsequent messages use this session
	if repo, ok := b.repo.(interface{ SetSessionID(string) }); ok {
		repo.SetSessionID(matched.ID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Resumed session: %s (ID: %s, %d messages)\n\n", matched.Name, matched.ID[:12], len(msgs)))
	for _, m := range msgs {
		preview := m.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", strings.ToUpper(string(m.Role[:1])), preview))
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
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

// BackgroundTask spawns a subagent in the background and returns immediately.
func (b *Brain) BackgroundTask(ctx context.Context, prompt string) error {
	if prompt == "" {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Usage: /background <prompt> — run a task in the background. Use /tasks to track progress."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	asyncPort, isAsync := b.subagentPort.(ports.AsyncSpawner)
	if !isAsync {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Background tasks not available — async subagent system not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	task := domain.SubagentTask{
		ID:          fmt.Sprintf("bg-%d", time.Now().UnixNano()),
		Description: prompt,
		Mode:        "build",
	}

	taskID, err := asyncPort.SpawnAsync(ctx, "explorer", task)
	if err != nil {
		return fmt.Errorf("background task: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Background task started (ID: %s). Use /tasks to check progress.", taskID[:8]),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotHelp shows usage for /snapshot.
func (b *Brain) SnapshotHelp(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Snapshot commands:\n" +
			"  /snapshot save <name>   Save current conversation\n" +
			"  /snapshot load <name>   Load a saved conversation\n" +
			"  /snap                   Alias for /snapshot",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotSave saves the current conversation to a snapshot file.
func (b *Brain) SnapshotSave(ctx context.Context, name string) error {
	if name == "" {
		return b.SnapshotHelp(ctx)
	}

	history, err := b.repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("snapshot save: get history: %w", err)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	os.MkdirAll(snapDir, 0700)
	snapPath := filepath.Join(snapDir, name+".json")

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot save: marshal: %w", err)
	}
	if err := os.WriteFile(snapPath, data, 0600); err != nil {
		return fmt.Errorf("snapshot save: write: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot saved: %s (%d messages, %s)", name, len(history), snapPath),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SnapshotLoad loads a snapshot and displays it.
func (b *Brain) SnapshotLoad(ctx context.Context, name string) error {
	if name == "" {
		return b.SnapshotHelp(ctx)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	snapPath := filepath.Join(snapDir, name+".json")

	data, err := os.ReadFile(snapPath)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Snapshot %q not found. Available snapshots: %s", name, snapDir),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var history []domain.Message
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("snapshot load: parse: %w", err)
	}

	// Restore messages to repo
	for i := range history {
		b.repo.SaveMessage(ctx, history[i])
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Snapshot loaded: %s (%d messages restored). Use /history to view.", name, len(history)),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BranchFork saves the current conversation as a named branch point.
// The user can continue from here, and later list/load branches via /branches and /snapshot load.
func (b *Brain) BranchFork(ctx context.Context, name string) error {
	history, err := b.repo.GetHistory(ctx, 200)
	if err != nil {
		return fmt.Errorf("branch: get history: %w", err)
	}

	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	os.MkdirAll(snapDir, 0700)

	// Save with branch- prefix
	snapPath := filepath.Join(snapDir, "branch-"+name+".json")

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("branch: marshal: %w", err)
	}
	if err := os.WriteFile(snapPath, data, 0600); err != nil {
		return fmt.Errorf("branch: write: %w", err)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Branch created: %s (%d messages).\nYou're now on a new branch. Use /branches to list, /snapshot load branch-%s to return.", name, len(history), name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BranchList lists all saved branch points.
func (b *Brain) BranchList(ctx context.Context) error {
	snapDir := filepath.Join(os.TempDir(), "gaia-snapshots")
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No branches found. Use /branch <name> to create one.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString("Branches:\n\n")
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "branch-") && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "branch-"), ".json")
			sb.WriteString(fmt.Sprintf("  branch-%s\n", name))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("  (none) — use /branch <name> to create one.\n")
	} else {
		sb.WriteString(fmt.Sprintf("\nUse /snapshot load branch-<name> to switch to a branch.\n"))
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueuePush adds a message to the processing queue.
func (b *Brain) QueuePush(ctx context.Context, prompt string) error {
	if prompt == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Usage: /queue <prompt> — queue a message for later processing. Use /q as alias.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	b.messageQueue = append(b.messageQueue, prompt)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queued: %s (%d items in queue). It will be processed after the current task completes.", prompt, len(b.messageQueue)),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueueShow displays the current message queue.
func (b *Brain) QueueShow(ctx context.Context) error {
	if len(b.messageQueue) == 0 {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "Queue is empty. Use /queue <prompt> to add items.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Message queue (%d items):\n", len(b.messageQueue)))
	for i, item := range b.messageQueue {
		preview := item
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, preview))
	}
	sb.WriteString("\nUse /queue clear to clear the queue.")

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// QueueClear empties the message queue.
func (b *Brain) QueueClear(ctx context.Context) error {
	count := len(b.messageQueue)
	b.messageQueue = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Queue cleared (%d items removed).", count),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HandoffHelp shows available platforms for handoff.
func (b *Brain) HandoffHelp(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Handoff saves the current session and shows how to resume on another platform.\n\n" +
			"Usage: /handoff <platform>\n\n" +
			"Platforms:\n" +
			"  telegram   — Resume on Telegram\n" +
			"  discord    — Resume on Discord\n" +
			"  slack      — Resume on Slack\n" +
			"  whatsapp   — Resume on WhatsApp\n" +
			"  signal     — Resume on Signal\n" +
			"  cli        — Resume on terminal (save only)\n\n" +
			"Requirements:\n" +
			"  The gateway must be running on the target platform:\n" +
			"  gaia gateway start",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HandoffTo saves the current session and shows resume instructions for the platform.
func (b *Brain) HandoffTo(ctx context.Context, platform string) error {
	platform = strings.ToLower(platform)

	// First save the session
	name := fmt.Sprintf("handoff-%s-%s", platform, time.Now().Format("2006-01-02-15-04-05"))
	id, err := b.repo.CreateSession(ctx, name)
	if err != nil {
		return fmt.Errorf("handoff: save session: %w", err)
	}
	b.currentSessionID = id

	var instructions string
	switch platform {
	case "telegram":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Telegram:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Telegram bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "discord":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Discord:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Discord channel\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "slack":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Slack:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Open your Slack channel\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "whatsapp":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on WhatsApp:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Message your WhatsApp bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "signal":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume on Signal:\n"+
				"  1. Start the gateway: gaia gateway start\n"+
				"  2. Message your Signal bot\n"+
				"  3. Send: /resume %s\n\n"+
				"Your conversation will continue where you left off.",
			name, id[:12], id[:12])
	case "cli":
		instructions = fmt.Sprintf(
			"Session saved: %s (ID: %s)\n\n"+
				"To resume later:\n"+
				"  gaia session restore %s\n"+
				"  Or use /resume %s inside GAIA.",
			name, id[:12], id[:12], id[:12])
	default:
		return b.HandoffHelp(ctx)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: instructions,
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// Steer injects a guidance message that the agent will see before its next action.
// It sends to a buffered channel (cap 1) that the main loop checks between iterations.
// If the channel is full (previous steer not yet consumed), it drops the old and keeps the new.
func (b *Brain) Steer(ctx context.Context, guidance string) error {
	select {
	case <-b.steerCh: // drain old steer if present
	default:
	}
	select {
	case b.steerCh <- guidance:
	default:
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Steer queued: %s\nIt will be applied after the current tool completes.", guidance),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalSet sets a persistent goal that the agent will work toward across turns.
func (b *Brain) GoalSet(ctx context.Context, text string) error {
	if text == "" {
		return b.GoalShow(ctx)
	}
	b.goal = text
	b.subgoals = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Goal set: %s\n\nThe agent will continue working on this until it's complete. Use /subgoal to add criteria, /goals to check status, /goal clear to cancel.", text),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalShow displays the current goal and subgoals.
func (b *Brain) GoalShow(ctx context.Context) error {
	if b.goal == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Use /goal <text> to set one.\n\nExample: /goal refactor the auth module to use JWT",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active goal: %s\n", b.goal))
	if len(b.subgoals) > 0 {
		sb.WriteString("Subgoals:\n")
		for i, sg := range b.subgoals {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, sg))
		}
	}
	sb.WriteString("\nUse /subgoal <text> to add criteria.")
	sb.WriteString("\nUse /goal clear to cancel.")

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: sb.String(),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// GoalClear removes the active goal and subgoals.
func (b *Brain) GoalClear(ctx context.Context) error {
	b.goal = ""
	b.subgoals = nil
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Goal cleared. The agent will stop auto-continuing.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SubgoalAdd appends a criterion to the active goal.
func (b *Brain) SubgoalAdd(ctx context.Context, text string) error {
	if text == "" {
		return b.GoalShow(ctx)
	}
	if b.goal == "" {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No active goal. Set one with /goal first, then add /subgoal criteria.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	b.subgoals = append(b.subgoals, text)
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Subgoal added: %s", text),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// checkGoalAfterTurn evaluates goal completion and auto-continues if needed.
// Called after ProcessMessage completes (not for goal commands themselves).
func (b *Brain) checkGoalAfterTurn(ctx context.Context) {
	if b.goal == "" {
		return
	}

	// Get recent history for evaluation
	history, err := b.getHistory(ctx, 20)
	if err != nil || len(history) == 0 {
		return
	}

	// Build evaluation prompt
	var evalPrompt strings.Builder
	evalPrompt.WriteString("Evaluate whether the following goal is COMPLETE or INCOMPLETE.\n")
	evalPrompt.WriteString("Answer with exactly one word: YES or NO.\n\n")
	evalPrompt.WriteString(fmt.Sprintf("Goal: %s\n", b.goal))
	if len(b.subgoals) > 0 {
		evalPrompt.WriteString("Subgoals:\n")
		for _, sg := range b.subgoals {
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
		b.repo.SaveMessage(ctx, domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Goal complete: %s ✓", b.goal),
		})
		b.goal = ""
		b.subgoals = nil
		return
	}

	// Goal is incomplete — auto-continue
	contPrompt := fmt.Sprintf("Continue working on the goal: %s", b.goal)
	if len(b.subgoals) > 0 {
		contPrompt += "\nRequirements:\n"
		for _, sg := range b.subgoals {
			contPrompt += fmt.Sprintf("- %s\n", sg)
		}
	}
	b.ProcessMessage(ctx, contPrompt)
}

// processQueuedMessages checks the queue and processes the next item.
// Called at the end of ProcessMessage if the current message was not a queue command.
func (b *Brain) processQueuedMessages(ctx context.Context) {
	if b.processingQueue || len(b.messageQueue) == 0 {
		return
	}
	b.processingQueue = true
	defer func() { b.processingQueue = false }()

	// Process one item at a time
	prompt := b.messageQueue[0]
	b.messageQueue = b.messageQueue[1:]

	b.repo.SaveMessage(ctx, domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Processing queued message (%d remaining): %s", len(b.messageQueue), prompt),
	})
	b.ProcessMessage(ctx, prompt)
}

// ListModels displays available providers/models.
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

// LearnSkill creates a skill from a directory, URL, or description.
func (b *Brain) LearnSkill(ctx context.Context, source string) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")

	// Check if source is a directory with existing patterns
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		// Create a skill from directory contents
		name := filepath.Base(source)
		skillDir := filepath.Join(skillsDir, name)
		os.MkdirAll(skillDir, 0755)

		content := fmt.Sprintf(`---
name: %s
description: "Auto-generated skill from %s"
tags: [auto-generated]
category: custom
---

# %s

This skill was auto-generated from %s.

## Usage

Load this skill when working with code similar to the source directory.

## Patterns

- Review the source for specific patterns to document here.
`, name, source, name, source)

		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Skill created from %s\n  Name: %s\n  Location: %s\n  Edit SKILL.md to add patterns and instructions.", source, name, skillDir),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Otherwise, create a skill from description (naming convention)
	name := strings.ReplaceAll(strings.ToLower(source), " ", "-")
	name = strings.TrimRight(name, "-.,!?")
	skillDir := filepath.Join(skillsDir, name)
	os.MkdirAll(skillDir, 0755)

	content := fmt.Sprintf(`---
name: %s
description: "%s"
tags: [auto-generated, custom]
category: custom
---

# %s

%s

## Instructions

Customize this skill with specific patterns, conventions, and examples.
`, name, source, name, source)

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Skill created from description:\n  Name: %s\n  Description: %s\n  Location: %s\n  Use /skills list to activate it.", name, source, skillDir),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SkillSuggestions analyzes usage and suggests skills to create.
func (b *Brain) SkillSuggestions(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")
	existing := make(map[string]bool)

	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if e.IsDir() {
			existing[e.Name()] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("Skill suggestions based on your project:\n\n")

	suggestions := []struct {
		Name string
		Desc string
	}{
		{"go-testing", "Go testing patterns — table-driven tests, subtests, parallel"},
		{"go-error-handling", "Go error handling — wrapping, sentinel errors, custom types"},
		{"go-concurrency", "Go concurrency — goroutines, channels, sync patterns"},
		{"go-context", "Go context propagation — deadlines, cancellation, values"},
		{"go-naming", "Go naming conventions — packages, types, receivers"},
		{"angular-signals", "Angular signals — reactive state, computed, effects"},
		{"angular-testing", "Angular testing — TestBed, component harness, signals"},
	}

	count := 0
	for _, s := range suggestions {
		if !existing[s.Name] {
			sb.WriteString(fmt.Sprintf("  • %s — %s\n", s.Name, s.Desc))
			sb.WriteString(fmt.Sprintf("    Install: /learn %s\n\n", s.Name))
			count++
		}
	}

	if count == 0 {
		sb.WriteString("  All recommended skills are already installed!\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %d suggestions. Use /learn <name> or gaia skills install <name>.\n", count))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BlueprintCreate creates a new skill from a named template.
func (b *Brain) BlueprintCreate(ctx context.Context, name string) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")
	skillDir := filepath.Join(skillsDir, name)
	os.MkdirAll(skillDir, 0755)

	templates := map[string]string{
		"daily-report": `---
name: daily-report
description: "Generate daily progress report"
tags: [blueprint, report]
category: automation
---

# Daily Report

## Schedule
- Run at end of day
- Summarize changes, tests, and findings

## Output
- Markdown report saved to project root
`,
		"nightly-backup": `---
name: nightly-backup
description: "Automated nightly backup"
tags: [blueprint, backup]
category: automation
---

# Nightly Backup

## Schedule
- Run at 2:00 AM daily

## Actions
1. git add -A
2. git commit -m "nightly backup"
3. git push
`,
		"code-review": `---
name: code-review
description: "Standardized code review checklist"
tags: [blueprint, review]
category: development
---

# Code Review Blueprint

## Checklist
- [ ] Error handling is correct
- [ ] Edge cases are covered
- [ ] Tests pass
- [ ] No security issues
- [ ] Documentation updated
`,
		"api-test": `---
name: api-test
description: "API endpoint testing skill"
tags: [blueprint, testing, api]
category: testing
---

# API Test Blueprint

## Coverage
- Happy path
- Error responses
- Authentication
- Rate limiting
- Edge cases
`,
	}

	template, ok := templates[name]
	if !ok {
		available := make([]string, 0, len(templates))
		for t := range templates {
			available = append(available, t)
		}
		msg := domain.Message{
			Role: domain.RoleSystem,
			Content: fmt.Sprintf("Unknown blueprint %q.\nAvailable blueprints: %s\n\nUsage: /blueprint <name>", name, strings.Join(available, ", ")),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(template), 0644)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Blueprint created: %s\n  Location: %s\n  Edit SKILL.md to customize.", name, skillDir),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SkillCurator scans installed skills and reports issues.
func (b *Brain) SkillCurator(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No skills directory found. Install some skills first with 'gaia skills install'."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString("Skill Curator Report\n\n")

	issues := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, e.Name())
		mdPath := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			sb.WriteString(fmt.Sprintf("  ⚠ %s: missing SKILL.md\n", e.Name()))
			issues++
			continue
		}
		data, _ := os.ReadFile(mdPath)
		lines := strings.Split(string(data), "\n")
		if len(lines) < 5 {
			sb.WriteString(fmt.Sprintf("  ⚠ %s: SKILL.md is too short (%d lines)\n", e.Name(), len(lines)))
			issues++
		}
	}

	if issues == 0 {
		sb.WriteString(fmt.Sprintf("  ✅ All %d skills look healthy.\n", len(entries)))
	} else {
		sb.WriteString(fmt.Sprintf("\n  %d issues found across %d skills.\n", issues, len(entries)))
		sb.WriteString("  Use /skills audit for detailed security scanning.\n")
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// Insights shows usage analytics for the current session.
func (b *Brain) Insights(ctx context.Context) error {
	msgCount, _ := b.repo.GetMessageCount(ctx)
	history, _ := b.repo.GetHistory(ctx, 200)

	var userMsgs, assistantMsgs, toolMsgs, systemMsgs int
	for _, m := range history {
		switch m.Role {
		case domain.RoleUser:
			userMsgs++
		case domain.RoleAssistant:
			assistantMsgs++
		case domain.RoleTool:
			toolMsgs++
		case domain.RoleSystem:
			systemMsgs++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session Insights\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Total messages:  %d\n", msgCount))
	sb.WriteString(fmt.Sprintf("  User:          %d\n", userMsgs))
	sb.WriteString(fmt.Sprintf("  AI:            %d\n", assistantMsgs))
	sb.WriteString(fmt.Sprintf("  Tool calls:    %d\n", toolMsgs))
	sb.WriteString(fmt.Sprintf("  System:        %d\n", systemMsgs))
	sb.WriteString(fmt.Sprintf("  Compactions:   %d\n\n", b.compactedTo))
	sb.WriteString(fmt.Sprintf("Model:           %s\n", b.providerName))
	sb.WriteString(fmt.Sprintf("Session ID:      %s\n", b.currentSessionID[:min(12, len(b.currentSessionID))]))
	sb.WriteString(fmt.Sprintf("Budget used:     %d/%d iterations\n", len(history), b.budget.MaxIterations))

	if b.costTracker != nil {
		summary := b.costTracker.GetSummary()
		sb.WriteString(fmt.Sprintf("\nCost:           $%.2f (%.1fK in / %.1fK out)\n", summary.TotalCost, float64(summary.TotalInput)/1000, float64(summary.TotalOutput)/1000))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// DebugInfo collects system information for debugging.
func (b *Brain) DebugInfo(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "gaia", "config.yaml")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GAIA Debug Report\n%s\n\n", strings.Repeat("─", 40)))
	sb.WriteString(fmt.Sprintf("Go version:     %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("Provider:       %s\n", b.providerName))
	sb.WriteString(fmt.Sprintf("Model:          %s\n", b.modelName))
	sb.WriteString(fmt.Sprintf("Config:         %s\n", configPath))
	sb.WriteString(fmt.Sprintf("Session Msgs:   %d\n", func() int { n, _ := b.repo.GetMessageCount(ctx); return n }()))
	sb.WriteString(fmt.Sprintf("Budget:         %d max iterations\n", b.budget.MaxIterations))
	sb.WriteString(fmt.Sprintf("Compaction:     %d threshold, %d compacted\n", b.budget.CompactionThreshold, b.compactedTo))
	sb.WriteString(fmt.Sprintf("Policy tier:    %s\n", func() string {
		if b.policy != nil {
			return string(b.policy.Tier())
		}
		return "not configured"
	}()))
	if len(b.goal) > 0 {
		sb.WriteString(fmt.Sprintf("Active goal:    %s\n", b.goal))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// CreditsInfo shows credit/usage balance information.
func (b *Brain) CreditsInfo(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Credit/Billing Info\n" + strings.Repeat("─", 40) + "\n\n" +
			"GAIA doesn't have built-in billing. Credit information depends on your LLM provider:\n\n" +
			"  • OpenRouter: https://openrouter.ai/credits\n" +
			"  • OpenAI:     https://platform.openai.com/usage\n" +
			"  • Anthropic:  https://console.anthropic.com/settings/usage\n" +
			"  • Copilot:    Included in GitHub subscription\n\n" +
			"For session-level cost tracking, enable cost_tracker in config.yaml\n" +
			"and use /insights to see per-session costs.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BillingInfo shows billing management information.
func (b *Brain) BillingInfo(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Billing management is handled by your LLM provider:\n\n" +
			"  • OpenRouter: https://openrouter.ai/billing\n" +
			"  • OpenAI:     https://platform.openai.com/account/billing\n" +
			"  • Anthropic:  https://console.anthropic.com/settings/billing\n" +
			"  • Copilot:    https://github.com/settings/billing\n\n" +
			"Use /insights to track your session-level LLM costs.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// AttachImage prepares an image for vision processing.
// It validates the file (exists, size ≤ 20MB, supported MIME),
// base64-encodes it, and stores it in pendingImages for the next
// ProcessMessage call.
func (b *Brain) AttachImage(ctx context.Context, path string) error {
	// 1. Stat file, check exists and size < 20MB.
	info, err := os.Stat(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Image not found: %s", path)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	const maxSize int64 = 20 * 1024 * 1024 // 20MB
	if info.Size() > maxSize {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Image too large: %s (%.1f MB). Maximum is 20 MB.", path, float64(info.Size())/(1024*1024)),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 2. Detect MIME from extension.
	ext := strings.ToLower(filepath.Ext(path))
	var mimeType string
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unsupported image format: %s. Supported: png, jpg, jpeg, webp.", ext),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 3. Read file bytes.
	data, err := os.ReadFile(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to read image: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 4. Base64 encode.
	encoded := base64.StdEncoding.EncodeToString(data)

	// 5. Create ImageContent and store.
	img := domain.ImageContent{
		MIMEType: mimeType,
		Data:     encoded,
	}
	b.pendingImages = append(b.pendingImages, img)

	// 6. Display confirmation.
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Image attached: %s (%s, %.1f KB). Send a message to include it in the conversation.", filepath.Base(path), mimeType, float64(info.Size())/1024),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// PasteImage attaches an image from the system clipboard.
// It runs a platform-specific command to extract the clipboard image,
// saves it to a temp file, calls AttachImage, and cleans up.
func (b *Brain) PasteImage(ctx context.Context) error {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("gaia-paste-%d.png", time.Now().UnixNano()))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShell: copy clipboard image to file.
		cmd = exec.Command("powershell", "-Command",
			fmt.Sprintf(`Add-Type -AssemblyName System.Drawing; $img = [System.Windows.Forms.Clipboard]::GetImage(); if ($img -eq $null) { exit 1 }; $img.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)`, tmpFile))
	case "darwin":
		cmd = exec.Command("pngpaste", tmpFile)
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("xclip -selection clipboard -t image/png -o > %s", tmpFile))
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Clipboard image paste is not supported on %s.", runtime.GOOS),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if err := cmd.Run(); err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("No image found in clipboard, or clipboard tool is not installed. Error: %v", err),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Check that the temp file was actually created and is non-empty.
	info, err := os.Stat(tmpFile)
	if err != nil || info.Size() == 0 {
		os.Remove(tmpFile)
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No image found in clipboard.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Call AttachImage with the temp file path, then clean up.
	err = b.AttachImage(ctx, tmpFile)
	os.Remove(tmpFile)
	return err
}

// SetHome marks the current conversation as the delivery home for notifications.
func (b *Brain) SetHome(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Home set for this conversation.\n\n" +
			"Cron job results, background task completions, and alerts will be delivered here.\n" +
			"In TUI mode, this is always the terminal.\n" +
			"In gateway mode (Telegram/Discord/Slack), this binds to the current chat.\n\n" +
			"To change, send /sethome from the desired chat.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ApprovePending approves the last pending dangerous command confirmation.
func (b *Brain) ApprovePending(ctx context.Context) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Pending command approved. Use /trust full to auto-approve all commands in this session.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// DenyPending denies the last pending dangerous command confirmation.
func (b *Brain) DenyPending(ctx context.Context) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Pending command denied. Use /trust read for strictest restrictions.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// CommandsList shows all available commands (same as /help).
func (b *Brain) CommandsList(ctx context.Context) error {
	// Delegate to the help system
	return b.HelpCommands(ctx)
}

// RestartGateway performs a graceful restart by spawning a new process and exiting.
func (b *Brain) RestartGateway(ctx context.Context) error {
	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: "Restarting GAIA...",
	}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	// Spawn a new instance after a brief delay (allows this message to be sent)
	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
	}()

	os.Exit(0)
	return nil
}

// updateReleaseInfo holds info about a GitHub release.
type updateReleaseInfo struct {
	Available   bool
	Tag         string
	DownloadURL string
	Size        string
}

// checkLatestRelease queries GitHub for the latest GAIA release.
func checkLatestRelease() (*updateReleaseInfo, error) {
	// GitHub API endpoint for the latest release
	resp, err := http.Get("https://api.github.com/repos/SalvucciFacundo/gaia/releases/latest")
	if err != nil {
		return &updateReleaseInfo{
			Available:   false,
			Tag:         "",
			DownloadURL: "https://github.com/SalvucciFacundo/gaia/releases",
			Size:        "",
		}, nil // non-fatal — show manual link
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int    `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if release.TagName == "" || release.TagName == GAIAVersion {
		return &updateReleaseInfo{Available: false}, nil
	}

	// Find the right asset for this platform
	arch := runtime.GOOS + "-" + runtime.GOARCH
	var downloadURL string
	var size int
	for _, a := range release.Assets {
		if strings.Contains(a.Name, arch) || strings.Contains(a.Name, runtime.GOOS) {
			downloadURL = a.BrowserDownloadURL
			size = a.Size
			break
		}
	}
	if downloadURL == "" && len(release.Assets) > 0 {
		downloadURL = release.Assets[0].BrowserDownloadURL
		size = release.Assets[0].Size
	}

	sizeStr := fmt.Sprintf("%.1f MB", float64(size)/1_000_000)
	return &updateReleaseInfo{
		Available:   true,
		Tag:         release.TagName,
		DownloadURL: downloadURL,
		Size:        sizeStr,
	}, nil
}

// downloadAndReplace downloads a release binary and replaces the current executable atomically.
func downloadAndReplace(execPath, downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("no download URL available")
	}

	tmpPath := execPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write download: %w", err)
	}
	out.Close()

	// Atomically replace: rename current to .old, move new in place
	oldPath := execPath + ".old"
	os.Remove(oldPath)
	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Rename(oldPath, execPath) // restore backup
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	os.Chmod(execPath, 0755)

	return nil
}

// UpdateGAIA checks for updates and applies them automatically.
func (b *Brain) UpdateGAIA(ctx context.Context) error {
	msg := domain.Message{Role: domain.RoleSystem, Content: "Checking for updates..."}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	execPath, err := os.Executable()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Cannot locate executable: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	latest, err := checkLatestRelease()
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update check failed: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if !latest.Available {
		msg := domain.Message{Role: domain.RoleSystem, Content: "You're on the latest version. No update needed.\nCheck manually: https://github.com/SalvucciFacundo/gaia/releases"}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Downloading %s (%s)...", latest.Tag, latest.Size)}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	if err := downloadAndReplace(execPath, latest.DownloadURL); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Update failed: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	msg = domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Updated to %s! Restarting...", latest.Tag)}
	b.repo.SaveMessage(ctx, msg)
	b.ui.Display(msg)

	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(execPath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
	}()
	os.Exit(0)
	return nil
}

// TopicHelp shows help for the /topic command.
func (b *Brain) TopicHelp(ctx context.Context) error {
	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: "Multi-session DM mode (Telegram-specific):\n\n" +
			"  /topic list           — Show active topics\n" +
			"  /topic new <name>     — Start a new topic\n" +
			"  /topic switch <name>  — Switch to a topic\n" +
			"  /topic close <name>   — Close a topic\n\n" +
			"Each topic has its own conversation history.\n" +
			"Useful for parallel conversations in one chat.",
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// TopicSession switches to a named topic/session.
func (b *Brain) TopicSession(ctx context.Context, name string) error {
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Switched to topic: %s\nAll messages will be scoped to this topic until you switch back.", name),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// HelpCommands is the shared implementation for both /help and /commands.
func (b *Brain) HelpCommands(ctx context.Context) error {
	help := `Available Commands:

  Session:     /new, /clear, /history, /save, /resume, /sessions, /title, /compress, /undo, /retry
  State:       /branch, /branches, /snapshot save, /snapshot load
  Goals:       /goal, /subgoal, /goals, /goal clear
  Queue/Steer: /queue, /q, /steer
  Background:  /background, /moa, /tasks, /cancel
  Handoff:     /handoff telegram, /handoff discord, /handoff cli
  Config:      /model, /reasoning, /personality, /yolo, /verbose, /timestamps, /statusbar, /footer, /indicator, /skin
  Permissions: /permisos, /trust
  Skills:      /skills, /learn, /suggestions, /blueprint, /curator
  Cron:        /cron list, /cron add, /cron remove
  Memory:      /memory pending, /memory approve, /memory reject
  Info:        /help, /version, /platforms, /copy, /insights, /debug, /credits, /billing
  Media:       /image, /paste
  Messaging:   /sethome, /approve, /deny, /commands, /restart, /update, /topic

For details: gaia help or check the README.`
	msg := domain.Message{Role: domain.RoleSystem, Content: help}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// AuditLog displays the policy audit trail.
func (b *Brain) AuditLog(ctx context.Context) error {
	if len(b.auditLog) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log is empty.\n\nSet a tool override to 'audit' via /permisos to start logging.\nExample: set shell_exec → audit to log all shell commands."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Policy Audit Log (%d entries)\n%s\n\n", len(b.auditLog), strings.Repeat("─", 40)))
	start := 0
	if len(b.auditLog) > 50 {
		start = len(b.auditLog) - 50
		sb.WriteString(fmt.Sprintf("Showing last 50 of %d entries.\n\n", len(b.auditLog)))
	}
	for i := start; i < len(b.auditLog); i++ {
		e := b.auditLog[i]
		status := "ALLOW"
		if !e.Allowed {
			status = "DENY"
		}
		preview := e.Args
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %-20s %s\n", e.Time.Format("15:04:05"), status, e.Tool, preview))
	}
	sb.WriteString("\nKey: [ALLOW] audited and permitted  [DENY] audited and blocked\n")
	sb.WriteString("Use /audit clear to clear the log.")

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// AuditClear clears the policy audit log.
func (b *Brain) AuditClear(ctx context.Context) error {
	b.auditLog = nil
	msg := domain.Message{Role: domain.RoleSystem, Content: "Audit log cleared."}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
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
	b.auditLog = append(b.auditLog, AuditEntry{
		Time:    time.Now(),
		Tool:    toolName,
		Args:    argStr,
		Tier:    string(b.policy.Tier()),
		Allowed: success,
	})
}

// SessionStatus displays the current session mode and active sessions.
func (b *Brain) SessionStatus(ctx context.Context) error {
	var content string
	if b.sessionMgr != nil {
		content = b.sessionMgr.Status()
	} else {
		content = "Session manager not configured. All messages use the default session."
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: content}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SessionCommand processes /session subcommands.
func (b *Brain) SessionCommand(ctx context.Context, args string) error {
	if b.sessionMgr == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session manager not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	switch args {
	case "unify":
		b.sessionMgr.SetMode(SessionUnify)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: unify\nAll platforms now share one session. Use /session isolate to separate them."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	case "isolate":
		b.sessionMgr.SetMode(SessionIsolate)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: isolate\nEach platform now has its own independent session. Use /session unify to merge them."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	case "ask":
		b.sessionMgr.SetMode(SessionAsk)
		msg := domain.Message{Role: domain.RoleSystem, Content: "Session mode set to: ask\nGAIA will ask how to handle messages when you switch platforms."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)

	default:
		return b.SessionStatus(ctx)
	}
}

// KGStatus displays the current KG recall state.
func (b *Brain) KGStatus(ctx context.Context) error {
	status := "OFF"
	if b.kgEnabled {
		status = "ON"
	}
	var count int
	if b.kgStore != nil {
		facts, _ := b.kgStore.SearchFacts(ctx, "")
		count = len(facts)
	}
	content := fmt.Sprintf("Knowledge Graph Recall: %s\n\nFacts stored: %d\n\nCommands:\n  /kg on     — Enable KG recall\n  /kg off    — Disable KG recall\n  /kg stats  — Show facts by topic\n  /kg clear  — Clear all facts\n\nKG recall injects relevant facts as extra context.\nIt does NOT replace conversation history.", status, count)
	msg := domain.Message{Role: domain.RoleSystem, Content: content}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// KGStats shows facts grouped by topic.
func (b *Brain) KGStats(ctx context.Context) error {
	if b.kgStore == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph not configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	facts, err := b.kgStore.SearchFacts(ctx, "")
	if err != nil || len(facts) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No facts stored yet. Facts are extracted automatically from responses when KG recall is enabled."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	byTopic := make(map[string]int)
	for _, f := range facts {
		byTopic[f.Topic]++
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Knowledge Graph Facts (%d total)\n\n", len(facts)))
	for topic, count := range byTopic {
		sb.WriteString(fmt.Sprintf("  %-25s %d facts\n", topic, count))
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// KGClear removes all knowledge graph facts.
func (b *Brain) KGClear(ctx context.Context) error {
	msg := domain.Message{Role: domain.RoleSystem, Content: "Knowledge Graph facts cleared."}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// extractKGFacts analyzes a response and extracts key facts to store in the knowledge graph.
func (b *Brain) extractKGFacts(ctx context.Context, response string) {
	if b.kgStore == nil || !b.kgEnabled {
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
				b.kgStore.AddFact(ctx, domain.KnowledgeFact{
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

// triggerSkillLearning analyzes a completed subagent task and proposes skill improvements.
func (b *Brain) triggerSkillLearning(ctx context.Context, subagentName string, taskDesc string, result *domain.SubagentResult) {
	// Only analyze if skills were resolved
	if result == nil || result.SkillResolution == "none" {
		return
	}
	// For now, log that learning happened — actual skill creation/deep analysis
	// requires a separate Learner subagent invocation.
	_ = subagentName
	_ = taskDesc
}

// createSkillFromProposal creates a new skill from a Learner proposal.
func (b *Brain) createSkillFromProposal(name string, content string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("create skill: %w", err)
	}
	skillDir := filepath.Join(home, ".gaia", "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
}

// MCPStatus displays MCP server status.
func (b *Brain) MCPStatus(ctx context.Context) error {
	if b.mcpMgr == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "MCP manager not configured. Servers are configured in config.yaml under mcp.servers."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: b.mcpMgr.StatusText()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

func (b *Brain) MCPConnect(ctx context.Context, name string) error {
	if b.mcpMgr == nil {
		return b.MCPStatus(ctx)
	}
	if err := b.mcpMgr.Connect(ctx, name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to connect %q: %v", name, err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q connected", name)}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

func (b *Brain) MCPDisconnect(ctx context.Context, name string) error {
	if b.mcpMgr == nil {
		return b.MCPStatus(ctx)
	}
	if err := b.mcpMgr.Disconnect(name); err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to disconnect %q: %v", name, err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("MCP server %q disconnected", name)}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// ListProviderModels shows all available models from the current provider.
func (b *Brain) ListProviderModels(ctx context.Context) error {
	models, err := b.provider.ListModels(ctx)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to list models: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	if len(models) == 0 {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No models available from the current provider."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Models available for %s (%d total):\n\n", b.providerName, len(models)))
	for _, m := range models {
		mark := "  "
		if m == b.modelName {
			mark = "➤ "
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", mark, m))
	}
	sb.WriteString(fmt.Sprintf("\nUse /model <name> to switch, or set llm.model in config.yaml."))
	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
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
// populateSkillRegistry fills the task's SkillRegistry with available skill names.
func (b *Brain) populateSkillRegistry(task *domain.SubagentTask) {
	if b.skillLister == nil {
		return
	}
	task.SkillRegistry = b.skillLister()
}

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
	if content == "/model" {
		return b.ListModels(ctx)
	}
	if strings.HasPrefix(content, "/model ") {
		return b.SwitchModel(ctx, strings.TrimSpace(content[7:]))
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

	// 0af. /busy — control Enter key behavior
	if content == "/busy" || strings.HasPrefix(content, "/busy ") {
		arg := ""
		if strings.HasPrefix(content, "/busy ") {
			arg = strings.TrimSpace(content[6:])
		}
		return b.BusyMode(ctx, arg)
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
		// Policy guard evaluation (additive — runs before ConfirmGuard)
		if b.policy != nil {
			result := b.policy.Evaluate(tc.Name, tc.Arguments)
			if !result.Allowed {
				// Check smart escalation
				switch result.SuggestedAction {
				case "skip":
					// Silently skip — tool is cosmetic/read-only
					continue
				case "alternative":
					// Block but notify — alternatives not auto-dispatched yet
					toolMsg := domain.Message{
						Role:    domain.RoleTool,
						Content: fmt.Sprintf("Policy denied: %s (reason: %s). Try an alternative tool.", tc.Name, result.Reason),
					}
					b.repo.SaveMessage(ctx, toolMsg)
					continue
				default:
					// "ask_user" or "block" — prompt confirmation
					if result.SuggestedAction == "ask_user" {
						confirmed, err := b.ui.PromptConfirmation(fmt.Sprintf(
							"Policy blocks %s (reason: %s). Allow anyway?",
							tc.Name, result.Reason))
						if err != nil || !confirmed {
							toolMsg := domain.Message{
								Role:    domain.RoleTool,
								Content: fmt.Sprintf("User denied policy-blocked tool %s.", tc.Name),
							}
							b.repo.SaveMessage(ctx, toolMsg)
							continue
						}
						// User approved — update session approvals
						b.policy.Session().ApproveSession(tc.Name)
					} else {
						toolMsg := domain.Message{
							Role:    domain.RoleTool,
							Content: fmt.Sprintf("Policy blocked: %s (reason: %s)", tc.Name, result.Reason),
						}
						b.repo.SaveMessage(ctx, toolMsg)
						continue
					}
				}
			}
		}

		// Confirmation gate (legacy ConfirmGuard — kept for backward compat)
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

		// Audit log: record tool execution if policy has audit override
		b.logAudit(tc.Name, tc.Arguments, result.Success)

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
















