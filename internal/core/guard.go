package core

import (
	"context"
	"sync"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// Deprecated: replaced by PolicyGuard.
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

// GuardRequest identifies one policy evaluation subject.
type GuardRequest struct {
	Backend  string // local, docker, ssh
	Category string // commands, files, network, execution
	Subject  string // command string, file path, host, etc.
}

// Scope resolves an ask decision: what an approval means over time.
type Scope string

const (
	ScopeOnce    Scope = "once"
	ScopeSession Scope = "session"
	ScopeAlways  Scope = "always"
	ScopeDeny    Scope = "deny"
)

// Approver resolves an ask decision interactively or policy-based.
type Approver func(ctx context.Context, req GuardRequest) Scope

// Aliases for core ports and types used across gateways and adapters.
type Transcriber = ports.Transcriber
type Synthesizer = ports.Synthesizer
type Attachment = domain.Attachment
type Role = domain.Role

const (
	RoleAssistant = domain.RoleAssistant
	RoleUser      = domain.RoleUser
)

type Repository = ports.Repository
type Message = domain.Message

const (
	MimeTypeMP3  = ports.MimeTypeMP3
	MimeTypeOpus = ports.MimeTypeOpus
	MimeTypeOGG  = ports.MimeTypeOGG
	MimeTypeAAC  = ports.MimeTypeAAC
	MimeTypeWAV  = ports.MimeTypeWAV
)
