package tui

import (
	"testing"

	"gaia/internal/core/domain"
)

func TestNullUI_DisplayCapturesMessage(t *testing.T) {
	ui := NewNullUI(true)

	msg := domain.Message{
		Role:    domain.RoleAssistant,
		Content: "Hello from headless mode",
	}
	err := ui.Display(msg)
	if err != nil {
		t.Fatalf("Display returned error: %v", err)
	}

	outputs := ui.Outputs()
	if len(outputs) != 1 {
		t.Fatalf("expected 1 captured message, got %d", len(outputs))
	}
	if outputs[0].Content != "Hello from headless mode" {
		t.Errorf("expected content 'Hello from headless mode', got %q", outputs[0].Content)
	}
}

func TestNullUI_LastOutputReturnsLatest(t *testing.T) {
	ui := NewNullUI(true)

	ui.Display(domain.Message{Role: domain.RoleAssistant, Content: "first"})
	ui.Display(domain.Message{Role: domain.RoleAssistant, Content: "second"})
	ui.Display(domain.Message{Role: domain.RoleAssistant, Content: "third"})

	last := ui.LastOutput()
	if last != "third" {
		t.Errorf("expected last output 'third', got %q", last)
	}

	outputs := ui.Outputs()
	if len(outputs) != 3 {
		t.Errorf("expected 3 captured messages, got %d", len(outputs))
	}
}

func TestNullUI_LastOutputEmpty(t *testing.T) {
	ui := NewNullUI(true)
	if out := ui.LastOutput(); out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

func TestNullUI_AppendTokenNoOp(t *testing.T) {
	ui := NewNullUI(true)
	err := ui.AppendToken("streaming token")
	if err != nil {
		t.Errorf("AppendToken should not error: %v", err)
	}
	if out := ui.LastOutput(); out != "" {
		t.Errorf("AppendToken should not capture output, got %q", out)
	}
}

func TestNullUI_RunNoOp(t *testing.T) {
	ui := NewNullUI(true)
	err := ui.Run()
	if err != nil {
		t.Errorf("Run should not error: %v", err)
	}
}

func TestNullUI_PromptConfirmationAutoApprove(t *testing.T) {
	ui := NewNullUI(true)
	confirmed, err := ui.PromptConfirmation("Allow tool X?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirmed {
		t.Error("autoApprove=true should return true")
	}
}

func TestNullUI_PromptConfirmationAutoDeny(t *testing.T) {
	ui := NewNullUI(false)
	confirmed, err := ui.PromptConfirmation("Allow tool X?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmed {
		t.Error("autoApprove=false should return false")
	}
}

func TestNullUI_ImplementsUIService(t *testing.T) {
	// Compile-time check that NullUI satisfies the interface.
	// If this compiles, the interface contract is satisfied.
	var _ uiService = NewNullUI(true)
	// Prevent unused variable warning.
	_ = uiService(nil)
}

// uiService is a local alias to verify interface satisfaction.
type uiService interface {
	Display(msg domain.Message) error
	AppendToken(content string) error
	PromptConfirmation(prompt string) (bool, error)
	Run() error
}

func TestNullUI_ThreadSafety(t *testing.T) {
	ui := NewNullUI(true)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			ui.Display(domain.Message{Role: domain.RoleAssistant, Content: "concurrent"})
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = ui.LastOutput()
	}

	<-done

	// After concurrent writes, outputs should not be corrupted.
	outputs := ui.Outputs()
	if len(outputs) != 100 {
		t.Errorf("expected 100 messages after concurrent writes, got %d", len(outputs))
	}
}
