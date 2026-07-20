package desktop

import (
	"context"
	"time"

	"gaia/internal/core/domain"
)

// BindingAPI is the exported API surface for Wails v3 frontend bindings.
// In Wails v3, exported methods on a struct passed to app.New() are
// automatically available as JavaScript functions in the webview.
//
// Usage in Wails v3:
//
//	app := application.New(application.Options{
//	    Services: []application.Service{
//	        application.NewService(&desktop.BindingAPI{...}),
//	    },
//	})
type BindingAPI struct {
	ui      *DesktopUI
	brain   BrainPort
	ctx     context.Context
}

// BrainPort abstracts the Brain to avoid circular imports.
// Implemented by the core.Brain struct.
type BrainPort interface {
	ProcessMessage(ctx context.Context, content string) error
}

// NewBindingAPI creates the Wails binding surface.
func NewBindingAPI(ui *DesktopUI, brain BrainPort) *BindingAPI {
	return &BindingAPI{
		ui:    ui,
		brain: brain,
		ctx:   context.Background(),
	}
}

// SendMessage processes a user message through the Brain.
// Called from JavaScript: bindingAPI.SendMessage("hello")
func (b *BindingAPI) SendMessage(content string) error {
	// Store the user message locally
	b.ui.SendMessage(b.ctx, content)

	// Process through Brain (this triggers the agent loop)
	err := b.brain.ProcessMessage(b.ctx, content)
	if err != nil {
		b.ui.Display(domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Error: " + err.Error(),
		})
		return err
	}
	return nil
}

// GetHistory returns the full message history for the frontend.
// Called from JavaScript: bindingAPI.GetHistory()
func (b *BindingAPI) GetHistory() []string {
	return b.ui.GetMessages()
}

// GetMessageCount returns the number of messages.
func (b *BindingAPI) GetMessageCount() int {
	return b.ui.GetMessageCount()
}

// ConfirmResponse sends a yes/no confirmation response.
// Called from JavaScript: bindingAPI.ConfirmResponse(true)
func (b *BindingAPI) ConfirmResponse(approved bool) {
	b.ui.RespondConfirmation(approved)
}

// ClearHistory clears the message history.
func (b *BindingAPI) ClearHistory() {
	b.ui.Clear()
}

// Ping is a health check for the frontend.
func (b *BindingAPI) Ping() string {
	return "pong:" + time.Now().Format(time.RFC3339)
}
