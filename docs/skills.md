# Skills Hub

Skills are procedural memory for GAIA — they teach the agent how to handle specific tasks, languages, and frameworks. Skills are **not bundled** with GAIA; you install what you need for your stack.

---

## Philosophy

```
GAIA ships with NO pre-installed skills.
You install only what your stack needs.

This keeps:
  • Context lean (only relevant skills in index)
  • Prompts focused (no irrelevant instructions)
  • Agent fast (less to load per turn)
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
gaia skills list                         # See what's installed
gaia skills activate go-testing          # Enable a skill
gaia skills deactivate go-linting        # Disable without uninstalling
gaia skills remove go-testing            # Delete permanently
```

---

## First-Run Wizard

On first run, GAIA's wizard:
1. Detects your project language (from go.mod, package.json, Cargo.toml, etc.)
2. Queries the Skills Hub for popular matching skills
3. Shows recommendations with descriptions
4. Installs your selections
5. Activates them by default

---

## Skill Format

Skills are `SKILL.md` files with YAML frontmatter:

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
2. Use `t.Run()` for subtests
3. Use `t.Parallel()` for independent tests
4. Use `cmp.Diff()` for complex comparisons

## Pitfalls
- Don't use `require` in goroutines (panics)
- Don't ignore `t.Cleanup` for resource cleanup
- Don't use `ioutil` (deprecated since Go 1.16)

## Verification
Run `go test ./... -count=1` and check all tests pass.
```

---

## Skill Sources

Skills come from multiple sources, checked in order:

| Source | Path | Priority | Read-only |
|---|---|---|---|
| Bundled | `skills/` (in GAIA binary) | Fallback | Yes |
| User-installed | `~/.gaia/skills/` | Primary | No |
| Community taps | `~/.gaia/taps/{name}/` | Extended | No |

### Bundled Skills

GAIA ships with a set of **Go-specific reference skills** in the `skills/` directory. These are read-only and version-locked with the binary.

### Community Taps

Add skill repositories from GitHub:

```bash
gaia skills add-tap github.com/user/gaia-skills
gaia skills add-tap https://github.com/community/awesome-skills
```

Taps are git-cloned into `~/.gaia/taps/` and scanned for `SKILL.md` files. Update with:

```bash
gaia skills add-tap my-tap  # Re-adds to update (git pull)
```

### Creating Your Own Skills

Skills are just markdown files. Create one in `~/.gaia/skinks/custom/`:

```bash
mkdir -p ~/.gaia/skills/custom
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

```
Level 0 (always in context):   [{name, description, tags}, ...]   ~3k tokens
Level 1 (on demand):           Full SKILL.md content               varies
Level 2 (on demand):           Reference files                    varies
```

The orchestrator only keeps Level 0 in context. When a subagent is spawned:
1. The orchestrator passes matching skill names
2. The subagent loads Level 1 content as needed
3. Reference files (Level 2) are loaded on explicit request
