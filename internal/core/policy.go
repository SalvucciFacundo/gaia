package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// PolicyTier defines the autonomy level for tool execution.
type PolicyTier string

const (
	TierRead    PolicyTier = "read"    // read-only tools only
	TierSandbox PolicyTier = "sandbox" // read + write + safe shell within project scope
	TierFull    PolicyTier = "full"    // all tools allowed
)

// OverridePolicy defines how a specific tool behaves relative to the tier.
type OverridePolicy string

const (
	OverrideAllow      OverridePolicy = "allow"       // always allow regardless of tier
	OverrideDeny       OverridePolicy = "deny"        // always deny regardless of tier
	OverrideSkip       OverridePolicy = "skip"        // deny silently, agent tries alternative
	OverrideAskOnce    OverridePolicy = "ask-once"    // prompt once, remember for this call
	OverrideAskSession OverridePolicy = "ask-session" // prompt once per session
	OverrideAskAlways  OverridePolicy = "ask-always"  // prompt every time
	OverrideAudit      OverridePolicy = "audit"       // allow but log for audit
)

// PathRule defines a glob-based access rule for filesystem paths.
type PathRule struct {
	Pattern string         `yaml:"pattern"` // glob pattern, e.g. "/etc/**"
	Policy  OverridePolicy `yaml:"policy"`
}

// PolicyResult represents the outcome of a policy evaluation.
type PolicyResult struct {
	Allowed         bool
	Reason          string // why denied: "hardline", "deny_rule", "tier_block", "override_deny"
	SuggestedAction string // "skip", "alternative", "ask_user", "block"
	BlockedTool     string // which tool was blocked
}

// PolicyConfig is the YAML-serializable policy configuration.
type PolicyConfig struct {
	Tier      PolicyTier                `yaml:"tier"`
	Overrides map[string]OverridePolicy `yaml:"overrides,omitempty"`
	DenyRules []string                  `yaml:"deny_rules,omitempty"`
	PathRules []PathRule                `yaml:"path_rules,omitempty"`
	Platforms map[string]PlatformTier   `yaml:"platforms,omitempty"`
}

// PlatformTier defines a per-platform policy tier override.
type PlatformTier struct {
	Tier PolicyTier `yaml:"tier"`
}

// SessionApprovals tracks per-session and one-time tool approvals.
type SessionApprovals struct {
	session map[string]bool // approved for the duration of this session
	once    map[string]bool // approved once for the current call chain
}

// NewSessionApprovals creates an empty session approvals tracker.
func NewSessionApprovals() *SessionApprovals {
	return &SessionApprovals{
		session: make(map[string]bool),
		once:    make(map[string]bool),
	}
}

// IsSessionApproved returns true if the tool was approved for this session.
func (sa *SessionApprovals) IsSessionApproved(toolName string) bool {
	if sa == nil {
		return false
	}
	return sa.session[toolName]
}

// ApproveSession marks a tool as approved for the remainder of the session.
func (sa *SessionApprovals) ApproveSession(toolName string) {
	if sa == nil {
		return
	}
	sa.session[toolName] = true
}

// IsOnceApproved returns true if the tool was approved for this call chain.
func (sa *SessionApprovals) IsOnceApproved(toolName string) bool {
	if sa == nil {
		return false
	}
	return sa.once[toolName]
}

// ApproveOnce marks a tool as approved for the current call chain.
func (sa *SessionApprovals) ApproveOnce(toolName string) {
	if sa == nil {
		return
	}
	sa.once[toolName] = true
}

// Reset clears all session and one-time approvals.
func (sa *SessionApprovals) Reset() {
	if sa == nil {
		return
	}
	sa.session = make(map[string]bool)
	sa.once = make(map[string]bool)
}

// PolicyGuard evaluates tool execution permissions based on tier,
// per-tool overrides, deny rules, and the hardline blocklist.
type PolicyGuard struct {
	mu        sync.RWMutex
	tier      PolicyTier
	overrides map[string]OverridePolicy
	store     PolicyStore
	session   *SessionApprovals
}

