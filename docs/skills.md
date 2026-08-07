# Skills Hub

Skills are procedural memory for GAIA — they teach the agent how to handle specific tasks, languages, and frameworks. GAIA is designed as a **language-neutral autonomous coding agent** and ships with **zero pre-installed skills**. You install only what your stack needs.

---

## Philosophy

```text
GAIA ships with NO pre-installed skills (100% Neutral core).
You install only what your specific stack requires.

This guarantees:
  • Context lean (only relevant skills in index)
  • Language agnostic (no bias towards any specific framework)
  • Prompts focused (no unused instructions clogging memory)
  • Agent fast (minimal footprint per execution turn)
```

---

## Quick Start

```bash
# Search for skills
gaia skills search "go testing"
gaia skills search "react typescript"

# Install skills for your stack
gaia skills install go
gaia skills install typescript-react

# Manage installed skills
gaia skills list                         # See what is installed
gaia skills activate go-testing          # Enable a skill
gaia skills deactivate go-linting        # Disable without uninstalling
gaia skills remove go-testing            # Delete permanently
```

---

## First-Run Wizard

On first run, GAIA setup wizard:
1. Detects your project language and framework (from go.mod, package.json, Cargo.toml, pyproject.toml, etc.)
2. Queries the Skills Hub for popular matching skills
3. Shows recommendations with descriptions
4. Installs your selections to ~/.gaia/skills/
5. Activates them for the workspace

---

## Skill Format

Skills are SKILL.md files with YAML frontmatter:

```yaml
---
name: go-testing
description: "Write Go tests — table-driven, subtests, parallel, fakes"
version: 1.0.0
languages: [go]
tags: [testing, tdd, go]
category: development
author: gaia-community
license: MIT
metadata:
  gaia:
    fallback_for_tools: [terminal]
    requires_tools: [terminal, read, write]
---

# Go Testing

## When to Use
When writing or reviewing Go test code.

## Procedure
1. Use table-driven tests with descriptive names
2. Use t.Run() for subtests
3. Use t.Parallel() for independent tests
4. Use cmp.Diff() for complex comparisons

## Pitfalls
- Do not use require in goroutines (panics)
- Do not ignore t.Cleanup for resource cleanup
- Do not use ioutil (deprecated since Go 1.16)

## Verification
Run go test ./... -count=1 and check all tests pass.
```

---

## Skill Sources

Skills are discovered and loaded from the following locations:

| Source | Path | Priority | Read-only |
|---|---|---|---|
| User-installed | `~/.gaia/skills/` | Primary | No |
| Community taps | `~/.gaia/taps/{name}/` | Extended | No |
| Project-local | `.gaia/skills/` (in project repo) | Workspace-specific | No |

### Community Taps

Add skill repositories from GitHub:

```bash
gaia skills add-tap github.com/user/gaia-skills
gaia skills add-tap https://github.com/community/awesome-skills
```

Taps are git-cloned into `~/.gaia/taps/` and scanned for `SKILL.md` files.

### Creating Your Own Skills

Skills are markdown files with YAML frontmatter. Create custom skills in `~/.gaia/skills/custom/`:

```bash
mkdir -p ~/.gaia/skills/custom/my-skill
cat > ~/.gaia/skills/custom/my-skill/SKILL.md << 'EOF'
---
name: my-skill
description: "My custom skill for specific task"
version: 1.0.0
tags: [custom]
---

# My Skill

...
EOF
```

---

## Progressive Loading

```text
Level 0 (always in context):   [{name, description, tags}, ...]   ~3k tokens
Level 1 (on demand):           Full SKILL.md content               varies
Level 2 (on demand):           Reference files                    varies
```

The orchestrator only keeps Level 0 in context. When a subagent is spawned:
1. The orchestrator passes matching skill names
2. The subagent loads Level 1 content as needed
3. Reference files (Level 2) are loaded on explicit request

---

## 🧠 Auto-Learning & Skill Creation

To keep skills up-to-date and tailored to your development flow, GAIA features automated learning mechanisms:

### Subagent Auto-Learning
After a subagent executes 5 times, the background learning loop automatically aggregates observed execution patterns and generates a corresponding patterns skill. These are stored locally under `~/.gaia/skills/`:
- `explorer-patterns/` — Patterns observed during codebase exploration.
- `implementer-patterns/` — Coding and refactoring patterns.
- `verifier-patterns/` — Testing and verification habits.

You can audit and edit these skills at any time to refine GAIA behavior. Use `gaia skills list` to view them.

### Learner Subagent
You can actively ask the `@learner` subagent to analyze your codebase and propose new skills by invoking:
```bash
@learner analyze
```
The subagent will scan the project, identify repetitive patterns, and generate a new `SKILL.md` template proposal.
