package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"gaia/internal/core/domain"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// processorStub simulates the Brain for TUI integration testing.
// When ProcessMessage is called, it stores the AI response in the TUI
// via Display(), mimicking the real Brain → TUI data flow.
type processorStub struct {
	model    *Model
	response string
	calls    []string
}

func (p *processorStub) ProcessMessage(ctx context.Context, content string) error {
	p.calls = append(p.calls, content)
	return p.model.Display(domain.Message{
		Role:    domain.RoleAssistant,
		Content: p.response,
	})
}

// TestIntegration_TUIToBrainToMockLLM verifies the full TUI → Brain → LLM flow.
// It uses teatest to run the Bubbletea program, simulates keystrokes,
// and verifies output appears. Because bubbletea rendering uses terminal
// escape codes, we verify via WaitFor on the output stream and a
// model-level assertion after quit.
func TestIntegration_TUIToBrainToMockLLM(t *testing.T) {
	model := NewTUI()
	proc := &processorStub{
		model:    model,
		response: "Hello, I am GAIA!",
	}
	model.SetBrain(proc)

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Send WindowSizeMsg to set ready=true explicitly.
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Give the program a moment to render the initial frame.
	time.Sleep(100 * time.Millisecond)

	// Simulate user typing "hi" and pressing Enter.
	tm.Type("hi")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait for the response text in the output stream.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Hello, I am GAIA!")
	}, teatest.WithDuration(8*time.Second), teatest.WithCheckInterval(100*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	// Verify the processor was called with the right input.
	if len(proc.calls) < 1 {
		t.Error("expected at least 1 ProcessMessage call")
	} else if proc.calls[0] != "hi" {
		t.Errorf("expected 'hi', got %q", proc.calls[0])
	}

	// Verify via model state (most reliable cross-platform check).
	model.mu.Lock()
	defer model.mu.Unlock()
	found := false
	for _, msg := range model.history {
		if msg.Content == "Hello, I am GAIA!" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AI response 'Hello, I am GAIA!' in model history")
	}
}

// TestIntegration_TUIStreamingReceivesTokens verifies that streaming tokens
// are rendered in the TUI via AppendToken.
func TestIntegration_TUIStreamingReceivesTokens(t *testing.T) {
	model := NewTUI()

	model.AppendToken("Streaming ")
	model.AppendToken("response ")
	model.AppendToken("in progress")

	model.mu.Lock()
	streaming := model.streaming
	model.mu.Unlock()

	if streaming != "Streaming response in progress" {
		t.Errorf("expected concatenated streaming text, got %q", streaming)
	}
}

// TestIntegration_TUIMultiTurnConversation verifies multiple user turns
// accumulate in the history, testing the TUI → processor → TUI feedback loop.
func TestIntegration_TUIMultiTurnConversation(t *testing.T) {
	model := NewTUI()
	model.ready = true

	proc := &processorStub{model: model}
	model.SetBrain(proc)

	// --- First turn ---
	proc.response = "First response."
	model.textInput.SetValue("hello")
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil command from Enter")
	}
	result := cmd()
	if _, ok := result.(processDoneMsg); !ok {
		t.Fatalf("expected processDoneMsg, got %T", result)
	}
	model.Update(result)

	// --- Second turn ---
	proc.response = "Second response."
	model.textInput.SetValue("how are you")
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil command from second Enter")
	}
	result = cmd()
	if _, ok := result.(processDoneMsg); !ok {
		t.Fatalf("expected processDoneMsg, got %T", result)
	}
	model.Update(result)

	// Verify history.
	model.mu.Lock()
	defer model.mu.Unlock()

	if len(model.history) != 4 {
		t.Errorf("expected 4 messages (2 user + 2 assistant), got %d", len(model.history))
	}
	if model.history[0].Content != "hello" {
		t.Errorf("expected first user msg 'hello', got %q", model.history[0].Content)
	}
	if model.history[1].Content != "First response." {
		t.Errorf("expected first AI response, got %q", model.history[1].Content)
	}
	if model.history[2].Content != "how are you" {
		t.Errorf("expected second user msg, got %q", model.history[2].Content)
	}
	if model.history[3].Content != "Second response." {
		t.Errorf("expected second AI response, got %q", model.history[3].Content)
	}

	if len(proc.calls) != 2 {
		t.Errorf("expected 2 ProcessMessage calls, got %d", len(proc.calls))
	}
}

// TestIntegration_TUIViewRendersHistory verifies the View() method
// produces expected content for both user and assistant messages.
func TestIntegration_TUIViewRendersHistory(t *testing.T) {
	model := NewTUI()
	// Send WindowSizeMsg so the viewport gets created with proper dimensions.
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Now model.ready is true and viewport has Width/Height.
	model.viewport.SetContent("USER > What is Go?\n\nGAIA > Go is a programming language.\n\n")

	view := model.View()
	if !strings.Contains(view, "What is Go?") {
		t.Errorf("expected user message in view, got: %q", view)
	}
	if !strings.Contains(view, "Go is a programming language.") {
		t.Errorf("expected assistant message in view, got: %q", view)
	}
}

// TestIntegration_TUIProcessorReceivesInput verifies that the mock processor
// correctly receives user input via the TUI's MessageProcessor interface.
func TestIntegration_TUIProcessorReceivesInput(t *testing.T) {
	model := NewTUI()
	model.ready = true

	proc := &processorStub{model: model, response: "OK"}
	model.SetBrain(proc)

	model.textInput.SetValue("test message")
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil command for non-empty input")
	}

	result := cmd()
	if done, ok := result.(processDoneMsg); !ok {
		t.Fatalf("expected processDoneMsg, got %T", result)
	} else if done.err != nil {
		t.Errorf("unexpected ProcessMessage error: %v", done.err)
	}
	model.Update(result)

	if len(proc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(proc.calls))
	}
	if proc.calls[0] != "test message" {
		t.Errorf("expected 'test message', got %q", proc.calls[0])
	}

	model.mu.Lock()
	defer model.mu.Unlock()
	found := false
	for _, msg := range model.history {
		if msg.Role == domain.RoleAssistant && msg.Content == "OK" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected assistant response 'OK' in history")
	}
}