// NewPolicyGuard creates a PolicyGuard with the given configuration.
// Pass nil for store to operate without persistence.
func NewPolicyGuard(tier PolicyTier, overrides map[string]OverridePolicy, store PolicyStore) *PolicyGuard {
	if tier == "" {
		tier = TierFull
	}
	if overrides == nil {
		overrides = make(map[string]OverridePolicy)
	}
	return &PolicyGuard{
		tier:      tier,
		overrides: overrides,
		store:     store,
		session:   NewSessionApprovals(),
	}
}

// Tier returns the current policy tier.
func (pg *PolicyGuard) Tier() PolicyTier {
	pg.mu.RLock()
	defer pg.mu.RUnlock()
	return pg.tier
}

// SetTier changes the policy tier and resets session approvals.
func (pg *PolicyGuard) SetTier(tier PolicyTier) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	pg.tier = tier
	if pg.session != nil {
		pg.session.Reset()
	}
}

// Override returns the configured override for a tool, if any.
func (pg *PolicyGuard) Override(toolName string) (OverridePolicy, bool) {
	pg.mu.RLock()
	defer pg.mu.RUnlock()
	ov, ok := pg.overrides[toolName]
	return ov, ok
}

// SetOverride configures a per-tool override at runtime.
func (pg *PolicyGuard) SetOverride(toolName string, policy OverridePolicy) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	pg.overrides[toolName] = policy
}

// RemoveOverride removes a per-tool override.
func (pg *PolicyGuard) RemoveOverride(toolName string) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	delete(pg.overrides, toolName)
}

// Session returns the session approvals tracker (may be nil).
func (pg *PolicyGuard) Session() *SessionApprovals {
	return pg.session
}

// Store returns the policy store (may be nil).
func (pg *PolicyGuard) Store() PolicyStore {
	return pg.store
}

// SetStore sets the policy store and reloads configuration.
func (pg *PolicyGuard) SetStore(store PolicyStore) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	pg.store = store
}

// ApplyConfig merges a PolicyConfig into the guard's runtime configuration.
// This is used to apply domain-level config at initialization time.
func (pg *PolicyGuard) ApplyConfig(cfg PolicyConfig) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	if cfg.Tier != "" {
		pg.tier = cfg.Tier
	}
	for k, v := range cfg.Overrides {
		pg.overrides[k] = v
	}
}

// --- Tool Classification ---

// toolTiers maps tool names to the minimum tier required for execution.
// Tools not listed here default to TierFull. The classification is data-driven
// so new tools added to the registry are automatically gated at the strictest level.
var toolTiers = map[string]PolicyTier{
	// Read-only tools — TierRead
	"read":           TierRead,
	"glob":           TierRead,
	"grep":           TierRead,
	"file_read":      TierRead,
	"file_list":      TierRead,
	"file_info":      TierRead,
	"list_dir":       TierRead,
	"git_status":     TierRead,
	"git_log":        TierRead,
	"git_diff":       TierRead,
	"webfetch":       TierRead,
	"mem_search":     TierRead,
	"mem_context":    TierRead,
	"mem_get":        TierRead,
	"codegraph_explore": TierRead,
	// Write tools — TierSandbox
	"file_write":     TierSandbox,
	"shell_exec":     TierSandbox,
	// Mutating git tools — TierSandbox
	"git_commit":     TierSandbox,
	"git_branch":     TierSandbox,
	"git_checkout":   TierSandbox,
	"git_add":        TierSandbox,
	"git_push":       TierSandbox,
}

// ClassifyTool returns the minimum tier required for a given tool name.
// Unknown tools default to TierFull.
func ClassifyTool(toolName string) PolicyTier {
	if tier, ok := toolTiers[toolName]; ok {
		return tier
	}
	// MCP tools: classify by prefix — read-oriented MCP servers default to read tier
	if strings.HasPrefix(toolName, "mem_") || strings.HasPrefix(toolName, "codegraph_") {
		return TierRead
	}
	// Unknown tools require full tier
	return TierFull
}

// TierAllows reports whether the current tier permits a tool at the given required tier.
// Order: read < sandbox < full.
func TierAllows(current, required PolicyTier) bool {
	switch current {
	case TierFull:
		return true
	case TierSandbox:
		return required == TierRead || required == TierSandbox
	case TierRead:
		return required == TierRead
	default:
		return false
	}
}

