package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gaia/internal/skills"
)

func setupTestSkillEnv(t *testing.T) (string, string) {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("GAIA_HOME", homeDir)

	skillsDir := filepath.Join(homeDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("creating test skills dir: %v", err)
	}

	return homeDir, skillsDir
}

func createSampleSkill(t *testing.T, skillsDir, name, trigger, desc string) {
	t.Helper()
	skillDir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("creating sample skill dir: %v", err)
	}

	content := `---
name: ` + name + `
description: ` + desc + `
trigger: ` + trigger + `
license: Apache-2.0
metadata:
  author: tester
  version: "1.0"
---

## When to Use
Use when executing ` + name + `.

## Critical Rules
1. Must follow standard protocol.

## Workflow
1. Execute step one.
2. Execute step two.

## Examples
Example execution of ` + name + `.
`
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing sample skill: %v", err)
	}
}

func TestRunSkillCLI_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "empty args", args: []string{}},
		{name: "help flag", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "help command", args: []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunSkillCLI(tt.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage: gaia skill") {
				t.Errorf("stdout missing usage info: %s", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunSkillCLI_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunSkillCLI([]string{"unknown_cmd"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr = %q, want unknown subcommand error", stderr.String())
	}
}

func TestRunSkillCLI_List(t *testing.T) {
	_, skillsDir := setupTestSkillEnv(t)

	t.Run("empty state", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"list"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "No skills found") {
			t.Errorf("unexpected output for empty list: %s", stdout.String())
		}
	})

	createSampleSkill(t, skillsDir, "git-rebase", "rebase", "Interactive git rebase guide")
	createSampleSkill(t, skillsDir, "docker-build", "docker", "Container build procedure")

	t.Run("table output", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"list"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "git-rebase") || !strings.Contains(out, "docker-build") {
			t.Errorf("stdout missing skills: %s", out)
		}
		if !strings.Contains(out, "NAME") || !strings.Contains(out, "TRIGGER") || !strings.Contains(out, "STATUS") {
			t.Errorf("stdout missing table headers: %s", out)
		}
	})

	t.Run("json output", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"list", "--json"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		var list []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Trigger     string `json:"trigger"`
			Source      string `json:"source"`
			Status      string `json:"status"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
			t.Fatalf("unmarshal json error: %v (%s)", err, stdout.String())
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 skills, got %d", len(list))
		}
	})
}

func TestRunSkillCLI_Create(t *testing.T) {
	_, skillsDir := setupTestSkillEnv(t)

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"create"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2 (stderr: %s)", code, stderr.String())
		}
	})

	t.Run("invalid name fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"create", "invalid name with spaces!"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2 (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "invalid skill name") {
			t.Errorf("expected invalid name message in stderr, got %q", stderr.String())
		}
	})

	t.Run("create standard skill success", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{
			"create", "k8s-deploy",
			"--desc", "Deploy services to Kubernetes cluster",
			"--trigger", "k8s,deploy",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}

		skillFile := filepath.Join(skillsDir, "k8s-deploy", "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			t.Fatalf("reading created skill file: %v", err)
		}

		if err := skills.ValidateSkillContent(string(data)); err != nil {
			t.Fatalf("created skill failed validation: %v\ncontent:\n%s", err, string(data))
		}
	})

	t.Run("create existing without overwrite fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{
			"create", "k8s-deploy",
			"--desc", "Duplicate",
		}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "already exists") {
			t.Errorf("expected 'already exists' in stderr, got: %s", stderr.String())
		}
	})

	t.Run("create existing with overwrite succeeds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{
			"create", "k8s-deploy",
			"--desc", "Updated description",
			"--overwrite",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
	})
}

func TestRunSkillCLI_Show(t *testing.T) {
	_, skillsDir := setupTestSkillEnv(t)
	createSampleSkill(t, skillsDir, "code-review", "review", "Conduct thorough code reviews")

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"show"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})

	t.Run("non-existent skill fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"show", "nonexistent"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "not found") {
			t.Errorf("expected not found error in stderr, got: %s", stderr.String())
		}
	})

	t.Run("show default formatted", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"show", "code-review"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "code-review") || !strings.Contains(out, "When to Use") {
			t.Errorf("stdout missing skill details: %s", out)
		}
	})

	t.Run("show raw file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"show", "code-review", "--raw"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		out := stdout.String()
		if !strings.HasPrefix(strings.TrimSpace(out), "---") {
			t.Errorf("expected frontmatter in raw output: %s", out)
		}
	})

	t.Run("show json", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"show", "code-review", "--json"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		var skill skills.Skill
		if err := json.Unmarshal(stdout.Bytes(), &skill); err != nil {
			t.Fatalf("unmarshal json error: %v (%s)", err, stdout.String())
		}
		if skill.Meta.Name != "code-review" {
			t.Errorf("expected name code-review, got %s", skill.Meta.Name)
		}
	})
}

func TestRunSkillCLI_Delete(t *testing.T) {
	_, skillsDir := setupTestSkillEnv(t)
	createSampleSkill(t, skillsDir, "to-delete", "temp", "Temporary skill to delete")

	t.Run("missing name argument fails with code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"delete"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})

	t.Run("non-existent skill fails with code 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunSkillCLI([]string{"delete", "unknown", "-f"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})

	t.Run("delete interactive decline", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("n\n")
		code := RunSkillCLIWithIn([]string{"delete", "to-delete"}, stdin, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(strings.ToLower(stdout.String()), "cancel") {
			t.Errorf("unexpected output on cancel: %s", stdout.String())
		}
		// Confirm file still exists
		if _, err := os.Stat(filepath.Join(skillsDir, "to-delete")); os.IsNotExist(err) {
			t.Errorf("skill directory was deleted despite negative confirmation")
		}
	})

	t.Run("delete interactive confirm", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("y\n")
		code := RunSkillCLIWithIn([]string{"delete", "to-delete"}, stdin, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		// Confirm file is deleted
		if _, err := os.Stat(filepath.Join(skillsDir, "to-delete")); !os.IsNotExist(err) {
			t.Errorf("skill directory still exists after delete")
		}
	})
}
