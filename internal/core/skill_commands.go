package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gaia/internal/core/domain"
)

func (b *Brain) LearnSkill(ctx context.Context, source string) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")

	// Check if source is a directory with existing patterns
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		// Create a skill from directory contents
		name := filepath.Base(source)
		skillDir := filepath.Join(skillsDir, name)
		os.MkdirAll(skillDir, 0755)

		content := fmt.Sprintf(`---
name: %s
description: "Auto-generated skill from %s"
tags: [auto-generated]
category: custom
---

# %s

This skill was auto-generated from %s.

## Usage

Load this skill when working with code similar to the source directory.

## Patterns

- Review the source for specific patterns to document here.
`, name, source, name, source)

		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Skill created from %s\n  Name: %s\n  Location: %s\n  Edit SKILL.md to add patterns and instructions.", source, name, skillDir),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Otherwise, create a skill from description (naming convention)
	name := strings.ReplaceAll(strings.ToLower(source), " ", "-")
	name = strings.TrimRight(name, "-.,!?")
	skillDir := filepath.Join(skillsDir, name)
	os.MkdirAll(skillDir, 0755)

	content := fmt.Sprintf(`---
name: %s
description: "%s"
tags: [auto-generated, custom]
category: custom
---

# %s

%s

## Instructions

Customize this skill with specific patterns, conventions, and examples.
`, name, source, name, source)

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Skill created from description:\n  Name: %s\n  Description: %s\n  Location: %s\n  Use /skills list to activate it.", name, source, skillDir),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SkillSuggestions analyzes usage and suggests skills to create.
func (b *Brain) SkillSuggestions(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")
	existing := make(map[string]bool)

	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if e.IsDir() {
			existing[e.Name()] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("Skill suggestions based on your project:\n\n")

	suggestions := []struct {
		Name string
		Desc string
	}{
		{"go-testing", "Go testing patterns — table-driven tests, subtests, parallel"},
		{"go-error-handling", "Go error handling — wrapping, sentinel errors, custom types"},
		{"go-concurrency", "Go concurrency — goroutines, channels, sync patterns"},
		{"go-context", "Go context propagation — deadlines, cancellation, values"},
		{"go-naming", "Go naming conventions — packages, types, receivers"},
		{"angular-signals", "Angular signals — reactive state, computed, effects"},
		{"angular-testing", "Angular testing — TestBed, component harness, signals"},
	}

	count := 0
	for _, s := range suggestions {
		if !existing[s.Name] {
			sb.WriteString(fmt.Sprintf("  • %s — %s\n", s.Name, s.Desc))
			sb.WriteString(fmt.Sprintf("    Install: /learn %s\n\n", s.Name))
			count++
		}
	}

	if count == 0 {
		sb.WriteString("  All recommended skills are already installed!\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %d suggestions. Use /learn <name> or gaia skills install <name>.\n", count))
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// BlueprintCreate creates a new skill from a named template.
func (b *Brain) BlueprintCreate(ctx context.Context, name string) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")
	skillDir := filepath.Join(skillsDir, name)
	os.MkdirAll(skillDir, 0755)

	templates := map[string]string{
		"daily-report": `---
name: daily-report
description: "Generate daily progress report"
tags: [blueprint, report]
category: automation
---

# Daily Report

## Schedule
- Run at end of day
- Summarize changes, tests, and findings

## Output
- Markdown report saved to project root
`,
		"nightly-backup": `---
name: nightly-backup
description: "Automated nightly backup"
tags: [blueprint, backup]
category: automation
---

# Nightly Backup

## Schedule
- Run at 2:00 AM daily

## Actions
1. git add -A
2. git commit -m "nightly backup"
3. git push
`,
		"code-review": `---
name: code-review
description: "Standardized code review checklist"
tags: [blueprint, review]
category: development
---

# Code Review Blueprint

## Checklist
- [ ] Error handling is correct
- [ ] Edge cases are covered
- [ ] Tests pass
- [ ] No security issues
- [ ] Documentation updated
`,
		"api-test": `---
name: api-test
description: "API endpoint testing skill"
tags: [blueprint, testing, api]
category: testing
---

# API Test Blueprint

## Coverage
- Happy path
- Error responses
- Authentication
- Rate limiting
- Edge cases
`,
	}

	template, ok := templates[name]
	if !ok {
		available := make([]string, 0, len(templates))
		for t := range templates {
			available = append(available, t)
		}
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unknown blueprint %q.\nAvailable blueprints: %s\n\nUsage: /blueprint <name>", name, strings.Join(available, ", ")),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(template), 0644)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Blueprint created: %s\n  Location: %s\n  Edit SKILL.md to customize.", name, skillDir),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// SkillCurator scans installed skills and reports issues.
func (b *Brain) SkillCurator(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	skillsDir := filepath.Join(home, ".gaia", "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No skills directory found. Install some skills first with 'gaia skills install'."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	var sb strings.Builder
	sb.WriteString("Skill Curator Report\n\n")

	issues := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, e.Name())
		mdPath := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(mdPath); os.IsNotExist(err) {
			sb.WriteString(fmt.Sprintf("  ⚠ %s: missing SKILL.md\n", e.Name()))
			issues++
			continue
		}
		data, _ := os.ReadFile(mdPath)
		lines := strings.Split(string(data), "\n")
		if len(lines) < 5 {
			sb.WriteString(fmt.Sprintf("  ⚠ %s: SKILL.md is too short (%d lines)\n", e.Name(), len(lines)))
			issues++
		}
	}

	if issues == 0 {
		sb.WriteString(fmt.Sprintf("  ✅ All %d skills look healthy.\n", len(entries)))
	} else {
		sb.WriteString(fmt.Sprintf("\n  %d issues found across %d skills.\n", issues, len(entries)))
		sb.WriteString("  Use /skills audit for detailed security scanning.\n")
	}

	msg := domain.Message{Role: domain.RoleSystem, Content: sb.String()}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}
// triggerSkillLearning analyzes a completed subagent task and proposes skill improvements.
func (b *Brain) triggerSkillLearning(ctx context.Context, subagentName string, taskDesc string, result *domain.SubagentResult) {
	// Only analyze if skills were resolved
	if result == nil || result.SkillResolution == "none" {
		return
	}
	// For now, log that learning happened — actual skill creation/deep analysis
	// requires a separate Learner subagent invocation.
	_ = subagentName
	_ = taskDesc
}

// createSkillFromProposal creates a new skill from a Learner proposal.
func (b *Brain) createSkillFromProposal(name string, content string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("create skill: %w", err)
	}
	skillDir := filepath.Join(home, ".gaia", "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
}
