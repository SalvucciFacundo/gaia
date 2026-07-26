package tui

import (
	"fmt"
	"sort"
	"strings"

	"gaia/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	policyTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	policyTierActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00FF00"))

	policyTierInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ADADAD"))

	policyOverrideAllowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D700"))

	policyOverrideDenyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5F00"))

	policyOverrideAskStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700"))

	policyOverrideSkipStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ADADAD"))

	policyOverrideAuditStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D7FF"))

	policyNoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	policyHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ADADAD")).
			Italic(true)

	policySelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFD700"))
)

// panelMode controls which view the policy panel shows.
type panelMode int

const (
	panelMain   panelMode = iota // main view: tier + tool list
	panelEdit                     // override editing for a specific tool
)

// overrideOption represents a selectable override value in the edit view.
type overrideOption struct {
	policy core.OverridePolicy
	label  string
	style  lipgloss.Style
}

// PolicyPanelModel is a Bubbletea sub-model for the /permisos panel.
// It supports tier selection, tool navigation, and inline override editing.
type PolicyPanelModel struct {
	guard         *core.PolicyGuard
	toolNames     []string // all available tool names
	width, height int
	done          bool

	// Tier selection
	tierNames []core.PolicyTier
	tierIdx   int

	// Tool list navigation (main mode)
	toolIdx int

	// Override editing (edit mode)
	mode       panelMode
	editTool   string // the tool being edited
	editOptIdx int    // selected override option index
}

// overrideChoices are the available override options shown in edit mode.
var overrideChoices = []overrideOption{
	{core.OverrideAllow, "Allow — always permit", policyOverrideAllowStyle},
	{core.OverrideDeny, "Deny — always block", policyOverrideDenyStyle},
	{core.OverrideSkip, "Skip — deny silently, agent tries alternative", policyOverrideSkipStyle},
	{core.OverrideAskOnce, "Ask Once — prompt this once", policyOverrideAskStyle},
	{core.OverrideAskSession, "Ask Session — prompt once per session", policyOverrideAskStyle},
	{core.OverrideAskAlways, "Ask Always — prompt every time", policyOverrideAskStyle},
	{core.OverrideAudit, "Audit — allow but log", policyOverrideAuditStyle},
}

// NewPolicyPanelModel creates the policy panel.
// toolNames is the complete list of available tools (from the registry).
func NewPolicyPanelModel(guard *core.PolicyGuard, toolNames []string) *PolicyPanelModel {
	tierNames := []core.PolicyTier{core.TierRead, core.TierSandbox, core.TierFull}
	idx := 2
	if guard != nil {
		switch guard.Tier() {
		case core.TierRead:
			idx = 0
		case core.TierSandbox:
			idx = 1
		case core.TierFull:
			idx = 2
		}
	}

	sortedTools := make([]string, len(toolNames))
	copy(sortedTools, toolNames)
	sort.Strings(sortedTools)

	return &PolicyPanelModel{
		guard:     guard,
		toolNames: sortedTools,
		tierNames: tierNames,
		tierIdx:   idx,
	}
}

func (m *PolicyPanelModel) Init() tea.Cmd { return nil }

func (m *PolicyPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case panelMain:
			return m.updateMain(msg)
		case panelEdit:
			return m.updateEdit(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// updateMain handles keys in the main overview mode.
func (m *PolicyPanelModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.done = true
		return m, nil

	case "enter":
		// Apply selected tier
		if m.guard != nil {
			m.guard.SetTier(m.tierNames[m.tierIdx])
		}
		// If a tool is selected, enter edit mode
		if len(m.toolNames) > 0 && m.toolIdx >= 0 && m.toolIdx < len(m.toolNames) {
			// Check if this tool has an active override
			m.editTool = m.toolNames[m.toolIdx]
			m.editOptIdx = 0
			m.mode = panelEdit
		}
		return m, nil

	case "left", "h":
		if m.tierIdx > 0 {
			m.tierIdx--
		}
	case "right", "l":
		if m.tierIdx < len(m.tierNames)-1 {
			m.tierIdx++
		}
	case "up", "k":
		if m.toolIdx > 0 {
			m.toolIdx--
		}
	case "down", "j":
		if m.toolIdx < len(m.toolNames)-1 {
			m.toolIdx++
		}
	}
	return m, nil
}

// updateEdit handles keys in the override editing mode.
func (m *PolicyPanelModel) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Back to main view
		m.mode = panelMain
		return m, nil

	case "enter":
		// Apply selected override
		if m.guard == nil {
			m.mode = panelMain
			return m, nil
		}
		// First option is always "Remove override" if one exists
			offset := 0
			if _, hasOverride := m.guard.Override(m.editTool); hasOverride {
				if m.editOptIdx == 0 {
					m.guard.RemoveOverride(m.editTool)
					m.mode = panelMain
					return m, nil
				}
				offset = 1
			}
			idx := m.editOptIdx - offset
			if idx >= 0 && idx < len(overrideChoices) {
				m.guard.SetOverride(m.editTool, overrideChoices[idx].policy)
			}
		m.mode = panelMain
		return m, nil

	case "up", "k":
		if m.editOptIdx > 0 {
			m.editOptIdx--
		}
	case "down", "j":
		maxIdx := len(overrideChoices)
		if _, hasOverride := m.guard.Override(m.editTool); hasOverride {
			maxIdx++ // +1 for "Remove override"
		}
		if m.editOptIdx < maxIdx-1 {
			m.editOptIdx++
		}
	}
	return m, nil
}

func (m *PolicyPanelModel) View() string {
	if m.done {
		tier := "full"
		if m.guard != nil {
			tier = string(m.guard.Tier())
		}
		return fmt.Sprintf("\n  Policy updated. Current tier: %s\n\n  Press Enter to continue...", tier)
	}

	switch m.mode {
	case panelMain:
		return m.renderMain()
	case panelEdit:
		return m.renderEdit()
	}
	return ""
}

