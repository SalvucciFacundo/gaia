# Skills & Tool Discovery Hub

Skills represent **procedural memory** in GAIA — they teach the agent how to execute domain-specific workflows, testing patterns, and architectural conventions. GAIA follows the open specification defined by [agentskills.io](https://agentskills.io).

Unlike monolithic prompts, GAIA pairs **procedural skills** with a **Global Tool Catalog**, providing dynamic tool discovery and autonomous skill self-learning.

---

## 🚀 Quick CLI Management

Manage skills using the modern `gaia skill` CLI:

```bash
# List all installed and bundled skills (with description and triggers)
gaia skill list

# Inspect a skill in detail
gaia skill show go-testing
gaia skill show go-testing --raw       # Dump raw markdown/frontmatter

# Create or update a skill interactively or with flags
gaia skill create my-workflow \
  --desc "Standardized release checklist" \
  --trigger "When cutting a new release" \
  --body "Procedure and steps..."

# Delete a skill with confirmation
gaia skill delete my-workflow -f

# Security audit all skills for dangerous AST patterns
gaia skills audit
```

---

## 🤖 Autonomous Skill Learning Tools (`internal/tools/`)

GAIA isn't limited to human-authored skills. The Brain and subagents can actively learn, document, and inspect skills during execution:

### 1. `create_skill`
Enables the agent to codify novel workflows, bug fixes, or architecture conventions directly into `SKILL.md`:
- **Validation**: Enforces canonical naming (`^[a-zA-Z0-9_-]+$`) and prevents unintentional file overwrites unless explicitly requested.
- **Structured Sections**: Requires `## When to Use`, `## Critical Rules`, `## Workflow`, and `## Examples`.
- **Immediate Reload**: Newly created skills are automatically registered in the active skill hub without restarting GAIA.

### 2. `read_skill`
Enables the agent to retrieve and study a skill definition on demand:
- **Lazy Hydration**: Reads the exact markdown procedure and examples when intent matches.
- **Usage Tracking**: Increments invocation counters and updates timestamp metrics for the skill.

---

## 🔍 Dynamic Tool Discovery & Introspection

When working on unfamiliar tasks, the agent doesn't need all 30+ tool schemas pre-loaded in its prompt. It discovers capabilities dynamically:

### 1. `tool_search`
Searches the `ToolCatalog` by keywords or category:
- **Categories**: `web`, `browser`, `memory`, `shell`, `filesystem`, `vcs`, `multimodal`, `gateway`, `skills`, `meta`.
- **Example**: Searching for `"browser"` or `"scrape"` returns `browser_navigate`, `browser_action`, `browser_screenshot`, and `web_fetch`.

### 2. `load_tool`
Loads the detailed JSON Schema, argument types, and behavioral constraints for a specific tool returned by `tool_search`:
- **Token Efficiency**: Keeps the base system prompt lean by only hydrating full parameter schemas when the agent decides to use a tool.

---

## 📝 Canonical Skill Format (`SKILL.md`)

All skills follow YAML frontmatter standards:

```markdown
---
name: go-testing
description: "Write Go tests — table-driven, subtests, parallel, fakes"
version: 1.0.0
languages: [go]
tags: [testing, tdd, go]
metadata:
  requires_tools: [exec_command, read_file, write_file]
---

# Go Testing

## When to Use
Use when writing new unit tests, refactoring test suites, or reviewing test coverage.

## Critical Rules
- Table-driven tests must use `t.Run()` for subtests.
- Never use `require` assertions inside goroutines.
- Always clean up temporary resources using `t.Cleanup()`.

## Workflow
1. Scaffold test cases in a slice of struct definitions.
2. Iterate test cases and call `t.Parallel()` when tests are thread-safe.
3. Compare expected and actual outputs using `cmp.Diff()` or standard comparisons.

## Examples
```go
func TestCalculate(t *testing.T) {
    tests := []struct{
        name string
        in   int
        want int
    }{
        {name: "zero", in: 0, want: 0},
        {name: "positive", in: 5, want: 10},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Calculate(tt.in)
            if got != tt.want {
                t.Errorf("Calculate(%d) = %d, want %d", tt.in, got, tt.want)
            }
        })
    }
}
```
```

---

## 🌐 Community Taps & Discovery

Extend GAIA's skill library with community repositories from GitHub:

```bash
# Add a tap from GitHub
gaia skills add-tap github.com/community/awesome-gaia-skills

# List active taps
gaia skills list-taps

# Remove a tap
gaia skills remove-tap github.com/community/awesome-gaia-skills
```

Taps are cloned locally and scanned with the **AST Security Scanner** to ensure they contain no malicious shell injections, credential sniffers, or obfuscated payloads before being made available to the agent.

