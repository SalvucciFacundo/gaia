// Package skills provides the Skills Hub for GAIA.
// It manages bundled (project-level) and user-installed skills,
// parses SKILL.md frontmatter, and supports activation/deactivation
// for prompt injection into subagents.
package skills

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMeta holds the parsed frontmatter metadata from a SKILL.md file.
type SkillMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Language    string   `yaml:"language"`
	Category    string   `yaml:"category"`

	// DirPath is the absolute directory containing this SKILL.md.
	DirPath string `yaml:"-"`
	// Source indicates whether the skill is bundled or user-installed.
	Source string `yaml:"-"` // "bundled", "user", or "tap"

	// Provenance tracks where the skill came from for security auditing.
	Provenance string `yaml:"-"` // install source URL or "bundled"
	InstalledAt string `yaml:"-"` // install timestamp
	ContentHash string `yaml:"-"` // sha256 of SKILL.md content for integrity
}

// Skill holds the full skill information including content.
type Skill struct {
	Meta    SkillMeta
	Content string // Full SKILL.md body (after frontmatter)
}

// ErrInvalidSkill is returned when a SKILL.md file fails validation.
var ErrInvalidSkill = errors.New("invalid SKILL.md format")

// ParseSkillFile reads a SKILL.md file from the given directory and parses
// its YAML frontmatter and Markdown body.
func ParseSkillFile(dir string) (*Skill, error) {
	mdPath := filepath.Join(dir, "SKILL.md")
	f, err := os.Open(mdPath)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", mdPath, err)
	}
	defer f.Close()

	return ParseSkill(f)
}

// ParseSkill reads a SKILL.md frontmatter and body from a reader.
func ParseSkill(r io.Reader) (*Skill, error) {
	fm, body, err := splitFrontmatter(r)
	if err != nil {
		return nil, err
	}

	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("%w: parsing frontmatter: %w", ErrInvalidSkill, err)
	}

	if meta.Name == "" {
		return nil, fmt.Errorf("%w: missing required field 'name'", ErrInvalidSkill)
	}

	return &Skill{
		Meta:    meta,
		Content: body,
	}, nil
}

// splitFrontmatter reads YAML frontmatter between --- delimiters.
// Returns the raw frontmatter string and the body after the second ---.
func splitFrontmatter(r io.Reader) (frontmatter string, body string, err error) {
	scanner := bufio.NewScanner(r)

	// Find opening ---
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
	}
	if !found {
		return "", "", fmt.Errorf("%w: missing opening '---' frontmatter delimiter", ErrInvalidSkill)
	}

	// Collect frontmatter lines until closing ---
	var fmLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines = append(fmLines, line)
	}

	// Collect remaining body
	var bodyLines []string
	for scanner.Scan() {
		bodyLines = append(bodyLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("reading skill file: %w", err)
	}

	return strings.Join(fmLines, "\n"), strings.Join(bodyLines, "\n"), nil
}

