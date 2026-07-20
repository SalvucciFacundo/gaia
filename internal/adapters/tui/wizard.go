package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gaia/internal/adapters/llm"
	"gaia/internal/skills"
	"github.com/cli/oauth/device"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WizardStep int

const (
	StepWelcome WizardStep = iota
	StepAuthenticating
	StepModelSelect
	StepLanguageSelect
	StepSkillRecommend
	StepFinishing
)

type item struct {
	id    string
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.id }
func (i item) FilterValue() string { return i.title }

// multiItem is used for skill selection (multi-select).
type multiItem struct {
	id       string
	title    string
	selected bool
}

func (m multiItem) Title() string       { return m.title }
func (m multiItem) Description() string { return m.id }
func (m multiItem) FilterValue() string { return m.title }

type WizardModel struct {
	step         WizardStep
	auth         *llm.GitHubAuth
	ghToken      string
	cpToken      string
	models       []string
	selectedModel string
	spinner      spinner.Model
	list         list.Model
	code         string
	url          string
	codeResp     *device.CodeResponse
	err          error
	width        int
	height       int

	// Language preference (added in Milestone 3).
	languagePref string

	// Skills Hub and recommendation (added in Milestone 3).
	hub              *skills.Hub
	projectRoot      string
	projectType      string
	recommendedSkills []skills.SkillMeta
	selectedSkills   map[string]bool // skill name -> selected
	selIndex         int
}

// NewWizard creates the first-run wizard.
// projectRoot is the current working directory used for project type detection.
func NewWizard(projectRoot string) *WizardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &WizardModel{
		step:           StepWelcome,
		auth:           llm.NewGitHubAuth(),
		spinner:        s,
		projectRoot:    projectRoot,
		selectedSkills: make(map[string]bool),
		selIndex:       0,
	}
}

// SetHub injects a Skills Hub for the recommendation step.
func (m *WizardModel) SetHub(hub *skills.Hub) {
	m.hub = hub
}

type authCodeMsg struct {
	code string
	url  string
}

type tokenMsg string
type modelsMsg []string
type errorMsg error

func (m *WizardModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k":
			if m.step == StepSkillRecommend && len(m.recommendedSkills) > 0 {
				if m.selIndex > 0 {
					m.selIndex--
				}
			}
			return m, nil
		case "down", "j":
			if m.step == StepSkillRecommend && len(m.recommendedSkills) > 0 {
				if m.selIndex < len(m.recommendedSkills)-1 {
					m.selIndex++
				}
			}
			return m, nil
		case " ":
			if m.step == StepSkillRecommend && m.selIndex >= 0 && m.selIndex < len(m.recommendedSkills) {
				name := m.recommendedSkills[m.selIndex].Name
				m.selectedSkills[name] = !m.selectedSkills[name]
			}
			return m, nil
		}

	case authCodeMsg:
		m.code = msg.code
		m.url = msg.url
		openBrowser(m.url)
		return m, m.waitForToken(m.code)

	case tokenMsg:
		m.ghToken = string(msg)
		return m, m.exchangeToken()

	case modelsMsg:
		m.models = msg
		items := []list.Item{}
		for _, mod := range m.models {
			items = append(items, item{id: mod, title: mod})
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 20, 10)
		m.list.Title = "Select model"
		m.step = StepModelSelect
		return m, nil

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	if m.step == StepModelSelect {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m *WizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepWelcome:
		m.step = StepAuthenticating
		return m, m.startAuth()

	case StepModelSelect:
		i, ok := m.list.SelectedItem().(item)
		if ok {
			m.selectedModel = i.id
			m.step = StepLanguageSelect
			return m, nil
		}

	case StepLanguageSelect:
		// Move to skill recommendation.
		m.detectProjectType()
		m.step = StepSkillRecommend
		return m, nil

	case StepSkillRecommend:
		// Confirm selected skills and finish.
		if m.hub != nil {
			for name := range m.selectedSkills {
				if err := m.hub.Install(name); err != nil {
					// Non-fatal: skill might already be installed.
					continue
				}
			}
		}
		m.step = StepFinishing
		return m, tea.Quit
	}
	return m, nil
}

