package llm

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"gaia/internal/core/domain"
	"gaia/internal/core/ports"
)

// credentialState tracks the runtime state of a single credential.
type credentialState struct {
	entry        domain.CredentialEntry
	provider     ports.LLMProvider
	cooldownUntil time.Time
	failCount    int
}

// CredentialPool wraps multiple credentials for a single provider with
// round-robin rotation, automatic failover on errors, and cooldown tracking.
// Implements ports.LLMProvider.
type CredentialPool struct {
	mu         sync.Mutex
	creds      []*credentialState
	nextIndex  int
	providerName string // e.g. "openai", "anthropic"
	constructor ProviderConstructor // creates a provider instance per credential
}

// NewCredentialPool creates a pool from credential entries and a provider constructor.
func NewCredentialPool(name string, entries []domain.CredentialEntry, ctor ProviderConstructor) (*CredentialPool, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("credential pool %q: no credentials provided", name)
	}

	pool := &CredentialPool{
		providerName: name,
		constructor:  ctor,
	}

	for _, entry := range entries {
		if entry.Key == "" {
			continue
		}
		// Set default cooldowns
		if entry.Cooldown429 <= 0 {
			entry.Cooldown429 = time.Hour
		}
		if entry.Cooldown401 <= 0 {
			entry.Cooldown401 = 5 * time.Minute
		}
		if entry.Cooldown402 <= 0 {
			entry.Cooldown402 = time.Hour
		}

		pool.creds = append(pool.creds, &credentialState{entry: entry})
	}

	if len(pool.creds) == 0 {
		return nil, fmt.Errorf("credential pool %q: no valid credentials", name)
	}

	return pool, nil
}

// Name returns the provider name.
func (p *CredentialPool) Name() string {
	return p.providerName
}

// Chat selects the next available credential and delegates the call.
// On credential-specific errors (429, 401, 402), marks the credential for
// cooldown and retries with the next available credential.
func (p *CredentialPool) Chat(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (*domain.Message, error) {
	provider, release, err := p.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	msg, err := provider.Chat(ctx, messages, opts...)
	if err != nil {
		if p.handleCredentialError(err) {
			// Retry with next credential
			return p.Chat(ctx, messages, opts...)
		}
	}
	return msg, err
}

// Stream selects the next available credential and delegates the streaming call.
func (p *CredentialPool) Stream(ctx context.Context, messages []domain.Message, opts ...ports.ChatOpt) (ports.TokenStream, error) {
	provider, release, err := p.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	stream, err := provider.Stream(ctx, messages, opts...)
	if err != nil {
		if p.handleCredentialError(err) {
			return p.Stream(ctx, messages, opts...)
		}
	}
	return stream, err
}

// Tools returns the tool definitions from the first available provider.
func (p *CredentialPool) Tools() []domain.ToolDef {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return tools from the first active credential
	for _, c := range p.creds {
		if c.provider != nil {
			return c.provider.Tools()
		}
	}
	// Initialize and return
	if len(p.creds) > 0 {
		p.lazyInitLocked(p.creds[0])
		return p.creds[0].provider.Tools()
	}
	return nil
}

// acquire picks the next non-cooldowned credential and ensures its provider is initialized.
// Returns a release function that must be called.
func (p *CredentialPool) acquire(ctx context.Context) (ports.LLMProvider, func(), error) {
	p.mu.Lock()

	// Find the next available credential (with retry through all)
	start := p.nextIndex
	for i := 0; i < len(p.creds); i++ {
		idx := (start + i) % len(p.creds)
		c := p.creds[idx]

		if time.Now().Before(c.cooldownUntil) {
			continue // still in cooldown
		}

		// Lazy init provider
		if c.provider == nil {
			if err := p.lazyInitLocked(c); err != nil {
				c.cooldownUntil = time.Now().Add(5 * time.Minute) // backoff on init failure
				continue
			}
		}

		p.nextIndex = (idx + 1) % len(p.creds)
		release := func() { p.mu.Unlock() }
		return c.provider, release, nil
	}

	p.mu.Unlock()
	return nil, nil, fmt.Errorf("credential pool %q: all credentials exhausted or in cooldown", p.providerName)
}

// lazyInitLocked initializes a credential's provider. Must hold p.mu.
func (p *CredentialPool) lazyInitLocked(c *credentialState) error {
	if c.provider != nil {
		return nil
	}

	// Build a temporary config with just this credential's key
	cfg := &domain.Config{
		APIKeys: map[string]string{p.providerName: c.entry.Key},
		LLM: struct {
			Provider      string   `yaml:"provider"`
			Model         string   `yaml:"model"`
			FallbackChain []string `yaml:"fallback_chain"`
			TrustMode     string   `yaml:"trust_mode"`
		}{
			Provider: p.providerName,
		},
	}

	prov, err := p.constructor(cfg)
	if err != nil {
		return fmt.Errorf("init credential for %q: %w", p.providerName, err)
	}
	c.provider = prov
	return nil
}

// handleCredentialError checks if the error is credential-specific and marks
// the current credential for cooldown. Returns true if retryable.
func (p *CredentialPool) handleCredentialError(err error) bool {
	errStr := err.Error()
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the credential that was just used (the one before nextIndex)
	usedIdx := (p.nextIndex - 1 + len(p.creds)) % len(p.creds)
	c := p.creds[usedIdx]
	c.failCount++

	var cooldown time.Duration
	switch {
	case strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests"):
		cooldown = c.entry.Cooldown429
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "invalid"):
		cooldown = c.entry.Cooldown401
	case strings.Contains(errStr, "402") || strings.Contains(errStr, "quota") || strings.Contains(errStr, "insufficient_quota"):
		cooldown = c.entry.Cooldown402
	default:
		return false // non-credential error
	}

	c.cooldownUntil = time.Now().Add(cooldown)

	// Check if there's any non-cooldowned credential left
	for _, cred := range p.creds {
		if time.Now().After(cred.cooldownUntil) {
			return true // retry available
		}
	}
	return false // all in cooldown
}

// ResetCooldowns clears all cooldown timers (e.g., after config reload).
func (p *CredentialPool) ResetCooldowns() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.creds {
		c.cooldownUntil = time.Time{}
	}
}

// Status returns the current state of all credentials for diagnostics.
func (p *CredentialPool) Status() []struct {
	Key           string
	CooldownUntil time.Time
	FailCount     int
	Healthy       bool
} {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]struct {
		Key           string
		CooldownUntil time.Time
		FailCount     int
		Healthy       bool
	}, len(p.creds))

	for i, c := range p.creds {
		masked := c.entry.Key
		if len(masked) > 8 {
			masked = masked[:8] + "..."
		}
		result[i] = struct {
			Key           string
			CooldownUntil time.Time
			FailCount     int
			Healthy       bool
		}{
			Key:           masked,
			CooldownUntil: c.cooldownUntil,
			FailCount:     c.failCount,
			Healthy:       time.Now().After(c.cooldownUntil),
		}
	}
	return result
}

// Compile-time interface check
var _ ports.LLMProvider = (*CredentialPool)(nil)

// Ensure io import is used (for TokenStream = io.ReadCloser)
var _ io.Reader // quiet lint
