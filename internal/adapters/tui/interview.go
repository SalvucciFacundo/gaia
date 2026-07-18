package tui

import (
	"fmt"
	"strings"

	"gaia/internal/agent"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// interviewStep represents the current phase in the subagent creation interview.
type interviewStep int

const (
	stepName        interviewStep = iota
	stepDescription
	stepTools
	stepSkills
	stepPersonality
	stepConfirm
	stepDone
)

// InterviewModel is a Bubbletea sub-model that walks the user through
// creating a dynamic subagent via a step-by-step interview.
type InterviewModel struct {
	textInput textinput.Model
	step      interviewStep
	answers   interviewAnswers
	width     int
	height    int
	done      bool
	err       error

	// Available tool names for the multi-select step.
	availableTools []string
	// Selected tools (toggled during stepTools).
	selectedTools map[string]bool
	// Callback invoked when the interview is complete.
	onComplete func(def agent.SubagentDef) error
}

type interviewAnswers struct {
	name        string
	description string
	tools       []string
	skills      []string
	personality string
}

var (
	interviewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	interviewPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D7FF"))

	interviewHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ADADAD")).
				Italic(true)

	interviewErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5F00"))

	interviewSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D700")).
				Bold(true)
)

// NewInterviewModel creates an interview model for creating a dynamic subagent.
// availableTools is the list of tool names from the ToolRegistry for the
// multi-select step. onComplete is called when the user confirms.
func NewInterviewModel(availableTools []string, onComplete func(def agent.SubagentDef) error) *InterviewModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	return &InterviewModel{
		textInput:      ti,
		step:           stepName,
		availableTools: availableTools,
		selectedTools:  make(map[string]bool),
		onComplete:     onComplete,
	}
}

func (m *InterviewModel) Init() tea.Cmd {
	m.textInput.Placeholder = "Enter subagent name (lowercase, no spaces)"
	return textinput.Blink
}

func (m *InterviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, tea.Quit
	}

	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.done = true
			return m, tea.Quit

		case tea.KeyEnter:
			return m.handleEnter()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, tiCmd
}

func (m *InterviewModel) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())

	switch m.step {
	case stepName:
		if input == "" {
			m.err = fmt.Errorf("name cannot be empty")
			return m, nil
		}
		if strings.ContainsAny(input, " \t\n\r") {
			m.err = fmt.Errorf("name must not contain spaces (use hyphens or underscores)")
			return m, nil
		}
		m.answers.name = strings.ToLower(input)
		m.advanceTo(stepDescription, "Describe what this subagent does")
		return m, textinput.Blink

	case stepDescription:
		if input == "" {
			m.err = fmt.Errorf("description cannot be empty")
			return m, nil
		}
		m.answers.description = input
		m.advanceTo(stepTools, "Type tool names to toggle (comma-separated), or 'all' for all tools, or press Enter to skip")
		return m, textinput.Blink

	case stepTools:
		return m.handleToolsInput(input)

	case stepSkills:
		m.answers.skills = parseCommaList(input)
		m.advanceTo(stepPersonality, "Describe the personality (e.g., 'friendly, concise, technical')")
		return m, textinput.Blink

	case stepPersonality:
		m.answers.personality = input
		m.advanceTo(stepConfirm, "")
		m.err = nil
		return m, nil

	case stepConfirm:
		val := strings.ToLower(input)
		confirmed := val == "y" || val == "yes" || val == "s"
		if confirmed {
			def := agent.SubagentDef{
				Name:         m.answers.name,
				Description:  m.answers.description,
				AllowedTools: m.answers.tools,
				Skills:       m.answers.skills,
				SystemPrompt: fmt.Sprintf("You are the %s subagent. %s", m.answers.name, m.answers.description),
				Personality:  m.answers.personality,
			}
			if m.onComplete != nil {
				if err := m.onComplete(def); err != nil {
					m.err = err
					return m, nil
				}
			}
			m.done = true
			return m, tea.Quit
		}
		if val == "back" || val == "b" {
			m.goBack()
			return m, nil
		}
		// Cancel or no
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *InterviewModel) handleToolsInput(input string) (tea.Model, tea.Cmd) {
	if input == "" {
		// Confirm selection or skip
		m.answers.tools = selectedToolsList(m.selectedTools)
		m.advanceTo(stepSkills, "Enter skill names (comma-separated), or press Enter to skip")
		return m, textinput.Blink
	}

	if input == "all" {
		for _, t := range m.availableTools {
			m.selectedTools[t] = true
		}
		m.err = nil
		m.textInput.SetValue("")
		return m, nil
	}

	if input == "back" || input == "b" {
		m.goBack()
		return m, nil
	}

	if input == "done" || input == "d" {
		m.answers.tools = selectedToolsList(m.selectedTools)
		m.advanceTo(stepSkills, "Enter skill names (comma-separated), or press Enter to skip")
		return m, textinput.Blink
	}

	// Toggle tools from comma-separated input
	names := parseCommaList(input)
	valid := true
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if !isValidTool(name, m.availableTools) {
			m.err = fmt.Errorf("unknown tool: %q (available: %s)", name, strings.Join(m.availableTools, ", "))
			valid = false
			break
		}
		if m.selectedTools[name] {
			delete(m.selectedTools, name)
		} else {
			m.selectedTools[name] = true
		}
	}
	if valid {
		m.err = nil
	}
	m.textInput.SetValue("")
	return m, nil
}

