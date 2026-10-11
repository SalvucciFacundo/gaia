package profiles

import (
	"fmt"
	"sort"
	"sync"

	"gaia/internal/core/domain"
)

// Manager coordinates profile registration, lifecycle, and tool resolution.
type Manager struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	cfg      *domain.Config
	saveFn   func(*domain.Config) error
}

// NewManager initializes a new profile manager.
func NewManager(cfg *domain.Config, saveFn func(*domain.Config) error) *Manager {
	if cfg == nil {
		cfg = &domain.Config{
			CurrentProfile: "assistant",
			Profiles: map[string]domain.ProfileConfig{
				"assistant": {Enabled: true},
				"dev":       {Enabled: false},
				"art":       {Enabled: false},
			},
		}
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = "assistant"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]domain.ProfileConfig{
			"assistant": {Enabled: true},
			"dev":       {Enabled: false},
			"art":       {Enabled: false},
		}
	}

	return &Manager{
		profiles: make(map[string]Profile),
		cfg:      cfg,
		saveFn:   saveFn,
	}
}

// Register adds a profile to the manager.
func (m *Manager) Register(p Profile) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[p.ID()] = p
}

// RegisterDefaults registers the built-in profiles: Assistant, Dev, and Art.
func (m *Manager) RegisterDefaults() {
	m.Register(NewAssistantProfile())
	m.Register(NewDevProfile())
	m.Register(NewArtProfile())
}

// Active returns the currently active profile, or nil if none is active.
func (m *Manager) Active() Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeLocked()
}

func (m *Manager) activeLocked() Profile {
	if m.cfg == nil || m.cfg.CurrentProfile == "" {
		return nil
	}
	return m.profiles[m.cfg.CurrentProfile]
}

// SetActive sets the active profile ID, validating that it exists and is enabled,
// updating cfg.CurrentProfile and persisting changes via saveFn.
func (m *Manager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, registered := m.profiles[id]
	if !registered || p == nil {
		return fmt.Errorf("profile %q is not registered", id)
	}

	if m.cfg.Profiles == nil {
		return fmt.Errorf("profile %q is disabled", id)
	}
	pCfg, ok := m.cfg.Profiles[id]
	if !ok || !pCfg.Enabled {
		return fmt.Errorf("profile %q is disabled", id)
	}

	m.cfg.CurrentProfile = id
	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// Enable marks a profile as enabled in the config and persists changes via saveFn.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, registered := m.profiles[id]; !registered {
		return fmt.Errorf("profile %q is not registered", id)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[id]
	pCfg.Enabled = true
	m.cfg.Profiles[id] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// Disable marks a profile as disabled in the config and persists changes via saveFn.
// Fails if attempting to disable the currently active profile.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, registered := m.profiles[id]; !registered {
		return fmt.Errorf("profile %q is not registered", id)
	}

	if m.cfg.CurrentProfile == id {
		return fmt.Errorf("cannot disable active profile %q", id)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[id]
	pCfg.Enabled = false
	m.cfg.Profiles[id] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// List returns a sorted slice of profile metadata summaries.
func (m *Manager) List() []ProfileInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ProfileInfo, 0, len(m.profiles))
	for _, p := range m.profiles {
		enabled := false
		if m.cfg != nil && m.cfg.Profiles != nil {
			if pCfg, ok := m.cfg.Profiles[p.ID()]; ok {
				enabled = pCfg.Enabled
			}
		}
		active := (m.cfg != nil && m.cfg.CurrentProfile == p.ID())
		result = append(result, ProfileInfo{
			ID:          p.ID(),
			Name:        p.Name(),
			Description: p.Description(),
			Active:      active,
			Enabled:     enabled,
			ToolCount:   len(p.Tools()),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// ActiveTools returns the tools exposed by the active profile.
func (m *Manager) ActiveTools() []domain.ToolDef {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := m.activeLocked()
	if active == nil {
		return nil
	}
	return active.Tools()
}

// ActiveSystemPrompt returns the system prompt of the active profile.
func (m *Manager) ActiveSystemPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := m.activeLocked()
	if active == nil {
		return ""
	}
	return active.SystemPrompt()
}

// ResolveTool searches for a profile that provides the given tool name.
// The active profile is checked first, followed by other registered profiles.
func (m *Manager) ResolveTool(name string) (Profile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if active := m.activeLocked(); active != nil {
		for _, t := range active.Tools() {
			if t.Name == name {
				return active, true
			}
		}
	}

	for _, p := range m.profiles {
		for _, t := range p.Tools() {
			if t.Name == name {
				return p, true
			}
		}
	}

	return nil, false
}
