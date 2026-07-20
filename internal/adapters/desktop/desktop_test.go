package desktop

import (
	"context"
	"sync"
	"testing"
	"time"

	"gaia/internal/core/domain"
)

// mockBrain implements BrainPort for testing.
type mockBrain struct {
	messages []string
}

func (m *mockBrain) ProcessMessage(ctx context.Context, content string) error {
	m.messages = append(m.messages, content)
	return nil
}

func TestDesktopUIDisplay(t *testing.T) {
	ui := NewDesktopUI()

	msg := domain.Message{
		Role:    domain.RoleAssistant,
		Content: "Hello, world!",
	}

	err := ui.Display(msg)
	if err != nil {
		t.Fatalf("Display failed: %v", err)
	}

	msgs := ui.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if msgs[0] != "[assistant] Hello, world!" {
		t.Errorf("unexpected message content: %s", msgs[0])
	}
}

func TestDesktopUIAppendToken(t *testing.T) {
	ui := NewDesktopUI()

	ui.AppendToken("Hel")
	ui.AppendToken("lo")

	msgs := ui.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if msgs[0] != "[assistant] Hello" {
		t.Errorf("unexpected token accumulation: %s", msgs[0])
	}
}

func TestDesktopUISendMessage(t *testing.T) {
	ui := NewDesktopUI()
	ctx := context.Background()

	ui.SendMessage(ctx, "test input")

	msgs := ui.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if msgs[0] != "[user] test input" {
		t.Errorf("unexpected message: %s", msgs[0])
	}
}

func TestDesktopUIConfirmation(t *testing.T) {
	ui := NewDesktopUI()

	var wg sync.WaitGroup
	wg.Add(1)
	var result bool

	go func() {
		defer wg.Done()
		result, _ = ui.PromptConfirmation("Are you sure?")
	}()

	// Give goroutine time to block on PromptConfirmation.
	// In production, the frontend would have already opened the dialog
	// and the user would be interacting with it.
	time.Sleep(time.Millisecond)
	ui.RespondConfirmation(true)

	wg.Wait()
	if !result {
		t.Error("expected confirmation to be approved")
	}
}

func TestDesktopUISubscribe(t *testing.T) {
	ui := NewDesktopUI()

	ch := ui.Subscribe()
	defer ui.Unsubscribe(ch)

	msg := domain.Message{
		Role:    domain.RoleUser,
		Content: "test",
	}
	ui.Display(msg)

	select {
	case received := <-ch:
		expected := "[user] test"
		if received != expected {
			t.Errorf("expected %q, got %q", expected, received)
		}
	default:
		t.Error("expected to receive message on channel")
	}
}

func TestDesktopUIClear(t *testing.T) {
	ui := NewDesktopUI()

	ui.Display(domain.Message{Role: domain.RoleUser, Content: "msg1"})
	ui.Display(domain.Message{Role: domain.RoleUser, Content: "msg2"})

	if ui.GetMessageCount() != 2 {
		t.Fatalf("expected 2 messages before clear")
	}

	ui.Clear()

	if ui.GetMessageCount() != 0 {
		t.Errorf("expected 0 messages after clear, got %d", ui.GetMessageCount())
	}
}

func TestBindingAPISendMessage(t *testing.T) {
	ui := NewDesktopUI()
	mock := &mockBrain{}
	bindings := NewBindingAPI(ui, mock)

	err := bindings.SendMessage("hello from frontend")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if len(mock.messages) != 1 || mock.messages[0] != "hello from frontend" {
		t.Errorf("brain did not receive expected message: %v", mock.messages)
	}

	history := bindings.GetHistory()
	if len(history) == 0 {
		t.Error("expected history to contain the user message")
	}
}

func TestBindingAPIPing(t *testing.T) {
	ui := NewDesktopUI()
	mock := &mockBrain{}
	bindings := NewBindingAPI(ui, mock)

	result := bindings.Ping()
	if result[:4] != "pong" {
		t.Errorf("expected ping to start with 'pong', got %q", result)
	}
}

func TestBindingAPIConfirmResponse(t *testing.T) {
	ui := NewDesktopUI()
	mock := &mockBrain{}
	bindings := NewBindingAPI(ui, mock)

	// Test that ConfirmResponse doesn't panic when no prompt is pending
	bindings.ConfirmResponse(true)

	var wg sync.WaitGroup
	wg.Add(1)
	var result bool

	go func() {
		defer wg.Done()
		result, _ = ui.PromptConfirmation("proceed?")
	}()

	time.Sleep(time.Millisecond)
	bindings.ConfirmResponse(false)

	wg.Wait()
	if result {
		t.Error("expected confirmation to be denied")
	}
}

func TestDesktopUIGetMessageCount(t *testing.T) {
	ui := NewDesktopUI()

	if ui.GetMessageCount() != 0 {
		t.Errorf("expected 0 messages, got %d", ui.GetMessageCount())
	}

	ui.Display(domain.Message{Role: domain.RoleUser, Content: "msg"})
	if ui.GetMessageCount() != 1 {
		t.Errorf("expected 1 message, got %d", ui.GetMessageCount())
	}
}

func TestDesktopUIUnsubscribe(t *testing.T) {
	ui := NewDesktopUI()

	ch := ui.Subscribe()
	ui.Unsubscribe(ch)

	// After unsubscribe, display should not send to the removed channel
	ui.Display(domain.Message{Role: domain.RoleUser, Content: "test"})

	// Read from the closed channel — should return zero value immediately
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}
