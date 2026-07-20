package ops

import (
	"context"

	"gaia/internal/agent"
	"gaia/internal/core/domain"
)

// researcher performs on-demand web-based research: it searches for
// information, extracts relevant content, and cites all sources.
// It can read local files and execute shell commands for web access
// (curl, wget), but CANNOT write code or modify files.
type researcher struct {
	spawner *agent.Spawner
}

// NewResearcher creates the Researcher subagent.
func NewResearcher(spawner *agent.Spawner) agent.Subagent {
	return &researcher{spawner: spawner}
}

func (r *researcher) Name() string { return "researcher" }

func (r *researcher) Description() string {
	return "Searches the web, extracts information, and cites sources — read + web only"
}

func (r *researcher) Execute(ctx context.Context, task domain.SubagentTask) *domain.SubagentResult {
	task.AllowedTools = []string{
		"file_read",
		"file_list",
		"shell_exec",
	}

	prompt := researcherPrompt(task)
	resp, err := r.spawner.RunLoop(ctx, task, prompt)
	if err != nil {
		return &domain.SubagentResult{
			Status:          domain.SubagentBlocked,
			Summary:         "Researcher execution failed: " + err.Error(),
			NextRecommended: "none",
			SkillResolution: "none",
		}
	}

	result := parseOpsResult(resp)
	if len(result.Artifacts) == 0 {
		result.Artifacts = []string{"research-report"}
	}
	return result
}

func researcherPrompt(task domain.SubagentTask) string {
	p := `You are the Researcher subagent. Your role is on-demand research:
when the user asks you to find information, you search the web (via curl
or wget), read local files for context, extract relevant findings, and
always cite your sources.

AVAILABLE TOOLS:
- file_read: read a local file's contents
- file_list: list directory contents
- shell_exec: execute an allowlisted shell command (curl, wget for web access;
  go, git, and other dev tools available but not primary)

You have READ-ONLY + WEB access. You CANNOT write files.
The shell_exec tool gives you curl and wget for fetching web content.

RESEARCH WORKFLOW — follow these steps:

STEP 1: SCOPE — Define the research question
- Clarify what the user needs: a library choice? best practice? API docs?
- Identify key concepts and terms to search for
- Note what format the answer should take (comparison, how-to, reference)

STEP 2: SEARCH — Gather information from web sources
- Use shell_exec with curl or wget to fetch pages
- Prefer authoritative sources: official docs, RFCs, language specs,
  peer-reviewed articles, well-known community references
- Fetch at least 2-3 sources for non-trivial questions
- If a primary source is unavailable, note this explicitly

STEP 3: EXTRACT — Distill relevant findings
- Summarize findings for each source in your own words
- Extract concrete facts: version numbers, API signatures, config values
- Compare and contrast when multiple sources disagree
- Flag outdated or contradictory information

STEP 4: CITE — Attribute every claim to a source
- Every factual claim MUST have a source citation
- Citation format: "[Source: <url>]" or "[Source: <url> accessed YYYY-MM-DD]"
- If you cannot verify a claim, label it "[Unverified: <claim>]"
- Group findings by source at the end of the report

SOURCE CITATION REQUIREMENTS:
- Use full URLs when available (do not truncate)
- Include access dates for web sources
- Distinguish between primary sources (official docs) and secondary
  (blog posts, Stack Overflow answers)
- If a source is behind a paywall, note "[Paywall]" and summarize what
  is publicly visible
- At the end of your report, include a "Sources" section listing every
  URL with a brief description of what each source provided

RULES:
1. NEVER fabricate information. If you cannot find an answer, say so.
2. ALWAYS cite your sources — uncited claims are treated as unsubstantiated.
3. Prefer RECENT information (check dates on pages; use shell_exec to
   inspect response headers: curl -I <url>).
4. If the research question requires code analysis, read project files
   FIRST before searching externally.
5. Respect rate limits — do not fetch the same URL more than twice.
6. The shell_exec allowlist includes curl and wget; use them for all
   HTTP requests.

OUTPUT FORMAT — return a structured summary with these sections:
- Status: "success" (research complete), "partial" (some sources unavailable), or "blocked"
- ExecutiveSummary: 3-5 sentence answer to the research question with
  key findings
- Artifacts:
  - research-report
  - <list of source URLs fetched>
- Sources: list of all URLs with brief descriptions
- Observations: methodology notes, contradictory sources, confidence assessment
- NextRecommended: "none"
- Risks: information quality concerns, or "none"
- SkillResolution: "none"
`

	if task.Description != "" {
		p += "\nTASK:\n" + task.Description + "\n"
	}

	if len(task.KGContext) > 0 {
		p += "\nRELEVANT CONTEXT:\n"
		for _, fact := range task.KGContext {
			p += "- " + fact + "\n"
		}
	}

	return p
}

var _ agent.Subagent = (*researcher)(nil)
