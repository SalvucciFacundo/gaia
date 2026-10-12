# Profiles & Persona Evolution System

GAIA strictly separates **Identity and Cognitive Stance** from **Operational Tools and Gateways**. The persona system controls how the agent communicates, reasons, and evolves, while the modular profile system lets users switch seamlessly between specialized identities.

Unlike static system prompts, GAIA profiles start with a foundational `SOUL.md` that undergoes **Bayesian Soul Evolution** based on real user interactions.

---

## 🎭 The Modular Profiles Engine (`internal/profiles/`)

GAIA comes with three built-in profiles and supports creating custom profiles:

| Profile | Target Domain | Core Focus | Default Suggested Tools |
|---|---|---|---|
| **`dev`** | Software Engineering | Architecture, clean code, refactoring, tests, Git ops, ODD/SDD workflows | `exec_command`, `read_file`, `write_file`, `browser_navigate`, `create_skill` |
| **`assistant`** | Daily Work & Research | General assistance, web research, meeting notes, reminders, multi-channel chat | `web_search`, `web_fetch`, `browser_navigate`, `read_file`, `read_skill` |
| **`art`** | Visual & Design | Visual reasoning, design systems, ComfyUI, aesthetic critiques, prompt engineering | `browser_navigate`, `browser_screenshot`, `read_file` |

### Profile Independence & Tool Decoupling
Any tool in the **Global Tool Catalog** can be enabled or disabled for any profile. Profiles do not own tools; they simply configure which tools from the catalog are active:

```bash
# List all profiles with active status and enabled tool counts
gaia profile list

# Switch the active profile
gaia profile switch dev

# Customize tools for a profile
gaia profile tools list dev
gaia profile tools enable dev browser_navigate
gaia profile tools disable dev exec_command
gaia profile tools reset dev
```

---

## 🧬 Bayesian Soul Evolution (`internal/persona/`)

Every profile maintains its own `SOUL.md`. As the agent interacts with you, the Bayesian soul engine records reinforcement signals and refines its behavioral priors:

```
Session 1:   Initial Seed → Strict adherence to comprehensive unit test scaffolding
                ↓
Session 10:  Learns that integration and end-to-end tests provide higher value for THIS project
                ↓
Session 50:  Adapted Persona: "Prioritize integration tests for HTTP gateways; enforce unit tests for state machines."
```

### What the Learning Loop Refines:
- **Tone & Conciseness**: Adapts response lengths and depth to user preferences.
- **Verification Strictness**: Learns which technical rules matter most in the current workspace.
- **Pushback vs. Compliance**: Discovers when to challenge architectural mistakes versus when to follow explicit instructions.

### Freezing Soul Evolution
If you prefer a deterministic, unvarying persona (traditional static `SOUL.md` behavior):

```yaml
# config.yaml
persona:
  evolution_enabled: false   # Freezes current state; disables dynamic Bayesian updates
```

---

## 🛡️ Real-Time Prompt Injection Defense (`internal/scan/`)

Before any inbound user message or gateway event reaches the Brain, GAIA's security scanner (`internal/scan/`) analyzes the prompt for adversarial patterns:

- **Jailbreak Detection**: Blocks classic evasion strategies (DAN, role reversal, simulated developer mode).
- **Exfiltration Mitigation**: Prevents prompt leak attacks designed to dump `SOUL.md` or internal system instructions.
- **Instruction Hijacking**: Detects indirect prompt injections embedded inside untrusted web pages, scraped HTML, or third-party Git repositories.

---

## ⚙️ Configuration Reference

In `~/.config/gaia/config.yaml`:

```yaml
# Select default active profile
current_profile: "dev"

profiles:
  dev:
    enabled: true
    model: "claude-3-5-sonnet-20241022"  # Profile-specific model override
    tools:
      - exec_command
      - read_file
      - write_file
      - browser_navigate
      - web_search
      - create_skill

  assistant:
    enabled: true
    model: "gpt-4o"
    tools:
      - web_search
      - web_fetch
      - browser_navigate
      - read_file
      - read_skill

  art:
    enabled: false
    model: "gemini-2.5-flash"
    tools:
      - browser_navigate
      - browser_screenshot
      - read_file

persona:
  evolution_enabled: true                # Set false to freeze persona
  evolution_review: "prompt"             # "prompt" (confirm changes) or "auto" (silent evolution)
  language: "auto"                       # "es", "en", or "auto"
```

---

## 💬 In-Session TUI Commands

While chatting in the terminal TUI, use these slash commands to inspect or adjust personas on the fly:

| Command | Action |
|---|---|
| `/persona status` | Show active profile, soul version, and Bayesian metrics |
| `/persona freeze` | Pause Bayesian updates and keep current persona fixed |
| `/persona unfreeze` | Re-enable automatic soul evolution |
| `/persona reset` | Revert current persona back to its initial `SOUL.md` seed |
| `/persona switch <id>` | Switch active profile without restarting the session |

