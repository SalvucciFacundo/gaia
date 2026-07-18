package skills

import (
	"crypto/sha256"
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
func (h *Hub) AddTap(url, branch string) error {
	if branch == "" {
		branch = "main"
	}

	// Validate URL: must be github.com, no path traversal.
	if !strings.HasPrefix(url, "github.com/") {
		return fmt.Errorf("tap URL must be a github.com repository, got %q", url)
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

	// Scan for skills.
	skillsDirs := scanTapSkills(cloneDir)
	h.mu.Lock()
	defer h.mu.Unlock()

	// Invalidate and rebuild the index to include tap skills.
	h.invalidateIndex()

	_ = skillsDirs // used by rebuild
	return nil
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
		taps = append(taps, TapInfo{
			InstalledPath: filepath.Join(tapsPath, entry.Name()),
			SkillCount:    countTapSkills(filepath.Join(tapsPath, entry.Name())),
		})
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
