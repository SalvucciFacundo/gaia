package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Hub manages skill discovery, installation, activation, and removal.
// It scans bundled skills (shipped with the binary) and user-installed
// skills (in ~/.gaia/skills/), merging them into a unified index.
//
// Bundled skills are read-only and version-locked. User-installed skills
// are mutable and take precedence over bundled skills with the same name.
//
// Active skills have their SKILL.md content loaded into the registry for
// prompt injection. Deactivated skills remain installed but are excluded
// from prompt context.
//
// Activation state is persisted to ~/.gaia/skills/state.json across sessions.
type Hub struct {
	mu sync.RWMutex

	bundledDir   string          // Project's "skills/" directory (read-only)
	installedDir string          // User's "~/.gaia/skills/" directory
	active       map[string]bool // Set of active skill names
	index        []SkillMeta      // Merged skill index (user overrides bundled)
}

// NewHub creates a Skills Hub scanning both bundled and installed directories.
// bundledDir is the project's skills/ path (shipped with the binary).
// installedDir is the user's ~/.gaia/skills/ path (created if missing).
func NewHub(bundledDir, installedDir string) *Hub {
	h := &Hub{
		bundledDir:   bundledDir,
		installedDir: installedDir,
		active:       make(map[string]bool),
	}
	h.loadState()
	return h
}

// Search finds skills whose name or tags match the given query.
// The query is a case-insensitive substring match against name, description,
// and tags. Returns skills sorted by name.
func (h *Hub) Search(query string) []SkillMeta {
	h.ensureIndex()
	h.mu.RLock()
	defer h.mu.RUnlock()

	if query == "" {
		return h.index
	}

	lower := strings.ToLower(query)
	var results []SkillMeta
	for _, m := range h.index {
		if matchSkill(m, lower) {
			results = append(results, m)
		}
	}
	return results
}

// List returns all available skills (bundled + installed) sorted by name.
// User-installed skills shadow bundled skills with the same name.
func (h *Hub) List() []SkillMeta {
	h.ensureIndex()
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]SkillMeta, len(h.index))
	copy(result, h.index)
	return result
}

// ListActive returns only active skills sorted by name.
func (h *Hub) ListActive() []SkillMeta {
	h.ensureIndex()
	h.mu.RLock()
	defer h.mu.RUnlock()

	var active []SkillMeta
	for _, m := range h.index {
		if h.active[m.Name] {
			active = append(active, m)
		}
	}
	return active
}

// Activate marks a skill as active for prompt injection.
// Returns an error if the skill is not found.
// Activation state is persisted to disk.
func (h *Hub) Activate(name string) error {
	h.ensureIndex()
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.exists(name) {
		return fmt.Errorf("skill %q not found in registry", name)
	}

	h.active[name] = true
	return h.saveState()
}

// Deactivate marks a skill as inactive (excluded from prompt context).
// The skill remains installed. Returns an error if the skill is not found.
// Activation state is persisted to disk.
func (h *Hub) Deactivate(name string) error {
	h.ensureIndex()
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.exists(name) {
		return fmt.Errorf("skill %q not found in registry", name)
	}

	delete(h.active, name)
	return h.saveState()
}

// Install copies a skill from the bundled directory to the user's
// installed directory. The bundled skill must already exist.
// Returns an error if the skill is already installed or not found.
func (h *Hub) Install(name string) error {
	// Find the bundled skill directory.
	bundledPath := filepath.Join(h.bundledDir, name)
	if info, err := os.Stat(bundledPath); err != nil || !info.IsDir() {
		return fmt.Errorf("bundled skill %q not found at %s", name, bundledPath)
	}

	// Check if already installed.
	installedPath := filepath.Join(h.installedDir, name)
	if info, err := os.Stat(installedPath); err == nil && info.IsDir() {
		return fmt.Errorf("skill %q is already installed", name)
	}

	// Ensure installedDir exists.
	if err := os.MkdirAll(h.installedDir, 0755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	// Copy the skill directory recursively.
	if err := copyDir(bundledPath, installedPath); err != nil {
		return fmt.Errorf("copying skill %q: %w", name, err)
	}

	// Activate by default after install.
	h.mu.Lock()
	h.active[name] = true
	h.invalidateIndex()
	h.mu.Unlock()

	return nil
}

// Remove deletes a user-installed skill from ~/.gaia/skills/.
// Bundled skills cannot be removed — only deactivated.
func (h *Hub) Remove(name string) error {
	installedPath := filepath.Join(h.installedDir, name)
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		return fmt.Errorf("skill %q is not installed", name)
	}

	if err := os.RemoveAll(installedPath); err != nil {
		return fmt.Errorf("removing skill %q: %w", name, err)
	}

	h.mu.Lock()
	delete(h.active, name)
	h.invalidateIndex()
	h.mu.Unlock()

	return nil
}

// IsActive returns whether the named skill is currently active.
func (h *Hub) IsActive(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.active[name]
}

