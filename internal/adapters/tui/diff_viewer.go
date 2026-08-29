package tui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"gaia/internal/diff"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewerMode represents the current interaction state of the diff viewer.
type ViewerMode int

const (
	ModeNavigating ViewerMode = iota
	ModeSteering
	ModeError
)

// DiffCloseMsg is dispatched when the user exits the diff viewer.
type DiffCloseMsg struct{}

// DiffSteeringMsg is dispatched when the user submits steering guidance for a specific hunk.
type DiffSteeringMsg struct {
	FilePath    string
	HunkHeader  string
	DiffSnippet string
	LineStart   int
	LineEnd     int
	Feedback    string
}

// GitApplyFunc is a function signature for applying git patches.
type GitApplyFunc func(workDir string, patch string, flags ...string) error

// GitDiffLoader is a function signature for loading working diffs.
type GitDiffLoader func(workDir string, staged bool) ([]diff.DiffFile, error)

// DefaultGitApply executes git apply with given patch and flags.
func DefaultGitApply(workDir string, patch string, flags ...string) error {
	args := append([]string{"apply"}, flags...)
	cmd := exec.Command("git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// DefaultGitDiffLoader runs `git diff` or `git diff --staged` and parses the output.
func DefaultGitDiffLoader(workDir string, staged bool) ([]diff.DiffFile, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	cmd := exec.Command("git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return diff.ParseUnifiedDiff(string(output))
}

// DiffViewerModel handles the interactive diff view lifecycle and interactions.
type DiffViewerModel struct {
	Files          []diff.DiffFile
	FocusedFile    int
	FocusedHunk    int
	Viewport       viewport.Model
	SteeringInput  textinput.Model
	Mode           ViewerMode
	StatusMessage  string
	WorkDir        string
	TerminalWidth  int
	TerminalHeight int

	GitApplier GitApplyFunc
	DiffLoader GitDiffLoader
}

// NewDiffViewerModel instantiates a new DiffViewerModel.
func NewDiffViewerModel(files []diff.DiffFile, width, height int) DiffViewerModel {
	vp := viewport.New(width, max(height-6, 5))

	ti := textinput.New()
	ti.Placeholder = "Type steering guidance for the agent..."
	ti.Prompt = "Steer > "
	ti.CharLimit = 500
	ti.Width = width - 10

	m := DiffViewerModel{
		Files:          files,
		FocusedFile:    0,
		FocusedHunk:    0,
		Viewport:       vp,
		SteeringInput:  ti,
		Mode:           ModeNavigating,
		WorkDir:        ".",
		TerminalWidth:  width,
		TerminalHeight: height,
		GitApplier:     DefaultGitApply,
		DiffLoader:     DefaultGitDiffLoader,
	}

	m.updateViewportContent()
	return m
}

// Init initializes viewport and text input.
func (m DiffViewerModel) Init() tea.Cmd {
	return nil
}

// Update processes key events and commands.
func (m DiffViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.TerminalWidth = msg.Width
		m.TerminalHeight = msg.Height
		m.Viewport.Width = msg.Width
		m.Viewport.Height = max(msg.Height-6, 5)
		m.SteeringInput.Width = msg.Width - 10
		m.updateViewportContent()

	case tea.KeyMsg:
		if m.Mode == ModeSteering {
			switch msg.Type {
			case tea.KeyEsc:
				m.Mode = ModeNavigating
				m.SteeringInput.Reset()
				return m, nil
			case tea.KeyEnter:
				feedback := strings.TrimSpace(m.SteeringInput.Value())
				if feedback != "" {
					steeringPayload := m.buildSteeringPayload(feedback)
					m.Mode = ModeNavigating
					m.SteeringInput.Reset()
					return m, func() tea.Msg {
						return steeringPayload
					}
				}
				m.Mode = ModeNavigating
				return m, nil
			default:
				var cmd tea.Cmd
				m.SteeringInput, cmd = m.SteeringInput.Update(msg)
				return m, cmd
			}
		}

		// Navigating mode keybindings
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return DiffCloseMsg{}
			}

		case "n":
			m.nextHunk()
			m.updateViewportContent()

		case "p":
			m.prevHunk()
			m.updateViewportContent()

		case "tab":
			m.nextFile()
			m.updateViewportContent()

		case "shift+tab":
			m.prevFile()
			m.updateViewportContent()

		case "s":
			m.stageFocusedHunk()
			m.updateViewportContent()

		case "u":
			m.unstageFocusedHunk()
			m.updateViewportContent()

		case "d":
			m.discardFocusedHunk()
			m.updateViewportContent()

		case "e", "r":
			m.Mode = ModeSteering
			m.SteeringInput.Focus()
			return m, textinput.Blink

		default:
			var vpCmd tea.Cmd
			m.Viewport, vpCmd = m.Viewport.Update(msg)
			if vpCmd != nil {
				cmds = append(cmds, vpCmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the visual diff viewer component.
func (m DiffViewerModel) View() string {
	var sb strings.Builder

	// Title header
	title := titleStyle.Render(" GAIA Interactive Diff Viewer ")
	help := infoStyle.Render(" [n/p] Next/Prev Hunk  [s] Stage  [u] Unstage  [d] Discard  [e/r] Steer  [q] Quit ")
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, " ", help)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Diff Content
	if len(m.Files) == 0 {
		sb.WriteString("\n" + diff.ContextStyle.Render("  No modified files found in working tree.\n\n"))
	} else {
		sb.WriteString(m.Viewport.View())
		sb.WriteString("\n")
	}

	// Status line or Steering Prompt overlay
	if m.Mode == ModeSteering {
		promptBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF00D7")).
			Padding(0, 1).
			Render(m.SteeringInput.View())
		sb.WriteString(promptBox)
	} else if m.StatusMessage != "" {
		sb.WriteString(infoStyle.Render("Status: " + m.StatusMessage))
	} else {
		sb.WriteString(infoStyle.Render(fmt.Sprintf("File %d/%d • Hunk %d", m.FocusedFile+1, max(len(m.Files), 1), m.FocusedHunk+1)))
	}

	return sb.String()
}

func (m *DiffViewerModel) updateViewportContent() {
	if len(m.Files) == 0 {
		m.Viewport.SetContent("No changes detected.")
		return
	}

	var sb strings.Builder
	for i, file := range m.Files {
		focusedHunk := -1
		if i == m.FocusedFile {
			focusedHunk = m.FocusedHunk
		}
		sb.WriteString(diff.RenderDiffFile(file, m.TerminalWidth, focusedHunk))
		sb.WriteString("\n")
	}

	m.Viewport.SetContent(sb.String())
}

func (m *DiffViewerModel) nextHunk() {
	if len(m.Files) == 0 {
		return
	}
	currentFile := m.Files[m.FocusedFile]
	if m.FocusedHunk+1 < len(currentFile.Hunks) {
		m.FocusedHunk++
		return
	}
	// Move to next file
	if m.FocusedFile+1 < len(m.Files) {
		m.FocusedFile++
		m.FocusedHunk = 0
	}
}

func (m *DiffViewerModel) prevHunk() {
	if len(m.Files) == 0 {
		return
	}
	if m.FocusedHunk > 0 {
		m.FocusedHunk--
		return
	}
	// Move to prev file
	if m.FocusedFile > 0 {
		m.FocusedFile--
		prevFile := m.Files[m.FocusedFile]
		if len(prevFile.Hunks) > 0 {
			m.FocusedHunk = len(prevFile.Hunks) - 1
		} else {
			m.FocusedHunk = 0
		}
	}
}

func (m *DiffViewerModel) nextFile() {
	if m.FocusedFile+1 < len(m.Files) {
		m.FocusedFile++
		m.FocusedHunk = 0
	}
}

func (m *DiffViewerModel) prevFile() {
	if m.FocusedFile > 0 {
		m.FocusedFile--
		m.FocusedHunk = 0
	}
}

func (m *DiffViewerModel) stageFocusedHunk() {
	if m.FocusedFile >= len(m.Files) {
		return
	}
	file := m.Files[m.FocusedFile]
	if m.FocusedHunk >= len(file.Hunks) {
		return
	}

	patch := file.BuildHunkPatch(m.FocusedHunk)
	if patch == "" {
		m.StatusMessage = "Cannot construct patch for hunk"
		return
	}

	applier := m.GitApplier
	if applier == nil {
		applier = DefaultGitApply
	}

	if err := applier(m.WorkDir, patch, "--cached"); err != nil {
		m.StatusMessage = fmt.Sprintf("Stage failed: %v", err)
		return
	}

	m.Files[m.FocusedFile].Hunks[m.FocusedHunk].IsStaged = true
	m.StatusMessage = fmt.Sprintf("Staged hunk %d of %s", m.FocusedHunk+1, file.NewPath)
}

func (m *DiffViewerModel) unstageFocusedHunk() {
	if m.FocusedFile >= len(m.Files) {
		return
	}
	file := m.Files[m.FocusedFile]
	if m.FocusedHunk >= len(file.Hunks) {
		return
	}

	patch := file.BuildHunkPatch(m.FocusedHunk)
	if patch == "" {
		m.StatusMessage = "Cannot construct patch for hunk"
		return
	}

	applier := m.GitApplier
	if applier == nil {
		applier = DefaultGitApply
	}

	if err := applier(m.WorkDir, patch, "--cached", "--reverse"); err != nil {
		m.StatusMessage = fmt.Sprintf("Unstage failed: %v", err)
		return
	}

	m.Files[m.FocusedFile].Hunks[m.FocusedHunk].IsStaged = false
	m.StatusMessage = fmt.Sprintf("Unstaged hunk %d of %s", m.FocusedHunk+1, file.NewPath)
}

func (m *DiffViewerModel) discardFocusedHunk() {
	if m.FocusedFile >= len(m.Files) {
		return
	}
	file := m.Files[m.FocusedFile]
	if m.FocusedHunk >= len(file.Hunks) {
		return
	}

	patch := file.BuildHunkPatch(m.FocusedHunk)
	if patch == "" {
		m.StatusMessage = "Cannot construct patch for hunk"
		return
	}

	applier := m.GitApplier
	if applier == nil {
		applier = DefaultGitApply
	}

	if err := applier(m.WorkDir, patch, "--reverse"); err != nil {
		m.StatusMessage = fmt.Sprintf("Discard failed: %v", err)
		return
	}

	// Remove the hunk from the file
	m.Files[m.FocusedFile].Hunks = append(
		m.Files[m.FocusedFile].Hunks[:m.FocusedHunk],
		m.Files[m.FocusedFile].Hunks[m.FocusedHunk+1:]...,
	)
	if m.FocusedHunk >= len(m.Files[m.FocusedFile].Hunks) && m.FocusedHunk > 0 {
		m.FocusedHunk--
	}
	m.StatusMessage = fmt.Sprintf("Discarded hunk in %s", file.NewPath)
}

func (m *DiffViewerModel) buildSteeringPayload(feedback string) DiffSteeringMsg {
	var filePath string
	var hunkHeader string
	var snippet string
	var lineStart, lineEnd int

	if m.FocusedFile < len(m.Files) {
		file := m.Files[m.FocusedFile]
		filePath = file.NewPath
		if filePath == "" {
			filePath = file.OldPath
		}
		if m.FocusedHunk < len(file.Hunks) {
			hunk := file.Hunks[m.FocusedHunk]
			hunkHeader = hunk.Header
			snippet = file.BuildHunkPatch(m.FocusedHunk)
			lineStart = hunk.NewStart
			lineEnd = hunk.NewStart + hunk.NewLines
		}
	}

	return DiffSteeringMsg{
		FilePath:    filePath,
		HunkHeader:  hunkHeader,
		DiffSnippet: snippet,
		LineStart:   lineStart,
		LineEnd:     lineEnd,
		Feedback:    feedback,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
