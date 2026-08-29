package tui

import (
	"strings"
	"testing"

	"gaia/internal/diff"

	tea "github.com/charmbracelet/bubbletea"
)

func createSampleDiffModel() DiffViewerModel {
	files := []diff.DiffFile{
		{
			OldPath: "file1.go",
			NewPath: "file1.go",
			Hunks: []diff.DiffHunk{
				{
					Header:   "@@ -1,3 +1,3 @@",
					OldStart: 1,
					OldLines: 3,
					NewStart: 1,
					NewLines: 3,
					Lines: []diff.DiffLine{
						{Type: diff.LineContext, Content: "package main", OldLine: 1, NewLine: 1},
						{Type: diff.LineDeletion, Content: "func old()", OldLine: 2, NewLine: 0},
						{Type: diff.LineAddition, Content: "func new()", OldLine: 0, NewLine: 2},
					},
				},
				{
					Header:   "@@ -10,3 +10,3 @@",
					OldStart: 10,
					OldLines: 3,
					NewStart: 10,
					NewLines: 3,
					Lines: []diff.DiffLine{
						{Type: diff.LineContext, Content: "func test()", OldLine: 10, NewLine: 10},
						{Type: diff.LineAddition, Content: "var added = 1", OldLine: 0, NewLine: 11},
					},
				},
			},
		},
		{
			OldPath: "file2.go",
			NewPath: "file2.go",
			Hunks: []diff.DiffHunk{
				{
					Header:   "@@ -5,2 +5,3 @@",
					OldStart: 5,
					OldLines: 2,
					NewStart: 5,
					NewLines: 3,
					Lines: []diff.DiffLine{
						{Type: diff.LineContext, Content: "package util", OldLine: 5, NewLine: 5},
						{Type: diff.LineAddition, Content: "const Pi = 3.14", OldLine: 0, NewLine: 6},
					},
				},
			},
		},
	}

	m := NewDiffViewerModel(files, 80, 24)
	return m
}

func TestDiffViewerModel_Navigation(t *testing.T) {
	m := createSampleDiffModel()

	if m.FocusedFile != 0 || m.FocusedHunk != 0 {
		t.Fatalf("expected initial focus (0, 0), got (%d, %d)", m.FocusedFile, m.FocusedHunk)
	}

	// Press 'n' -> next hunk in file 0
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updatedModel.(DiffViewerModel)
	if m.FocusedFile != 0 || m.FocusedHunk != 1 {
		t.Errorf("expected focus (0, 1), got (%d, %d)", m.FocusedFile, m.FocusedHunk)
	}

	// Press 'n' again -> crosses to file 1, hunk 0
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updatedModel.(DiffViewerModel)
	if m.FocusedFile != 1 || m.FocusedHunk != 0 {
		t.Errorf("expected focus (1, 0), got (%d, %d)", m.FocusedFile, m.FocusedHunk)
	}

	// Press 'p' -> back to file 0, hunk 1
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updatedModel.(DiffViewerModel)
	if m.FocusedFile != 0 || m.FocusedHunk != 1 {
		t.Errorf("expected focus (0, 1), got (%d, %d)", m.FocusedFile, m.FocusedHunk)
	}
}

func TestDiffViewerModel_Exit(t *testing.T) {
	m := createSampleDiffModel()

	// Press 'q' -> should emit DiffCloseMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected command on 'q'")
	}
	msg := cmd()
	if _, ok := msg.(DiffCloseMsg); !ok {
		t.Errorf("expected DiffCloseMsg, got %T", msg)
	}

	// Press 'esc' -> should emit DiffCloseMsg
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg = cmd()
	if _, ok := msg.(DiffCloseMsg); !ok {
		t.Errorf("expected DiffCloseMsg, got %T", msg)
	}
}

func TestDiffViewerModel_SteeringFlow(t *testing.T) {
	m := createSampleDiffModel()

	// Press 'e' -> enters ModeSteering
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updatedModel.(DiffViewerModel)
	if m.Mode != ModeSteering {
		t.Fatalf("expected ModeSteering, got %v", m.Mode)
	}

	// Type steering feedback
	m.SteeringInput.SetValue("Refactor this hunk to use errors.Is")

	// Press Enter -> submits DiffSteeringMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on submit steering")
	}
	msg := cmd()
	steeringMsg, ok := msg.(DiffSteeringMsg)
	if !ok {
		t.Fatalf("expected DiffSteeringMsg, got %T", msg)
	}

	if steeringMsg.FilePath != "file1.go" {
		t.Errorf("expected FilePath 'file1.go', got %q", steeringMsg.FilePath)
	}
	if steeringMsg.Feedback != "Refactor this hunk to use errors.Is" {
		t.Errorf("expected Feedback 'Refactor this hunk to use errors.Is', got %q", steeringMsg.Feedback)
	}
	if !strings.Contains(steeringMsg.DiffSnippet, "func old()") {
		t.Errorf("expected diff snippet to contain hunk contents, got %q", steeringMsg.DiffSnippet)
	}
}

func TestDiffViewerModel_StagingActions(t *testing.T) {
	m := createSampleDiffModel()

	// Mock git apply runner
	var executedPatch string
	var executedFlags []string
	m.GitApplier = func(workDir string, patch string, flags ...string) error {
		executedPatch = patch
		executedFlags = flags
		return nil
	}

	// Stage hunk with 's'
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updatedModel.(DiffViewerModel)

	if !m.Files[0].Hunks[0].IsStaged {
		t.Errorf("expected hunk to be marked staged")
	}
	if !strings.Contains(executedPatch, "func old()") {
		t.Errorf("expected executed patch to contain hunk content: %s", executedPatch)
	}
	if len(executedFlags) == 0 || executedFlags[0] != "--cached" {
		t.Errorf("expected --cached flag for staging, got %v", executedFlags)
	}

	// Unstage hunk with 'u'
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = updatedModel.(DiffViewerModel)

	if m.Files[0].Hunks[0].IsStaged {
		t.Errorf("expected hunk to be unstaged")
	}
}

func TestDiffViewerModel_ViewRendering(t *testing.T) {
	m := createSampleDiffModel()
	view := m.View()

	if !strings.Contains(view, "file1.go") {
		t.Errorf("view should render file1.go: %s", view)
	}
	if !strings.Contains(view, "GAIA Interactive Diff Viewer") {
		t.Errorf("view should render header title: %s", view)
	}
}
