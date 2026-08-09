package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"gaia/internal/adapters/llm"
	"gaia/internal/skills"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/oauth/device"
)

type WizardStep int

const (
	StepWelcome WizardStep = iota
	StepProviderSelect
	StepKeyInput
	StepAuthenticating
	StepModelSelect
	StepLanguageSelect
	StepSecurityMode
	StepStackSelect
	StepSkillRecommend
	StepFinishing
)

type item struct {
	id    string
	title string
	desc  string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.id }

type WizardModel struct {
	step             WizardStep
	provider         string
	apiKey           string
	textInput        textinput.Model
	auth             *llm.GitHubAuth
	ghToken          string
	cpToken          string
	models           []string
	selectedModel    string
	spinner          spinner.Model
	list             list.Model
	code             string
	url              string
	codeResp         *device.CodeResponse
	err              error
	width            int
	height           int

	languagePref     string
	langIndex        int // 0 = EN, 1 = ES, 2 = PT

	securityMode     bool
	secIndex         int // 0 = No (default), 1 = Yes

	// Stack selection (User chooses their stacks)
	stacks           []string
	selectedStacks   map[string]bool
	stackIndex       int

	// Skills Hub and recommendation
	hub              *skills.Hub
	projectRoot      string
	recommendedSkills []skills.SkillMeta
	selectedSkills   map[string]bool
	selIndex         int
}

var providerCatalog = []struct {
	id   string
	name string
	desc string
}{
	{"copilot", "GitHub Copilot", "GitHub Copilot API via OAuth device flow"},
	{"openai", "OpenAI", "GPT-4o, GPT-4o-mini, o3-mini models"},
	{"anthropic", "Anthropic Claude", "Claude 3.7 Sonnet, Claude 3.5 Sonnet"},
	{"ollama", "Ollama (Local)", "Run local LLM models on http://localhost:11434"},
	{"deepseek", "DeepSeek", "DeepSeek-Chat, DeepSeek-Reasoner"},
	{"openrouter", "OpenRouter", "Multi-provider router (GPT-4o, Claude, Llama, etc.)"},
	{"groq", "Groq", "Ultra-fast inference (Llama 3.3 70B)"},
	{"qwen", "Alibaba Qwen", "Qwen-Max, Qwen-2.5 models"},
	{"together", "Together AI", "Mixtral, Llama, and open source models"},
	{"perplexity", "Perplexity AI", "Sonar-Pro search-augmented models"},
	{"fireworks", "Fireworks AI", "Fast Llama and Mixtral inference"},
	{"opencode-go", "OpenCode Go", "Grok 4.5 and developer-tuned models"},
	{"opencode-zen", "OpenCode Zen", "Claude-Sonnet-4 optimized endpoint"},
	{"kimi", "Moonshot Kimi", "Kimi-K3 context models"},
	{"glm", "Zhipu GLM", "GLM-4-Plus reasoning models"},
	{"nvidia", "NVIDIA NIM", "NVIDIA Llama-3.1 Nemotron models"},
	{"huggingface", "HuggingFace Router", "GPT-OSS and hosted HuggingFace endpoints"},
	{"deepinfra", "DeepInfra", "Hosted open source model APIs"},
	{"cerebras", "Cerebras AI", "Ultra-high speed Llama inference"},
}

var availableStacks = []struct {
	id   string
	name string
}{
	{"go", "Go (Golang)"},
	{"typescript", "TypeScript / JavaScript"},
	{"python", "Python"},
	{"rust", "Rust"},
	{"generic", "General / DevOps / Docker"},
}