func (m *InterviewModel) advanceTo(step interviewStep, placeholder string) {
	m.step = step
	m.err = nil
	m.textInput.SetValue("")
	m.textInput.Placeholder = placeholder
	if step == stepPersonality {
		m.textInput.Placeholder = "friendly, concise, technical"
	}
	if step == stepConfirm {
		m.textInput.Placeholder = "yes / no / back"
	}
}

func (m *InterviewModel) goBack() {
	m.err = nil
	m.textInput.SetValue("")
	switch m.step {
	case stepDescription:
		m.step = stepName
		m.textInput.Placeholder = "Enter subagent name (lowercase, no spaces)"
	case stepTools:
		m.step = stepDescription
		m.textInput.Placeholder = "Describe what this subagent does"
	case stepSkills:
		m.step = stepTools
		m.textInput.Placeholder = "Type tool names to toggle, 'all', or 'done'"
	case stepPersonality:
		m.step = stepSkills
		m.textInput.Placeholder = "Enter skill names (comma-separated)"
	case stepConfirm:
		m.step = stepPersonality
		m.textInput.Placeholder = "friendly, concise, technical"
		m.textInput.SetValue(m.answers.personality)
	}
}

func (m *InterviewModel) View() string {
	if m.done && m.err == nil {
		return interviewSuccessStyle.Render(
			fmt.Sprintf("Subagent %q created! Type @%s to chat.\n\nPress any key to continue...",
				m.answers.name, m.answers.name))
	}

	var sb strings.Builder
	sb.WriteString(interviewTitleStyle.Render(" Create Dynamic Subagent "))
	sb.WriteString("\n\n")

	// Step progress indicator
	for s := stepName; s <= stepConfirm; s++ {
		marker := "[ ]"
		if s < m.step {
			marker = interviewSuccessStyle.Render("[✓]")
		} else if s == m.step {
			marker = interviewPromptStyle.Render("[▶]")
		}
		sb.WriteString(marker + " " + stepNameDisplay(s) + "\n")
	}
	sb.WriteString("\n")

	// Current step prompt
	switch m.step {
	case stepName:
		sb.WriteString(interviewPromptStyle.Render("Name: "))
		sb.WriteString("Choose a unique name for your subagent. Use lowercase letters and hyphens.\n\n")
	case stepDescription:
		sb.WriteString(interviewPromptStyle.Render("Description: "))
		sb.WriteString("What does this subagent do? Be specific about its role and capabilities.\n\n")
	case stepTools:
		sb.WriteString(interviewPromptStyle.Render("Tools: "))
		sb.WriteString("Select which tools this subagent can use.\n")
		sb.WriteString("Type tool names to toggle, 'all' for everything, 'done' to confirm.\n")
		sb.WriteString(interviewHelpStyle.Render(fmt.Sprintf("Available: %s\n", strings.Join(m.availableTools, ", "))))
		if len(m.selectedTools) > 0 {
			sb.WriteString(interviewSuccessStyle.Render(fmt.Sprintf("Selected: %s\n", strings.Join(selectedToolsList(m.selectedTools), ", "))))
		}
		sb.WriteString("\n")
	case stepSkills:
		sb.WriteString(interviewPromptStyle.Render("Skills: "))
		sb.WriteString("Which skills should this subagent load? Comma-separated list.\n\n")
	case stepPersonality:
		sb.WriteString(interviewPromptStyle.Render("Personality: "))
		sb.WriteString("Describe the subagent's tone and behavior. Examples: 'friendly, concise', 'formal and thorough'.\n\n")
	case stepConfirm:
		sb.WriteString(interviewPromptStyle.Render("Confirm: "))
		sb.WriteString("Review the subagent definition:\n\n")
		sb.WriteString(fmt.Sprintf("  Name:        %s\n", m.answers.name))
		sb.WriteString(fmt.Sprintf("  Description: %s\n", m.answers.description))
		sb.WriteString(fmt.Sprintf("  Tools:       %s\n", strings.Join(m.answers.tools, ", ")))
		sb.WriteString(fmt.Sprintf("  Skills:      %s\n", strings.Join(m.answers.skills, ", ")))
		sb.WriteString(fmt.Sprintf("  Personality: %s\n", m.answers.personality))
		sb.WriteString("\n")
		sb.WriteString(interviewHelpStyle.Render("Type 'yes' to create, 'no' to cancel, 'back' to edit.\n\n"))
	}

	// Error display
	if m.err != nil {
		sb.WriteString(interviewErrorStyle.Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	sb.WriteString(m.textInput.View())
	return sb.String()
}

func stepNameDisplay(s interviewStep) string {
	switch s {
	case stepName:
		return "Name"
	case stepDescription:
		return "Description"
	case stepTools:
		return "Tools"
	case stepSkills:
		return "Skills"
	case stepPersonality:
		return "Personality"
	case stepConfirm:
		return "Confirm"
	default:
		return ""
	}
}

// parseCommaList splits a comma-separated string into trimmed non-empty parts.
func parseCommaList(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// isValidTool checks if a tool name exists in the available list.
func isValidTool(name string, available []string) bool {
	for _, a := range available {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
}

// selectedToolsList returns sorted tool names from the selected map.
func selectedToolsList(selected map[string]bool) []string {
	var names []string
	for name := range selected {
		names = append(names, name)
	}
	return names
}
