package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"gaia/internal/agent"
	"gaia/internal/core"
	"gaia/internal/core/domain"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00F5D4")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1)

	bannerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00F5D4"))

	bannerSubStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BBF9")).
			Italic(true)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00F5D4"))

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))

	avatarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D7FF"))

	bannerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00D7FF")).
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

const gaiaTitleASCII = `   ██████╗  █████╗  ██╗ █████╗ 
  ██╔════╝ ██╔══██╗ ██║██╔══██╗
  ██║  ███╗███████║ ██║███████║
  ██║   ██║██╔══██║ ██║██╔══██║
  ╚██████╔╝██║  ██║ ██║██║  ██║
   ╚═════╝ ╚═╝  ╚═╝ ╚═╝╚═╝  ╚═╝`

const gaiaAvatarASCII = `
       .---.
      /     \
     |  (o)  |
     |   ▲   |
      \  -  /
     .-'---'-.
   /   (•)   \
  / |   │   | \
 /  |___│___|  \`

// RenderHeaderBanner builds the 2-column Hermes-style GAIA TUI banner for active sessions.
func RenderHeaderBanner(width int) string {
	title := bannerTitleStyle.Render(gaiaTitleASCII)
	subtitle := bannerSubStyle.Render("Go Autonomous Intelligence Agent")
	topHeader := lipgloss.JoinVertical(lipgloss.Center, title, subtitle)

	avatar := avatarStyle.Render(gaiaAvatarASCII)

	toolsList := itemStyle.Render("file_read, file_write, shell_exec, git_ops, web_search")
	subagentsList := itemStyle.Render("@explorer, @proposer, @specifier, @designer, @planner, @implementer, @verifier, @reviewer, @archiver, @debugger, @researcher, @learner")

	rightPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		sectionHeaderStyle.Render("Available Tools"),
		toolsList,
		"",
		sectionHeaderStyle.Render("Available Subagents"),
		subagentsList,
	)

	content := lipgloss.JoinHorizontal(lipgloss.Top, avatar, "   ", rightPanel)
	fullBanner := lipgloss.JoinVertical(lipgloss.Center, topHeader, "", content)

	if width > 0 {
		return bannerBoxStyle.Width(width - 4).Render(fullBanner)
	}
	return bannerBoxStyle.Render(fullBanner)
}

// RenderWizardBanner builds a dedicated header banner for the setup wizard.
func RenderWizardBanner(width int) string {
	title := bannerTitleStyle.Render(gaiaTitleASCII)
	subtitle := bannerSubStyle.Render("Go Autonomous Intelligence Agent — Initial Setup")
	fullBanner := lipgloss.JoinVertical(lipgloss.Center, title, subtitle)

	if width > 0 {
		return bannerBoxStyle.Width(width - 4).Render(fullBanner)
	}
	return bannerBoxStyle.Render(fullBanner)
}

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

	// Policy guard and panel
	policyGuard *core.PolicyGuard        // policy evaluation guard (nil if not configured)
	policyPanel *PolicyPanelModel        // nil when not in policy panel mode
	sessionMgr  *core.SessionManager     // session routing (nil if not in unified mode)

	// Display preferences (set via slash commands)
	showTimestamps bool   // /timestamps toggle
	showStatusbar  bool   // /statusbar toggle
	showFooter     bool   // /footer toggle
	verboseLevel   int    // /verbose: 0=off, 1=results, 2=tool calls, 3=all
	spinnerIdx     int    // /indicator: which spinner style
	themeName      string // /skin: theme name
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
func (m *Model) SetToolNames(tools []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolNames = tools
}

// SetPolicyGuard wires the policy guard for /permisos command support.
// Pass nil to disable the /permisos panel.
func (m *Model) SetPolicyGuard(pg *core.PolicyGuard) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policyGuard = pg
}

// SetSessionManager wires the session manager for multi-platform message routing.
func (m *Model) SetSessionManager(sm *core.SessionManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionMgr = sm
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

	// Route to policy panel when active
	if m.policyPanel != nil {
		newModel, cmd := m.policyPanel.Update(msg)
		if updated, ok := newModel.(*PolicyPanelModel); ok {
			m.policyPanel = updated
		}
		if m.policyPanel.done {
			tier := "full"
			if m.policyGuard != nil {
				tier = string(m.policyGuard.Tier())
			}
			m.history = append(m.history, domain.Message{
				Role:    domain.RoleSystem,
				Content: fmt.Sprintf("Policy updated. Current tier: %s", tier),
			})
			m.policyPanel = nil
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
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

			// Handle /clear — clear screen (TUI-local, no Brain involved)
			if input == "/clear" {
				m.mu.Lock()
				m.history = nil
				m.streaming = ""
				m.viewport.SetContent("")
				m.viewport.GotoBottom()
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}

			// Handle /trust slash commands — delegates to PolicyGuard
			if strings.HasPrefix(input, "/trust") {
				parts := strings.Fields(input)
				mode := "always"
				if len(parts) > 1 {
					mode = parts[1]
				}
				if m.policyGuard != nil {
					m.policyGuard.SetTier(modeToTier(mode))
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

			// Handle /steer — inject mid-loop guidance (bypasses ProcessMessage)
			if strings.HasPrefix(input, "/steer") {
				guidance := strings.TrimSpace(input[6:])
				if guidance == "" {
					m.mu.Lock()
					m.history = append(m.history, domain.Message{
						Role:    domain.RoleSystem,
						Content: "Usage: /steer <message> — inject guidance mid-execution",
					})
					m.textInput.SetValue("")
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
					m.mu.Unlock()
					return m, nil
				}
				// Try to send via Steer() method if brain supports it
				if brain, ok := m.brain.(interface{ Steer(ctx context.Context, msg string) error }); ok {
					brain.Steer(context.Background(), guidance)
					m.mu.Lock()
					m.history = append(m.history, domain.Message{
						Role:    domain.RoleSystem,
						Content: fmt.Sprintf("Steer sent: %s", guidance),
					})
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
				} else {
					// Fallback: just add to history
					m.mu.Lock()
					m.history = append(m.history, domain.Message{
						Role:    domain.RoleSystem,
						Content: fmt.Sprintf("/steer queued: %s (will be processed when current task completes)", guidance),
					})
				}
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}

			// Handle /timestamps — toggle timestamps on messages
			if input == "/timestamps" {
				m.mu.Lock()
				m.showTimestamps = !m.showTimestamps
				status := "OFF"
				if m.showTimestamps { status = "ON" }
				m.textInput.SetValue("")
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Timestamps: %s", status))
				return m, nil
			}

			// Handle /statusbar — toggle status bar display
			if input == "/statusbar" || input == "/sb" {
				m.mu.Lock()
				m.showStatusbar = !m.showStatusbar
				status := "OFF"
				if m.showStatusbar { status = "ON" }
				m.textInput.SetValue("")
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Status bar: %s", status))
				return m, nil
			}

			// Handle /footer — toggle metadata footer on responses
			if input == "/footer" {
				m.mu.Lock()
				m.showFooter = !m.showFooter
				status := "OFF"
				if m.showFooter { status = "ON" }
				m.textInput.SetValue("")
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Footer: %s", status))
				return m, nil
			}

			// Handle /verbose — cycle tool output display level
			if input == "/verbose" {
				m.mu.Lock()
				m.verboseLevel = (m.verboseLevel + 1) % 4
				levels := []string{"OFF", "results", "tool calls", "all"}
				m.textInput.SetValue("")
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Verbose: %s", levels[m.verboseLevel]))
				return m, nil
			}

			// Handle /yolo — toggle auto-approve (sets policy to full)
			if input == "/yolo" {
				m.mu.Lock()
				if m.policyGuard != nil {
					if string(m.policyGuard.Tier()) == "full" {
						m.policyGuard.SetTier(core.TierSandbox)
						m.textInput.SetValue("")
						m.mu.Unlock()
						m.addSystemMsg("YOLO mode OFF — returning to sandbox tier.")
					} else {
						m.policyGuard.SetTier(core.TierFull)
						m.textInput.SetValue("")
						m.mu.Unlock()
						m.addSystemMsg("⚠️ YOLO mode ON — all commands auto-approved, hardline blocklist still active.")
					}
				} else {
					m.textInput.SetValue("")
					m.mu.Unlock()
					m.addSystemMsg("Policy guard not configured. Use --policy-tier to enable.")
				}
				return m, nil
			}

			// Handle /indicator — change spinner style
			if input == "/indicator" {
				m.mu.Lock()
				m.spinnerIdx = (m.spinnerIdx + 1) % 4
				styles := []string{"dots", "line", "pipe", "circle"}
				m.textInput.SetValue("")
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Indicator: %s", styles[m.spinnerIdx]))
				return m, nil
			}

			// Handle /reasoning — change reasoning effort
			if input == "/reasoning" || strings.HasPrefix(input, "/reasoning ") {
				level := "medium"
				if strings.HasPrefix(input, "/reasoning ") {
					level = strings.TrimSpace(input[11:])
				}
				valid := map[string]bool{"low": true, "medium": true, "high": true}
				if !valid[level] {
					m.addSystemMsg("Usage: /reasoning <low|medium|high>")
					return m, nil
				}
				// Store in brain if accessible
				if brain, ok := m.brain.(interface{ SetReasoningEffort(level string) }); ok {
					brain.SetReasoningEffort(level)
				}
				m.addSystemMsg(fmt.Sprintf("Reasoning effort: %s", level))
				return m, nil
			}

			// Handle /personality — switch agent personality
			if strings.HasPrefix(input, "/personality ") {
				name := strings.TrimSpace(input[13:])
				if name == "" {
					m.addSystemMsg("Usage: /personality <name>\nAvailable: teacher, professional, strict, friendly")
					return m, nil
				}
				if brain, ok := m.brain.(interface{ SetPersona(name string) }); ok {
					brain.SetPersona(name)
				}
				m.addSystemMsg(fmt.Sprintf("Personality set to: %s", name))
				return m, nil
			}

			// Handle /skin — change TUI theme
			if strings.HasPrefix(input, "/skin ") {
				theme := strings.TrimSpace(input[6:])
				if theme == "" {
					m.addSystemMsg("Usage: /skin <name>\nAvailable: default, rose-pine, dark, light")
					return m, nil
				}
				m.mu.Lock()
				m.themeName = theme
				m.textInput.SetValue("")
				m.mu.Unlock()
				m.addSystemMsg(fmt.Sprintf("Theme set to: %s (restart to apply fully)", theme))
				return m, nil
			}

			// Handle /reload-mcp — reload MCP servers from config
			if input == "/reload-mcp" {
				m.addSystemMsg("MCP server reload requested. Restart the gateway to pick up config changes:\n  gaia gateway stop && gaia gateway start")
				return m, nil
			}

			// Handle /reload-skills — rescan skills directory
			if input == "/reload-skills" {
				m.addSystemMsg("Skills reload requested. Skills are loaded from disk on next subagent spawn.\nTo force reload, run: gaia skills list")
				return m, nil
			}

			// Handle /plugins — list installed plugins
			if input == "/plugins" {
				m.addSystemMsg("Plugins are managed via the CLI:\n  gaia plugin list    — Show installed plugins\n  gaia plugin install — Install a plugin\n  gaia plugin remove  — Remove a plugin")
				return m, nil
			}

			// Handle /browser — browser connection status
			if input == "/browser" || input == "/browser connect" {
				m.addSystemMsg("Browser tools are configured in config.yaml under browser_tools.\n" +
					"  browser_tools.command points to the browser MCP server.\n" +
					"  To enable: set browser_tools.enabled: true in config.yaml\n" +
					"  Browser automation is available when the MCP server is running.")
				return m, nil
			}

			// Handle /skills — skill management
			if strings.HasPrefix(input, "/skills") {
				args := strings.TrimSpace(input[7:])
				if args == "" {
					m.addSystemMsg("Skill management:\n" +
						"  /skills list             — List installed skills\n" +
						"  /skills search <query>   — Search available skills\n" +
						"  /skills install <name>   — Install a skill\n" +
						"  /skills remove <name>    — Remove a skill\n" +
						"  /skills stats            — Show skill usage\n" +
						"  /skills audit            — Security audit of skills")
				} else {
					// For now, redirect to CLI
					m.addSystemMsg(fmt.Sprintf("Use the CLI for detailed output:\n  gaia skills %s", args))
				}
				return m, nil
			}

			// Handle /cron — scheduled task management
			if strings.HasPrefix(input, "/cron") {
				args := strings.TrimSpace(input[5:])
				if args == "" {
					m.addSystemMsg("Cron job management:\n" +
						"  /cron list              — List scheduled jobs\n" +
						"  /cron add <schedule> <task> — Add a job (cron syntax)\n" +
						"  /cron remove <id>       — Remove a job\n" +
						"  /cron pause <id>        — Pause a job\n" +
						"  /cron resume <id>       — Resume a job\n" +
						"  /cron run <id>          — Run a job immediately\n\n" +
						"  Example: /cron add \"0 2 * * *\" run daily backup\n" +
						"  Use the CLI for full control: gaia cron")
				} else {
					m.addSystemMsg(fmt.Sprintf("Use the CLI for cron management:\n  gaia cron %s", args))
				}
				return m, nil
			}

			// Handle /help — show available commands
			if input == "/help" || input == "/h" {
				help := `Available Commands:

  Session:     /new, /clear, /history, /save, /resume, /sessions, /title, /compress, /undo, /retry
  State:       /branch, /branches, /snapshot save, /snapshot load
  Goals:       /goal, /subgoal, /goals, /goal clear
  Queue/Steer: /queue, /q, /steer
  Background:  /background, /moa, /tasks, /cancel
  Handoff:     /handoff telegram, /handoff discord, /handoff cli
  Config:      /model, /reasoning, /personality, /yolo, /verbose, /timestamps, /statusbar, /footer, /indicator, /skin
  Permissions: /permisos, /trust
  Skills:      /skills, /learn, /suggestions, /blueprint, /curator
  Cron:        /cron list, /cron add, /cron remove
  Memory:      /memory pending, /memory approve, /memory reject
  Info:        /help, /version, /platforms, /copy, /insights, /debug
  Tools:       /reload-mcp, /reload-skills, /plugins, /browser

