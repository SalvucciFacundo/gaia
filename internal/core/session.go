package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
)

// SessionMode controls how messages are routed across platforms.
type SessionMode string

const (
	SessionUnify   SessionMode = "unify"   // All platforms share one session
	SessionIsolate SessionMode = "isolate" // Each platform has its own session
	SessionAsk     SessionMode = "ask"     // Prompt user when switching platforms
)

// platformSession tracks a platform's session state.
type platformSession struct {
	SessionID string
	CreatedAt time.Time
	Name      string
}

// SessionManager routes messages from multiple platforms to the correct session.
// It supports three modes: unify (default), isolate, and ask.
type SessionManager struct {
	mu      sync.RWMutex
	mode    SessionMode
	brain   *Brain
	repo    interface{ SetSessionID(string) }                      // *db.SQLiteRepo or similar
	creator func(ctx context.Context, name string) (string, error) // repo.CreateSession wrapper

	// Tracks the unified session ID (for unify/ask modes)
	unifiedID string

	// Tracks per-platform sessions (for isolate mode)
	platforms map[string]*platformSession

	// Last platform that sent a message (for ask mode)
	lastPlatform string

	// Pending message waiting for user's mode choice (ask mode)
	pendingMessage *pendingMessage

	// Per-platform policy tier overrides (from phase 4)
	platformTiers map[string]PolicyTier
}

// pendingMessage holds a message that's waiting for the user to choose a session mode.
type pendingMessage struct {
	Platform   string
	Content    string
	SenderName string
	SessID     string
}

// NewSessionManager creates a session manager with the given mode.
func NewSessionManager(mode SessionMode, brain *Brain, repo interface{ SetSessionID(string) }) *SessionManager {
	if mode == "" {
		mode = SessionUnify
	}
	return &SessionManager{
		mode:      mode,
		brain:     brain,
		repo:      repo,
		platforms: make(map[string]*platformSession),
		creator: func(ctx context.Context, name string) (string, error) {
			// Default creator: generates a session ID with timestamp
			id := fmt.Sprintf("sess-%s-%d", strings.ReplaceAll(name, " ", "-"), time.Now().UnixNano())
			return id, nil
		},
	}
}

// SetSessionCreator allows injecting a custom session creation function.
// Used by main.go to wire in the actual SQLite repo's CreateSession.
func (sm *SessionManager) SetSessionCreator(fn func(ctx context.Context, name string) (string, error)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.creator = fn
}

// Mode returns the current session mode.
func (sm *SessionManager) Mode() SessionMode {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.mode
}

// SetMode changes the session mode and resets platform tracking if switching to isolate.
func (sm *SessionManager) SetMode(mode SessionMode) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mode = mode
	switch mode {
	case SessionUnify:
		// In unify mode, clear platform sessions — everything goes to unifiedID
		sm.platforms = make(map[string]*platformSession)
	case SessionIsolate:
		// In isolate mode, clear unified — each platform will get its own
		sm.unifiedID = ""
	}
}

// SetPlatformTier sets a policy tier override for a specific platform.
func (sm *SessionManager) SetPlatformTier(platform string, tier PolicyTier) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.platformTiers == nil {
		sm.platformTiers = make(map[string]PolicyTier)
	}
	sm.platformTiers[platform] = tier
}

// SetPlatformTiers sets multiple policy tier overrides at once.
func (sm *SessionManager) SetPlatformTiers(tiers map[string]PolicyTier) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.platformTiers = make(map[string]PolicyTier, len(tiers))
	for k, v := range tiers {
		sm.platformTiers[k] = v
	}
}

