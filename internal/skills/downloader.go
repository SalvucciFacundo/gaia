package skills

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Downloader fetches skills from remote registries and installs them locally.
type Downloader struct {
	httpClient *http.Client
	destDir    string // ~/.gaia/skills/
}

// NewDownloader creates a skill downloader that installs into destDir.
func NewDownloader(destDir string) *Downloader {
	return &Downloader{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		destDir:    destDir,
	}
}

// DownloadFromURL fetches a SKILL.md file from a URL and installs it
// as a skill in the destination directory. It validates the SKILL.md
// format before saving. The skill name is derived from the parsed frontmatter.
func (d *Downloader) DownloadFromURL(url string) (*SkillMeta, error) {
	resp, err := d.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading skill from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading skill: HTTP %d from %s", resp.StatusCode, url)
	}

	// Read the full body into memory (SKILL.md files are small).
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("reading skill body: %w", err)
	}

	// Validate and parse the SKILL.md content.
	skill, err := ParseSkill(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("invalid SKILL.md at %s: %w", url, err)
	}

	// Install to destDir/{name}/SKILL.md.
	targetDir := filepath.Join(d.destDir, skill.Meta.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("creating skill directory: %w", err)
	}

	targetFile := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(targetFile, body, 0644); err != nil {
		return nil, fmt.Errorf("writing SKILL.md: %w", err)
	}

	skill.Meta.Source = "user"
	skill.Meta.DirPath = targetDir
	return &skill.Meta, nil
}

// DownloadFromDir scans a local directory for SKILL.md files and installs
// them into the destination. Used for importing skills from local sources.
func (d *Downloader) DownloadFromDir(srcDir string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("reading source directory: %w", err)
	}

	var installed []SkillMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		mdPath := filepath.Join(src, "SKILL.md")
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			continue
		}

		skill, err := ParseSkillFile(src)
		if err != nil {
			continue
		}

		targetDir := filepath.Join(d.destDir, skill.Meta.Name)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("creating skill directory %s: %w", skill.Meta.Name, err)
		}

		targetFile := filepath.Join(targetDir, "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		if err := os.WriteFile(targetFile, data, 0644); err != nil {
			return nil, fmt.Errorf("writing SKILL.md for %s: %w", skill.Meta.Name, err)
		}

		skill.Meta.Source = "user"
		skill.Meta.DirPath = targetDir
		installed = append(installed, skill.Meta)
	}

	return installed, nil
}