// LoadSkills returns the full Skill content for all active skills.
// This is used for prompt injection into subagents.
func (h *Hub) LoadSkills() []Skill {
	h.ensureIndex()
	h.mu.RLock()
	defer h.mu.RUnlock()

	var skills []Skill
	for name := range h.active {
		dir := h.findSkillDir(name)
		if dir == "" {
			continue
		}
		s, err := ParseSkillFile(dir)
		if err != nil {
			continue
		}
		s.Meta.DirPath = dir
		s.Meta.Source = h.sourceFor(name)
		skills = append(skills, *s)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Meta.Name < skills[j].Meta.Name
	})
	return skills
}

// RecommendFor returns skills matching the given project language.
// Used by the first-run wizard to suggest relevant skills.
func (h *Hub) RecommendFor(language string) []SkillMeta {
	h.ensureIndex()
	h.mu.RLock()
	defer h.mu.RUnlock()

	lower := strings.ToLower(language)
	var results []SkillMeta
	for _, m := range h.index {
		if strings.ToLower(m.Language) == lower {
			results = append(results, m)
		}
	}
	return results
}

// -------- internal helpers --------

// ensureIndex builds the merged skill index lazily on first access.
func (h *Hub) ensureIndex() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.index != nil {
		return
	}
	h.buildIndex()
}

// buildIndex scans both bundled, installed, and tap directories and merges
// skills into a single deduplicated index. User-installed skills take
// precedence over bundled and tap skills with the same name.
func (h *Hub) buildIndex() {
	var raw []SkillMeta

	if h.bundledDir != "" {
		raw = append(raw, scanDir(h.bundledDir, "bundled")...)
	}
	if h.installedDir != "" {
		raw = append(raw, scanDir(h.installedDir, "user")...)
	}

	// Include tap skills.
	raw = append(raw, h.buildTapIndex()...)

	// Merge: user skills override bundled/tap ones by name.
	merged := make(map[string]SkillMeta, len(raw))
	for _, m := range raw {
		existing, ok := merged[m.Name]
		if !ok {
			merged[m.Name] = m
			continue
		}
		// User skills take precedence.
		if m.Source == "user" {
			merged[m.Name] = m
		} else if existing.Source != "user" {
			merged[m.Name] = m
		}
	}

	h.index = make([]SkillMeta, 0, len(merged))
	for _, m := range merged {
		h.index = append(h.index, m)
	}
	sort.Slice(h.index, func(i, j int) bool {
		return h.index[i].Name < h.index[j].Name
	})
}

// invalidateIndex forces a rebuild on the next access.
func (h *Hub) invalidateIndex() {
	h.index = nil
}

// exists checks whether a skill name exists in the current index.
// Caller must hold at least a read lock.
func (h *Hub) exists(name string) bool {
	for _, m := range h.index {
		if m.Name == name {
			return true
		}
	}
	return false
}

// findSkillDir returns the absolute directory for a skill name.
// Checks installed directory first (user overrides bundled), then bundled.
func (h *Hub) findSkillDir(name string) string {
	if h.installedDir != "" {
		dir := filepath.Join(h.installedDir, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	if h.bundledDir != "" {
		dir := filepath.Join(h.bundledDir, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// sourceFor returns "user" if the skill is installed, "bundled" otherwise.
func (h *Hub) sourceFor(name string) string {
	if h.installedDir != "" {
		dir := filepath.Join(h.installedDir, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return "user"
		}
	}
	return "bundled"
}

// scanDir scans a directory for skill directories containing SKILL.md files
// and returns their parsed metadata.
func scanDir(root string, source string) []SkillMeta {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var metas []SkillMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		mdPath := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			continue
		}

		skill, err := ParseSkillFile(dir)
		if err != nil {
			continue
		}

		skill.Meta.DirPath = dir
		skill.Meta.Source = source
		metas = append(metas, skill.Meta)
	}
	return metas
}

// matchSkill checks whether a SkillMeta matches a lowercase query string.
// It searches name, description, and tags.
func matchSkill(m SkillMeta, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(m.Name), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(m.Description), lowerQuery) {
		return true
	}
	for _, t := range m.Tags {
		if strings.Contains(strings.ToLower(t), lowerQuery) {
			return true
		}
	}
	return false
}

// stateFile returns the path to the activation state file.
func (h *Hub) stateFile() string {
	if h.installedDir == "" {
		return ""
	}
	return filepath.Join(h.installedDir, "state.json")
}

// loadState reads the persisted activation state from disk.
func (h *Hub) loadState() {
	path := h.stateFile()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // File doesn't exist yet — start fresh.
	}

	var active []string
	if err := json.Unmarshal(data, &active); err != nil {
		return
	}
	for _, name := range active {
		h.active[name] = true
	}
}

// saveState writes the current activation state to disk.
func (h *Hub) saveState() error {
	path := h.stateFile()
	if path == "" {
		return nil
	}

	active := make([]string, 0, len(h.active))
	for name := range h.active {
		active = append(active, name)
	}
	sort.Strings(active)

	if err := os.MkdirAll(h.installedDir, 0755); err != nil {
		return fmt.Errorf("creating skills state directory: %w", err)
	}

	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// copyDir recursively copies a directory from src to dst.
// Only regular files are copied; directories and symlinks are created.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
