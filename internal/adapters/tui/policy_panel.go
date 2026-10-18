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
				Foreground(lipgloss.Color("#00FF66"))

	policyTierInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666"))

	policyAllowBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00FF66"))

	policyAskBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700"))

	policyBlockBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF3366"))

	policyScopeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D7FF"))

	policyHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	policySelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00FFFF"))
)

// PolicyPanelModel provides the Cyberpunk TUI permissions control panel.
type PolicyPanelModel struct {
	guard         *core.PolicyGuard
	toolNames     []string
	width, height int
	done          bool

	// Tier selection
	tierNames []core.PolicyTier
	tierIdx   int

	// 4 Functional Categories
	categories  []core.CategorySummary
	catIdx      int
	expandedCat map[core.PolicyCategoryID]bool
	standalone  bool
}

// SetStandalone configures the model to exit via tea.Quit when running as a standalone CLI.
func (m *PolicyPanelModel) SetStandalone(standalone bool) {
	m.standalone = standalone
}

// NewPolicyPanelModel creates the policy panel.
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

	categories := core.GetCategorySummaries(guard, sortedTools)

	return &PolicyPanelModel{
		guard:       guard,
		toolNames:   sortedTools,
		tierNames:   tierNames,
		tierIdx:     idx,
		categories:  categories,
		catIdx:      0,
		expandedCat: make(map[core.PolicyCategoryID]bool),
	}
}

func (m *PolicyPanelModel) Init() tea.Cmd { return nil }

func (m *PolicyPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.done = true
			if m.standalone {
				return m, tea.Quit
			}
			return m, nil

		case "up", "k":
			if m.catIdx > 0 {
				m.catIdx--
			}
		case "down", "j":
			if m.catIdx < len(m.categories)-1 {
				m.catIdx++
			}

		case "left", "h":
			if m.tierIdx > 0 {
				m.tierIdx--
				if m.guard != nil {
					m.guard.SetTier(m.tierNames[m.tierIdx])
					m.refreshCategories()
				}
			}
		case "right", "l":
			if m.tierIdx < len(m.tierNames)-1 {
				m.tierIdx++
				if m.guard != nil {
					m.guard.SetTier(m.tierNames[m.tierIdx])
					m.refreshCategories()
				}
			}

		case " ":
			// Spacebar toggles category state: Allow -> Ask -> Block -> Allow
			if len(m.categories) > 0 && m.catIdx >= 0 && m.catIdx < len(m.categories) {
				cat := &m.categories[m.catIdx]
				var nextState core.SimplifiedState
				switch cat.State {
				case core.StateAllow:
					nextState = core.StateAsk
				case core.StateAsk:
					nextState = core.StateBlock
				case core.StateBlock:
					nextState = core.StateAllow
				default:
					nextState = core.StateAllow
				}
				core.SetCategoryState(m.guard, cat.ID, nextState)
				m.refreshCategories()
			}

		case "enter":
			// Toggle accordion expansion of the selected category
			if len(m.categories) > 0 && m.catIdx >= 0 && m.catIdx < len(m.categories) {
				catID := m.categories[m.catIdx].ID
				m.expandedCat[catID] = !m.expandedCat[catID]
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *PolicyPanelModel) refreshCategories() {
	m.categories = core.GetCategorySummaries(m.guard, m.toolNames)
}

func (m *PolicyPanelModel) View() string {
	if m.done {
		tier := "full"
		if m.guard != nil {
			tier = string(m.guard.Tier())
		}
		return fmt.Sprintf("\n  Policy updated. Current tier: %s\n\n  Press Enter to continue...", tier)
	}

	var sb strings.Builder

	sb.WriteString(policyTitleStyle.Render(" GAIA Policy Guard — Permisos & Control de Acceso "))
	sb.WriteString("\n\n")

	// Tier Selector
	sb.WriteString("  Tier Global: ")
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
	sb.WriteString(policyHelpStyle.Render("    ← → cambiar tier global · Espacio: alternar permiso · Enter: ver tools"))
	sb.WriteString("\n\n")

	// Categories Section
	sb.WriteString("  Categorías Funcionales (4x3 Control Matrix):\n\n")

	for i, cat := range m.categories {
		cursor := "  "
		if i == m.catIdx {
			cursor = "➤ "
		}

		// State Badge
		badge := ""
		switch cat.State {
		case core.StateAllow:
			badge = policyAllowBadge.Render("🟢 ALLOW ")
		case core.StateAsk:
			badge = policyAskBadge.Render("🟡 ASK   ")
		case core.StateBlock:
			badge = policyBlockBadge.Render("🔴 BLOCK ")
		}

		expandIcon := "▶"
		if m.expandedCat[cat.ID] {
			expandIcon = "▼"
		}

		line := fmt.Sprintf("%s%s %-22s %s  Scope: %s  (%d tools)\n",
			cursor, expandIcon, cat.Name, badge,
			policyScopeStyle.Render("["+cat.BlastRadius+"]"),
			len(cat.Tools),
		)

		if i == m.catIdx {
			sb.WriteString(policySelectedStyle.Render(line))
		} else {
			sb.WriteString(line)
		}

		// Description
		sb.WriteString(fmt.Sprintf("      %s\n", policyHelpStyle.Render(cat.Description)))

		// Expanded accordion child tools
		if m.expandedCat[cat.ID] {
			sb.WriteString("      Tools incluidas:\n")
			for _, t := range cat.Tools {
				toolBadge := ""
				switch t.State {
				case core.StateAllow:
					toolBadge = policyAllowBadge.Render("allow")
				case core.StateAsk:
					toolBadge = policyAskBadge.Render("ask")
				case core.StateBlock:
					toolBadge = policyBlockBadge.Render("block")
				}
				sb.WriteString(fmt.Sprintf("        • %-26s [%s]\n", t.Name, toolBadge))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(policyHelpStyle.Render("  ↑ ↓ seleccionar categoría · Espacio: ciclar 🟢/🟡/🔴 · Enter: expandir/colapsar · Esc/Q: cerrar"))
	sb.WriteString("\n")

	return sb.String()
}
