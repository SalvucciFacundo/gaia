// Package core implements the central orchestrator and routing for GAIA.
package core

import (
	"fmt"
	"strings"
)

// ODDRouteType represents the execution topology selected by Organic Driven Development.
type ODDRouteType string

const (
	// RouteInline executes directly in the parent loop within bounded call/token budget.
	RouteInline ODDRouteType = "inline"
	// RouteDelegatedWorker delegates to a bounded, specialized dynamic worker.
	RouteDelegatedWorker ODDRouteType = "delegated_worker"
	// RouteFeatureTracking manages complex multi-stage tasks with tracking specs.
	RouteFeatureTracking ODDRouteType = "feature_tracking"
)

// Command prefixes
const (
	// ODDCommandPrefix triggers the ODD feature tracking workflow.
	ODDCommandPrefix = "/odd"
	// InlineCommandPrefix forces direct inline execution.
	InlineCommandPrefix = "/inline"
	// LegacySDDCommandPrefix is the deprecated backup command for feature/pipeline mode.
	LegacySDDCommandPrefix = "/sdd"
	// LegacyDirectCommandPrefix is the deprecated backup command for inline execution.
	LegacyDirectCommandPrefix = "/direct"
)

// ODDTriggerResult describes the result of evaluating an incoming prompt against ODD rules.
type ODDTriggerResult struct {
	Route             ODDRouteType
	WorkerRole        string // "explorer", "writer", "verifier", or ""
	SuggestFeature    bool   // If true, recommend formal feature tracking
	IsDeprecatedAlias bool   // True if triggered via legacy /sdd or /direct
	Reason            string
}

// ExplorerSignals detect tasks requiring deep read-only codebase mapping.
var ExplorerSignals = []string{
	"map codebase", "mapear codebase", "investigate codebase",
	"investigar arquitectura", "broad search", "analizar dependencias",
	"deep search", "trace all references", "find all usages of",
}

// WriterSignals detect tasks requiring multi-file edits or heavy implementation.
var WriterSignals = []string{
	"refactor multiple files", "refactorizar multiples archivos",
	"implement across packages", "migrar componentes",
}

// VerifierSignals detect heavy validation, test suites, or build checks.
var VerifierSignals = []string{
	"run full test suite", "ejecutar todos los tests",
	"verify all packages", "verificar suite completa",
}

// DetectODDRoute evaluates user input according to Organic Driven Development rules:
// 1. Explicit /inline or legacy /direct -> RouteInline
// 2. Explicit /odd or legacy /sdd -> RouteFeatureTracking
// 3. Active phase continuation signals -> RouteFeatureTracking
// 4. Architectural breaking changes -> RouteFeatureTracking
// 5. Large mapping / multi-file write / full verification signals -> RouteDelegatedWorker
// 6. Proactive multi-module signals -> SuggestFeature
// 7. Default -> RouteInline
func DetectODDRoute(content string) ODDTriggerResult {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ODDTriggerResult{
			Route:  RouteInline,
			Reason: "empty input",
		}
	}

	// 1. Explicit inline commands
	if strings.HasPrefix(trimmed, InlineCommandPrefix) {
		return ODDTriggerResult{
			Route:  RouteInline,
			Reason: "user requested /inline execution",
		}
	}
	if strings.HasPrefix(trimmed, LegacyDirectCommandPrefix) {
		return ODDTriggerResult{
			Route:             RouteInline,
			IsDeprecatedAlias: true,
			Reason:            "user requested legacy /direct — executing inline",
		}
	}

	// 2. Explicit feature/pipeline commands
	if strings.HasPrefix(trimmed, ODDCommandPrefix) {
		return ODDTriggerResult{
			Route:  RouteFeatureTracking,
			Reason: "user requested /odd feature tracking",
		}
	}
	if strings.HasPrefix(trimmed, LegacySDDCommandPrefix) {
		return ODDTriggerResult{
			Route:             RouteFeatureTracking,
			IsDeprecatedAlias: true,
			Reason:            "user requested legacy /sdd — routing to feature tracking",
		}
	}

	lower := strings.ToLower(trimmed)

	// 3. Active phase continuation tokens
	for _, ps := range SDDActivePhaseSignals {
		if strings.Contains(lower, ps) {
			return ODDTriggerResult{
				Route:  RouteFeatureTracking,
				Reason: "active phase continuation signal detected: '" + ps + "'",
			}
		}
	}

	// 4. Architectural breaking change signals
	for _, kw := range SDDKeywords {
		if strings.Contains(lower, kw) {
			return ODDTriggerResult{
				Route:  RouteFeatureTracking,
				Reason: "architectural breaking keyword '" + kw + "' detected",
			}
		}
	}

	// 5. Worker delegation triggers
	for _, s := range ExplorerSignals {
		if strings.Contains(lower, s) {
			return ODDTriggerResult{
				Route:      RouteDelegatedWorker,
				WorkerRole: "explorer",
				Reason:     "read-only mapping signal detected: '" + s + "'",
			}
		}
	}

	for _, s := range WriterSignals {
		if strings.Contains(lower, s) {
			return ODDTriggerResult{
				Route:      RouteDelegatedWorker,
				WorkerRole: "writer",
				Reason:     "multi-file write signal detected: '" + s + "'",
			}
		}
	}

	for _, s := range VerifierSignals {
		if strings.Contains(lower, s) {
			return ODDTriggerResult{
				Route:      RouteDelegatedWorker,
				WorkerRole: "verifier",
				Reason:     "full verification signal detected: '" + s + "'",
			}
		}
	}

	// 6. Proactive feature suggestion signals
	for _, sig := range SDDProactiveSignals {
		if strings.Contains(lower, sig) {
			return ODDTriggerResult{
				Route:          RouteInline,
				SuggestFeature: true,
				Reason:         "substantial multi-module signal '" + sig + "' detected",
			}
		}
	}

	// 7. Default to inline direct execution
	return ODDTriggerResult{
		Route:  RouteInline,
		Reason: "within inline execution budget",
	}
}

