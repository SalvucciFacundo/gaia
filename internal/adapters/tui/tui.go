package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gaia/internal/agent"
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

	taskRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D700"))

	taskDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ADADAD"))

	taskFailedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F00"))
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

// taskUpdateMsg signals that a task state has changed.
type taskUpdateMsg struct {
	state agent.TaskState
}

// waitForTaskUpdate returns a tea.Cmd that waits for the next task state.
func waitForTaskUpdate(sub <-chan agent.TaskState) tea.Cmd {
	return func() tea.Msg {
		state, ok := <-sub
		if !ok {
			return nil
		}
		return taskUpdateMsg{state: state}
	}
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

	brain       MessageProcessor
	taskManager *agent.TaskManager
	tasks       map[string]agent.TaskState // current task states by TaskID
	taskSub     <-chan agent.TaskState     // SubscribeAll channel

	// Dynamic subagent creation support
	dynamicCreator func(def agent.SubagentDef) error // nil if not configured
	toolNames      []string                          // available tool names for interview
	interview      *InterviewModel                   // nil when not in interview mode
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
		tasks:     make(map[string]agent.TaskState),
	}
}

// SetBrain wires the Brain into the TUI so user messages get processed.
func (m *Model) SetBrain(b MessageProcessor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brain = b
}

// SetDynamicCreator configures the callback for creating dynamic subagents.
// When nil, the /create-agent command is disabled.
func (m *Model) SetDynamicCreator(creator func(def agent.SubagentDef) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dynamicCreator = creator
}

// SetToolNames sets the available tool names for the interview multi-select step.
func (m *Model) SetToolNames(names []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolNames = names
}