func (m *PolicyPanelModel) renderMain() string {
	var sb strings.Builder

	sb.WriteString(policyTitleStyle.Render(" Policy Guard — /permisos "))
	sb.WriteString("\n\n")

	// Tier selection
	sb.WriteString("  Tier: ")
	for i, name := range m.tierNames {
		label := fmt.Sprintf("[ %s ]", name)
		if i == m.tierIdx {
			sb.WriteString(policyTierActiveStyle.Render(label))
		} else {
			sb.WriteString(policyTierInactiveStyle.Render(label))
		}
		sb.WriteString("  ")
	}
	sb.WriteString("\n")
	sb.WriteString(policyHelpStyle.Render("    ← → change tier · Enter to apply"))
	sb.WriteString("\n\n")

	// Tier description
	switch m.tierNames[m.tierIdx] {
	case core.TierRead:
		sb.WriteString("    Read-only: glob, grep, read, file_info, mem_search\n")
	case core.TierSandbox:
		sb.WriteString("    Sandbox: read + write + safe shell within project\n")
	case core.TierFull:
		sb.WriteString("    Full: all tools (hardline blocklist still active)\n")
	}
	sb.WriteString("\n")

	// Hardline note
	sb.WriteString("  Hardline: ACTIVE (rm -rf /, fork bombs, dd to devices, curl|sh)\n\n")

	// Tool overrides header
	sb.WriteString("  Tool Overrides (select with ↑ ↓, Enter to edit):\n")
	sb.WriteString(policyHelpStyle.Render("    Overrides let you allow/deny individual tools regardless of tier."))
	sb.WriteString("\n\n")

	if len(m.toolNames) == 0 {
		sb.WriteString(policyHelpStyle.Render("    No tools registered.\n"))
	} else {
		// Show visible subset
		start, end := visibleRange(m.toolIdx, len(m.toolNames), 15)
		for i, tool := range m.toolNames {
			if i < start || i > end {
				continue
			}
			cursor := "  "
			if i == m.toolIdx {
				cursor = "➤ "
			}

			var overrideStr string
			if m.guard != nil {
				if ov, ok := m.guard.Override(tool); ok {
					overrideStr = m.styleForOverride(ov)
				} else {
					overrideStr = policyNoneStyle.Render("[default]")
				}
			}

			line := fmt.Sprintf("%s%-28s %s\n", cursor, tool, overrideStr)
			if i == m.toolIdx {
				sb.WriteString(policySelectedStyle.Render(line))
			} else {
				sb.WriteString(line)
			}
		}

		if len(m.toolNames) > end-start+1 {
			sb.WriteString(policyHelpStyle.Render(fmt.Sprintf("    ... %d more tools\n", len(m.toolNames)-(end-start+1))))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(policyHelpStyle.Render("  ↑ ↓ navigate · ← → change tier · Enter: apply tier / edit tool · Esc: close"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *PolicyPanelModel) renderEdit() string {
	var sb strings.Builder

	sb.WriteString(policyTitleStyle.Render(fmt.Sprintf(" Override: %s ", m.editTool)))
	sb.WriteString("\n\n")

	// Current override
	if m.guard != nil {
		if ov, ok := m.guard.Override(m.editTool); ok {
			sb.WriteString(fmt.Sprintf("  Current: %s\n\n", m.styleForOverride(ov)))
		} else {
			sb.WriteString(policyNoneStyle.Render("  Current: no override (uses tier default)\n\n"))
		}
	}

	sb.WriteString("  Choose an override:\n\n")

	// Show options
	offset := 0
	if m.guard != nil {
		if _, hasOverride := m.guard.Override(m.editTool); hasOverride {
			// "Remove override" as first option
			cursor := "  "
			if m.editOptIdx == 0 {
				cursor = "➤ "
			}
			line := fmt.Sprintf("%sRemove override (back to tier default)\n", cursor)
			if m.editOptIdx == 0 {
				sb.WriteString(policySelectedStyle.Render(line))
			} else {
				sb.WriteString(line)
			}
			offset = 1
		}
	}

	for i, opt := range overrideChoices {
		cursor := "  "
		if m.editOptIdx == i+offset {
			cursor = "➤ "
		}
		line := fmt.Sprintf("%s%s\n", cursor, opt.label)
		if m.editOptIdx == i+offset {
			sb.WriteString(policySelectedStyle.Render(line))
		} else {
			sb.WriteString(opt.style.Render(line))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(policyHelpStyle.Render("  ↑ ↓ navigate · Enter: apply · Esc: back"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *PolicyPanelModel) styleForOverride(policy core.OverridePolicy) string {
	switch policy {
	case core.OverrideAllow:
		return policyOverrideAllowStyle.Render("ALLOW")
	case core.OverrideDeny:
		return policyOverrideDenyStyle.Render("DENY")
	case core.OverrideAskOnce, core.OverrideAskSession, core.OverrideAskAlways:
		return policyOverrideAskStyle.Render("ASK")
	case core.OverrideSkip:
		return policyOverrideSkipStyle.Render("SKIP")
	case core.OverrideAudit:
		return policyOverrideAuditStyle.Render("AUDIT")
	default:
		return string(policy)
	}
}

// visibleRange computes the visible slice of items around the cursor.
func visibleRange(cursor, total, window int) (start, end int) {
	half := window / 2
	start = cursor - half
	if start < 0 {
		start = 0
	}
	end = start + window - 1
	if end >= total {
		end = total - 1
		start = end - window + 1
		if start < 0 {
			start = 0
		}
	}
	return
}
