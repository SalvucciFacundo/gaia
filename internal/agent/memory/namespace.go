// Package memory provides the Engram namespace wrapper for subagent memory isolation.
// Each subagent gets its own namespace: gaia/{subagent}/{project}
// Shared cross-domain concepts use: gaia/shared/{project}
package memory

import (
	"fmt"
	"strings"
)

// NamespaceManager generates Engram topic_key prefixes for subagent memory isolation.
// The wrapper enforces the namespace format, prepending the prefix automatically
// so subagents cannot self-report into another subagent's namespace.
type NamespaceManager struct {
	project   string
	sessionID string
}

// NewNamespaceManager creates a namespace manager for the given project.
// project is typically the project name from the working directory.
func NewNamespaceManager(project string) *NamespaceManager {
	if project == "" {
		project = "default"
	}
	return &NamespaceManager{project: project}
}

// SetSessionID sets the authoritative active runtime session identity.
func (n *NamespaceManager) SetSessionID(sessionID string) {
	n.sessionID = sessionID
}

// SessionID returns the authoritative active runtime session identity.
func (n *NamespaceManager) SessionID() string {
	return n.sessionID
}

// SubagentPrefix returns the topic_key prefix for a specific subagent.
// If the subagent name is "shared", it is safe-aliased to prevent collision with SharedPrefix().
// Example: SubagentPrefix("explorer") → "gaia/explorer/myproject"
func (n *NamespaceManager) SubagentPrefix(name string) string {
	cleanName := strings.ReplaceAll(name, "/", "-")
	if cleanName == "shared" {
		cleanName = "agent-shared"
	}
	return fmt.Sprintf("gaia/%s/%s", cleanName, n.project)
}

// SharedPrefix returns the topic_key prefix for cross-domain knowledge.
// The shared namespace is read-only for subagents.
// Example: SharedPrefix() → "gaia/shared/myproject"
func (n *NamespaceManager) SharedPrefix() string {
	return fmt.Sprintf("gaia/shared/%s", n.project)
}

// Project returns the project name.
func (n *NamespaceManager) Project() string {
	return n.project
}

// TopicKey builds a fully-qualified topic key for a subagent with sanitized path delimiters.
// Example: TopicKey("explorer", "architecture-patterns") → "gaia/explorer/myproject/architecture-patterns"
func (n *NamespaceManager) TopicKey(subagent, topic string) string {
	cleanTopic := strings.TrimPrefix(topic, "/")
	return fmt.Sprintf("%s/%s", n.SubagentPrefix(subagent), cleanTopic)
}

// SaveInstructions returns prompt text for subagents explaining how to use
// their scoped Engram namespace. Injected into subagent system prompts.
func (n *NamespaceManager) SaveInstructions(name string) string {
	prefix := n.SubagentPrefix(name)
	shared := n.SharedPrefix()
	sessionLine := ""
	if n.sessionID != "" {
		sessionLine = fmt.Sprintf("\n- Active session_id: \"%s\" (include when calling mem_save)", n.sessionID)
	}
	return fmt.Sprintf(`MEMORY (ENGRAM) INSTRUCTIONS:
- Your namespace: "%s"
- When saving to Engram memory (mem_save), use topic_key prefix: "%s/{topic}"%s
- Automated or system artifacts: set capture_prompt: false to prevent polluting user prompts
- Lifecycle states: "active" is verified knowledge; "needs_review" is stale context requiring verification
- If mem_save returns judgment_required, resolve candidates using mem_judge
- When searching your memory, include your namespace prefix
- Shared knowledge graph: "%s" — you may READ from it but MUST NOT write
- Example save: mem_save(title: "...", topic_key: "%s/pattern-discovered", type: "discovery", content: "...", capture_prompt: false)
- Example search: mem_search(query: "deployment patterns", project: "%s")`, prefix, prefix, sessionLine, shared, prefix, n.project)
}

