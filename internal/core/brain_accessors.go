package core

import (
	"context"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Repo returns the persistence repository.
func (b *Brain) Repo() ports.Repository {
	return b.repo
}

// UI returns the UI service.
func (b *Brain) UI() ports.UIService {
	return b.ui
}

// Budget returns the agent's budget configuration.
func (b *Brain) Budget() domain.BudgetConfig {
	return b.budget
}

// CompactedTo returns the count of compacted messages.
func (b *Brain) CompactedTo() int {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.compactedTo
}

// SetCompactedTo updates the count of compacted messages.
func (b *Brain) SetCompactedTo(n int) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.compactedTo = n
}

// ProviderName returns the active LLM provider name.
func (b *Brain) ProviderName() string {
	return b.providerName
}

// ModelName returns the active LLM model name.
func (b *Brain) ModelName() string {
	return b.modelName
}

// CostTracker returns the cost tracker, if wired.
func (b *Brain) CostTracker() *CostTracker {
	return b.costTracker
}

// AvailableProviders returns configured alternative providers.
func (b *Brain) AvailableProviders() map[string]ports.LLMProvider {
	return b.availableProviders
}

// SetAvailableProviders configures alternative providers for runtime switching.
func (b *Brain) SetAvailableProviders(providers map[string]ports.LLMProvider) {
	b.availableProviders = providers
}

// SetActiveProvider switches the primary LLM provider in runtime.
func (b *Brain) SetActiveProvider(name string, p ports.LLMProvider) {
	b.provider = p
	b.providerName = name
}

// CompactHistory forces context compaction for older messages.
func (b *Brain) CompactHistory(ctx context.Context) error {
	return b.compactHistory(ctx)
}

// KGStore returns the knowledge graph store, if wired.
func (b *Brain) KGStore() ports.KnowledgeGraphStore {
	return b.kgStore
}

// SessionManager returns the multi-platform session manager, if wired.
func (b *Brain) SessionManager() *SessionManager {
	return b.sessionMgr
}

// MessageQueue returns a copy of queued messages.
func (b *Brain) MessageQueue() []string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	queue := make([]string, len(b.messageQueue))
	copy(queue, b.messageQueue)
	return queue
}

// EnqueueMessage appends a message to the message queue.
func (b *Brain) EnqueueMessage(msg string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.messageQueue = append(b.messageQueue, msg)
}

// ClearMessageQueue clears all queued messages.
func (b *Brain) ClearMessageQueue() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.messageQueue = nil
}

// Goal returns the persistent active goal.
func (b *Brain) Goal() string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.goal
}

// SetGoal updates the active goal and resets turn counter.
func (b *Brain) SetGoal(goal string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.goal = goal
	b.goalTurns = 0
}

// ClearGoal clears the active goal and all subgoals.
func (b *Brain) ClearGoal() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.goal = ""
	b.subgoals = nil
	b.goalTurns = 0
}

// Subgoals returns the active subgoals list.
func (b *Brain) Subgoals() []string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	sub := make([]string, len(b.subgoals))
	copy(sub, b.subgoals)
	return sub
}

// AddSubgoal appends a criterion to active subgoals.
func (b *Brain) AddSubgoal(subgoal string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.subgoals = append(b.subgoals, subgoal)
}

// AuditEntries returns a copy of the security audit log.
func (b *Brain) AuditEntries() []AuditEntry {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	entries := make([]AuditEntry, len(b.auditLog))
	copy(entries, b.auditLog)
	return entries
}

// ClearAuditEntries clears the security audit log.
func (b *Brain) ClearAuditEntries() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.auditLog = nil
}

// FastModeState returns whether fast mode is enabled, and model names.
func (b *Brain) FastModeState() (enabled bool, origModel, fastModel string) {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.fastModeEnabled, b.originalModel, b.fastModel
}

// SetFastMode updates the fast mode status.
func (b *Brain) SetFastMode(enabled bool, origModel, fastModel string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.fastModeEnabled = enabled
	b.originalModel = origModel
	b.fastModel = fastModel
	if enabled && fastModel != "" {
		b.modelName = fastModel
	} else if !enabled && origModel != "" {
		b.modelName = origModel
	}
}

// BusyMode returns the busy mode string.
func (b *Brain) BusyMode() string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.busyMode
}

// SetBusyMode updates the busy mode string.
func (b *Brain) SetBusyMode(mode string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.busyMode = mode
}

// PendingConfirm returns the tool awaiting interactive approval.
func (b *Brain) PendingConfirm() string {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	return b.pendingConfirm
}

// SetPendingConfirm updates the tool awaiting interactive approval.
func (b *Brain) SetPendingConfirm(tool string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.pendingConfirm = tool
}

// AddPendingImage queues an image for the next agent turn.
func (b *Brain) AddPendingImage(img domain.ImageContent) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.pendingImages = append(b.pendingImages, img)
}

// PendingImages returns queued images for the next turn.
func (b *Brain) PendingImages() []domain.ImageContent {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	imgs := make([]domain.ImageContent, len(b.pendingImages))
	copy(imgs, b.pendingImages)
	return imgs
}

// ClearPendingImages removes queued images.
func (b *Brain) ClearPendingImages() {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.pendingImages = nil
}

// SubagentPort returns the subagent port, if wired.
func (b *Brain) SubagentPort() ports.SubagentPort {
	return b.subagentPort
}

// Guard returns the interactive confirmation guard.
func (b *Brain) Guard() *ConfirmGuard {
	return b.guard
}

// Policy returns the policy guard, if wired.
func (b *Brain) Policy() *PolicyGuard {
	return b.policy
}

// SkillLister returns the configured skill lister function.
func (b *Brain) SkillLister() func() []string {
	return b.skillLister
}

// SkillMatcher returns the configured skill matcher function.
func (b *Brain) SkillMatcher() func(string, []string) []string {
	return b.skillMatcher
}

// SetMCPManager wires an MCP connection manager into the Brain.
func (b *Brain) SetMCPManager(mgr ports.MCPManager) {
	b.mcpMgr = mgr
}

// MCPManager returns the MCP manager, if wired.
func (b *Brain) MCPManager() ports.MCPManager {
	return b.mcpMgr
}

// SteerChannel returns the channel for steering messages.
func (b *Brain) SteerChannel() chan string {
	return b.steerCh
}

// QueryKGContext searches the knowledge graph for facts relevant to the given text.
func (b *Brain) QueryKGContext(ctx context.Context, text string) []string {
	return b.recallKnowledgeGraph(ctx, text)
}

// SaveSubagentDiscoveries persists subagent insights into the knowledge graph.
func (b *Brain) SaveSubagentDiscoveries(ctx context.Context, name, description string, result *domain.SubagentResult) {
	if b.kgRecorder != nil {
		b.kgRecorder.RecordSubagentResult(ctx, name, description, result)
	}
}

// PostTurnHooks executes background facts extraction, message queue processing, and goal checks.
func (b *Brain) PostTurnHooks(ctx context.Context, content string) {
	if b.kgRecorder != nil && b.KGEnabled() {
		b.kgRecorder.RecordResponseFacts(ctx, content)
	}
	b.processQueuedMessages(ctx)
	b.checkGoalAfterTurn(ctx)
}
