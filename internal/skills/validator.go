package skills

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	skillNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	reWhenToUse     = regexp.MustCompile(`(?im)^##\s+(when\s+to\s+use|description\s*&\s*intent)\b`)
	reCriticalRules = regexp.MustCompile(`(?im)^##\s+(critical\s+rules|rules)\b`)
	reWorkflow      = regexp.MustCompile(`(?im)^##\s+(workflow|steps|procedure)\b`)
	reExamples      = regexp.MustCompile(`(?im)^##\s+(examples|reference)\b`)
)

// ValidateSkillName checks if a skill name conforms to the standard:
// 1 to 64 alphanumeric, underscore, or hyphen characters, no path traversal.
func ValidateSkillName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("skill name is required")
	}
	if len(trimmed) > 64 {
		return fmt.Errorf("skill name %q exceeds maximum length of 64 characters", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") || filepath.Clean(name) != name {
		return fmt.Errorf("invalid skill name %q: path traversal not allowed", name)
	}
	if !skillNameRegex.MatchString(trimmed) {
		return fmt.Errorf("invalid skill name %q: must match regex ^[a-zA-Z0-9_-]+$", name)
	}
	return nil
}

// ValidateSkillSections checks that the markdown body contains all required level-2 sections.
func ValidateSkillSections(body string) error {
	if !reWhenToUse.MatchString(body) {
		return errors.New("missing required section: '## When to Use' (or '## Description & Intent')")
	}
	if !reCriticalRules.MatchString(body) {
		return errors.New("missing required section: '## Critical Rules' (or '## Rules')")
	}
	if !reWorkflow.MatchString(body) {
		return errors.New("missing required section: '## Workflow' (or '## Steps' / '## Procedure')")
	}
	if !reExamples.MatchString(body) {
		return errors.New("missing required section: '## Examples' (or '## Reference')")
	}
	return nil
}

// ValidateSkillContent validates a raw skill Markdown string with YAML frontmatter.
func ValidateSkillContent(raw string) error {
	const fence = "---"

	rest := strings.TrimLeft(raw, "\ufeff\n ")
	if !strings.HasPrefix(rest, fence+"\n") && rest != fence {
		return errors.New("missing frontmatter: skill file must start with ---")
	}
	body := strings.TrimPrefix(rest, fence+"\n")

	end := strings.Index(body, "\n"+fence)
	if end < 0 {
		return errors.New("unclosed frontmatter: missing closing ---")
	}

	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(body[:end]), &fm); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}

	if err := ValidateSkillName(fm.Name); err != nil {
		return err
	}

	desc := strings.TrimSpace(fm.Description)
	if desc == "" {
		return errors.New("missing description: skill description is required")
	}
	if len(desc) > 500 {
		return fmt.Errorf("skill description too long (%d chars, max 500)", len(desc))
	}

	content := strings.TrimSpace(body[end+len(fence)+1:])
	if content == "" {
		return errors.New("empty content body")
	}

	if err := ValidateSkillSections(content); err != nil {
		return err
	}

	return nil
}
