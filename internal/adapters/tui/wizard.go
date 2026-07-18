package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"gaia/internal/adapters/llm"
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
	StepFinishing
)

type item struct {
	id    string
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.id }
func (i item) FilterValue() string { return i.title }

type WizardModel struct {
	step      WizardStep
	auth      *llm.GitHubAuth
	ghToken   string
	cpToken   string
	models    []string
	selectedModel string
	spinner   spinner.Model
	list      list.Model
	code      string
	url       string
	codeResp  *device.CodeResponse
	err       error
	width     int
	height    int
}

func NewWizard() *WizardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &WizardModel{
		step:    StepWelcome,
		auth:    llm.NewGitHubAuth(),
		spinner: s,
	}
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
			if m.step == StepWelcome {
				m.step = StepAuthenticating
				return m, m.startAuth()
			}
			if m.step == StepModelSelect {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.selectedModel = i.id
					m.step = StepFinishing
					return m, tea.Quit // We return to main to save
				}
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
			items = append(items, item{id: mod, title: mod})
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 20, 10)
		m.list.Title = "Seleccioná el modelo"
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

func (m *WizardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.", m.err)
	}

	doc := strings.Builder{}

	switch m.step {
	case StepWelcome:
		doc.WriteString(titleStyle.Render(" BIENVENIDO A GAIA ") + "\n\n")
		doc.WriteString("GAIA necesita conectarse a GitHub Copilot para funcionar.\n")
		doc.WriteString("Presioná " + lipgloss.NewStyle().Bold(true).Render("ENTER") + " para iniciar la autorización.\n")

	case StepAuthenticating:
		doc.WriteString(titleStyle.Render(" AUTORIZACIÓN ") + "\n\n")
		if m.code == "" {
			doc.WriteString(m.spinner.View() + " Solicitando código a GitHub...")
		} else {
			doc.WriteString("1. Visitá: " + lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Underline(true).Render(m.url) + "\n")
			doc.WriteString("2. Ingresá este código: " + lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("57")).Render(" "+m.code+" ") + "\n\n")
			doc.WriteString(m.spinner.View() + " Esperando autorización en el navegador...")
		}

	case StepModelSelect:
		return m.list.View()

	case StepFinishing:
		doc.WriteString("¡Todo listo! Configuración completada.\n")
	}

	return lipgloss.NewStyle().Margin(2, 4).Render(doc.String())
}

// Commands

func (m *WizardModel) startAuth() tea.Cmd {
	return func() tea.Msg {
		// This is a simplification of the actual call which doesn't give url separately easily
		// but the library returns a CodeResponse
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

// GetResults returns the configuration obtained
func (m *WizardModel) GetResults() (string, string) {
	return m.cpToken, m.selectedModel
}
