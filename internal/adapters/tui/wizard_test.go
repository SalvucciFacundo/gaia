package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizard_ProfileStepFlow(t *testing.T) {
	wiz := NewWizard(".")

	// Set step to Language selection
	wiz.step = StepLanguageSelect
	wiz.langIndex = 1 // Spanish

	// Simulate ENTER -> should transition to StepProfileSelect
	model, _ := wiz.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wiz = model.(*WizardModel)

	if wiz.step != StepProfileSelect {
		t.Fatalf("expected step StepProfileSelect, got %v", wiz.step)
	}
	if wiz.profileIndex != 0 {
		t.Fatalf("expected default profileIndex 0, got %d", wiz.profileIndex)
	}

	// Verify View contains profile options
	viewStr := wiz.View()
	if !strings.Contains(viewStr, "DEFAULT PROFILE") {
		t.Errorf("expected view to contain 'DEFAULT PROFILE', got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "Asistente Personal (assistant)") {
		t.Errorf("expected view to contain 'Asistente Personal (assistant)', got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "Ingeniero de Software (dev)") {
		t.Errorf("expected view to contain 'Ingeniero de Software (dev)', got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "Razonador Visual (art)") {
		t.Errorf("expected view to contain 'Razonador Visual (art)', got:\n%s", viewStr)
	}

	// Navigate down to 'dev' profile (index 1)
	model, _ = wiz.Update(tea.KeyMsg{Type: tea.KeyDown})
	wiz = model.(*WizardModel)
	if wiz.profileIndex != 1 {
		t.Fatalf("expected profileIndex 1, got %d", wiz.profileIndex)
	}

	// Press ENTER -> should transition to StepSecurityMode and set selectedProfile = 'dev'
	model, _ = wiz.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wiz = model.(*WizardModel)
	if wiz.step != StepSecurityMode {
		t.Fatalf("expected step StepSecurityMode, got %v", wiz.step)
	}
	if wiz.selectedProfile != "dev" {
		t.Fatalf("expected selectedProfile 'dev', got %q", wiz.selectedProfile)
	}

	// Verify GetResults returns "dev"
	_, _, _, _, _, _, profile := wiz.GetResults()
	if profile != "dev" {
		t.Fatalf("expected GetResults() to return profile 'dev', got %q", profile)
	}
}

func TestWizard_ProfileDefault(t *testing.T) {
	wiz := NewWizard(".")
	_, _, _, _, _, _, profile := wiz.GetResults()
	if profile != "assistant" {
		t.Fatalf("expected default profile 'assistant', got %q", profile)
	}
}