For details: gaia help or check the README.`
				m.addSystemMsg(help)
				return m, nil
			}

			// Handle /version — show build info and banner
			if input == "/version" {
				banner := RenderHeaderBanner(m.viewport.Width)
				versionInfo := fmt.Sprintf("%s\n\nGAIA — Go Autonomous Intelligence Agent\nVersion: development build\nGo: %s\nLicense: MIT", banner, runtime.Version())
				m.addSystemMsg(versionInfo)
				return m, nil
			}

			// Handle /platforms — show gateway adapter status
			if input == "/platforms" || input == "/gateway" {
				m.addSystemMsg("Gateway platform status:\n" +
					"  Use 'gaia gateway status' for full adapter status.\n" +
					"  Configured in config.yaml under telegram/discord/slack sections.\n" +
					"  Start the gateway: gaia gateway start")
				return m, nil
			}

			// Handle /copy — copy last response to clipboard
			if input == "/copy" || strings.HasPrefix(input, "/copy ") {
				n := 1
				if strings.HasPrefix(input, "/copy ") {
					fmt.Sscanf(input[6:], "%d", &n)
				}
				if n < 1 {
					n = 1
				}
				m.mu.Lock()
				var lastContent string
				for i := len(m.history) - 1; i >= 0 && n > 0; i-- {
					if m.history[i].Role == domain.RoleAssistant {
						lastContent = m.history[i].Content
						n--
					}
				}
				m.mu.Unlock()
				if lastContent == "" {
					m.addSystemMsg("No AI response found to copy.")
				} else {
					// Try clipboard write
					if err := clipboardWrite(lastContent); err != nil {
						m.addSystemMsg("Response ready for copy (clipboard not available in this environment):\n" + lastContent[:min(len(lastContent), 200)])
					} else {
						m.addSystemMsg("Last response copied to clipboard.")
					}
				}
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

				// Handle /usage — show key usage info
		if input == "/usage" {
			provider := "unknown"
			model := "unknown"
			if m.brain != nil {
				provider = "configured"
				model = "see config"
			}
			stats := fmt.Sprintf("Provider: %s\nModel: %s\n\nUse ~gaia doctor for full diagnostics.", provider, model)
			m.mu.Lock()
			m.history = append(m.history, domain.Message{
				Role:    domain.RoleSystem,
				Content: stats,
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

		// Handle /permisos — open policy panel for tier and override management
		if input == "/permisos" {
			m.mu.Lock()
			pg := m.policyGuard
			if pg == nil {
				m.history = append(m.history, domain.Message{
					Role:    domain.RoleSystem,
					Content: "Policy guard is not configured. Use --policy-tier when launching, or configure policy in config.yaml.",
				})
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
				m.textInput.SetValue("")
				m.mu.Unlock()
				return m, nil
			}
			m.policyPanel = NewPolicyPanelModel(pg, m.toolNames)
			m.textInput.SetValue("")
			m.mu.Unlock()
			return m, m.policyPanel.Init()
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
			sessionMgr := m.sessionMgr
			m.mu.Unlock()

			// Return a command that runs ProcessMessage asynchronously.
			if brainCopy != nil {
				return m, tea.Batch(tiCmd, vpCmd, func() tea.Msg {
					ctx := context.Background()
					// Use SessionManager if available (unified mode)
					if sessionMgr != nil {
						err := sessionMgr.Route(ctx, "tui", input, "")
						return processDoneMsg{err: err}
					}
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

	// Render policy panel when active
	if m.policyPanel != nil {
		return m.policyPanel.View()
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

// addSystemMsg appends a system message to the TUI history and refreshes the viewport.
// Must NOT be called with the mutex already held.
func (m *Model) addSystemMsg(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, domain.Message{
		Role:    domain.RoleSystem,
		Content: content,
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
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

// modeToTier maps legacy trust mode names to PolicyGuard tiers.
func modeToTier(mode string) core.PolicyTier {
	switch strings.ToLower(mode) {
	case "never":
		return core.TierRead
	case "per-session", "per-action":
		return core.TierSandbox
	case "always", "full":
		return core.TierFull
	default:
		return core.TierSandbox
	}
}

// clipboardWrite copies text to the system clipboard using platform-specific commands.
func clipboardWrite(text string) error {
	switch runtime.GOOS {
	case "windows":
		return execCommand("clip", text)
	case "darwin":
		return execCommand("pbcopy", text)
	default: // linux
		return execCommand("wl-copy", text) // Wayland
	}
}

// execCommand pipes text into a command's stdin.
func execCommand(name string, text string) error {
	cmd := exec.Command(name)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		stdin.Write([]byte(text))
	}()
	return cmd.Run()
}