// SearchInstructions returns prompt text for memory retrieval within the subagent's scope.
func (n *NamespaceManager) SearchInstructions(name string) string {
	prefix := n.SubagentPrefix(name)
	shared := n.SharedPrefix()
	return fmt.Sprintf(`MEMORY SEARCH:
- Search your scope first with topic_key prefix: "%s"
- Then search the shared graph: "%s"
- Use mem_get_observation(id) for full content after search returns previews
- Verify "needs_review" observations against source code before treating as authoritative`, prefix, shared)
}

// DynamicPrefix returns the namespace prefix for a dynamically-created subagent.
// Format: gaia/subagent/{name}/{project} (matches SubagentPrefix pattern for memory isolation).
func (n *NamespaceManager) DynamicPrefix(name string) string {
	cleanName := strings.ReplaceAll(name, "/", "-")
	return fmt.Sprintf("gaia/subagent/%s/%s", cleanName, n.project)
}

// DynamicSaveInstructions returns prompt text for dynamically created subagents,
// using DynamicPrefix to guarantee memory isolation.
func (n *NamespaceManager) DynamicSaveInstructions(name string) string {
	prefix := n.DynamicPrefix(name)
	shared := n.SharedPrefix()
	sessionLine := ""
	if n.sessionID != "" {
		sessionLine = fmt.Sprintf("\n- Active session_id: \"%s\" (include when calling mem_save)", n.sessionID)
	}
	return fmt.Sprintf(`MEMORY (ENGRAM) INSTRUCTIONS:
- Your namespace: "%s"
- When saving to Engram memory (mem_save), use topic_key prefix: "%s/{topic}"%s
- Automated or system artifacts: set capture_prompt: false to prevent polluting user prompts
- Lifecycle states: "active" is verified knowledge; "needs_review" is stale context requiring verification
- If mem_save returns judgment_required, resolve candidates using mem_judge
- When searching your memory, include your namespace prefix
- Shared knowledge graph: "%s" — you may READ from it but MUST NOT write
- Example save: mem_save(title: "...", topic_key: "%s/pattern-discovered", type: "discovery", content: "...", capture_prompt: false)
- Example search: mem_search(query: "deployment patterns", project: "%s")`, prefix, prefix, sessionLine, shared, prefix, n.project)
}

// ProfilePrefix returns the topic_key prefix for a specific profile persona.
// Example: ProfilePrefix("dev") → "gaia/profile/dev/myproject"
func (n *NamespaceManager) ProfilePrefix(profile string) string {
	cleanProfile := strings.ReplaceAll(profile, "/", "-")
	return fmt.Sprintf("gaia/profile/%s/%s", cleanProfile, n.project)
}

// ProfileTopicKey builds a fully-qualified topic key for a profile persona.
// Example: ProfileTopicKey("dev", "code-standards") → "gaia/profile/dev/myproject/code-standards"
func (n *NamespaceManager) ProfileTopicKey(profile, topic string) string {
	cleanTopic := strings.TrimPrefix(topic, "/")
	return fmt.Sprintf("%s/%s", n.ProfilePrefix(profile), cleanTopic)
}

// TopicKeyInfo describes the parsed components of a Gaia topic key.
type TopicKeyInfo struct {
	Domain  string // "profile", "subagent", "shared"
	Owner   string // profile name, subagent name, or "shared"
	Project string
	Topic   string
}

// ParseTopicKey parses a topic key into its constituent namespace components.
// Returns ok=false if the topic key does not adhere to the gaia/ prefix convention.
func (n *NamespaceManager) ParseTopicKey(key string) (info TopicKeyInfo, ok bool) {
	clean := strings.TrimPrefix(key, "/")
	parts := strings.Split(clean, "/")
	if len(parts) < 3 || parts[0] != "gaia" {
		return TopicKeyInfo{}, false
	}

	if parts[1] == "profile" && len(parts) >= 4 {
		return TopicKeyInfo{
			Domain:  "profile",
			Owner:   parts[2],
			Project: parts[3],
			Topic:   strings.Join(parts[4:], "/"),
		}, true
	}

	if parts[1] == "subagent" && len(parts) >= 4 {
		return TopicKeyInfo{
			Domain:  "subagent",
			Owner:   parts[2],
			Project: parts[3],
			Topic:   strings.Join(parts[4:], "/"),
		}, true
	}

	if parts[1] == "shared" {
		return TopicKeyInfo{
			Domain:  "shared",
			Owner:   "shared",
			Project: parts[2],
			Topic:   strings.Join(parts[3:], "/"),
		}, true
	}

	// Legacy subagent format: gaia/{subagent}/{project}/{topic...}
	return TopicKeyInfo{
		Domain:  "subagent",
		Owner:   parts[1],
		Project: parts[2],
		Topic:   strings.Join(parts[3:], "/"),
	}, true
}

