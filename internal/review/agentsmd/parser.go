// Package agentsmd parses AGENTS.md files following the same YAML
// frontmatter + markdown body pattern used by SKILL.md files.
package agentsmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Standards holds team coding standards extracted from AGENTS.md.
type Standards struct {
	Rules       []Rule   `yaml:"rules"`       // structured rule patterns
	Forbidden   []string `yaml:"forbidden"`   // forbidden patterns or practices
	Conventions []string `yaml:"conventions"` // naming or style conventions
	Prose       string   // markdown body (guidelines, rationale)
}

// Rule is a structured coding rule with a regex pattern and severity.
type Rule struct {
	ID       string `yaml:"id"`       // unique rule identifier
	Pattern  string `yaml:"pattern"`  // regex pattern to match violations
	Severity string `yaml:"severity"` // "error" or "warning"
	Message  string `yaml:"message"`  // human-readable message
}

// FindAndParse searches for AGENTS.md starting at projectRoot and
// walking up parent directories. Returns the parsed standards, or
// an error if no AGENTS.md is found.
func FindAndParse(projectRoot string) (*Standards, error) {
	dir := filepath.Clean(projectRoot)
	for {
		path := filepath.Join(dir, "AGENTS.md")
		if _, err := os.Stat(path); err == nil {
			return Parse(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("AGENTS.md not found in %s or any parent", projectRoot)
		}
		dir = parent
	}
}

// Parse reads an AGENTS.md file and extracts YAML frontmatter from the
// markdown body. The file format is:
//
//	---
//	rules:
//	  - id: ...
//	    pattern: ...
//	    severity: ...
//	    message: ...
//	forbidden:
//	  - ...
//	conventions:
//	  - ...
//	---
//	# Prose guidelines
//	...
func Parse(path string) (*Standards, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse AGENTS.md: %w", err)
	}

	content := string(data)
	std := &Standards{}
	prose := content

	// Split on YAML frontmatter delimiters: --- ... ---
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			yamlContent := strings.TrimSpace(parts[1])
			prose = strings.TrimSpace(parts[2])

			if yamlContent != "" {
				if err := yaml.Unmarshal([]byte(yamlContent), std); err != nil {
					// Frontmatter parse failure: treat entire file as prose.
					std = &Standards{}
					prose = content
				}
			}
		}
	}

	std.Prose = prose
	return std, nil
}

// InjectIntoPrompt appends the standards to a review prompt as context
// for the reviewer LLM. If standards is nil, returns prompt unchanged.
func (s *Standards) InjectIntoPrompt(prompt string) string {
	if s == nil {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## Team Standards (from AGENTS.md)\n")

	if len(s.Rules) > 0 {
		sb.WriteString("\n### Rules\n")
		for _, r := range s.Rules {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s (pattern: `%s`)\n",
				r.Severity, r.ID, r.Message, r.Pattern))
		}
	}

	if len(s.Forbidden) > 0 {
		sb.WriteString("\n### Forbidden Patterns\n")
		for _, f := range s.Forbidden {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(s.Conventions) > 0 {
		sb.WriteString("\n### Conventions\n")
		for _, c := range s.Conventions {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	if s.Prose != "" {
		sb.WriteString("\n### Guidelines\n")
		sb.WriteString(s.Prose)
		sb.WriteString("\n")
	}

	return sb.String()
}
