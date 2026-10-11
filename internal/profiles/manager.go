package profiles

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"gaia/internal/core/domain"
	"gaia/internal/tools"
)

// Manager coordinates profile registration, lifecycle, and tool resolution.
type Manager struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	catalog  *tools.ToolCatalog
	cfg      *domain.Config
	saveFn   func(*domain.Config) error
}

// NewManager initializes a new profile manager with default tool catalog.
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
		catalog:  tools.DefaultCatalog(),
		cfg:      cfg,
		saveFn:   saveFn,
	}
}

// NewManagerWithCatalog initializes a new profile manager with an explicit tool catalog.
func NewManagerWithCatalog(cfg *domain.Config, saveFn func(*domain.Config) error, cat *tools.ToolCatalog) *Manager {
	mgr := NewManager(cfg, saveFn)
	if cat != nil {
		mgr.catalog = cat
	}
	return mgr
}

// SetCatalog sets the active global tool catalog for the manager.
func (m *Manager) SetCatalog(cat *tools.ToolCatalog) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catalog = cat
}

// Catalog returns the currently bound tool catalog.
func (m *Manager) Catalog() *tools.ToolCatalog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.catalog
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

// effectiveToolNamesLocked returns configured tools if non-empty, otherwise default tools.
func (m *Manager) effectiveToolNamesLocked(p Profile) []string {
	if p == nil {
		return nil
	}
	if m.cfg != nil && m.cfg.Profiles != nil {
		if pCfg, ok := m.cfg.Profiles[p.ID()]; ok && len(pCfg.Tools) > 0 {
			return pCfg.Tools
		}
	}
	return p.DefaultTools()
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
		toolCount := len(m.effectiveToolNamesLocked(p))

		result = append(result, ProfileInfo{
			ID:          p.ID(),
			Name:        p.Name(),
			Description: p.Description(),
			Active:      active,
			Enabled:     enabled,
			ToolCount:   toolCount,
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

	toolNames := m.effectiveToolNamesLocked(active)

	if m.catalog != nil {
		var defs []domain.ToolDef
		for _, name := range toolNames {
			if entry, ok := m.catalog.Get(name); ok {
				defs = append(defs, entry.Definition)
			} else {
				// Fallback to active profile's Tools() if present
				for _, t := range active.Tools() {
					if t.Name == name {
						defs = append(defs, t)
						break
					}
				}
			}
		}
		return defs
	}

	// Catalog not set: find in active profile's Tools()
	if len(toolNames) > 0 {
		var defs []domain.ToolDef
		for _, name := range toolNames {
			for _, t := range active.Tools() {
				if t.Name == name {
					defs = append(defs, t)
					break
				}
			}
		}
		return defs
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

func (m *Manager) profileHasToolLocked(p Profile, toolName string) bool {
	if p == nil {
		return false
	}
	toolNames := m.effectiveToolNamesLocked(p)
	for _, t := range toolNames {
		if t == toolName {
			return true
		}
	}
	for _, t := range p.Tools() {
		if t.Name == toolName {
			return true
		}
	}
	return false
}

func (m *Manager) resolveToolLocked(name string) (Profile, bool) {
	if active := m.activeLocked(); active != nil {
		if m.profileHasToolLocked(active, name) {
			return active, true
		}
	}

	for _, p := range m.profiles {
		if m.profileHasToolLocked(p, name) {
			return p, true
		}
	}

	return nil, false
}

// ResolveTool searches for a profile that provides the given tool name.
// The active profile is checked first, followed by other registered profiles.
func (m *Manager) ResolveTool(name string) (Profile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.resolveToolLocked(name)
}

// ExecuteTool verifies that the tool is enabled for the active profile (or resolved profile)
// and executes it via the catalog, falling back to the profile's ExecuteTool.
func (m *Manager) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (*domain.ToolResult, error) {
	m.mu.RLock()
	active := m.activeLocked()
	cat := m.catalog

	var targetProfile Profile
	if active != nil && m.profileHasToolLocked(active, name) {
		targetProfile = active
	} else {
		if p, ok := m.resolveToolLocked(name); ok {
			targetProfile = p
		}
	}
	m.mu.RUnlock()

	if targetProfile == nil {
		activeID := "unknown"
		if active != nil {
			activeID = active.ID()
		}
		return &domain.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q is not enabled for profile %q", name, activeID),
		}, fmt.Errorf("tool %q is not enabled for profile %q", name, activeID)
	}

	if cat != nil {
		if _, exists := cat.Get(name); exists {
			return cat.Execute(ctx, name, args)
		}
	}

	return targetProfile.ExecuteTool(ctx, name, args)
}

// ConfigureTools updates the explicit tool list for a given profile and persists config.
func (m *Manager) ConfigureTools(profileID string, toolNames []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.profiles[profileID]; !ok {
		return fmt.Errorf("profile %q is not registered", profileID)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[profileID]
	pCfg.Tools = append([]string{}, toolNames...)
	m.cfg.Profiles[profileID] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// EnableTool appends toolName to the profile's configured tools if not present, and saves config.
func (m *Manager) EnableTool(profileID, toolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[profileID]
	if !ok {
		return fmt.Errorf("profile %q is not registered", profileID)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[profileID]

	var current []string
	if len(pCfg.Tools) == 0 {
		current = append([]string{}, p.DefaultTools()...)
	} else {
		current = append([]string{}, pCfg.Tools...)
	}

	alreadyPresent := false
	for _, t := range current {
		if t == toolName {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent {
		current = append(current, toolName)
	}

	pCfg.Tools = current
	m.cfg.Profiles[profileID] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// DisableTool removes toolName from the profile's configured tools, and saves config.
func (m *Manager) DisableTool(profileID, toolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[profileID]
	if !ok {
		return fmt.Errorf("profile %q is not registered", profileID)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[profileID]

	var current []string
	if len(pCfg.Tools) == 0 {
		current = append([]string{}, p.DefaultTools()...)
	} else {
		current = append([]string{}, pCfg.Tools...)
	}

	updated := make([]string, 0, len(current))
	for _, t := range current {
		if t != toolName {
			updated = append(updated, t)
		}
	}

	pCfg.Tools = updated
	m.cfg.Profiles[profileID] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// ResetTools clears cfg.Profiles[profileID].Tools back to default tools, and saves config.
func (m *Manager) ResetTools(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.profiles[profileID]; !ok {
		return fmt.Errorf("profile %q is not registered", profileID)
	}

	if m.cfg.Profiles == nil {
		m.cfg.Profiles = make(map[string]domain.ProfileConfig)
	}
	pCfg := m.cfg.Profiles[profileID]
	pCfg.Tools = nil
	m.cfg.Profiles[profileID] = pCfg

	if m.saveFn != nil {
		if err := m.saveFn(m.cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// ProfileTools returns the active CatalogEntry list for the given profile.
func (m *Manager) ProfileTools(profileID string) []tools.CatalogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.profiles[profileID]
	if !ok {
		return nil
	}

	toolNames := m.effectiveToolNamesLocked(p)
	if m.catalog == nil {
		return nil
	}

	return m.catalog.Filter(toolNames)
}
