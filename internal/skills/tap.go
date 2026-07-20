package skills

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tap represents a community skill source from a GitHub repository.
type Tap struct {
	URL    string `json:"url"`
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

// TapInfo holds metadata about an installed tap.
type TapInfo struct {
	Tap
	InstalledPath string `json:"installed_path"`
	SkillCount    int    `json:"skill_count"`
}

// tapsDir returns the directory where taps are stored.
func (h *Hub) tapsDir() string {
	if h.installedDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(h.installedDir), "taps")
}

// addTap clones a GitHub tap repository and indexes its skills.
// The tap is cloned to ~/.gaia/taps/<hash>/ and scanned for SKILL.md
// files in subdirectories. Tap skills are added to the hub index with
// source "tap" and lower precedence than user-installed skills.
//
// URL formats accepted:
//   - "owner/repo"
//   - "github.com/owner/repo"
//   - "https://github.com/owner/repo"
func (h *Hub) AddTap(url, branch string) error {
	if branch == "" {
		branch = "main"
	}

	// Normalize URL: accept owner/repo, github.com/owner/repo, or full URL.
	url = normalizeTapURL(url)

	// Validate URL: must be a github.com path, no path traversal.
	if !strings.HasPrefix(url, "github.com/") {
		return fmt.Errorf("tap URL must be a GitHub repository, got %q", url)
	}
	if strings.Contains(url, "..") {
		return fmt.Errorf("tap URL contains path traversal: %q", url)
	}

	tapName := tapNameFromURL(url)
	tapsPath := h.tapsDir()

	// Hash the URL for a stable directory name.
	hash := sha256.Sum256([]byte(url))
	hashStr := fmt.Sprintf("%x", hash[:8])
	cloneDir := filepath.Join(tapsPath, tapName+"-"+hashStr)

	// Clone or pull.
	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tapsPath, 0755); err != nil {
			return fmt.Errorf("tap: create taps dir: %w", err)
		}

		cloneURL := "https://" + url
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, cloneURL, cloneDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("tap: git clone failed: %s: %w", string(out), err)
		}
	} else {
		// Already cloned — pull latest.
		cmd := exec.Command("git", "-C", cloneDir, "pull", "origin", branch)
		cmd.Run() // Best effort.
	}

	// Persist tap metadata so ListTaps can show the URL.
	saveTapMeta(cloneDir, url, branch)

	// Scan for skills.
	skillsDirs := scanTapSkills(cloneDir)
	h.mu.Lock()
	defer h.mu.Unlock()

	// Invalidate and rebuild the index to include tap skills.
	h.invalidateIndex()

	_ = skillsDirs // used by rebuild
	return nil
}

// tapMetaFile returns the path to the metadata file for a tap directory.
func tapMetaFile(dir string) string {
	return filepath.Join(dir, ".tap-meta.json")
}

// tapMeta holds persisted metadata for an installed tap.
type tapMeta struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

// saveTapMeta writes tap metadata to the cloned directory.
func saveTapMeta(dir, url, branch string) {
	meta := tapMeta{URL: url, Branch: branch}
	data, _ := json.Marshal(meta)
	os.WriteFile(tapMetaFile(dir), data, 0644)
}

// loadTapMeta reads tap metadata from the cloned directory.
func loadTapMeta(dir string) (tapMeta, error) {
	data, err := os.ReadFile(tapMetaFile(dir))
	if err != nil {
		return tapMeta{}, err
	}
	var meta tapMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return tapMeta{}, err
	}
	return meta, nil
}

// RemoveTap removes a tap by URL and deletes its cloned directory.
func (h *Hub) RemoveTap(url string) error {
	if !strings.HasPrefix(url, "github.com/") {
		return fmt.Errorf("invalid tap URL: %q", url)
	}

	tapsPath := h.tapsDir()
	tapName := tapNameFromURL(url)
	hash := sha256.Sum256([]byte(url))
	hashStr := fmt.Sprintf("%x", hash[:8])
	cloneDir := filepath.Join(tapsPath, tapName+"-"+hashStr)

	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		return fmt.Errorf("tap %q is not installed", url)
	}

	if err := os.RemoveAll(cloneDir); err != nil {
		return fmt.Errorf("tap: remove dir: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.invalidateIndex()
	return nil
}

// ListTaps returns all installed taps with their skill counts.
func (h *Hub) ListTaps() ([]TapInfo, error) {
	tapsPath := h.tapsDir()
	if _, err := os.Stat(tapsPath); os.IsNotExist(err) {
		return nil, nil // No taps installed.
	}

	entries, err := os.ReadDir(tapsPath)
	if err != nil {
		return nil, fmt.Errorf("tap: read taps dir: %w", err)
	}

	var taps []TapInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(tapsPath, entry.Name())
		info := TapInfo{
			InstalledPath: dir,
			SkillCount:    countTapSkills(dir),
		}
		if meta, err := loadTapMeta(dir); err == nil {
			info.URL = meta.URL
			info.Branch = meta.Branch
		}
		taps = append(taps, info)
	}
	return taps, nil
}

// scanTapSkills scans a tap directory for skill subdirectories containing SKILL.md.
func scanTapSkills(dir string) []string {
	var skills []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, entry.Name())
		mdPath := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(mdPath); err == nil {
			skills = append(skills, skillDir)
		}
	}
	return skills
}

// countTapSkills counts SKILL.md files in a tap directory.
func countTapSkills(dir string) int {
	return len(scanTapSkills(dir))
}

// normalizeTapURL converts various URL formats to the normalized "github.com/owner/repo" form.
func normalizeTapURL(url string) string {
	// Already normalized
	if strings.HasPrefix(url, "github.com/") {
		return url
	}
	// Full HTTPS URL
	if strings.HasPrefix(url, "https://github.com/") {
		return strings.TrimPrefix(url, "https://")
	}
	// Bare owner/repo — prepend github.com/
	if !strings.Contains(url, "/") {
		return url // not valid, but let further validation catch it
	}
	return "github.com/" + url
}

// tapNameFromURL extracts a short name from a GitHub URL.
func tapNameFromURL(url string) string {
	// github.com/owner/repo → owner-repo
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "-" + parts[len(parts)-1]
	}
	return url
}

// buildTapIndex adds tap-discovered skills to the existing index.
// This is called from buildIndex() to merge tap sources.
func (h *Hub) buildTapIndex() []SkillMeta {
	tapsPath := h.tapsDir()
	if tapsPath == "" {
		return nil
	}

	if _, err := os.Stat(tapsPath); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(tapsPath)
	if err != nil {
		return nil
	}

	var metas []SkillMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(tapsPath, entry.Name())
		metas = append(metas, scanDir(dir, "tap")...)
	}
	return metas
}
