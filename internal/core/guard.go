package core

import (
	"sync"

	"gaia/internal/core/domain"
)

// ConfirmGuard gates tool execution based on trust mode.
// Modes: always (prompt every time), per-session (approve once per tool),
// per-action (prompt every invocation), never (no prompts).
type ConfirmGuard struct {
	mu       sync.RWMutex
	mode     domain.TrustMode
	approved map[string]bool // per-session tool approvals
	headless bool
}

// NewConfirmGuard creates a guard with the given mode.
func NewConfirmGuard(mode domain.TrustMode, headless bool) *ConfirmGuard {
	if mode == "" {
		mode = domain.TrustAlways
	}
	if headless {
		mode = domain.TrustNever
	}
	return &ConfirmGuard{
		mode:     mode,
		approved: make(map[string]bool),
		headless: headless,
	}
}

// ShouldConfirm returns true if the tool requires user confirmation.
func (g *ConfirmGuard) ShouldConfirm(toolName string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	switch g.mode {
	case domain.TrustNever:
		return false
	case domain.TrustAlways:
		return true
	case domain.TrustPerSession:
		return !g.approved[toolName]
	case domain.TrustPerAction:
		return true
	default:
		return true
	}
}

// Approve marks a tool as approved for per-session mode.
func (g *ConfirmGuard) Approve(toolName string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.approved[toolName] = true
}

// SetMode changes the trust mode and clears session approvals.
func (g *ConfirmGuard) SetMode(mode domain.TrustMode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.mode = mode
	g.approved = make(map[string]bool)
	if g.headless {
		g.mode = domain.TrustNever
	}
}

// Mode returns the current trust mode.
func (g *ConfirmGuard) Mode() domain.TrustMode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.mode
}

// Reset clears all per-session approvals.
func (g *ConfirmGuard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.approved = make(map[string]bool)
}