func shortSessionID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// Route directs a message from a platform to the correct session.
// It sets the repo's active session and optionally prefixes the message.
func (sm *SessionManager) Route(ctx context.Context, platform string, content string, senderName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	sm.mu.Lock()

	// If there's a pending message awaiting user choice in ask mode:
	if sm.pendingMessage != nil {
		return sm.handleAskResponse(ctx, platform, content)
	}

	var sessID string

	switch sm.mode {
	case SessionUnify:
		if sm.unifiedID == "" {
			id, err := sm.creator(ctx, fmt.Sprintf("unified-%s", platform))
			if err != nil {
				sm.mu.Unlock()
				return fmt.Errorf("create unified session: %w", err)
			}
			sm.unifiedID = id
		}
		sessID = sm.unifiedID

	case SessionIsolate:
		ps, ok := sm.platforms[platform]
		if !ok {
			id, err := sm.creator(ctx, fmt.Sprintf("platform-%s", platform))
			if err != nil {
				sm.mu.Unlock()
				return fmt.Errorf("create platform session: %w", err)
			}
			ps = &platformSession{SessionID: id, CreatedAt: time.Now(), Name: platform}
			sm.platforms[platform] = ps
		}
		sessID = ps.SessionID

	case SessionAsk:
		if sm.unifiedID == "" {
			id, err := sm.creator(ctx, "unified")
			if err != nil {
				sm.mu.Unlock()
				return fmt.Errorf("create unified session: %w", err)
			}
			sm.unifiedID = id
			sessID = sm.unifiedID
		} else if sm.lastPlatform != "" && sm.lastPlatform != platform {
			sessID = sm.unifiedID
			if sm.repo != nil {
				sm.repo.SetSessionID(sessID)
			}
			if sm.brain != nil {
				sm.brain.SetActiveConversation(sessID)
			}

			prompt := fmt.Sprintf("📱 You switched from **%s** to **%s**.\n\n"+
				"Your message: \"%s\"\n\n"+
				"How should I handle this?\n\n"+
				"  • Reply **unify** to continue the current session\n"+
				"  • Reply **isolate** to start a fresh conversation here\n"+
				"  • Reply **always unify** to stop asking and always merge\n"+
				"  • Reply **always isolate** to stop asking and keep separate\n\n"+
				"Waiting for your choice...", sm.lastPlatform, platform, content)

			sm.pendingMessage = &pendingMessage{
				Platform:   platform,
				Content:    content,
				SenderName: senderName,
				SessID:     sessID,
			}
			brain := sm.brain
			sm.mu.Unlock()

			if brain != nil {
				promptMsg := domain.Message{Role: domain.RoleSystem, Content: prompt}
				if brain.repo != nil {
					_ = brain.repo.SaveMessage(ctx, promptMsg)
				}
				if brain.ui != nil {
					_ = brain.ui.Display(promptMsg)
				}
			}
			return nil
		} else {
			sessID = sm.unifiedID
		}
	}

	if sm.repo != nil {
		sm.repo.SetSessionID(sessID)
	}
	if sm.brain != nil {
		sm.brain.SetActiveConversation(sessID)
	}
	sm.lastPlatform = platform

	// Set platform-specific policy tier if configured
	sm.applyPlatformTier(platform)

	prefixed := content
	if senderName != "" {
		prefixed = fmt.Sprintf("[%s] %s: %s", platform, senderName, content)
	} else {
		prefixed = fmt.Sprintf("[%s] %s", platform, content)
	}

	brain := sm.brain
	sm.mu.Unlock()

	if brain != nil {
		return brain.ProcessMessage(ctx, prefixed)
	}
	return nil
}

