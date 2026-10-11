package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gaia/internal/skills"
)

type fakeSkillHub struct {
	skills map[string]*skills.Skill
	used   []string
}

func newFakeSkillHub() *fakeSkillHub {
	return &fakeSkillHub{
		skills: make(map[string]*skills.Skill),
	}
}

func (f *fakeSkillHub) GetSkill(name string) (*skills.Skill, bool) {
	s, ok := f.skills[name]
	if !ok {
		return nil, false
	}
	return s, true
}

func (f *fakeSkillHub) RecordUse(name string) {
	f.used = append(f.used, name)
}

func TestReadSkillRunner_Metadata(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewReadSkillRunner(hub)

	if runner.Name() != "read_skill" {
		t.Errorf("Name() = %q, want 'read_skill'", runner.Name())
	}
	if runner.Backend() != "internal" {
		t.Errorf("Backend() = %q, want 'internal'", runner.Backend())
	}
	if runner.Description() == "" {
		t.Errorf("Description() is empty")
	}
}

func TestReadSkillRunner_Found(t *testing.T) {
	hub := newFakeSkillHub()
	hub.skills["golang-testing"] = &skills.Skill{
		Meta: skills.SkillMeta{
			Name:        "golang-testing",
			Description: "Run unit tests with race detector",
			Trigger:     "test",
		},
		Content: "## Workflow\n1. Run go test -race ./...",
	}

	runner := NewReadSkillRunner(hub)
	out, err := runner.Run(context.Background(), `{"name": "golang-testing"}`)
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}

	var parsed struct {
		Status      string `json:"status"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Trigger     string `json:"trigger"`
		Content     string `json:"content"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v, raw output: %s", err, out)
	}

	if parsed.Status != "found" {
		t.Errorf("status = %q, want 'found'", parsed.Status)
	}
	if parsed.Name != "golang-testing" {
		t.Errorf("name = %q, want 'golang-testing'", parsed.Name)
	}
	if parsed.Description != "Run unit tests with race detector" {
		t.Errorf("description = %q", parsed.Description)
	}
	if parsed.Trigger != "test" {
		t.Errorf("trigger = %q, want 'test'", parsed.Trigger)
	}
	if parsed.Content != "## Workflow\n1. Run go test -race ./..." {
		t.Errorf("content = %q", parsed.Content)
	}

	if len(hub.used) != 1 || hub.used[0] != "golang-testing" {
		t.Errorf("hub.used = %v, want ['golang-testing']", hub.used)
	}
}

func TestReadSkillRunner_NotFound(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewReadSkillRunner(hub)

	_, err := runner.Run(context.Background(), `{"name": "nonexistent"}`)
	if err == nil {
		t.Fatalf("Run() expected error for nonexistent skill, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestReadSkillRunner_EmptyName(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewReadSkillRunner(hub)

	_, err := runner.Run(context.Background(), `{"name": ""}`)
	if err == nil {
		t.Fatalf("Run() expected error for empty name, got nil")
	}

	_, err = runner.Run(context.Background(), `{}`)
	if err == nil {
		t.Fatalf("Run() expected error for missing name, got nil")
	}
}

func TestReadSkillRunner_InvalidJSON(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewReadSkillRunner(hub)

	_, err := runner.Run(context.Background(), `{invalid-json`)
	if err == nil {
		t.Fatalf("Run() expected error for invalid JSON, got nil")
	}
}

func TestReadSkillRunner_InvalidName(t *testing.T) {
	hub := newFakeSkillHub()
	runner := NewReadSkillRunner(hub)

	_, err := runner.Run(context.Background(), `{"name": "../bad/name"}`)
	if err == nil {
		t.Fatalf("Run() expected error for bad name, got nil")
	}
}