func NewWizard(projectRoot string) *WizardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00F5D4"))

	ti := textinput.New()
	ti.Placeholder = "Enter your API key or endpoint URL..."
	ti.CharLimit = 256
	ti.Width = 60

	items := make([]list.Item, 0, len(providerCatalog))
	for _, p := range providerCatalog {
		items = append(items, item{id: p.id, title: p.name, desc: p.desc})
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#00F5D4")).Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("#00BBF9"))

	l := list.New(items, delegate, 65, 14)
	l.Title = "Select LLM Provider"
	l.SetShowStatusBar(false)
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("ctrl+c"))

	return &WizardModel{
		step:           StepWelcome,
		auth:           llm.NewGitHubAuth(),
		spinner:        s,
		textInput:      ti,
		list:           l,
		projectRoot:    projectRoot,
		selectedStacks: map[string]bool{"go": true},
		selectedSkills: make(map[string]bool),
		stacks:         []string{"go", "typescript", "python", "rust", "generic"},
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
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.step != StepKeyInput {
				return m, tea.Quit
			}
		case "enter":
			return m.handleEnter()
		case "up", "k":
			if m.step == StepLanguageSelect {
				if m.langIndex > 0 {
					m.langIndex--
				}
				return m, nil
			}
			if m.step == StepSecurityMode {
				m.secIndex = 0
				return m, nil
			}
			if m.step == StepStackSelect {
				if m.stackIndex > 0 {
					m.stackIndex--
				}
				return m, nil
			}
			if m.step == StepSkillRecommend && len(m.recommendedSkills) > 0 {
				if m.selIndex > 0 {
					m.selIndex--
				}
				return m, nil
			}
		case "down", "j":
			if m.step == StepLanguageSelect {
				if m.langIndex < 2 {
					m.langIndex++
				}
				return m, nil
			}
			if m.step == StepSecurityMode {
				m.secIndex = 1
				return m, nil
			}
			if m.step == StepStackSelect {
				if m.stackIndex < len(availableStacks)-1 {
					m.stackIndex++
				}
				return m, nil
			}
			if m.step == StepSkillRecommend && len(m.recommendedSkills) > 0 {
				if m.selIndex < len(m.recommendedSkills)-1 {
					m.selIndex++
				}
				return m, nil
			}
		case " ":
			if m.step == StepStackSelect && m.stackIndex >= 0 && m.stackIndex < len(availableStacks) {
				st := availableStacks[m.stackIndex].id
				m.selectedStacks[st] = !m.selectedStacks[st]
				return m, nil
			}
			if m.step == StepSkillRecommend && m.selIndex >= 0 && m.selIndex < len(m.recommendedSkills) {
				name := m.recommendedSkills[m.selIndex].Name
				m.selectedSkills[name] = !m.selectedSkills[name]
				return m, nil
			}
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
			items = append(items, item{id: mod, title: mod, desc: ""})
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 65, 12)
		m.list.Title = "Select Default Model"
		m.step = StepModelSelect
		return m, nil

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.spinner, cmd = m.spinner.Update(msg)
	if m.step == StepProviderSelect || m.step == StepModelSelect {
		m.list, cmd = m.list.Update(msg)
	} else if m.step == StepKeyInput {
		m.textInput, cmd = m.textInput.Update(msg)
	}
	return m, cmd
}

