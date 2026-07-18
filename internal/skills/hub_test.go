package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseSkill(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string // expected skill name
		wantErr bool
	}{
		{
			name: "valid skill with all fields",
			input: `---
name: go-concurrency
description: Use when writing concurrent Go code.
tags: [go, concurrency, goroutines]
language: go
category: code-quality
---
# Go Concurrency

Body content here.`,
			want: "go-concurrency",
		},
		{
			name: "valid skill minimal fields",
			input: `---
name: minimal-skill
description: A minimal skill.
---
Body only.`,
			want: "minimal-skill",
		},
		{
			name: "missing name",
			input: `---
description: Forgot the name.
tags: [test]
---
Body.`,
			wantErr: true,
		},
		{
			name:    "missing opening delimiter",
			input:   "# Just markdown, no frontmatter",
			wantErr: true,
		},
		{
			name: "empty body",
			input: `---
name: empty-body
description: No body.
---
`,
			want: "empty-body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, err := ParseSkill(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skill.Meta.Name != tt.want {
				t.Errorf("name = %q, want %q", skill.Meta.Name, tt.want)
			}
		})
	}
}

func TestParseSkillFile(t *testing.T) {
	// Create a temp directory with a SKILL.md file.
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: test-skill
description: A test skill.
language: go
tags: [testing, go]
---
# Test Skill

Test body.`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	skill, err := ParseSkillFile(skillDir)
	if err != nil {
		t.Fatalf("ParseSkillFile: %v", err)
	}
	if skill.Meta.Name != "test-skill" {
		t.Errorf("name = %q, want test-skill", skill.Meta.Name)
	}
	if skill.Meta.Language != "go" {
		t.Errorf("language = %q, want go", skill.Meta.Language)
	}
	if len(skill.Meta.Tags) != 2 {
		t.Errorf("tags count = %d, want 2", len(skill.Meta.Tags))
	}
}

func TestHubSearch(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-concurrency": `---
name: go-concurrency
description: Write concurrent Go code.
tags: [go, concurrency]
language: go
---
Body.
`,
		"go-testing": `---
name: go-testing
description: Write Go tests.
tags: [go, testing]
language: go
---
Body.
`,
		"angular-component": `---
name: angular-component
description: Create Angular components.
tags: [angular, frontend]
language: typescript
---
Body.
`,
	})

	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	t.Run("search by name", func(t *testing.T) {
		results := hub.Search("concurrency")
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Name != "go-concurrency" {
			t.Errorf("name = %q", results[0].Name)
		}
	})

	t.Run("search by tag", func(t *testing.T) {
		results := hub.Search("angular")
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Name != "angular-component" {
			t.Errorf("name = %q", results[0].Name)
		}
	})

	t.Run("search by language concept", func(t *testing.T) {
		results := hub.Search("go")
		if len(results) != 2 {
			t.Fatalf("got %d results, want 2", len(results))
		}
	})

	t.Run("search no match", func(t *testing.T) {
		results := hub.Search("rust")
		if len(results) != 0 {
			t.Fatalf("got %d results, want 0", len(results))
		}
	})

	t.Run("empty search returns all", func(t *testing.T) {
		results := hub.Search("")
		if len(results) != 3 {
			t.Fatalf("got %d results, want 3", len(results))
		}
	})
}

func TestHubList(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"skill-a": `---\nname: skill-a\ndescription: First.\n---\nBody.\n`,
		"skill-b": `---\nname: skill-b\ndescription: Second.\n---\nBody.\n`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	results := hub.List()
	if len(results) != 2 {
		t.Fatalf("got %d skills, want 2", len(results))
	}

	// Verify sorted order.
	if results[0].Name != "skill-a" || results[1].Name != "skill-b" {
		t.Errorf("unexpected order: %v", names(results))
	}
}

func TestHubActivateDeactivate(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-testing": `---
name: go-testing
description: Go testing skill.
language: go
---
Body.`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	// Activate.
	if err := hub.Activate("go-testing"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !hub.IsActive("go-testing") {
		t.Error("expected go-testing to be active")
	}

	// Activate non-existent skill.
	if err := hub.Activate("nonexistent"); err == nil {
		t.Error("expected error for non-existent skill")
	}

	// List active.
	active := hub.ListActive()
	if len(active) != 1 || active[0].Name != "go-testing" {
		t.Errorf("active = %v, want [go-testing]", names(active))
	}

	// Deactivate.
	if err := hub.Deactivate("go-testing"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if hub.IsActive("go-testing") {
		t.Error("expected go-testing to be inactive after deactivation")
	}
}

func TestHubInstall(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-testing": `---
name: go-testing
description: Go testing skill.
language: go
---
Body.`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	// Install a bundled skill.
	if err := hub.Install("go-testing"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Verify it was copied.
	installedPath := filepath.Join(installedDir, "go-testing", "SKILL.md")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Error("installed SKILL.md not found")
	}

	// Verify it's now active.
	if !hub.IsActive("go-testing") {
		t.Error("expected installed skill to be auto-activated")
	}

	// Double install should fail.
	if err := hub.Install("go-testing"); err == nil {
		t.Error("expected error for already installed skill")
	}
}

func TestHubRemove(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-testing": `---
name: go-testing
description: Go testing skill.
language: go
---
Body.`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	// Install first.
	if err := hub.Install("go-testing"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove.
	if err := hub.Remove("go-testing"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify removed from disk.
	installedPath := filepath.Join(installedDir, "go-testing")
	if _, err := os.Stat(installedPath); !os.IsNotExist(err) {
		t.Error("installed skill directory should be removed")
	}

	// Verify deactivated.
	if hub.IsActive("go-testing") {
		t.Error("removed skill should be inactive")
	}
}

func TestHubRecommendFor(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-concurrency": `---
name: go-concurrency
description: Write concurrent Go code.
tags: [go, concurrency]
language: go
---
Body.`,
		"go-testing": `---
name: go-testing
description: Write Go tests.
tags: [go, testing]
language: go
---
Body.`,
		"angular-component": `---
name: angular-component
description: Create Angular components.
tags: [angular, frontend]
language: typescript
---
Body.`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	t.Run("recommend for go", func(t *testing.T) {
		results := hub.RecommendFor("go")
		if len(results) != 2 {
			t.Fatalf("got %d recommendations, want 2", len(results))
		}
	})

	t.Run("recommend for typescript", func(t *testing.T) {
		results := hub.RecommendFor("typescript")
		if len(results) != 1 {
			t.Fatalf("got %d recommendations, want 1", len(results))
		}
		if results[0].Name != "angular-component" {
			t.Errorf("name = %q", results[0].Name)
		}
	})

	t.Run("recommend for unknown language", func(t *testing.T) {
		results := hub.RecommendFor("rust")
		if len(results) != 0 {
			t.Fatalf("got %d recommendations, want 0", len(results))
		}
	})
}

func TestHubLoadSkills(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"go-testing": `---
name: go-testing
description: Go testing skill.
language: go
---
This is the body content for go-testing.`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	// Activate then load.
	if err := hub.Activate("go-testing"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	skills := hub.LoadSkills()
	if len(skills) != 1 {
		t.Fatalf("got %d loaded skills, want 1", len(skills))
	}

	s := skills[0]
	if s.Meta.Name != "go-testing" {
		t.Errorf("name = %q", s.Meta.Name)
	}
	if !strings.Contains(s.Content, "body content for go-testing") {
		t.Errorf("body doesn't contain expected text: %q", s.Content)
	}
}

func TestHubIndexInvalidation(t *testing.T) {
	bundledDir := createTempSkillsDir(t, "bundled", map[string]string{
		"skill-a": `---\nname: skill-a\ndescription: A.\n---\nBody.\n`,
	})
	installedDir := createTempSkillsDir(t, "installed", nil)

	hub := NewHub(bundledDir, installedDir)

	// First access builds the index.
	if got := len(hub.List()); got != 1 {
		t.Fatalf("initial list: got %d, want 1", got)
	}

	// Install a skill from bundled — this should invalidate the index.
	if err := hub.Install("skill-a"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// List should reflect the installed + bundled (user overrides bundled).
	skills := hub.List()
	if len(skills) != 1 {
		t.Fatalf("after install: got %d, want 1", len(skills))
	}
	if skills[0].Source != "user" {
		t.Errorf("source = %q, want user", skills[0].Source)
	}
}

func TestDownloader(t *testing.T) {
	destDir := createTempSkillsDir(t, "downloads", nil)

	// Create a local "remote" directory that simulates a downloaded skill.
	remoteDir := t.TempDir()
	remoteSkill := filepath.Join(remoteDir, "test-remote")
	if err := os.MkdirAll(remoteSkill, 0755); err != nil {
		t.Fatal(err)
	}
	mdContent := `---
name: test-remote
description: A remote skill.
language: go
---
# Test Remote

Content.`
	if err := os.WriteFile(filepath.Join(remoteSkill, "SKILL.md"), []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test DownloadFromDir (local directory import).
	dl := NewDownloader(destDir)
	metas, err := dl.DownloadFromDir(remoteDir)
	if err != nil {
		t.Fatalf("DownloadFromDir: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("got %d metas, want 1", len(metas))
	}
	if metas[0].Name != "test-remote" {
		t.Errorf("name = %q", metas[0].Name)
	}

	// Verify the file was copied.
	targetFile := filepath.Join(destDir, "test-remote", "SKILL.md")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("reading installed SKILL.md: %v", err)
	}
	if string(data) != mdContent {
		t.Error("installed content doesn't match original")
	}
}

// -------- helpers --------

// createTempSkillsDir creates a temporary directory with skill subdirectories
// containing SKILL.md files from the provided map of name -> content.
func createTempSkillsDir(t *testing.T, prefix string, skills map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, prefix)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	for name, content := range skills {
		skillDir := filepath.Join(root, name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Replace literal \n with actual newlines for tests that use \\n.
		content = strings.ReplaceAll(content, `\n`, "\n")
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func names(metas []SkillMeta) []string {
	names := make([]string, len(metas))
	for i, m := range metas {
		names[i] = m.Name
	}
	sort.Strings(names)
	return names
}
