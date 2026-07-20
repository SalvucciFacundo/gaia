package tui

import (
	"sync"

	"gaia/internal/core/domain"
)

// NullUI implements ports.UIService for headless mode.
// It captures output for structured return without rendering to a TUI.
type NullUI struct {
	mu          sync.Mutex
	messages    []domain.Message
	autoApprove bool
}

// NewNullUI creates a headless UI service.
// autoApprove controls whether PromptConfirmation returns true (trust mode)
// or false (dry-run / blocking mode).
func NewNullUI(autoApprove bool) *NullUI {
	return &NullUI{
		autoApprove: autoApprove,
	}
}

// Display captures the message for later retrieval.
func (n *NullUI) Display(msg domain.Message) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, msg)
	return nil
}

// AppendToken is a no-op in headless mode — no streaming output.
func (n *NullUI) AppendToken(content string) error {
	return nil
}

// PromptConfirmation returns the configured auto-approval state.
// In trust mode (autoApprove=true), all confirmations pass.
// In dry-run or blocking mode (autoApprove=false), all are denied.
func (n *NullUI) PromptConfirmation(prompt string) (bool, error) {
	return n.autoApprove, nil
}

// Run is a no-op — headless mode has no interactive loop.
func (n *NullUI) Run() error {
	return nil
}

// LastOutput returns the content of the most recently captured message.
func (n *NullUI) LastOutput() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) == 0 {
		return ""
	}
	return n.messages[len(n.messages)-1].Content
}

// Outputs returns all captured messages.
func (n *NullUI) Outputs() []domain.Message {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]domain.Message, len(n.messages))
	copy(out, n.messages)
	return out
}
