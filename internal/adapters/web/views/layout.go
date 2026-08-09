package views

import (
	"context"
	"fmt"
	"io"
	"strings"

	"gaia/internal/core/domain"
	"github.com/a-h/templ"
)

// RenderLayout renders the complete HTML5 TailwindCSS + HTMX SPA Dashboard.
func RenderLayout(data WebDashboardData) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="h-full bg-[#0F172A] text-slate-100 font-sans antialiased">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GAIA — Autonomous AI Agent Web Dashboard</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://unpkg.com/htmx.org/dist/ext/sse.js"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
      tailwind.config = {
        theme: {
          extend: {
            colors: {
              gaiaCyan: '#00F5D4',
              gaiaTeal: '#00BBF9',
              gaiaDark: '#0F172A',
              gaiaCard: '#1E293B'
            }
          }
        }
      }
    </script>
</head>
<body class="h-full flex flex-col overflow-hidden">
    <!-- Navbar -->
    <header class="h-16 border-b border-slate-800 bg-[#0F172A]/90 backdrop-blur flex items-center justify-between px-6 z-10">
        <div class="flex items-center space-x-3">
            <div class="w-9 h-9 rounded-lg bg-gradient-to-tr from-[#00F5D4] to-[#00BBF9] flex items-center justify-center font-bold text-slate-900 shadow-lg shadow-[#00F5D4]/20">
                G
            </div>
            <div>
                <h1 class="font-bold text-lg text-white tracking-wide flex items-center gap-2">
                    GAIA <span class="text-xs px-2 py-0.5 rounded-full bg-[#00F5D4]/10 text-[#00F5D4] border border-[#00F5D4]/30 font-mono">Web Dashboard</span>
                </h1>
            </div>
        </div>
        <div class="flex items-center space-x-4">
            <div class="text-xs text-slate-400 bg-slate-800/80 px-3 py-1.5 rounded-md border border-slate-700 font-mono flex items-center gap-2">
                <span class="w-2 h-2 rounded-full bg-[#00F5D4] animate-pulse"></span>
                <span>Provider: <strong class="text-slate-200">%s</strong></span>
                <span class="text-slate-600">|</span>
                <span>Model: <strong class="text-[#00BBF9]">%s</strong></span>
            </div>
        </div>
    </header>

    <!-- Main Container -->
    <div class="flex-1 flex overflow-hidden">
        <!-- Sidebar Projects -->
        <aside class="w-72 border-r border-slate-800 bg-[#0F172A] flex flex-col p-4">
            <div class="flex items-center justify-between mb-4">
                <h2 class="text-xs uppercase tracking-wider text-slate-400 font-semibold font-mono">Active Workspaces</h2>
                <button class="text-xs px-2 py-1 bg-slate-800 hover:bg-slate-700 text-[#00F5D4] border border-[#00F5D4]/30 rounded transition" hx-get="/web/projects/new" hx-target="#modal-container">+ New</button>
            </div>
            <div id="project-list" class="flex-1 overflow-y-auto space-y-2 pr-1">
                %s
            </div>
        </aside>

        <!-- Main Workspace (Chat & Subagent Visualizer) -->
        <main class="flex-1 flex flex-col bg-[#0F172A]/50">
            <div class="flex-1 flex overflow-hidden p-4 gap-4">
                <!-- Chat Panel (Island 1) -->
                <section class="flex-1 flex flex-col bg-[#1E293B]/70 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
                    <div class="h-12 border-b border-slate-800/80 px-4 flex items-center justify-between bg-slate-900/50">
                        <span class="text-sm font-semibold text-slate-200 flex items-center gap-2">
                            <span>💬 Live Interactive Chat</span>
                        </span>
                        <span class="text-xs text-slate-400 font-mono" id="active-project-path">%s</span>
                    </div>
                    
                    <!-- Message Stream Viewport -->
                    <div id="message-container" class="flex-1 overflow-y-auto p-4 space-y-4 font-sans text-sm">
                        %s
                    </div>

                    <!-- Input Form -->
                    <form hx-post="/web/message" hx-target="#message-container" hx-swap="beforeend" class="p-3 border-t border-slate-800/80 bg-slate-900/40 flex items-center gap-2">
                        <input type="text" name="content" placeholder="Ask GAIA or instruct subagents (@explorer, @implementer)..." class="flex-1 bg-slate-950/80 border border-slate-800 rounded-lg px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-[#00F5D4] transition" autocomplete="off" required />
                        <button type="submit" class="px-5 py-2.5 bg-gradient-to-r from-[#00F5D4] to-[#00BBF9] hover:opacity-90 text-slate-950 font-semibold text-sm rounded-lg shadow-lg shadow-[#00F5D4]/20 transition flex items-center gap-1">
                            <span>Send</span>
                        </button>
                    </form>
                </section>

                <!-- Right Side: Pipeline & Tasks (Island 2 & 3) -->
                <section class="w-96 flex flex-col gap-4">
                    <!-- Subagent Execution Pipeline Visualizer -->
                    <div class="bg-[#1E293B]/70 border border-slate-800 rounded-xl p-4 shadow-xl">
                        <h3 class="text-xs uppercase tracking-wider text-[#00F5D4] font-semibold font-mono mb-3 flex items-center gap-2">
                            <span>⚙️ SDD Subagent Pipeline</span>
                        </h3>
                        <div class="space-y-2 font-mono text-xs">
                            %s
                        </div>
                    </div>

                    <!-- Tasks Checklist -->
                    <div class="flex-1 bg-[#1E293B]/70 border border-slate-800 rounded-xl p-4 shadow-xl flex flex-col">
                        <h3 class="text-xs uppercase tracking-wider text-[#00BBF9] font-semibold font-mono mb-3">
                            📋 Active SDD Tasks
                        </h3>
                        <div class="flex-1 overflow-y-auto space-y-2 text-xs">
                            %s
                        </div>
                    </div>
                </section>
            </div>
        </main>
    </div>

    <!-- Modal Container -->
    <div id="modal-container"></div>