// --- Hardline Blocklist ---

// hardlinePatterns are immutable compiled-in patterns that block destructive
// commands regardless of tier or user configuration. They cannot be overridden.
// Patterns are matched case-insensitively against the normalized command string.
var hardlinePatterns = []struct {
	pattern string
	isRegex bool
}{
	// rm -rf / (any variant including --no-preserve-root) — must target root, not subdirs
	{pattern: `rm\s+-rf\s+(--no-preserve-root\s+)?/(\s|$)`, isRegex: true},
	// Fork bombs
	{pattern: `:\(\)\s*\{`, isRegex: true},
	// mkfs on any device
	{pattern: "mkfs.", isRegex: false},
	// dd writing to block devices
	{pattern: `dd\s+if=.+\s+of=/dev/sd`, isRegex: true},
	{pattern: `dd\s+if=.+\s+of=/dev/nvme`, isRegex: true},
	{pattern: `dd\s+if=.+\s+of=/dev/hd`, isRegex: true},
	{pattern: `dd\s+if=.+\s+of=/dev/xvd`, isRegex: true},
	// curl/wget piped to shell at root level (no directory prefix)
	{pattern: `(curl|wget)\s+.+\s*\|\s*(ba)?sh`, isRegex: true},
}

var hardlineRegexps []*regexp.Regexp

func init() {
	for _, hp := range hardlinePatterns {
		if hp.isRegex {
			re, err := regexp.Compile("(?i)" + hp.pattern)
			if err == nil {
				hardlineRegexps = append(hardlineRegexps, re)
			}
		}
	}
}

// IsHardlineBlocked checks whether a command string matches any hardline blocklist pattern.
// Returns true and the matched reason if blocked, false otherwise.
// The cmd argument should be the full shell command string (e.g., "rm -rf /tmp").
func IsHardlineBlocked(cmd string) (bool, string) {
	if cmd == "" {
		return false, ""
	}
	normalized := strings.TrimSpace(strings.ToLower(cmd))

	// Check literal patterns first
	for _, hp := range hardlinePatterns {
		if hp.isRegex {
			continue
		}
		if strings.Contains(normalized, hp.pattern) {
			return true, "hardline blocklist: matches destructive pattern"
		}
	}

	// Check regex patterns
	for _, re := range hardlineRegexps {
		if re.MatchString(normalized) {
			return true, "hardline blocklist: matches destructive pattern"
		}
	}

	return false, ""
}

// --- PolicyGuard Evaluation ---