func (m *WizardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.", m.err)
	}

	doc := strings.Builder{}

	switch m.step {
	case StepWelcome:
		doc.WriteString(titleStyle.Render(" WELCOME TO GAIA ") + "\n\n")
		doc.WriteString("GAIA needs to connect to GitHub Copilot to function.\n")
		doc.WriteString("Press " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to start authorization.\n")

	case StepAuthenticating:
		doc.WriteString(titleStyle.Render(" AUTHORIZATION ") + "\n\n")
		if m.code == "" {
			doc.WriteString(m.spinner.View() + " Requesting code from GitHub...")
		} else {
			doc.WriteString("1. Visit: " + lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Underline(true).Render(m.url) + "\n")
			doc.WriteString("2. Enter this code: " + lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("57")).Render(" "+m.code+" ") + "\n\n")
			doc.WriteString(m.spinner.View() + " Waiting for authorization in the browser...")
		}

	case StepModelSelect:
		return m.list.View()

	case StepLanguageSelect:
		doc.WriteString(titleStyle.Render(" LANGUAGE PREFERENCE ") + "\n\n")
		doc.WriteString("Choose the language GAIA will respond in:\n\n")
		doc.WriteString(lipgloss.NewStyle().Bold(true).Render("  ➤ EN") + "  English\n")
		doc.WriteString("     ES  Spanish (Español)\n")
		doc.WriteString("     PT  Portuguese (Português)\n\n")
		doc.WriteString("Use " + lipgloss.NewStyle().Bold(true).Render("UP/DOWN") + " arrows to select, " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to confirm.\n")

	case StepSkillRecommend:
		doc.WriteString(titleStyle.Render(" SKILL RECOMMENDATIONS ") + "\n\n")
		detected := m.projectType
		if detected == "" {
			detected = "unknown"
		}
		doc.WriteString(fmt.Sprintf("Project type detected: %s\n\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render(detected)))
		doc.WriteString("We found these skills that match your project:\n")
		doc.WriteString("Press " + lipgloss.NewStyle().Bold(true).Render("SPACE") + " to select/deselect, " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to install and finish.\n\n")

		for i, sk := range m.recommendedSkills {
			cursor := " "
			mark := "[ ]"
			if i == m.selIndex {
				cursor = "➤"
			}
			if m.selectedSkills[sk.Name] {
				mark = "[x]"
			}
			doc.WriteString(fmt.Sprintf("  %s %s %s — %s\n", cursor, mark,
				lipgloss.NewStyle().Bold(true).Render(sk.Name),
				sk.Description))
		}

		if len(m.recommendedSkills) == 0 {
			doc.WriteString("  (no matching skills found — press ENTER to skip)\n")
		}

	case StepFinishing:
		doc.WriteString("All set! Configuration complete.\n")
	}

	return lipgloss.NewStyle().Margin(2, 4).Render(doc.String())
}

// detectProjectType inspects the file system to determine the project's
// primary language. It looks at well-known files and directory structures.
func (m *WizardModel) detectProjectType() {
	if m.projectRoot == "" {
		return
	}

	// Check for well-known files.
	indicators := map[string]string{
		"go.mod":          "go",
		"package.json":    "typescript",
		"tsconfig.json":   "typescript",
		"Cargo.toml":      "rust",
		"requirements.txt": "python",
		"pyproject.toml":  "python",
		"Pipfile":         "python",
		"setup.py":        "python",
		"Gemfile":         "ruby",
		"pom.xml":         "java",
		"build.gradle":    "java",
		"build.gradle.kts": "java",
		"Makefile":        "makefile",
	}

	for file, lang := range indicators {
		if _, err := os.Stat(filepath.Join(m.projectRoot, file)); err == nil {
			m.projectType = lang
			break
		}
	}

	// If still unknown, check for a Go project by scanning for .go files.
	if m.projectType == "" {
		m.projectType = detectGoProject(m.projectRoot)
	}

	// Fallback: generic project.
	if m.projectType == "" {
		m.projectType = "generic"
	}

	// Get recommendations from the hub if available.
	if m.hub != nil {
		m.recommendedSkills = m.hub.RecommendFor(m.projectType)
		if m.recommendedSkills == nil {
			m.recommendedSkills = []skills.SkillMeta{}
		}
		sort.Slice(m.recommendedSkills, func(i, j int) bool {
			return m.recommendedSkills[i].Name < m.recommendedSkills[j].Name
		})
	}
}

// detectGoProject checks if the project contains .go files in its top level.
func detectGoProject(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return "go"
		}
	}
	return ""
}

// Commands

func (m *WizardModel) startAuth() tea.Cmd {
	return func() tea.Msg {
		resp, err := m.auth.RequestDeviceCode(context.Background())
		if err != nil {
			return errorMsg(err)
		}
		m.codeResp = resp
		return authCodeMsg{code: resp.UserCode, url: resp.VerificationURI}
	}
}

func (m *WizardModel) waitForToken(code string) tea.Cmd {
	return func() tea.Msg {
		token, err := m.auth.WaitToken(context.Background(), m.codeResp)
		if err != nil {
			return errorMsg(err)
		}
		return tokenMsg(token)
	}
}

func (m *WizardModel) exchangeToken() tea.Cmd {
	return func() tea.Msg {
		token, err := m.auth.ExchangeCopilotToken(context.Background(), m.ghToken)
		if err != nil {
			return errorMsg(err)
		}
		m.cpToken = token
		client := llm.NewCopilotClient(token, "")
		models, err := client.FetchModels(context.Background())
		if err != nil {
			return errorMsg(err)
		}
		return modelsMsg(models)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("Could not open browser: %v\n", err)
	}
}

// GetResults returns the configuration obtained from the wizard.
// Returns: copilot token, selected model, language preference, selected skill names.
func (m *WizardModel) GetResults() (token, model, language string, skills []string) {
	names := make([]string, 0, len(m.selectedSkills))
	for name := range m.selectedSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	return m.cpToken, m.selectedModel, m.languagePref, names
}