func (m *WizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepWelcome:
		m.step = StepProviderSelect
		return m, nil

	case StepProviderSelect:
		i, ok := m.list.SelectedItem().(item)
		if ok {
			m.provider = i.id
			if m.provider == "copilot" {
				m.step = StepAuthenticating
				return m, m.startAuth()
			} else if m.provider == "ollama" {
				m.apiKey = "http://localhost:11434"
				m.selectedModel = "llama3"
				m.step = StepLanguageSelect
				return m, nil
			} else {
				m.step = StepKeyInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}

	case StepKeyInput:
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			m.apiKey = val
			m.step = StepLanguageSelect
			return m, nil
		}

	case StepModelSelect:
		i, ok := m.list.SelectedItem().(item)
		if ok {
			m.selectedModel = i.id
			m.step = StepLanguageSelect
			return m, nil
		}

	case StepLanguageSelect:
		langs := []string{"en", "es", "pt"}
		m.languagePref = langs[m.langIndex]
		m.step = StepSecurityMode
		m.secIndex = 0
		return m, nil

	case StepSecurityMode:
		m.securityMode = m.secIndex == 1
		m.step = StepStackSelect
		return m, nil

	case StepStackSelect:
		m.loadSkillsForSelectedStacks()
		m.step = StepSkillRecommend
		return m, nil

	case StepSkillRecommend:
		if m.hub != nil {
			for name := range m.selectedSkills {
				_ = m.hub.Install(name)
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
	banner := RenderWizardBanner(m.width)
	doc.WriteString(banner + "\n\n")

	switch m.step {
	case StepWelcome:
		doc.WriteString(bannerTitleStyle.Render("WELCOME TO GAIA — SETUP WIZARD") + "\n\n")
		doc.WriteString("GAIA will guide you through setting up your LLM Provider, Security Policy, Language, and Development Skills.\n\n")
		doc.WriteString("Press " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00F5D4")).Render("ENTER") + " to begin setup.\n")

	case StepProviderSelect:
		return doc.String() + m.list.View()

	case StepKeyInput:
		doc.WriteString(bannerTitleStyle.Render(" API KEY CONFIGURATION ") + "\n\n")
		doc.WriteString(fmt.Sprintf("Enter API Key or Endpoint URL for provider %s:\n\n", bannerSubStyle.Render(m.provider)))
		doc.WriteString(m.textInput.View() + "\n\n")
		doc.WriteString("Press " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00F5D4")).Render("ENTER") + " to save and continue.\n")

	case StepAuthenticating:
		doc.WriteString(bannerTitleStyle.Render(" GITHUB COPILOT AUTHORIZATION ") + "\n\n")
		if m.code == "" {
			doc.WriteString(m.spinner.View() + " Requesting code from GitHub...")
		} else {
			doc.WriteString("1. Visit: " + lipgloss.NewStyle().Foreground(lipgloss.Color("#00F5D4")).Underline(true).Render(m.url) + "\n")
			doc.WriteString("2. Enter code: " + lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#00BBF9")).Foreground(lipgloss.Color("#0F172A")).Render(" "+m.code+" ") + "\n\n")
			doc.WriteString(m.spinner.View() + " Waiting for browser authorization...")
		}

	case StepModelSelect:
		return doc.String() + m.list.View()

	case StepLanguageSelect:
		doc.WriteString(bannerTitleStyle.Render(" LANGUAGE PREFERENCE ") + "\n\n")
		doc.WriteString("Select default interaction language for GAIA:\n\n")
		opts := []string{"English (EN)", "Spanish (ES - Español)", "Portuguese (PT - Português)"}
		for i, opt := range opts {
			cursor := "  "
			if i == m.langIndex {
				cursor = "➤ "
			}
			doc.WriteString(cursor + itemStyle.Render(opt) + "\n")
		}
		doc.WriteString("\nUse " + lipgloss.NewStyle().Bold(true).Render("UP/DOWN") + " arrows, " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to confirm.\n")

	case StepSecurityMode:
		doc.WriteString(bannerTitleStyle.Render(" SECURITY MODE (PolicyGuard) ") + "\n\n")
		doc.WriteString("Restrict agent capabilities with tier-based permissions:\n")
		doc.WriteString("  • Read-only tools (read, glob, grep) — always allowed\n")
		doc.WriteString("  • Write/edit tools — scoped to project directory\n")
		doc.WriteString("  • Destructive commands (rm -rf /, git push --force) — blocked\n\n")
		opts := []string{"No — Basic mode (default)", "Yes — Security mode (Recommended)"}
		for i, opt := range opts {
			cursor := "  "
			if i == m.secIndex {
				cursor = "➤ "
			}
			doc.WriteString(cursor + itemStyle.Render(opt) + "\n")
		}
		doc.WriteString("\nUse " + lipgloss.NewStyle().Bold(true).Render("UP/DOWN") + " arrows, " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to confirm.\n")

	case StepStackSelect:
		doc.WriteString(bannerTitleStyle.Render(" SELECT YOUR DEVELOPMENT STACKS ") + "\n\n")
		doc.WriteString("Select the languages and technologies you work with:\n")
		doc.WriteString("Press " + lipgloss.NewStyle().Bold(true).Render("SPACE") + " to select/deselect, " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " to confirm.\n\n")

		for i, st := range availableStacks {
			cursor := " "
			mark := "[ ]"
			if i == m.stackIndex {
				cursor = "➤"
			}
			if m.selectedStacks[st.id] {
				mark = "[x]"
			}
			doc.WriteString(fmt.Sprintf("  %s %s %s\n", cursor, mark, lipgloss.NewStyle().Bold(true).Render(st.name)))
		}

	case StepSkillRecommend:
		doc.WriteString(bannerTitleStyle.Render(" RECOMMENDED SKILLS ") + "\n\n")
		doc.WriteString("Skills available for your selected stacks:\n")
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
				lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00F5D4")).Render(sk.Name),
				sk.Description))
		}

		if len(m.recommendedSkills) == 0 {
			doc.WriteString("  (no matching skills found — press ENTER to finish)\n")
		}

	case StepFinishing:
		doc.WriteString(bannerTitleStyle.Render(" ALL SET! ") + "\n\n")
		doc.WriteString("GAIA setup is complete. Configuration saved to ~/.config/gaia/config.yaml\n")
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(doc.String())
}

func (m *WizardModel) loadSkillsForSelectedStacks() {
	if m.hub == nil {
		return
	}
	var skillsList []skills.SkillMeta
	seen := make(map[string]bool)

	for st, selected := range m.selectedStacks {
		if !selected {
			continue
		}
		recs := m.hub.RecommendFor(st)
		for _, sk := range recs {
			if !seen[sk.Name] {
				seen[sk.Name] = true
				skillsList = append(skillsList, sk)
			}
		}
	}

	m.recommendedSkills = skillsList
	sort.Slice(m.recommendedSkills, func(i, j int) bool {
		return m.recommendedSkills[i].Name < m.recommendedSkills[j].Name
	})
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
// Returns: provider, apiKey, model, language preference, security mode, selected skill names.
func (m *WizardModel) GetResults() (provider, apiKey, model, language string, security bool, skills []string) {
	names := make([]string, 0, len(m.selectedSkills))
	for name := range m.selectedSkills {
		names = append(names, name)
	}
	sort.Strings(names)

	p := m.provider
	if p == "" {
		p = "copilot"
	}
	k := m.apiKey
	if p == "copilot" {
		k = m.cpToken
	}

	return p, k, m.selectedModel, m.languagePref, m.securityMode, names
}
