package agent

import (
	"fmt"
	"sync"
)

// SubagentFactory is a constructor function that builds a Subagent.
// Factories receive a Spawner reference so subagents can delegate to
// other subagents if needed (composability).
type SubagentFactory func(spawner *Spawner) Subagent

// Registry maps subagent names to factory functions.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	agents  map[string]SubagentFactory
}

// NewRegistry creates an empty subagent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]SubagentFactory),
	}
}

// Register adds a subagent factory under the given name.
// It returns an error if the name is already registered.
func (r *Registry) Register(name string, factory SubagentFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[name]; ok {
		return fmt.Errorf("subagent %q already registered", name)
	}
	r.agents[name] = factory
	return nil
}

// Get retrieves the factory for the named subagent.
// Returns false if not found.
func (r *Registry) Get(name string) (SubagentFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.agents[name]
	return f, ok
}

// Available returns a sorted list of registered subagent names.
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Unregister removes a subagent factory from the registry.
// Returns an error if the subagent is not registered.
// This is used for dynamic subagent removal.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[name]; !ok {
		return fmt.Errorf("subagent %q is not registered", name)
	}
	delete(r.agents, name)
	return nil
}