// In-Flight Mid-Turn Delegation Triggers and Budget Limits (ODD Contract):
// - Evidence budget: <= 3 calls, <= 10,000 tokens
// - Sequential lookups / mapping: > 5 lookups -> delegate to explorer
// - Non-trivial writes: 2+ non-trivial file modifications -> delegate to writer
const (
	MaxInlineEvidenceCalls  = 3
	MaxInlineEvidenceTokens = 10000
	MaxInlineSequentialRead = 5
	MaxInlineFileWrites     = 2
)

// MidTurnBudgetTracker monitors in-flight tool calls and resource usage during an inline turn.
type MidTurnBudgetTracker struct {
	EvidenceCalls  int
	EvidenceTokens int
	SequentialRead int
	FilesWritten   map[string]bool
	ToolHistory    []string // Formatted log of tools and outputs in current turn
}

// NewMidTurnBudgetTracker creates an initialized tracker for a single turn.
func NewMidTurnBudgetTracker() *MidTurnBudgetTracker {
	return &MidTurnBudgetTracker{
		FilesWritten: make(map[string]bool),
		ToolHistory:  make([]string, 0),
	}
}

// Reset clears in-flight counters (used when a mid-turn delegation fails, preventing busy loops).
func (t *MidTurnBudgetTracker) Reset() {
	t.EvidenceCalls = 0
	t.EvidenceTokens = 0
	t.SequentialRead = 0
	t.FilesWritten = make(map[string]bool)
	t.ToolHistory = make([]string, 0)
}

// RecordToolCall updates tracker state based on tool execution.
func (t *MidTurnBudgetTracker) RecordToolCall(name string, args map[string]interface{}, output string) {
	lowerName := strings.ToLower(name)

	// Approximate token count: chars / 4
	tokens := len(output) / 4

	// Read / investigation tools
	if strings.Contains(lowerName, "read") || strings.Contains(lowerName, "grep") ||
		strings.Contains(lowerName, "search") || strings.Contains(lowerName, "find") ||
		strings.Contains(lowerName, "list") {
		t.EvidenceCalls++
		t.EvidenceTokens += tokens
		t.SequentialRead++
	}

	// Write / edit tools
	if strings.Contains(lowerName, "write") || strings.Contains(lowerName, "edit") ||
		strings.Contains(lowerName, "patch") || strings.Contains(lowerName, "create") {
		if path, ok := args["path"].(string); ok && path != "" {
			t.FilesWritten[path] = true
		} else if file, ok := args["file"].(string); ok && file != "" {
			t.FilesWritten[file] = true
		} else if target, ok := args["target"].(string); ok && target != "" {
			t.FilesWritten[target] = true
		}
	}

	// Truncate output for history log
	snip := output
	if len(snip) > 120 {
		snip = snip[:120] + "..."
	}
	t.ToolHistory = append(t.ToolHistory, fmt.Sprintf("%s(%v) -> %s", name, args, snip))
}

// CheckDelegationTrigger checks if in-flight thresholds were exceeded.
// Returns targetRole ("explorer" or "writer"), reason, and true if delegation is required.
func (t *MidTurnBudgetTracker) CheckDelegationTrigger() (string, string, bool) {
	if t.SequentialRead > MaxInlineSequentialRead {
		return "explorer", fmt.Sprintf("sequential lookups (%d) exceeded budget (> %d)", t.SequentialRead, MaxInlineSequentialRead), true
	}
	if t.EvidenceCalls > MaxInlineEvidenceCalls {
		return "explorer", fmt.Sprintf("evidence calls (%d) exceeded budget (> %d)", t.EvidenceCalls, MaxInlineEvidenceCalls), true
	}
	if t.EvidenceTokens > MaxInlineEvidenceTokens {
		return "explorer", fmt.Sprintf("evidence tokens (~%d) exceeded budget (> %d)", t.EvidenceTokens, MaxInlineEvidenceTokens), true
	}
	if len(t.FilesWritten) >= MaxInlineFileWrites {
		return "writer", fmt.Sprintf("multi-file edits (%d files) reached write boundary (>= %d)", len(t.FilesWritten), MaxInlineFileWrites), true
	}
	return "", "", false
}