// ValidateMemoryAccess evaluates whether a caller profile can access the given topic key.
// - Reads (isWrite = false) are always permitted across all namespaces and profiles.
// - Writes (isWrite = true) to the caller's own profile namespace are directly allowed.
// - Writes to any other profile or to the shared namespace require human confirmation.
// - If the target key does not follow the gaia namespace schema, it returns allowed=true without approval requirements.
func (n *NamespaceManager) ValidateMemoryAccess(callerProfile, targetTopicKey string, isWrite bool) (allowed bool, requiresHumanApproval bool, reason string) {
	if !isWrite {
		return true, false, ""
	}

	info, ok := n.ParseTopicKey(targetTopicKey)
	if !ok {
		return true, false, ""
	}

	// Writes to shared namespace require approval
	if info.Domain == "shared" {
		return false, true, fmt.Sprintf("profile '%s' cannot write to shared namespace without approval", callerProfile)
	}

	// If target is in profile domain
	if info.Domain == "profile" {
		if callerProfile != "" && info.Owner != callerProfile {
			return false, true, fmt.Sprintf("cross-profile memory mutation: profile '%s' attempting to write to profile '%s'", callerProfile, info.Owner)
		}
	}

	// If target is in subagent domain and caller profile doesn't match
	if info.Domain == "subagent" && callerProfile != "" && info.Owner != callerProfile {
		return false, true, fmt.Sprintf("cross-domain memory mutation: profile '%s' attempting to write to subagent '%s'", callerProfile, info.Owner)
	}

	return true, false, ""
}

// ProfileSaveInstructions returns prompt text for a persona profile explaining its scoped
// Engram namespace and cross-profile permissions.
func (n *NamespaceManager) ProfileSaveInstructions(profile string) string {
	prefix := n.ProfilePrefix(profile)
	shared := n.SharedPrefix()
	sessionLine := ""
	if n.sessionID != "" {
		sessionLine = fmt.Sprintf("\n- Active session_id: \"%s\" (include when calling mem_save)", n.sessionID)
	}
	return fmt.Sprintf(`PROFILE MEMORY (ENGRAM) INSTRUCTIONS:
- Your profile: "%s"
- Your dedicated namespace prefix: "%s"
- When saving profile memory (mem_save), use topic_key prefix: "%s/{topic}"%s
- Automated or system artifacts: set capture_prompt: false to prevent polluting user prompts
- Cross-Profile Collaboration: You can freely READ from other profiles or shared memory ("%s").
- Mutation Restriction: Writing or mutating another profile's namespace requires explicit human approval.
- Example save: mem_save(title: "...", topic_key: "%s/decision", type: "decision", content: "...")
- Example search: mem_search(query: "...", project: "%s")`, profile, prefix, prefix, sessionLine, shared, prefix, n.project)
}

// ProfileSearchInstructions returns search prompt text for persona profiles.
func (n *NamespaceManager) ProfileSearchInstructions(profile string) string {
	prefix := n.ProfilePrefix(profile)
	shared := n.SharedPrefix()
	return fmt.Sprintf(`PROFILE MEMORY SEARCH:
- Search your profile scope: "%s"
- Cross-profile retrieval: You may search any profile or the shared graph: "%s"
- Use mem_get_observation(id) for full content
- Verify "needs_review" observations before relying on them`, prefix, shared)
}

