// Package memory provides the Engram namespace wrapper for subagent memory isolation.
// Each subagent gets its own namespace: gaia/{subagent}/{project}
// Shared cross-domain concepts use: gaia/shared/{project}
package memory

import (
	"fmt"
)

// NamespaceManager generates Engram topic_key prefixes for subagent memory isolation.
// The wrapper enforces the namespace format, prepending the prefix automatically
// so subagents cannot self-report into another subagent's namespace.
type NamespaceManager struct {
	project string
}

// NewNamespaceManager creates a namespace manager for the given project.
// project is typically the project name from the working directory.
func NewNamespaceManager(project string) *NamespaceManager {
	if project == "" {
		project = "default"
	}
	return &NamespaceManager{project: project}
}

// SubagentPrefix returns the topic_key prefix for a specific subagent.
// Example: SubagentPrefix("explorer") → "gaia/explorer/myproject"
func (n *NamespaceManager) SubagentPrefix(name string) string {
	return fmt.Sprintf("gaia/%s/%s", name, n.project)
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

// TopicKey builds a fully-qualified topic key for a subagent.
// Example: TopicKey("explorer", "architecture-patterns") → "gaia/explorer/myproject/architecture-patterns"
func (n *NamespaceManager) TopicKey(subagent, topic string) string {
	return fmt.Sprintf("%s/%s", n.SubagentPrefix(subagent), topic)
}

// SaveInstructions returns prompt text for subagents explaining how to use
// their scoped Engram namespace. Injected into subagent system prompts.
func (n *NamespaceManager) SaveInstructions(name string) string {
	prefix := n.SubagentPrefix(name)
	shared := n.SharedPrefix()
	return fmt.Sprintf(`MEMORY (ENGRAM) INSTRUCTIONS:
- Your namespace: "%s"
- When saving to Engram memory (mem_save), use topic_key prefix: "%s/{topic}"
- When searching your memory, include your namespace prefix
- Shared knowledge graph: "%s" — you may READ from it but MUST NOT write
- Example save: mem_save(title: "...", topic_key: "%s/pattern-discovered", type: "discovery", content: "...")
- Example search: mem_search(query: "deployment patterns", project: "%s")`, prefix, prefix, shared, prefix, n.project)
}

// SearchInstructions returns prompt text for memory retrieval within the subagent's scope.
func (n *NamespaceManager) SearchInstructions(name string) string {
	prefix := n.SubagentPrefix(name)
	shared := n.SharedPrefix()
	return fmt.Sprintf(`MEMORY SEARCH:
- Search your scope first with topic_key prefix: "%s"
- Then search the shared graph: "%s"
- Use mem_get_observation(id) for full content after search returns previews`, prefix, shared)
}

// DynamicPrefix returns the namespace prefix for a dynamically-created subagent.
// Format: gaia/subagent/{name}/ (matches SubagentPrefix pattern for memory isolation).
func (n *NamespaceManager) DynamicPrefix(name string) string {
	return fmt.Sprintf("gaia/subagent/%s/%s", name, n.project)
}