// Evaluate runs the full policy evaluation chain for a tool call.
//
// Evaluation order:
//  1. Hardline blocklist check — unconditional denial
//  2. User deny rules check — glob/contains match on command args
//  3. Path-based rules check — glob match on filesystem path arguments
//  4. Tier check — does the current tier allow the tool's classification?
//  5. Per-tool override lookup — allow/deny/skip/ask
//
// Returns a PolicyResult with the outcome and suggested escalation action.
func (pg *PolicyGuard) Evaluate(toolName string, args map[string]interface{}) PolicyResult {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	// Extract command string from args for shell_exec and similar tools
	cmd := extractCmd(args)

	// Step 1: Hardline blocklist
	if blocked, reason := IsHardlineBlocked(cmd); blocked {
		return pg.resultWithEscalation(PolicyResult{
			Allowed:     false,
			Reason:      "hardline",
			BlockedTool: toolName,
		}, toolName, reason)
	}

	// Step 2: User deny rules (from merged config)
	if len(pg.storeDenyRules()) > 0 {
		if denied, rule := IsDeniedByRules(cmd, pg.storeDenyRules()); denied {
			return pg.resultWithEscalation(PolicyResult{
				Allowed:     false,
				Reason:      "deny_rule",
				BlockedTool: toolName,
			}, toolName, fmt.Sprintf("deny rule %q", rule))
		}
	}

	// Step 3: Path-based rules
	if blocked, reason := pg.isBlockedByPathRules(args); blocked {
		return pg.resultWithEscalation(PolicyResult{
			Allowed:     false,
			Reason:      "path_rule",
			BlockedTool: toolName,
		}, toolName, reason)
	}

	// Step 4: Tier check
	requiredTier := ClassifyTool(toolName)
	if !TierAllows(pg.tier, requiredTier) {
		// Tier blocks, but check if an override allows it
		ov, hasOverride := pg.overrides[toolName]
		if hasOverride && ov == OverrideAllow {
			return PolicyResult{Allowed: true}
		}
		if hasOverride && ov == OverrideDeny {
			return pg.resultWithEscalation(PolicyResult{
				Allowed:     false,
				Reason:      "override_deny",
				BlockedTool: toolName,
			}, toolName, fmt.Sprintf("tool %s is denied by override", toolName))
		}
		return pg.resultWithEscalation(PolicyResult{
			Allowed:     false,
			Reason:      "tier_block",
			BlockedTool: toolName,
		}, toolName, fmt.Sprintf("tool %s requires %s tier (current: %s)", toolName, requiredTier, pg.tier))
	}

	// Step 5: Per-tool override lookup (tier allows, but check override)
	ov, hasOverride := pg.overrides[toolName]
	if hasOverride {
		switch ov {
		case OverrideDeny, OverrideSkip:
			return pg.resultWithEscalation(PolicyResult{
				Allowed:     false,
				Reason:      "override_deny",
				BlockedTool: toolName,
			}, toolName, fmt.Sprintf("tool %s denied by override", toolName))
		case OverrideAskOnce:
			if pg.session == nil || !pg.session.IsOnceApproved(toolName) {
				return PolicyResult{
					Allowed:         false,
					Reason:          "ask_once",
					SuggestedAction: "ask_user",
					BlockedTool:     toolName,
				}
			}
		case OverrideAskSession:
			if pg.session == nil || !pg.session.IsSessionApproved(toolName) {
				return PolicyResult{
					Allowed:         false,
					Reason:          "ask_session",
					SuggestedAction: "ask_user",
					BlockedTool:     toolName,
				}
			}
		case OverrideAskAlways:
			return PolicyResult{
				Allowed:         false,
				Reason:          "ask_always",
				SuggestedAction: "ask_user",
				BlockedTool:     toolName,
			}
		case OverrideAudit:
			// Allow but log — return allowed
		}
	}

	// Default: tier allows, no blocking override
	return PolicyResult{Allowed: true}
}

// resultWithEscalation applies the smart escalation chain to a denied result.
func (pg *PolicyGuard) resultWithEscalation(result PolicyResult, toolName string, msg string) PolicyResult {
	// Escalation: check if skippable
	if isSkippable(toolName) {
		result.SuggestedAction = "skip"
		result.Reason += ": " + msg
		return result
	}
	// Escalation: check for alternative tool
	if alt, ok := alternativeTools[toolName]; ok {
		result.SuggestedAction = "alternative"
		result.Reason += " (alternative: " + alt + "): " + msg
		return result
	}
	// Final: block and notify
	result.SuggestedAction = "ask_user"
	result.Reason += ": " + msg
	return result
}

// storeDenyRules returns merged deny rules from the store (global + project).
func (pg *PolicyGuard) storeDenyRules() []string {
	if pg.store == nil {
		return nil
	}
	global, _ := pg.store.LoadGlobal()
	project, _ := pg.store.LoadProject("")
	return mergeDenyRules(global, project)
}

// mergeDenyRules combines deny rules from global and project configs.
// Rules are ordered global-first, project-second.
func mergeDenyRules(global, project *PolicyConfig) []string {
	var rules []string
	if global != nil {
		rules = append(rules, global.DenyRules...)
	}
	if project != nil {
		rules = append(rules, project.DenyRules...)
	}
	return rules
}

// --- Smart Escalation Chain ---

// skippableTools maps tools that can be safely skipped without breaking the agent.
// These are typically read/cosmetic tools.
var skippableTools = map[string]bool{
	"read":           true,
	"glob":           true,
	"grep":           true,
	"file_read":      true,
	"file_list":      true,
	"file_info":      true,
	"list_dir":       true,
	"git_status":     true,
	"git_log":        true,
	"git_diff":       true,
	"webfetch":       true,
	"mem_search":     true,
	"mem_context":    true,
	"mem_get":        true,
	"codegraph_explore": true,
}

