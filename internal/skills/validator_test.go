package skills

import (
	"strings"
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "git-rebase", wantErr: false},
		{name: "valid underscores", input: "docker_build_123", wantErr: false},
		{name: "valid single char", input: "a", wantErr: false},
		{name: "valid 64 chars", input: strings.Repeat("a", 64), wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "too long (65 chars)", input: strings.Repeat("a", 65), wantErr: true},
		{name: "path traversal slash", input: "foo/bar", wantErr: true},
		{name: "path traversal backslash", input: "foo\\bar", wantErr: true},
		{name: "path traversal dotdot", input: "../foo", wantErr: true},
		{name: "dotdot in name", input: "foo..bar", wantErr: true},
		{name: "special characters", input: "skill@name", wantErr: true},
		{name: "space in name", input: "skill name", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkillName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSkillSections(t *testing.T) {
	validBody := `## When to Use
Use when building containers.

## Critical Rules
1. Do not run as root.

## Workflow
1. Run docker build.

## Examples
docker build -t app .
`
	if err := ValidateSkillSections(validBody); err != nil {
		t.Fatalf("unexpected error for valid body: %v", err)
	}

	// Missing section
	invalidBody := `## Workflow
1. Only workflow here.
`
	if err := ValidateSkillSections(invalidBody); err == nil {
		t.Fatal("expected error for body missing sections, got nil")
	}
}

func TestValidateSkillContent(t *testing.T) {
	validContent := `---
name: container-deploy
description: Deploy container to registry
---

## When to Use
Deploying images.

## Critical Rules
Always verify signature.

## Workflow
1. Build.
2. Sign.
3. Push.

## Examples
cosign sign ...
`
	if err := ValidateSkillContent(validContent); err != nil {
		t.Fatalf("unexpected error for valid content: %v", err)
	}

	// Missing frontmatter
	if err := ValidateSkillContent("Just markdown"); err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}
