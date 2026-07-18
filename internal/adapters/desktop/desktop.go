// Package desktop provides a Wails-compatible UI adapter for GAIA.
// It implements ports.UIService and exposes Brain methods for a webview frontend.
// The frontend communicates through the Bindings API (bindings.go).
package desktop

import (
	"context"
	"fmt"
	"sync"

	"gaia/internal/core/domain"
)

// DesktopUI implements ports.UIService with an event-driven bridge
// for Wails v3 frontend communication via Go↔JS bindings.
type DesktopUI struct {
	mu           sync.Mutex
	messages     []string      // ordered message history
	listeners    []chan string // event listeners for frontend push
	confirmChan  chan bool     // channel for confirmation responses (set per prompt)
	confirmMutex sync.Mutex    // guards confirmChan
}

// NewDesktopUI creates a new DesktopUI adapter.
func NewDesktopUI() *DesktopUI {
	return &DesktopUI{
		messages: make([]string, 0),
	}
}

// Display records a message and notifies listeners. Implements ports.UIService.
func (d *DesktopUI) Display(msg domain.Message) error {
	content := fmt.Sprintf("[%s] %s", msg.Role, msg.Content)
	d.mu.Lock()
	d.messages = append(d.messages, content)
	listeners := make([]chan string, len(d.listeners))
	copy(listeners, d.listeners)
	d.mu.Unlock()

	// Non-blocking notify all listeners
	for _, ch := range listeners {
		select {
		case ch <- content:
		default:
		}
	}
	return nil
}

// AppendToken streams a token fragment. Implements ports.UIService.
func (d *DesktopUI) AppendToken(content string) error {
	// Accumulate in last message or create new token stream entry.
	d.mu.Lock()
	if len(d.messages) > 0 {
		last := d.messages[len(d.messages)-1]
		if len(last) > 0 && last[0] == '[' {
			// Token stream in progress — append to last message
			d.messages[len(d.messages)-1] = last + content
		} else {
			d.messages = append(d.messages, "[assistant] "+content)
		}
	} else {
		d.messages = append(d.messages, "[assistant] "+content)
	}
	listeners := make([]chan string, len(d.listeners))
	copy(listeners, d.listeners)
	d.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- content:
		default:
		}
	}
	return nil
}

// PromptConfirmation blocks until a response is received from the frontend.
// Implements ports.UIService.
func (d *DesktopUI) PromptConfirmation(prompt string) (bool, error) {
	d.confirmMutex.Lock()
	ch := make(chan bool, 1)
	d.confirmChan = ch
	d.confirmMutex.Unlock()

	// Notify listeners about the confirmation request
	d.mu.Lock()
	confirmMsg := "[confirmation] " + prompt
	d.messages = append(d.messages, confirmMsg)
	listeners := make([]chan string, len(d.listeners))
	copy(listeners, d.listeners)
	d.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- confirmMsg:
		default:
		}
	}

	// Wait for response
	select {
	case result := <-ch:
		return result, nil
	case <-context.Background().Done():
		return false, fmt.Errorf("confirmation cancelled")
	}
}

// Run starts the desktop UI. For Wails, the app.Run() call is in cmd/gaia/desktop.go.
// Implements ports.UIService.
func (d *DesktopUI) Run() error {
	// Desktop mode: the Wails app lifecycle is managed externally.
	// This method is a no-op — the frontend calls bindings directly.
	return nil
}

// --- Public API for Wails Bindings ---

// GetMessages returns the full message history for the frontend.
func (d *DesktopUI) GetMessages() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]string, len(d.messages))
	copy(result, d.messages)
	return result
}

// GetMessageCount returns the number of messages.
func (d *DesktopUI) GetMessageCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.messages)
}

// SendMessage processes a user message through the Brain.
// Call this from the frontend when the user sends input.
func (d *DesktopUI) SendMessage(ctx context.Context, content string) error {
	// Store the user message
	d.mu.Lock()
	d.messages = append(d.messages, "[user] "+content)
	listeners := make([]chan string, len(d.listeners))
	copy(listeners, d.listeners)
	d.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- content:
		default:
		}
	}
	return nil
}

// RespondConfirmation sends a yes/no response to a pending confirmation prompt.
func (d *DesktopUI) RespondConfirmation(approved bool) {
	d.confirmMutex.Lock()
	defer d.confirmMutex.Unlock()
	if d.confirmChan != nil {
		d.confirmChan <- approved
		d.confirmChan = nil
	}
}

// Subscribe returns a channel that receives new message events.
// The frontend calls this to get real-time updates.
func (d *DesktopUI) Subscribe() chan string {
	ch := make(chan string, 100)
	d.mu.Lock()
	d.listeners = append(d.listeners, ch)
	d.mu.Unlock()
	return ch
}

// Unsubscribe removes a listener channel.
func (d *DesktopUI) Unsubscribe(ch chan string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, listener := range d.listeners {
		if listener == ch {
			d.listeners = append(d.listeners[:i], d.listeners[i+1:]...)
			close(ch)
			return
		}
	}
}

// Clear clears the message history.
func (d *DesktopUI) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messages = make([]string, 0)
}