// alternativeTools maps blocked tools to safer fallback alternatives.
// For example, "shell_exec:rm" → "shell_exec:echo" (safe paths only).
var alternativeTools = map[string]string{
	"file_write":    "file_read",
	"git_commit":    "git_diff",
	"git_push":      "git_status",
	"git_branch":    "git_status",
	"git_checkout":  "git_status",
	"git_add":       "git_status",
}

func isSkippable(toolName string) bool {
	return skippableTools[toolName]
}

// Escalate is the public entry point for smart escalation logic.
// Given a denied PolicyResult and toolName, it applies the escalation chain
// and returns the suggested action. Callers use this to decide how to handle
// a denied tool call.
func (pg *PolicyGuard) Escalate(result PolicyResult, toolName string) PolicyResult {
	if result.Allowed {
		return result
	}
	return pg.resultWithEscalation(result, toolName, result.Reason)
}

// --- Path-Based Rules ---

// isBlockedByPathRules checks tool arguments for filesystem paths and evaluates
// them against configured PathRules. Returns (blocked, reason).
func (pg *PolicyGuard) isBlockedByPathRules(args map[string]interface{}) (bool, string) {
	// Collect path rules from the guard config
	pathRules := pg.configPathRules()
	if len(pathRules) == 0 {
		return false, ""
	}

	// Extract path-like values from args
	paths := extractPaths(args)
	for _, p := range paths {
		for _, rule := range pathRules {
			if matchGlob(rule.Pattern, p) {
				if rule.Policy == OverrideDeny || rule.Policy == OverrideSkip {
					return true, fmt.Sprintf("path %q blocked by rule %q", p, rule.Pattern)
				}
				if rule.Policy == OverrideAllow {
					return false, "" // explicit allow overrides deny
				}
			}
		}
	}
	return false, ""
}

// configPathRules returns path rules from the guard's config.
func (pg *PolicyGuard) configPathRules() []PathRule {
	if pg.store != nil {
		global, _ := pg.store.LoadGlobal()
		if global != nil {
			return global.PathRules
		}
	}
	return nil
}

// extractPaths pulls filesystem-path-like values from tool arguments.
// It looks for common path keys: "path", "file", "filePath", "target", "source", "dest".
func extractPaths(args map[string]interface{}) []string {
	var paths []string
	pathKeys := []string{"path", "file", "filePath", "target", "source", "dest", "dir", "directory"}
	for _, key := range pathKeys {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
	}
	// Also check "arguments" sub-map for shell_exec-style tools
	if argsMap, ok := args["arguments"].(map[string]interface{}); ok {
		for _, key := range pathKeys {
			if v, ok := argsMap[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					paths = append(paths, s)
				}
			}
		}
	}
	return paths
}

// matchGlob performs simple glob matching for filesystem paths.
// Supports * (single directory segment) and ** (any depth).
func matchGlob(pattern, path string) bool {
	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Fast path: exact match
	if pattern == path {
		return true
	}

	// "**" matches any depth
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if prefix == "" {
				return strings.HasSuffix(path, suffix)
			}
			if suffix == "" {
				return strings.HasPrefix(path, prefix)
			}
			return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
		}
	}

	// Simple * wildcard (single segment)
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// extractCmd extracts a command string from tool arguments.
// For shell_exec, this is the "command" field. For other tools,
// it extracts a representative string for deny rule matching.
func extractCmd(args map[string]interface{}) string {
	if cmd, ok := args["command"]; ok {
		if s, ok := cmd.(string); ok {
			return s
		}
	}
	// For file_write and similar: use the path as the "command"
	if path, ok := args["path"]; ok {
		if s, ok := path.(string); ok {
			return s
		}
	}
	if path, ok := args["file"]; ok {
		if s, ok := path.(string); ok {
			return s
		}
	}
	if path, ok := args["filePath"]; ok {
		if s, ok := path.(string); ok {
			return s
		}
	}
	return ""
}