func (sm *SessionManager) handleAskResponse(ctx context.Context, platform string, choice string) error {
	pending := sm.pendingMessage
	if pending == nil {
		sm.mu.Unlock()
		return nil
	}

	normalized := strings.ToLower(strings.TrimSpace(choice))

	switch {
	case strings.Contains(normalized, "always unify"):
		sm.mode = SessionUnify
		fallthrough
	case strings.Contains(normalized, "unify"):
		sm.pendingMessage = nil
		sessID := pending.SessID
		if sm.repo != nil {
			sm.repo.SetSessionID(sessID)
		}
		if sm.brain != nil {
			sm.brain.SetActiveConversation(sessID)
		}
		sm.lastPlatform = pending.Platform
		sm.applyPlatformTier(pending.Platform)

		prefixed := fmt.Sprintf("[%s] %s", pending.Platform, pending.Content)
		if pending.SenderName != "" {
			prefixed = fmt.Sprintf("[%s] %s: %s", pending.Platform, pending.SenderName, pending.Content)
		}

		brain := sm.brain
		sm.mu.Unlock()

		if brain != nil {
			info := domain.Message{Role: domain.RoleSystem, Content: "✅ Continuing in the unified session."}
			if brain.repo != nil {
				_ = brain.repo.SaveMessage(ctx, info)
			}
			if brain.ui != nil {
				_ = brain.ui.Display(info)
			}
			return brain.ProcessMessage(ctx, prefixed)
		}
		return nil

	case strings.Contains(normalized, "always isolate"):
		sm.mode = SessionIsolate
		sm.unifiedID = ""
		fallthrough
	case strings.Contains(normalized, "isolate") || strings.Contains(normalized, "separate") || strings.Contains(normalized, "nuev"):
		sm.pendingMessage = nil
		id, err := sm.creator(ctx, fmt.Sprintf("platform-%s", pending.Platform))
		if err != nil {
			sm.mu.Unlock()
			return fmt.Errorf("create platform session: %w", err)
		}
		sm.platforms[pending.Platform] = &platformSession{
			SessionID: id, CreatedAt: time.Now(), Name: pending.Platform,
		}
		if sm.repo != nil {
			sm.repo.SetSessionID(id)
		}
		if sm.brain != nil {
			sm.brain.SetActiveConversation(id)
		}
		sm.lastPlatform = pending.Platform
		sm.applyPlatformTier(pending.Platform)

		brain := sm.brain
		sm.mu.Unlock()

		if brain != nil {
			info := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("✅ New session created for %s. Messages here won't affect other platforms.", pending.Platform)}
			if brain.repo != nil {
				_ = brain.repo.SaveMessage(ctx, info)
			}
			if brain.ui != nil {
				_ = brain.ui.Display(info)
			}
			return brain.ProcessMessage(ctx, pending.Content)
		}
		return nil

	default:
		brain := sm.brain
		sm.mu.Unlock()

		if brain != nil {
			retry := domain.Message{Role: domain.RoleSystem, Content: "I didn't understand that choice. Please reply with **unify**, **isolate**, **always unify**, or **always isolate**."}
			if brain.repo != nil {
				_ = brain.repo.SaveMessage(ctx, retry)
			}
			if brain.ui != nil {
				_ = brain.ui.Display(retry)
			}
		}
		return nil
	}
}

// Status returns a human-readable summary of the current session state.
func (sm *SessionManager) Status() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session mode: %s\n\n", sm.mode))

	switch sm.mode {
	case SessionUnify:
		if sm.unifiedID != "" {
			sb.WriteString(fmt.Sprintf("Unified session: %s\n", shortSessionID(sm.unifiedID)))
		} else {
			sb.WriteString("No active unified session.\n")
		}
	case SessionIsolate:
		sb.WriteString("Platform sessions:\n")
		if len(sm.platforms) == 0 {
			sb.WriteString("  (none yet)\n")
		}
		for platform, ps := range sm.platforms {
			sb.WriteString(fmt.Sprintf("  %s → %s (since %s)\n", platform, shortSessionID(ps.SessionID), ps.CreatedAt.Format("15:04")))
		}
	case SessionAsk:
		if sm.unifiedID != "" {
			sb.WriteString(fmt.Sprintf("Unified session: %s\n", shortSessionID(sm.unifiedID)))
		} else {
			sb.WriteString("No active unified session.\n")
		}
		if sm.pendingMessage != nil {
			sb.WriteString(fmt.Sprintf("Pending switch from %s waiting for user choice.\n", sm.pendingMessage.Platform))
		}
	}

	sb.WriteString("\nCommands:\n")
	sb.WriteString("  /session        — Show this status\n")
	sb.WriteString("  /session unify  — All platforms share one session\n")
	sb.WriteString("  /session isolate— Each platform has its own session\n")
	sb.WriteString("  /session ask    — Ask when switching platforms\n")

	return sb.String()
}

// GetSessionID returns the active session ID for a given platform.
func (sm *SessionManager) GetSessionID(platform string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	switch sm.mode {
	case SessionUnify, SessionAsk:
		return sm.unifiedID
	case SessionIsolate:
		if ps, ok := sm.platforms[platform]; ok {
			return ps.SessionID
		}
	}
	return ""
}

// Ensure SessionManager satisfies a common interface if needed.
var _ interface{} = (*SessionManager)(nil)

// applyPlatformTier sets the policy tier based on the active platform.
func (sm *SessionManager) applyPlatformTier(platform string) {
	if sm.brain == nil || sm.brain.policy == nil || sm.platformTiers == nil {
		return
	}
	if tier, ok := sm.platformTiers[platform]; ok {
		sm.brain.policy.SetTier(tier)
	}
}

