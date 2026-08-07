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
	mu       sync.RWMutex
	mode     SessionMode
	brain    *Brain
	repo     interface{ SetSessionID(string) } // *db.SQLiteRepo or similar
	creator  func(ctx context.Context, name string) (string, error) // repo.CreateSession wrapper

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
	if mode == SessionUnify {
		// In unify mode, clear platform sessions — everything goes to unifiedID
		sm.platforms = make(map[string]*platformSession)
	} else if mode == SessionIsolate {
		// In isolate mode, clear unified — each platform will get its own
		sm.unifiedID = ""
	}
}

// Route directs a message from a platform to the correct session.
// It sets the repo's active session and optionally prefixes the message.
func (sm *SessionManager) Route(ctx context.Context, platform string, content string, senderName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var sessID string

	switch sm.mode {
	case SessionUnify:
		if sm.unifiedID == "" {
			id, err := sm.creator(ctx, fmt.Sprintf("unified-%s", platform))
			if err != nil {
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
				return fmt.Errorf("create unified session: %w", err)
			}
			sm.unifiedID = id
			sessID = sm.unifiedID
		} else if sm.lastPlatform != "" && sm.lastPlatform != platform {
			sessID = sm.unifiedID
			if sm.repo != nil {
				sm.repo.SetSessionID(sessID)
			}
			sm.brain.currentSessionID = sessID

			var prompt strings.Builder
			prompt.WriteString(fmt.Sprintf("📱 You switched from **%s** to **%s**.\n\n", sm.lastPlatform, platform))
			prompt.WriteString(fmt.Sprintf("Your message: \"%s\"\n\n", content))
			prompt.WriteString("\nHow should I handle this?\n\n")
			prompt.WriteString("  • Reply **unify** to continue the current session\n")
			prompt.WriteString("  • Reply **isolate** to start a fresh conversation here\n")
			prompt.WriteString("  • Reply **always unify** to stop asking and always merge\n")
			prompt.WriteString("  • Reply **always isolate** to stop asking and keep separate\n\n")
			prompt.WriteString("Waiting for your choice...")

			promptMsg := domain.Message{Role: domain.RoleSystem, Content: prompt.String()}
			sm.brain.repo.SaveMessage(ctx, promptMsg)
			sm.brain.ui.Display(promptMsg)

			sm.pendingMessage = &pendingMessage{
				Platform:   platform,
				Content:    content,
				SenderName: senderName,
				SessID:     sessID,
			}
			return nil
		} else {
			sessID = sm.unifiedID
		}
	}

	if sm.repo != nil {
		sm.repo.SetSessionID(sessID)
	}
	sm.brain.currentSessionID = sessID
	sm.lastPlatform = platform

	// Set platform-specific policy tier if configured
	sm.applyPlatformTier(platform)

	if sm.pendingMessage != nil {
		return sm.handleAskResponse(ctx, platform, content)
	}

	prefixed := content
	if senderName != "" {
		prefixed = fmt.Sprintf("[%s] %s: %s", platform, senderName, content)
	} else {
		prefixed = fmt.Sprintf("[%s] %s", platform, content)
	}
	return sm.brain.ProcessMessage(ctx, prefixed)
}

func (sm *SessionManager) handleAskResponse(ctx context.Context, platform string, choice string) error {
	if sm.pendingMessage == nil {
		return nil
	}
	pending := sm.pendingMessage
	sm.pendingMessage = nil

	normalized := strings.ToLower(strings.TrimSpace(choice))

	switch {
	case strings.Contains(normalized, "always unify"):
		sm.mode = SessionUnify
		fallthrough
	case strings.Contains(normalized, "unify"):
		if sm.repo != nil {
			sm.repo.SetSessionID(pending.SessID)
		}
		sm.brain.currentSessionID = pending.SessID
		prefixed := fmt.Sprintf("[%s] %s", pending.Platform, pending.Content)
		if pending.SenderName != "" {
			prefixed = fmt.Sprintf("[%s] %s: %s", pending.Platform, pending.SenderName, pending.Content)
		}
		info := domain.Message{Role: domain.RoleSystem, Content: "✅ Continuing in the unified session."}
		sm.brain.repo.SaveMessage(ctx, info)
		sm.brain.ui.Display(info)
		return sm.brain.ProcessMessage(ctx, prefixed)

	case strings.Contains(normalized, "always isolate"):
		sm.mode = SessionIsolate
		sm.unifiedID = ""
		fallthrough
	case strings.Contains(normalized, "isolate") || strings.Contains(normalized, "separate") || strings.Contains(normalized, "nuev"):
		id, err := sm.creator(ctx, fmt.Sprintf("platform-%s", pending.Platform))
		if err != nil {
			return fmt.Errorf("create platform session: %w", err)
		}
		sm.platforms[pending.Platform] = &platformSession{
			SessionID: id, CreatedAt: time.Now(), Name: pending.Platform,
		}
		if sm.repo != nil {
			sm.repo.SetSessionID(id)
		}
		sm.brain.currentSessionID = id
		info := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("✅ New session created for %s. Messages here won't affect other platforms.", pending.Platform)}
		sm.brain.repo.SaveMessage(ctx, info)
		sm.brain.ui.Display(info)
		return sm.brain.ProcessMessage(ctx, pending.Content)

	default:
		retry := domain.Message{Role: domain.RoleSystem, Content: "I didn't understand that choice. Please reply with **unify**, **isolate**, **always unify**, or **always isolate**."}
		sm.brain.repo.SaveMessage(ctx, retry)
		sm.brain.ui.Display(retry)
		sm.pendingMessage = pending
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
			sb.WriteString(fmt.Sprintf("Unified session: %s\n", sm.unifiedID[:12]))
		} else {
			sb.WriteString("No active unified session.\n")
		}
	case SessionIsolate:
		sb.WriteString("Platform sessions:\n")
		if len(sm.platforms) == 0 {
			sb.WriteString("  (none yet)\n")
		}
		for platform, ps := range sm.platforms {
			sb.WriteString(fmt.Sprintf("  %s → %s (since %s)\n", platform, ps.SessionID[:12], ps.CreatedAt.Format("15:04")))
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
	case SessionUnify:
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
	if sm.brain == nil || sm.brain.policy == nil {
		return
	}
	if sm.platformTiers != nil {
		if tier, ok := sm.platformTiers[platform]; ok {
			sm.brain.policy.SetTier(tier)
			return
		}
	}
}

