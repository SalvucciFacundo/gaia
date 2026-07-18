package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"gaia/internal/core/domain"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#212121")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ADADAD")).
			Italic(true)

	userStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D7FF"))

	aiStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF00D7"))
)

// MessageProcessor is the interface the TUI uses to send user input
// to the Brain for processing. It runs asynchronously so the TUI
// stays responsive during LLM calls.
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, content string) error
}

// processDoneMsg signals that an async ProcessMessage call finished.
type processDoneMsg struct {
	err error
}

type Model struct {
	mu sync.Mutex

	viewport   viewport.Model
	textInput  textinput.Model
	history    []domain.Message
	err        error
	ready      bool
	confirming bool
	confirmMsg string
	confirmCh  chan bool
	streaming  string // current in-progress AI response text

	brain MessageProcessor
}

func NewTUI() *Model {
	ti := textinput.New()
	ti.Placeholder = "Talk to GAIA..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	return &Model{
		textInput: ti,
		history:   []domain.Message{},
	}
}

// SetBrain wires the Brain into the TUI so user messages get processed.
func (m *Model) SetBrain(b MessageProcessor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brain = b
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			// Confirmation prompt mode — resolve the channel.
			if m.confirming {
				val := strings.ToLower(m.textInput.Value())
				m.mu.Lock()
				m.confirming = false
				ch := m.confirmCh
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Talk to GAIA..."
				m.mu.Unlock()
				if ch != nil {
					ch <- (val == "y" || val == "s")
				}
				return m, nil
			}

			input := m.textInput.Value()
			if input == "" {
				return m, nil
			}

			// Handle /trust slash commands
			if strings.HasPrefix(input, "/trust") {
				parts := strings.Fields(input)
				mode := "always"
				if len(parts) > 1 {
					mode = parts[1]
				}
				m.mu.Lock()
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: fmt.Sprintf("Trust mode set to: %s", mode),
				})
				m.textInput.SetValue("")
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.mu.Unlock()
				return m, nil
			}

			// Normal message — append user message and dispatch Brain call.
			m.mu.Lock()
			m.history = append(m.history, domain.Message{
				Role:    domain.RoleUser,
				Content: input,
			})
			m.textInput.SetValue("")
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			brainCopy := m.brain
			m.mu.Unlock()

			// Return a command that runs ProcessMessage asynchronously.
			if brainCopy != nil {
				return m, tea.Batch(tiCmd, vpCmd, func() tea.Msg {
					ctx := context.Background()
					err := brainCopy.ProcessMessage(ctx, input)
					return processDoneMsg{err: err}
				})
			}
			return m, tea.Batch(tiCmd, vpCmd)

		}

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.viewport.YPosition = 4
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}

	case processDoneMsg:
		m.mu.Lock()
		if msg.err != nil {
			m.err = msg.err
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.mu.Unlock()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *Model) View() string {
	if !m.ready {
		return "\n  Inicializando GAIA..."
	}

	header := titleStyle.Render(" GAIA v0.1 ") + " " + infoStyle.Render("Go-powered Intelligence Automator")
	footer := fmt.Sprintf("\n%s\n", m.textInput.View())

	if m.confirming {
		footer = fmt.Sprintf("\n%s\n%s [y/N]: %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F00")).Render("!!! CONFIRMACIÓN REQUERIDA"),
			m.confirmMsg,
			m.textInput.View())
	}

	return fmt.Sprintf("%s\n\n%s\n%s", header, m.viewport.View(), footer)
}

// AppendToken adds a streaming token to the current assistant message.
// Called from the Brain's goroutine during streaming.
func (m *Model) AppendToken(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streaming += content
	m.viewport.SetContent(m.renderHistory())
	return nil
}

// Display persists a complete message to history.
// Called from the Brain's goroutine when a response is ready.
func (m *Model) Display(msg domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, msg)
	m.streaming = ""
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return nil
}

// PromptConfirmation blocks until the user responds y/n via the TUI.
// It is called from the Brain's goroutine and returns asynchronously
// by waiting on a channel that the Bubbletea Update loop resolves.
func (m *Model) PromptConfirmation(prompt string) (bool, error) {
	m.mu.Lock()
	m.confirming = true
	m.confirmMsg = prompt
	m.textInput.SetValue("")
	m.textInput.Placeholder = "y/n"
	ch := make(chan bool, 1)
	m.confirmCh = ch
	m.mu.Unlock()

	confirmed := <-ch

	m.mu.Lock()
	m.confirmCh = nil
	m.textInput.Placeholder = "Talk to GAIA..."
	m.mu.Unlock()
	return confirmed, nil
}

func (m *Model) renderHistory() string {
	var sb strings.Builder
	for _, msg := range m.history {
		prefix := userStyle.Render("USER > ")
		if msg.Role == domain.RoleAssistant {
			prefix = aiStyle.Render("GAIA > ")
		}
		sb.WriteString(prefix + msg.Content + "\n\n")
	}
	// Show streaming content if present
	if m.streaming != "" {
		sb.WriteString(aiStyle.Render("GAIA > ") + m.streaming + "\n")
	}
	return sb.String()
}

func (m *Model) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
