package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Mock MessageProcessor ---

type mockProcessor struct {
	mu          sync.Mutex
	called      bool
	calledWith  string
	processFn   func(ctx context.Context, content string) error
}

func (m *mockProcessor) ProcessMessage(ctx context.Context, content string) error {
	m.mu.Lock()
	m.called = true
	m.calledWith = content
	m.mu.Unlock()
	if m.processFn != nil {
		return m.processFn(ctx, content)
	}
	return nil
}

func (m *mockProcessor) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

// --- Tests ---

func TestTUI_EnterTriggersProcessMessage(t *testing.T) {
	mock := &mockProcessor{}
	model := NewTUI()
	model.SetBrain(mock)
	model.ready = true
	model.textInput.SetValue("Hello brain")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected non-nil command from Enter key")
	}

	// Execute the returned tea.Cmd — it runs ProcessMessage.
	msg := cmd()
	if done, ok := msg.(processDoneMsg); !ok {
		t.Fatalf("expected processDoneMsg, got %T", msg)
	} else if done.err != nil {
		t.Errorf("unexpected error: %v", done.err)
	}

	if !mock.wasCalled() {
		t.Error("expected ProcessMessage to be called")
	}
}

func TestTUI_EnterProcessesBrainResponseIntoHistory(t *testing.T) {
	model := NewTUI()
	model.ready = true

	// Create a mock that simulates the real Brain: calls Display inside ProcessMessage.
	mock := &mockProcessor{
		processFn: func(ctx context.Context, content string) error {
			return model.Display(domain.Message{
				Role:    domain.RoleAssistant,
				Content: "Hello, I'm GAIA!",
			})
		},
	}
	model.SetBrain(mock)
	model.textInput.SetValue("Hi")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Run the command — ProcessMessage calls Display internally.
	result := cmd()
	if done, ok := result.(processDoneMsg); !ok {
		t.Fatalf("expected processDoneMsg, got %T", result)
	} else if done.err != nil {
		t.Errorf("unexpected error: %v", done.err)
	}

	// Feed completion message back so viewport updates.
	model.Update(result)

	// Verify the response landed in history.
	model.mu.Lock()
	defer model.mu.Unlock()

	found := false
	for _, msg := range model.history {
		if msg.Role == domain.RoleAssistant && msg.Content == "Hello, I'm GAIA!" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected assistant response in history, got %v", model.history)
	}
}

func TestTUI_PromptConfirmationAsyncChannel(t *testing.T) {
	model := NewTUI()
	model.ready = true

	// Call PromptConfirmation in a goroutine (simulating Brain's goroutine).
	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		confirmed, err := model.PromptConfirmation("Allow tool X?")
		resultCh <- confirmed
		errCh <- err
	}()

	// Give the goroutine time to set confirming=true and create the channel.
	time.Sleep(50 * time.Millisecond)

	model.mu.Lock()
	if !model.confirming {
		model.mu.Unlock()
		t.Fatal("expected confirming=true after PromptConfirmation")
	}
	model.mu.Unlock()

	// Simulate user typing "y" and pressing Enter.
	model.textInput.SetValue("y")
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Log("got cmd from confirmation Enter — running it")
		cmd()
	}

	// Wait for the PromptConfirmation goroutine to receive the answer.
	select {
	case confirmed := <-resultCh:
		if !confirmed {
			t.Error("expected true for 'y' answer")
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for PromptConfirmation to resolve")
	}

	// After resolution, confirming should be false.
	model.mu.Lock()
	if model.confirming {
		model.mu.Unlock()
		t.Error("expected confirming=false after resolution")
	} else {
		model.mu.Unlock()
	}
}

func TestTUI_PromptConfirmationDenial(t *testing.T) {
	model := NewTUI()
	model.ready = true

	resultCh := make(chan bool, 1)
	go func() {
		confirmed, _ := model.PromptConfirmation("Dangerous action?")
		resultCh <- confirmed
	}()

	time.Sleep(50 * time.Millisecond)

	// Simulate user typing "n" and pressing Enter.
	model.textInput.SetValue("n")
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	select {
	case confirmed := <-resultCh:
		if confirmed {
			t.Error("expected false for 'n' answer")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for PromptConfirmation denial")
	}
}

func TestTUI_EmptyInputDoesNothing(t *testing.T) {
	mock := &mockProcessor{}
	model := NewTUI()
	model.SetBrain(mock)
	model.ready = true
	model.textInput.SetValue("")

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if mock.wasCalled() {
		t.Error("ProcessMessage should NOT be called for empty input")
	}
	if cmd != nil {
		t.Log("cmd returned for empty input — this is fine, it just wraps textinput blink")
	}
}

func TestTUI_AppendTokenStreaming(t *testing.T) {
	model := NewTUI()
	model.ready = true

	model.AppendToken("Hello")
	model.AppendToken(" World")

	model.mu.Lock()
	defer model.mu.Unlock()
	if model.streaming != "Hello World" {
		t.Errorf("expected streaming='Hello World', got %q", model.streaming)
	}
}

func TestTUI_DisplayClearsStreaming(t *testing.T) {
	model := NewTUI()
	model.ready = true

	model.AppendToken("partial ")
	model.Display(domain.Message{Role: domain.RoleAssistant, Content: "partial final"})

	model.mu.Lock()
	defer model.mu.Unlock()
	if model.streaming != "" {
		t.Errorf("expected streaming to be cleared after Display, got %q", model.streaming)
	}

	found := false
	for _, msg := range model.history {
		if msg.Content == "partial final" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected final message in history")
	}
}
