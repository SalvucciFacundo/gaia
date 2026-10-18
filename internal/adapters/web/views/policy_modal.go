package views

import (
	"fmt"
	"strings"

	"gaia/internal/core"
)

// RenderPolicyModal generates the HTML fragment for the interactive 4-category permission modal.
func RenderPolicyModal(categories []core.CategorySummary) string {
	var b strings.Builder

	b.WriteString(`
<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="bg-[#1E293B] border border-slate-700/80 rounded-2xl w-full max-w-3xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        <!-- Header -->
        <div class="h-16 px-6 border-b border-slate-800 bg-slate-900/60 flex items-center justify-between">
            <div class="flex items-center gap-3">
                <span class="text-2xl">🛡️</span>
                <div>
                    <h2 class="font-bold text-white text-base tracking-wide flex items-center gap-2">
                        Security & Permissions Control Panel
                    </h2>
                    <p class="text-xs text-slate-400 font-mono">4 Functional Categories • 3-State Traffic Light</p>
                </div>
            </div>
            <button onclick="document.getElementById('modal-container').innerHTML = ''" class="text-slate-400 hover:text-white px-2 py-1 rounded-md text-sm hover:bg-slate-800 transition">
                ✕
            </button>
        </div>

        <!-- Body: Categories -->
        <div class="p-6 overflow-y-auto space-y-4">
`)

	for _, cat := range categories {
		icon := "📁"
		switch cat.ID {
		case core.CategoryExecution:
			icon = "⚡"
		case core.CategoryNetwork:
			icon = "🌐"
		case core.CategoryState:
			icon = "💾"
		}

		// State indicator badges & active styles
		allowActive := "bg-slate-800/80 text-slate-400 hover:text-white border-slate-700"
		askActive := "bg-slate-800/80 text-slate-400 hover:text-white border-slate-700"
		blockActive := "bg-slate-800/80 text-slate-400 hover:text-white border-slate-700"

		switch cat.State {
		case core.StateAllow:
			allowActive = "bg-emerald-500/20 text-emerald-300 border-emerald-500/50 font-semibold shadow-sm shadow-emerald-500/20"
		case core.StateAsk:
			askActive = "bg-amber-500/20 text-amber-300 border-amber-500/50 font-semibold shadow-sm shadow-amber-500/20"
		case core.StateBlock:
			blockActive = "bg-rose-500/20 text-rose-300 border-rose-500/50 font-semibold shadow-sm shadow-rose-500/20"
		}

		b.WriteString(fmt.Sprintf(`
            <div class="p-4 rounded-xl border border-slate-800 bg-slate-900/40 hover:border-slate-700 transition flex flex-col gap-3">
                <div class="flex items-start justify-between gap-4">
                    <div class="flex items-start gap-3">
                        <span class="text-2xl mt-0.5">%s</span>
                        <div>
                            <div class="flex items-center gap-2">
                                <span class="font-bold text-slate-100 text-sm">%s</span>
                                <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-cyan-400 border border-cyan-500/20">
                                    💥 %s
                                </span>
                            </div>
                            <p class="text-xs text-slate-400 mt-0.5 leading-relaxed">%s</p>
                        </div>
                    </div>

                    <!-- 3-State Traffic Light Buttons -->
                    <div class="flex items-center gap-1.5 p-1 rounded-lg bg-slate-950/70 border border-slate-800 font-mono text-xs shrink-0">
                        <button hx-post="/web/policy/update?category=%s&state=allow" hx-target="#modal-container" class="px-2.5 py-1 rounded border transition flex items-center gap-1 %s">
                            <span>🟢</span><span>Allow</span>
                        </button>
                        <button hx-post="/web/policy/update?category=%s&state=ask" hx-target="#modal-container" class="px-2.5 py-1 rounded border transition flex items-center gap-1 %s">
                            <span>🟡</span><span>Ask</span>
                        </button>
                        <button hx-post="/web/policy/update?category=%s&state=block" hx-target="#modal-container" class="px-2.5 py-1 rounded border transition flex items-center gap-1 %s">
                            <span>🔴</span><span>Block</span>
                        </button>
                    </div>
                </div>

                <!-- Progressive Disclosure: Collapsible Tool List -->
                <details class="text-xs font-mono border-t border-slate-800/80 pt-2 text-slate-400">
                    <summary class="cursor-pointer hover:text-slate-200 select-none flex items-center gap-1.5 text-[11px]">
                        <span>🔍 Tools Breakdown (%d tools)</span>
                    </summary>
                    <div class="grid grid-cols-2 sm:grid-cols-3 gap-2 mt-2 pt-1">
`,
			icon,
			cat.Name,
			cat.BlastRadius,
			cat.Description,
			string(cat.ID), allowActive,
			string(cat.ID), askActive,
			string(cat.ID), blockActive,
			len(cat.Tools),
		))

		for _, t := range cat.Tools {
			dot := "🟢"
			stateText := "Allow"
			textColor := "text-emerald-400"
			switch t.State {
			case core.StateAsk:
				dot = "🟡"
				stateText = "Ask"
				textColor = "text-amber-400"
			case core.StateBlock:
				dot = "🔴"
				stateText = "Block"
				textColor = "text-rose-400"
			}

			b.WriteString(fmt.Sprintf(`
                        <div class="p-1.5 rounded bg-slate-950/40 border border-slate-800/60 flex items-center justify-between text-[11px]">
                            <span class="truncate text-slate-300" title="%s">%s</span>
                            <span class="%s flex items-center gap-1 ml-1 text-[10px]">
                                <span>%s</span><span>%s</span>
                            </span>
                        </div>
`, t.Name, t.Name, textColor, dot, stateText))
		}

		b.WriteString(`
                    </div>
                </details>
            </div>
`)
	}

	b.WriteString(`
        </div>

        <!-- Footer -->
        <div class="h-14 px-6 border-t border-slate-800 bg-slate-900/60 flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>Policies are enforced in real-time by PolicyGuard</span>
            <button onclick="document.getElementById('modal-container').innerHTML = ''" class="px-4 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg border border-slate-700 transition">
                Done
            </button>
        </div>
    </div>
</div>
`)

	return b.String()
}