// SetTaskManager wires the TaskManager for async task display and control.
func (m *Model) SetTaskManager(tm *agent.TaskManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskManager = tm
	if tm != nil {
		m.taskSub = tm.SubscribeAll()
	}
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.taskSub != nil {
		cmds = append(cmds, waitForTaskUpdate(m.taskSub))
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	// Route to interview when active
	if m.interview != nil {
		newModel, cmd := m.interview.Update(msg)
		if updated, ok := newModel.(*InterviewModel); ok {
			m.interview = updated
		}
		if m.interview.done {
			msg := ""
			if m.interview.err != nil {
				msg = fmt.Sprintf("Error creating subagent: %v", m.interview.err)
			}
			m.interview = nil
			if msg != "" {
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: msg,
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
			}
		}
		return m, cmd
	}

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

			// Handle /tasks — list all async tasks
			if input == "/tasks" {
				m.mu.Lock()
				if m.taskManager != nil {
					tasks := m.taskManager.ListTasks()
					var sb strings.Builder
					if len(tasks) == 0 {
						sb.WriteString("No active tasks.")
					} else {
						sb.WriteString("Active tasks:\n")
						for _, t := range tasks {
							elapsed := time.Since(t.CreatedAt).Truncate(time.Second)
							sb.WriteString(fmt.Sprintf("  [%s] %-8s @%-15s %s",
								t.TaskID[:8], t.Status, t.SubagentName, elapsed))
							if t.Error != "" {
								sb.WriteString(fmt.Sprintf(" — %s", t.Error))
							}
							sb.WriteString("\n")
						}
					}
					m.history = append(m.history, domain.Message{
						Role:    domain.RoleSystem,
						Content: sb.String(),
					})
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
				}
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}

			// Handle /cancel <taskid> — cancel an async task
			if input == "/cancel" {
				m.mu.Lock()
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: "Usage: /cancel <taskid> — cancel an async task. Use /tasks to list active tasks.",
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}
			if strings.HasPrefix(input, "/cancel ") {
				taskID := strings.TrimSpace(input[len("/cancel "):])
				m.mu.Lock()
				var response string
				if m.taskManager != nil {
					if err := m.taskManager.CancelTask(taskID); err != nil {
						response = fmt.Sprintf("Cancel failed: %v", err)
					} else {
						response = fmt.Sprintf("Task %s cancelled.", taskID[:min(8, len(taskID))])
					}
				} else {
					response = "Task manager not available."
				}
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: response,
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}

			// Handle /create-agent — start interview for dynamic subagent creation
		if input == "/create-agent" {
			m.mu.Lock()
			creator := m.dynamicCreator
			tools := make([]string, len(m.toolNames))
			copy(tools, m.toolNames)
			if creator == nil {
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: "Dynamic subagent creation is not configured.",
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}
			m.interview = NewInterviewModel(tools, creator)
			m.textInput.SetValue("")
			m.mu.Unlock()
			return m, m.interview.Init()
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
		taskPaneHeight := m.taskPaneHeight()
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6-taskPaneHeight)
			m.viewport.YPosition = 4 + taskPaneHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6 - taskPaneHeight
		}

	case processDoneMsg:
		m.mu.Lock()
		if msg.err != nil {
			m.err = msg.err
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.mu.Unlock()

	case taskUpdateMsg:
		m.mu.Lock()

		// Check if this is a terminal transition (was active, now done)
		prev, hadPrev := m.tasks[msg.state.TaskID]
		isTerminal := msg.state.Status == agent.TaskCompleted ||
			msg.state.Status == agent.TaskFailed ||
			msg.state.Status == agent.TaskCancelled

		m.tasks[msg.state.TaskID] = msg.state

		// Add notification to chat history on terminal transitions
		if isTerminal && hadPrev && prev.Status != msg.state.Status {
			var notification string
			switch msg.state.Status {
			case agent.TaskCompleted:
				notification = fmt.Sprintf("✅ Task %s — @%s completado",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName)
			case agent.TaskFailed:
				notification = fmt.Sprintf("❌ Task %s — @%s falló: %s",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName, msg.state.Error)
			case agent.TaskCancelled:
				notification = fmt.Sprintf("⛔ Task %s — @%s cancelado",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName)
			}
			m.history = append(m.history, domain.Message{
				Role:    domain.RoleAssistant,
				Content: notification,
			})
		} else if isTerminal && !hadPrev {
			// Task was created and completed before any update cycle
			var notification string
			switch msg.state.Status {
			case agent.TaskCompleted:
				notification = fmt.Sprintf("✅ Task %s — @%s completado",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName)
			case agent.TaskFailed:
				notification = fmt.Sprintf("❌ Task %s — @%s falló: %s",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName, msg.state.Error)
			case agent.TaskCancelled:
				notification = fmt.Sprintf("⛔ Task %s — @%s cancelado",
					msg.state.TaskID[:min(8, len(msg.state.TaskID))],
					msg.state.SubagentName)
			}
			m.history = append(m.history, domain.Message{
				Role:    domain.RoleAssistant,
				Content: notification,
			})
		}

		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.mu.Unlock()

		// Re-subscribe for next task update
		if m.taskSub != nil {
			return m, tea.Batch(tiCmd, vpCmd, waitForTaskUpdate(m.taskSub))
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *Model) View() string {
	// Render interview when active
	if m.interview != nil {
		return m.interview.View()
	}

	if !m.ready {
		return "\n  Inicializando GAIA..."
	}

	header := titleStyle.Render(" GAIA v0.1 ") + " " + infoStyle.Render("Go-powered Intelligence Automator")

	// Task pane
	taskPane := m.renderTaskPane()

	footer := fmt.Sprintf("\n%s\n", m.textInput.View())

	if m.confirming {
		footer = fmt.Sprintf("\n%s\n%s [y/N]: %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F00")).Render("!!! CONFIRMACIÓN REQUERIDA"),
			m.confirmMsg,
			m.textInput.View())
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s", header, taskPane, m.viewport.View(), footer)
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

// taskPaneHeight returns the number of lines the task pane occupies.
func (m *Model) taskPaneHeight() int {
	if len(m.tasks) == 0 {
		return 0
	}
	return 1 + len(m.tasks) // header + one line per task
}

// renderTaskPane builds the async task status bar.
func (m *Model) renderTaskPane() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tasks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5F5F87")).
		Render("── Tasks ──") + "\n")

	// Show only active (non-terminal) tasks
	activeCount := 0
	for _, state := range m.tasks {
		if state.Status == agent.TaskCompleted || state.Status == agent.TaskFailed || state.Status == agent.TaskCancelled {
			// Show terminal tasks briefly, then drop
			continue
		}

		elapsed := time.Since(state.CreatedAt).Truncate(time.Second)
		var statusStyle lipgloss.Style
		switch state.Status {
		case agent.TaskRunning:
			statusStyle = taskRunningStyle
		case agent.TaskFailed:
			statusStyle = taskFailedStyle
		default:
			statusStyle = taskDoneStyle
		}

		shortID := state.TaskID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		sb.WriteString(fmt.Sprintf(" %s %s %s %s\n",
			statusStyle.Render(fmt.Sprintf("[%s]", state.Status)),
			taskDoneStyle.Render(shortID),
			statusStyle.Render(fmt.Sprintf("@%s", state.SubagentName)),
			infoStyle.Render(elapsed.String()),
		))
		activeCount++
	}

	if activeCount == 0 {
		return ""
	}

	return sb.String()
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