</body>
</html>`,
			templ.EscapeString(data.ProviderName),
			templ.EscapeString(data.ModelName),
			renderProjectListHTML(data.Projects),
			templ.EscapeString(data.ActiveProject.Path),
			renderMessagesHTML(data.Messages),
			renderPipelineHTML(data.Subagents),
			renderTasksHTML(data.Tasks),
		)
		_, err := io.WriteString(w, html)
		return err
	})
}

func renderProjectListHTML(projects []ProjectViewModel) string {
	var b strings.Builder
	for _, p := range projects {
		activeClass := "border-slate-800 hover:border-slate-700 bg-slate-900/40 text-slate-300"
		if p.Active {
			activeClass = "border-[#00F5D4]/50 bg-gradient-to-r from-[#00F5D4]/10 to-transparent text-[#00F5D4] font-semibold"
		}
		b.WriteString(fmt.Sprintf(`
<div class="p-3 rounded-lg border text-xs cursor-pointer transition flex items-center justify-between %s" hx-post="/web/projects/select?id=%s" hx-target="body">
    <div>
        <div class="font-medium text-sm">%s</div>
        <div class="text-[10px] text-slate-500 font-mono truncate max-w-[170px]">%s</div>
    </div>
    <span class="text-[10px] px-2 py-0.5 rounded bg-slate-800 text-slate-400 font-mono">%d tasks</span>
</div>`, activeClass, templ.EscapeString(p.ID), templ.EscapeString(p.Name), templ.EscapeString(p.Path), p.TaskCount))
	}
	return b.String()
}

func renderMessagesHTML(messages []domain.Message) string {
	var b strings.Builder
	for _, m := range messages {
		if m.Role == domain.RoleUser {
			b.WriteString(fmt.Sprintf(`
<div class="flex justify-end">
    <div class="max-w-[80%%] bg-gradient-to-r from-[#00F5D4]/20 to-[#00BBF9]/20 border border-[#00F5D4]/30 rounded-xl rounded-tr-none p-3 text-slate-100 shadow-md">
        <div class="text-[10px] font-mono text-[#00F5D4] mb-1 font-semibold">User</div>
        <div class="whitespace-pre-wrap">%s</div>
    </div>
</div>`, templ.EscapeString(m.Content)))
		} else {
			b.WriteString(fmt.Sprintf(`
<div class="flex justify-start">
    <div class="max-w-[85%%] bg-slate-900/90 border border-slate-800 rounded-xl rounded-tl-none p-3 text-slate-200 shadow-md">
        <div class="text-[10px] font-mono text-[#00BBF9] mb-1 font-semibold flex items-center gap-1">
            <span>GAIA Assistant</span>
        </div>
        <div class="whitespace-pre-wrap text-sm leading-relaxed">%s</div>
    </div>
</div>`, templ.EscapeString(m.Content)))
		}
	}
	return b.String()
}

func renderPipelineHTML(subagents []SubagentStateViewModel) string {
	var b strings.Builder
	for _, s := range subagents {
		statusBg := "bg-slate-800/50 border-slate-800 text-slate-500"
		if s.Active {
			statusBg = "bg-[#00F5D4]/10 border-[#00F5D4]/40 text-[#00F5D4] font-semibold animate-pulse"
		} else if s.Completed {
			statusBg = "bg-emerald-500/10 border-emerald-500/30 text-emerald-400"
		}
		b.WriteString(fmt.Sprintf(`
<div class="p-2 rounded border flex items-center justify-between %s">
    <span>@%s</span>
    <span class="text-[10px] font-mono opacity-80">%s</span>
</div>`, statusBg, templ.EscapeString(s.Name), templ.EscapeString(s.Role)))
	}
	return b.String()
}

func renderTasksHTML(tasks []string) string {
	var b strings.Builder
	if len(tasks) == 0 {
		return `<div class="text-slate-500 italic p-2">No active tasks for this workspace</div>`
	}
	for i, t := range tasks {
		b.WriteString(fmt.Sprintf(`
<div class="p-2 rounded bg-slate-900/60 border border-slate-800 text-slate-300 flex items-start gap-2">
    <span class="text-[#00F5D4] font-bold font-mono">%d.</span>
    <span>%s</span>
</div>`, i+1, templ.EscapeString(t)))
	}
	return b.String()
}
